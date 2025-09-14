// Package core provides performance optimization utilities for hot paths
// in the qf Interactive Log Filter Composer.
package core

import (
	"context"
	"regexp"
	"sync"
	"time"
)

// BatchProcessor optimizes batch operations for improved performance
type BatchProcessor struct {
	batchSize  int
	maxWorkers int
	workerPool chan struct{}
	mu         sync.RWMutex
	stats      ProcessingStats
}

// ProcessingStats tracks performance metrics
type ProcessingStats struct {
	TotalProcessed   int64
	BatchesProcessed int64
	AverageTime      time.Duration
	LastProcessTime  time.Time
}

// NewBatchProcessor creates a new batch processor with specified configuration
func NewBatchProcessor(batchSize, maxWorkers int) *BatchProcessor {
	return &BatchProcessor{
		batchSize:  batchSize,
		maxWorkers: maxWorkers,
		workerPool: make(chan struct{}, maxWorkers),
	}
}

// ProcessLinesBatch processes lines in optimized batches
func (bp *BatchProcessor) ProcessLinesBatch(ctx context.Context, lines []string, patterns []FilterPattern) ([]string, error) {
	if len(lines) == 0 {
		return []string{}, nil
	}

	start := time.Now()
	var result []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process in batches
	for i := 0; i < len(lines); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(lines) {
			end = len(lines)
		}

		batch := lines[i:end]

		// Acquire worker from pool
		bp.workerPool <- struct{}{}
		wg.Add(1)

		go func(batch []string) {
			defer func() {
				<-bp.workerPool
				wg.Done()
			}()

			// Process batch
			batchResult := bp.processBatch(ctx, batch, patterns)

			// Append results thread-safely
			mu.Lock()
			result = append(result, batchResult...)
			mu.Unlock()
		}(batch)
	}

	// Wait for all workers to complete
	wg.Wait()

	// Update statistics
	bp.updateStats(len(lines), time.Since(start))

	return result, nil
}

// processBatch processes a single batch of lines
func (bp *BatchProcessor) processBatch(ctx context.Context, lines []string, patterns []FilterPattern) []string {
	var result []string

	for _, line := range lines {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result
		default:
		}

		// Use shared filter logic for consistency and performance
		shouldInclude, err := ShouldIncludeLine(line, getIncludePatterns(patterns), getExcludePatterns(patterns))
		if err != nil {
			continue // Skip lines with processing errors
		}

		if shouldInclude {
			result = append(result, line)
		}
	}

	return result
}

// updateStats updates processing statistics
func (bp *BatchProcessor) updateStats(processed int, duration time.Duration) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.stats.TotalProcessed += int64(processed)
	bp.stats.BatchesProcessed++
	bp.stats.LastProcessTime = time.Now()

	// Calculate rolling average
	if bp.stats.BatchesProcessed == 1 {
		bp.stats.AverageTime = duration
	} else {
		bp.stats.AverageTime = (bp.stats.AverageTime + duration) / 2
	}
}

// GetStats returns current processing statistics
func (bp *BatchProcessor) GetStats() ProcessingStats {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.stats
}

// RegexCache provides optimized regex compilation caching
type RegexCache struct {
	cache   sync.Map // map[string]*cacheEntry
	maxSize int
	hits    int64
	misses  int64
	mu      sync.RWMutex
}

type regexCacheEntry struct {
	regex      *regexp.Regexp
	lastUsed   time.Time
	useCount   int64
	expression string
}

// NewRegexCache creates a new regex cache with specified maximum size
func NewRegexCache(maxSize int) *RegexCache {
	return &RegexCache{
		maxSize: maxSize,
	}
}

// Get retrieves a compiled regex from cache or compiles and caches it
func (rc *RegexCache) Get(expression string) (*regexp.Regexp, error) {
	// Check cache first
	if cached, ok := rc.cache.Load(expression); ok {
		entry := cached.(*regexCacheEntry)
		entry.lastUsed = time.Now()
		entry.useCount++

		rc.mu.Lock()
		rc.hits++
		rc.mu.Unlock()

		return entry.regex, nil
	}

	// Cache miss - compile and store
	rc.mu.Lock()
	rc.misses++
	rc.mu.Unlock()

	compiled, err := regexp.Compile(expression)
	if err != nil {
		return nil, err
	}

	entry := &regexCacheEntry{
		regex:      compiled,
		lastUsed:   time.Now(),
		useCount:   1,
		expression: expression,
	}

	rc.cache.Store(expression, entry)
	rc.evictIfNeeded()

	return compiled, nil
}

