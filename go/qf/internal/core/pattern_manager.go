// Package core provides the core business logic for the qf interactive log filter composer.
package core

import (
	"container/list"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
)

// CacheStats provides statistics about the pattern cache performance.
// These statistics help monitor cache effectiveness and tune cache size.
type CacheStats struct {
	// Hits tracks the number of cache hits (successful lookups)
	Hits int64 `json:"hits"`

	// Misses tracks the number of cache misses (failed lookups)
	Misses int64 `json:"misses"`

	// Size is the current number of patterns in the cache
	Size int `json:"size"`

	// MaxSize is the maximum cache capacity
	MaxSize int `json:"max_size"`
}

// HitRate calculates the cache hit rate as a percentage.
// Returns 0.0 if no cache operations have occurred.
func (cs CacheStats) HitRate() float64 {
	total := cs.Hits + cs.Misses
	if total == 0 {
		return 0.0
	}
	return float64(cs.Hits) / float64(total)
}

// lruCacheEntry represents an entry in the LRU cache.
// Each entry contains the compiled regex and metadata for LRU management.
type lruCacheEntry struct {
	// pattern is the raw pattern string used as the cache key
	pattern string

	// compiled is the compiled regexp.Regexp object
	compiled *regexp.Regexp

	// element is the list element in the LRU list for efficient removal
	element *list.Element
}

// PatternManagerOption defines functional options for PatternManager configuration.
type PatternManagerOption func(*PatternManager)

// WithMaxSize sets the maximum cache size (default: 100).
func WithMaxSize(size int) PatternManagerOption {
	return func(pm *PatternManager) {
		if size > 0 {
			pm.maxSize = size
		}
	}
}

// WithStatsEnabled controls whether statistics are tracked (default: true).
func WithStatsEnabled(enabled bool) PatternManagerOption {
	return func(pm *PatternManager) {
		pm.statsEnabled = enabled
	}
}

// PatternManager provides thread-safe LRU caching for compiled regex patterns.
// It optimizes performance by avoiding repeated compilation of the same patterns
// and maintains an LRU eviction policy to manage memory usage.
//
// The implementation uses a doubly-linked list for O(1) LRU operations and
// a hash map for O(1) pattern lookups. Thread safety is ensured through
// a read-write mutex that allows concurrent reads while serializing writes.
type PatternManager struct {
	// cache maps pattern strings to their cache entries
	cache map[string]*lruCacheEntry

	// lruList maintains insertion order for LRU eviction
	// Most recently used items are at the front
	lruList *list.List

	// maxSize is the maximum number of patterns to cache
	maxSize int

	// mutex protects concurrent access to the cache and LRU list
	mutex sync.RWMutex

	// stats track cache performance metrics
	hits   int64
	misses int64

	// statsEnabled controls whether statistics are collected
	statsEnabled bool
}

// NewPatternManager creates a new PatternManager with the specified options.
// Default configuration: maxSize=100, statsEnabled=true.
func NewPatternManager(options ...PatternManagerOption) *PatternManager {
	pm := &PatternManager{
		cache:        make(map[string]*lruCacheEntry),
		lruList:      list.New(),
		maxSize:      100, // Default cache size
		statsEnabled: true,
	}

	// Apply configuration options
	for _, option := range options {
		option(pm)
	}

	return pm
}

// Get retrieves a compiled regex from the cache or compiles and caches it.
// Returns the compiled regex and true if found in cache, false if newly compiled.
// Thread-safe for concurrent access.
func (pm *PatternManager) Get(pattern string) (*regexp.Regexp, bool) {
	if pattern == "" {
		return nil, false
	}

	// First, try to get from cache with read lock
	pm.mutex.RLock()
	entry, exists := pm.cache[pattern]
	if exists {
		// Move to front (most recently used)
		pm.lruList.MoveToFront(entry.element)
		pm.mutex.RUnlock()

		// Update statistics
		if pm.statsEnabled {
			atomic.AddInt64(&pm.hits, 1)
		}

		return entry.compiled, true
	}
	pm.mutex.RUnlock()

	// Cache miss - compile the pattern
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		// Update miss statistics for failed compilation
		if pm.statsEnabled {
			atomic.AddInt64(&pm.misses, 1)
		}
		return nil, false
	}

	// Store in cache with write lock
	pm.mutex.Lock()
	pm.putLocked(pattern, compiled)
	pm.mutex.Unlock()

	// Update statistics
	if pm.statsEnabled {
		atomic.AddInt64(&pm.misses, 1)
	}

	return compiled, false
}