// evictIfNeeded evicts least recently used entries if cache is full
func (rc *RegexCache) evictIfNeeded() {
	size := 0
	var oldestKey string
	var oldestTime time.Time

	rc.cache.Range(func(key, value interface{}) bool {
		size++
		entry := value.(*regexCacheEntry)
		if oldestKey == "" || entry.lastUsed.Before(oldestTime) {
			oldestKey = key.(string)
			oldestTime = entry.lastUsed
		}
		return true
	})

	if size > rc.maxSize && oldestKey != "" {
		rc.cache.Delete(oldestKey)
	}
}

// GetCacheStats returns cache performance statistics
func (rc *RegexCache) GetCacheStats() (hits, misses int64, size int) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	hits = rc.hits
	misses = rc.misses

	rc.cache.Range(func(key, value interface{}) bool {
		size++
		return true
	})

	return hits, misses, size
}

// StreamingFilterProcessor optimizes streaming operations for large files
type StreamingFilterProcessor struct {
	bufferSize int
	processor  *BatchProcessor
	cache      *RegexCache
}

// NewStreamingFilterProcessor creates a new streaming filter processor
func NewStreamingFilterProcessor(bufferSize, batchSize, maxWorkers int) *StreamingFilterProcessor {
	return &StreamingFilterProcessor{
		bufferSize: bufferSize,
		processor:  NewBatchProcessor(batchSize, maxWorkers),
		cache:      NewRegexCache(1000), // Cache up to 1000 compiled patterns
	}
}

// ProcessStream processes a stream of lines with optimized buffering
func (sfp *StreamingFilterProcessor) ProcessStream(ctx context.Context, lineChan <-chan string, patterns []FilterPattern) (<-chan string, error) {
	resultChan := make(chan string, sfp.bufferSize)

	go func() {
		defer close(resultChan)

		var buffer []string
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-lineChan:
				if !ok {
					// Process remaining buffer
					if len(buffer) > 0 {
						sfp.processBuffer(ctx, buffer, patterns, resultChan)
					}
					return
				}

				buffer = append(buffer, line)

				// Process buffer when full
				if len(buffer) >= sfp.bufferSize {
					sfp.processBuffer(ctx, buffer, patterns, resultChan)
					buffer = buffer[:0] // Reset buffer
				}
			}
		}
	}()

	return resultChan, nil
}

// processBuffer processes a buffer of lines and sends results to output channel
func (sfp *StreamingFilterProcessor) processBuffer(ctx context.Context, buffer []string, patterns []FilterPattern, resultChan chan<- string) {
	results, err := sfp.processor.ProcessLinesBatch(ctx, buffer, patterns)
	if err != nil {
		return // Skip batch on error
	}

	for _, result := range results {
		select {
		case <-ctx.Done():
			return
		case resultChan <- result:
		}
	}
}

// Helper functions

// getIncludePatterns filters patterns to include-only
func getIncludePatterns(patterns []FilterPattern) []FilterPattern {
	var includes []FilterPattern
	for _, pattern := range patterns {
		if pattern.Type == FilterInclude {
			includes = append(includes, pattern)
		}
	}
	return includes
}

// getExcludePatterns filters patterns to exclude-only
func getExcludePatterns(patterns []FilterPattern) []FilterPattern {
	var excludes []FilterPattern
	for _, pattern := range patterns {
		if pattern.Type == FilterExclude {
			excludes = append(excludes, pattern)
		}
	}
	return excludes
}

// Global optimization instances
var (
	// DefaultBatchProcessor is a shared batch processor instance
	DefaultBatchProcessor = NewBatchProcessor(1000, 4)

	// DefaultRegexCache is a shared regex cache instance
	DefaultRegexCache = NewRegexCache(1000)

	// DefaultStreamingProcessor is a shared streaming processor instance
	DefaultStreamingProcessor = NewStreamingFilterProcessor(5000, 1000, 4)
)

// Convenience functions using global instances

// ProcessLinesBatchOptimized processes lines using the default optimized batch processor
func ProcessLinesBatchOptimized(ctx context.Context, lines []string, patterns []FilterPattern) ([]string, error) {
	return DefaultBatchProcessor.ProcessLinesBatch(ctx, lines, patterns)
}

// GetCompiledRegexOptimized gets compiled regex using the default optimized cache
func GetCompiledRegexOptimized(expression string) (*regexp.Regexp, error) {
	return DefaultRegexCache.Get(expression)
}

// ProcessStreamOptimized processes a stream using the default optimized streaming processor
func ProcessStreamOptimized(ctx context.Context, lineChan <-chan string, patterns []FilterPattern) (<-chan string, error) {
	return DefaultStreamingProcessor.ProcessStream(ctx, lineChan, patterns)
}