// Put stores a compiled regex in the cache.
// This method is useful when you want to pre-compile and cache patterns.
// Thread-safe for concurrent access.
func (pm *PatternManager) Put(pattern string, compiled *regexp.Regexp) {
	if pattern == "" || compiled == nil {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.putLocked(pattern, compiled)
}

// putLocked is the internal implementation of Put that assumes the write lock is held.
// This method handles LRU eviction and maintains cache size limits.
func (pm *PatternManager) putLocked(pattern string, compiled *regexp.Regexp) {
	// Check if pattern already exists
	if entry, exists := pm.cache[pattern]; exists {
		// Update existing entry and move to front
		entry.compiled = compiled
		pm.lruList.MoveToFront(entry.element)
		return
	}

	// Create new entry
	element := pm.lruList.PushFront(pattern)
	entry := &lruCacheEntry{
		pattern:  pattern,
		compiled: compiled,
		element:  element,
	}
	pm.cache[pattern] = entry

	// Evict least recently used entry if cache is full
	if pm.lruList.Len() > pm.maxSize {
		pm.evictLRU()
	}
}

// evictLRU removes the least recently used entry from the cache.
// Must be called with write lock held.
func (pm *PatternManager) evictLRU() {
	if pm.lruList.Len() == 0 {
		return
	}

	// Get the least recently used element (back of the list)
	element := pm.lruList.Back()
	if element != nil {
		pattern := element.Value.(string)

		// Remove from cache and list
		delete(pm.cache, pattern)
		pm.lruList.Remove(element)
	}
}

// Remove removes a pattern from the cache.
// Returns true if the pattern was found and removed, false otherwise.
// Thread-safe for concurrent access.
func (pm *PatternManager) Remove(pattern string) bool {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	entry, exists := pm.cache[pattern]
	if !exists {
		return false
	}

	// Remove from both cache and LRU list
	delete(pm.cache, pattern)
	pm.lruList.Remove(entry.element)

	return true
}

// Clear removes all cached patterns.
// Thread-safe for concurrent access.
func (pm *PatternManager) Clear() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Clear the cache map
	pm.cache = make(map[string]*lruCacheEntry)

	// Clear the LRU list
	pm.lruList.Init()
}

// Size returns the current number of cached patterns.
// Thread-safe for concurrent access.
func (pm *PatternManager) Size() int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	return len(pm.cache)
}

// Stats returns the current cache statistics.
// Thread-safe for concurrent access.
func (pm *PatternManager) Stats() CacheStats {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	return CacheStats{
		Hits:    atomic.LoadInt64(&pm.hits),
		Misses:  atomic.LoadInt64(&pm.misses),
		Size:    len(pm.cache),
		MaxSize: pm.maxSize,
	}
}

// ResetStats resets the cache statistics to zero.
// Thread-safe for concurrent access.
func (pm *PatternManager) ResetStats() {
	atomic.StoreInt64(&pm.hits, 0)
	atomic.StoreInt64(&pm.misses, 0)
}

// GetMaxSize returns the maximum cache size.
func (pm *PatternManager) GetMaxSize() int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.maxSize
}

// SetMaxSize updates the maximum cache size and evicts entries if necessary.
// Thread-safe for concurrent access.
func (pm *PatternManager) SetMaxSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("cache size must be positive, got %d", size)
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.maxSize = size

	// Evict entries if current size exceeds new maximum
	for pm.lruList.Len() > pm.maxSize {
		pm.evictLRU()
	}

	return nil
}

// IsStatsEnabled returns whether statistics collection is enabled.
func (pm *PatternManager) IsStatsEnabled() bool {
	return pm.statsEnabled
}

// SetStatsEnabled controls whether statistics are collected.
func (pm *PatternManager) SetStatsEnabled(enabled bool) {
	pm.statsEnabled = enabled
}

// GetPatterns returns a slice of all cached pattern strings.
// The patterns are returned in LRU order (most recently used first).
// Thread-safe for concurrent access.
func (pm *PatternManager) GetPatterns() []string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	patterns := make([]string, 0, len(pm.cache))

	// Traverse the LRU list from front (most recent) to back (least recent)
	for element := pm.lruList.Front(); element != nil; element = element.Next() {
		patterns = append(patterns, element.Value.(string))
	}

	return patterns
}

// Contains checks if a pattern exists in the cache without affecting LRU order.
// Thread-safe for concurrent access.
func (pm *PatternManager) Contains(pattern string) bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	_, exists := pm.cache[pattern]
	return exists
}
