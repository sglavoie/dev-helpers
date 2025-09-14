// Package core provides the core business logic for the qf interactive log filter composer.
// This package implements the FilterEngine interface for high-performance log filtering.
package core

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// FilterPatternType defines whether a pattern includes or excludes content
// This type matches the contract test interface requirements
type FilterPatternType int

const (
	// FilterInclude patterns use OR logic - content matching any include pattern passes
	FilterInclude FilterPatternType = iota
	// FilterExclude patterns use veto logic - content matching any exclude pattern is filtered out
	FilterExclude
)

// String returns a string representation of the FilterPatternType
func (fpt FilterPatternType) String() string {
	switch fpt {
	case FilterInclude:
		return "Include"
	case FilterExclude:
		return "Exclude"
	default:
		return "Unknown"
	}
}

// FilterPattern represents a compiled filter pattern with metadata.
// This type matches the contract test interface requirements.
type FilterPattern struct {
	ID         string            // UUID for identification
	Expression string            // Raw regex pattern
	Type       FilterPatternType // Include or Exclude
	MatchCount int               // Usage statistics
	Color      string            // Highlighting color
	Created    time.Time         // Metadata
	IsValid    bool              // Compilation status
	compiled   *regexp.Regexp    // Internal compiled regex (not exported)
}

// Highlight represents a highlighted match within a line
type Highlight struct {
	Start     int    // Start position in line
	End       int    // End position in line
	PatternID string // Which pattern caused this highlight
	Color     string // Color to use for highlighting
}

// FilterStats provides performance and usage statistics
type FilterStats struct {
	TotalLines     int           // Total lines processed
	MatchedLines   int           // Lines that passed filters
	ProcessingTime time.Duration // Time taken to process
	PatternsUsed   int           // Number of patterns applied
	CacheHits      int           // Pattern cache hits
	CacheMisses    int           // Pattern cache misses
}

// FilterResult represents the result of applying filters to content
type FilterResult struct {
	MatchedLines    []string            // Lines that passed all filters
	LineNumbers     []int               // Original line numbers of matched lines
	MatchHighlights map[int][]Highlight // Highlighting information per line
	Stats           FilterStats         // Performance and match statistics
}

// ValidationError represents pattern compilation or validation errors
type ValidationError struct {
	PatternID string
	Pattern   string
	Reason    string
	Err       error
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("pattern validation failed for %s (%s): %s - %v",
		e.PatternID, e.Pattern, e.Reason, e.Err)
}

// cacheEntry represents a cached compiled pattern with metadata
type cacheEntry struct {
	pattern    *regexp.Regexp
	lastUsed   time.Time
	useCount   int
	patternID  string
	expression string
}

// FilterEngine defines the core filtering interface that implementations must satisfy
type FilterEngine interface {
	// AddPattern adds a new pattern to the filter set
	// Returns ValidationError if pattern is invalid
	AddPattern(pattern FilterPattern) error

	// RemovePattern removes a pattern by ID
	RemovePattern(patternID string) error

	// UpdatePattern updates an existing pattern
	// Returns ValidationError if new pattern is invalid
	UpdatePattern(patternID string, pattern FilterPattern) error

	// GetPatterns returns all current patterns
	GetPatterns() []FilterPattern

	// ValidatePattern checks if a pattern is valid without adding it
	ValidatePattern(expression string) error

	// ApplyFilters processes content lines through all active filters
	// Uses OR logic for Include patterns, veto logic for Exclude patterns
	// Empty includes = show all (minus excludes)
	ApplyFilters(ctx context.Context, lines []string) (FilterResult, error)

	// ClearPatterns removes all patterns
	ClearPatterns()

	// GetCacheStats returns pattern compilation cache statistics
	GetCacheStats() (hits int, misses int, size int)
}

// filterEngineImpl implements the FilterEngine interface
type filterEngineImpl struct {
	// patterns stores all active filter patterns
	patterns map[string]FilterPattern

	// patternCache caches compiled regex patterns for performance
	patternCache sync.Map // map[string]*cacheEntry

	// Mutex for thread-safe pattern management
	patternsMutex sync.RWMutex

	// Cache statistics
	cacheHits   int64
	cacheMisses int64
	statsMutex  sync.RWMutex

	// Configuration options
	debounceDelay time.Duration
	maxCacheSize  int
	maxWorkers    int
}

// FilterEngineOption represents configuration options for the FilterEngine
type FilterEngineOption func(*filterEngineImpl)

// WithDebounceDelay sets the debounce delay for filter updates
func WithDebounceDelay(delay time.Duration) FilterEngineOption {
	return func(fe *filterEngineImpl) {
		fe.debounceDelay = delay
	}
}

// WithCacheSize sets the maximum cache size for compiled patterns
func WithCacheSize(size int) FilterEngineOption {
	return func(fe *filterEngineImpl) {
		fe.maxCacheSize = size
	}
}

// WithMaxWorkers sets the maximum number of worker goroutines for parallel processing
func WithMaxWorkers(workers int) FilterEngineOption {
	return func(fe *filterEngineImpl) {
		fe.maxWorkers = workers
	}
}

// NewFilterEngine creates a new FilterEngine instance with default configuration
func NewFilterEngine(options ...FilterEngineOption) FilterEngine {
	fe := &filterEngineImpl{
		patterns:      make(map[string]FilterPattern),
		debounceDelay: 150 * time.Millisecond,
		maxCacheSize:  1000, // Default cache size
		maxWorkers:    4,    // Default worker count
	}

	// Apply configuration options
	for _, option := range options {
		option(fe)
	}

	return fe
}

// AddPattern adds a new pattern to the filter set
func (fe *filterEngineImpl) AddPattern(pattern FilterPattern) error {
	// Validate pattern
	if err := fe.ValidatePattern(pattern.Expression); err != nil {
		return ValidationError{
			PatternID: pattern.ID,
			Pattern:   pattern.Expression,
			Reason:    "invalid regex expression",
			Err:       err,
		}
	}

	fe.patternsMutex.Lock()
	defer fe.patternsMutex.Unlock()

	// Check for duplicate pattern ID
	if _, exists := fe.patterns[pattern.ID]; exists {
		return fmt.Errorf("pattern with ID %s already exists", pattern.ID)
	}

	// Set pattern as valid
	pattern.IsValid = true
	fe.patterns[pattern.ID] = pattern

	return nil
}

// RemovePattern removes a pattern by ID
func (fe *filterEngineImpl) RemovePattern(patternID string) error {
	fe.patternsMutex.Lock()
	defer fe.patternsMutex.Unlock()

	if _, exists := fe.patterns[patternID]; !exists {
		return fmt.Errorf("pattern with ID %s not found", patternID)
	}

	delete(fe.patterns, patternID)

	// Remove from cache if exists
	fe.patternCache.Delete(patternID)

	return nil
}

// UpdatePattern updates an existing pattern
func (fe *filterEngineImpl) UpdatePattern(patternID string, pattern FilterPattern) error {
	// Validate new pattern
	if err := fe.ValidatePattern(pattern.Expression); err != nil {
		return ValidationError{
			PatternID: pattern.ID,
			Pattern:   pattern.Expression,
			Reason:    "invalid regex expression",
			Err:       err,
		}
	}

	fe.patternsMutex.Lock()
	defer fe.patternsMutex.Unlock()

	// Check if pattern exists
	if _, exists := fe.patterns[patternID]; !exists {
		return fmt.Errorf("pattern with ID %s not found", patternID)
	}

	// Ensure ID consistency
	pattern.ID = patternID
	pattern.IsValid = true
	fe.patterns[patternID] = pattern

	// Remove old compiled pattern from cache
	fe.patternCache.Delete(patternID)

	return nil
}

// GetPatterns returns all current patterns
func (fe *filterEngineImpl) GetPatterns() []FilterPattern {
	fe.patternsMutex.RLock()
	defer fe.patternsMutex.RUnlock()

	patterns := make([]FilterPattern, 0, len(fe.patterns))
	for _, pattern := range fe.patterns {
		patterns = append(patterns, pattern)
	}

	return patterns
}

// ValidatePattern checks if a pattern is valid without adding it
func (fe *filterEngineImpl) ValidatePattern(expression string) error {
	return ValidatePattern(expression) // Use shared validation logic
}

// ApplyFilters processes content lines through all active filters
func (fe *filterEngineImpl) ApplyFilters(ctx context.Context, lines []string) (FilterResult, error) {
	start := time.Now()

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return FilterResult{}, ctx.Err()
	default:
	}

	result := FilterResult{
		MatchedLines:    make([]string, 0),
		LineNumbers:     make([]int, 0),
		MatchHighlights: make(map[int][]Highlight),
		Stats: FilterStats{
			TotalLines: len(lines),
		},
	}

	// Get current patterns safely
	fe.patternsMutex.RLock()
	includePatterns := make([]FilterPattern, 0)
	excludePatterns := make([]FilterPattern, 0)

	for _, pattern := range fe.patterns {
		if pattern.Type == FilterInclude {
			includePatterns = append(includePatterns, pattern)
		} else {
			excludePatterns = append(excludePatterns, pattern)
		}
	}
	fe.patternsMutex.RUnlock()

	result.Stats.PatternsUsed = len(includePatterns) + len(excludePatterns)

	// Compile patterns and cache them
	compiledIncludes, err := fe.compilePatterns(includePatterns)
	if err != nil {
		return result, fmt.Errorf("failed to compile include patterns: %w", err)
	}

	compiledExcludes, err := fe.compilePatterns(excludePatterns)
	if err != nil {
		return result, fmt.Errorf("failed to compile exclude patterns: %w", err)
	}

	// Use optimized batch processing for large datasets
	allPatterns := append(includePatterns, excludePatterns...)
	if len(lines) > 5000 { // Use batch processing for large datasets
		matchedLines, err := ProcessLinesBatchOptimized(ctx, lines, allPatterns)
		if err != nil {
			return result, fmt.Errorf("batch processing failed: %w", err)
		}

		// Build result from batch processing
		for _, line := range matchedLines {
			// Find original line index
			for i, originalLine := range lines {
				if originalLine == line {
					result.MatchedLines = append(result.MatchedLines, line)
					result.LineNumbers = append(result.LineNumbers, i)

					// Generate highlights for this line
					highlights := fe.generateHighlights(line, allPatterns, append(compiledIncludes, compiledExcludes...))
					if len(highlights) > 0 {
						result.MatchHighlights[i] = highlights
					}
					break
				}
			}
		}
	} else {
		// Process lines sequentially for smaller datasets
		for i, line := range lines {
			// Check for context cancellation periodically
			if i%1000 == 0 {
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				default:
				}
			}

			// Apply filtering logic
			if fe.shouldIncludeLine(line, compiledIncludes, compiledExcludes) {
				result.MatchedLines = append(result.MatchedLines, line)
				result.LineNumbers = append(result.LineNumbers, i)

				// Generate highlights for this line
				highlights := fe.generateHighlights(line, allPatterns, append(compiledIncludes, compiledExcludes...))
				if len(highlights) > 0 {
					result.MatchHighlights[i] = highlights
				}
			}
		}
	}

	result.Stats.MatchedLines = len(result.MatchedLines)
	result.Stats.ProcessingTime = time.Since(start)

	// Get cache stats
	hits, misses, _ := fe.GetCacheStats()
	result.Stats.CacheHits = hits
	result.Stats.CacheMisses = misses

	return result, nil
}

// shouldIncludeLine determines if a line should be included based on filter logic
func (fe *filterEngineImpl) shouldIncludeLine(line string, includes, excludes []*regexp.Regexp) bool {
	// First check exclude patterns (veto logic)
	for _, exclude := range excludes {
		if exclude.MatchString(line) {
			return false
		}
	}

	// If no include patterns, show all (minus excludes)
	if len(includes) == 0 {
		return true
	}

	// Check include patterns (OR logic)
	for _, include := range includes {
		if include.MatchString(line) {
			return true
		}
	}

	return false
}

// generateHighlights creates highlight information for matching patterns in a line
func (fe *filterEngineImpl) generateHighlights(line string, patterns []FilterPattern, compiled []*regexp.Regexp) []Highlight {
	// Use shared highlight generation logic for better consistency
	highlights, err := GenerateHighlights(line, patterns)
	if err != nil {
		// Fallback to empty highlights on error
		return []Highlight{}
	}
	return highlights
}

// compilePatterns compiles a list of patterns and caches them
func (fe *filterEngineImpl) compilePatterns(patterns []FilterPattern) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, len(patterns))

	for i, pattern := range patterns {
		regex, err := fe.getCompiledPattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %s: %w", pattern.ID, err)
		}
		compiled[i] = regex
	}

	return compiled, nil
}

// getCompiledPattern gets a compiled regex pattern from cache or compiles and caches it
func (fe *filterEngineImpl) getCompiledPattern(pattern FilterPattern) (*regexp.Regexp, error) {
	// Use optimized regex cache for better performance
	compiled, err := GetCompiledRegexOptimized(pattern.Expression)
	if err != nil {
		return nil, err
	}

	// Update local cache stats for compatibility
	fe.statsMutex.Lock()
	hits, misses, _ := DefaultRegexCache.GetCacheStats()
	fe.cacheHits = hits
	fe.cacheMisses = misses
	fe.statsMutex.Unlock()

	return compiled, nil
}

// evictOldCacheEntries implements LRU cache eviction
func (fe *filterEngineImpl) evictOldCacheEntries() {
	size := 0
	var oldest *cacheEntry
	var oldestKey string
	oldestTime := time.Now()

	// Count entries and find oldest
	fe.patternCache.Range(func(key, value interface{}) bool {
		size++
		entry := value.(*cacheEntry)
		if entry.lastUsed.Before(oldestTime) {
			oldest = entry
			oldestKey = key.(string)
			oldestTime = entry.lastUsed
		}
		return true
	})

	// Evict if over limit
	if size > fe.maxCacheSize && oldest != nil {
		fe.patternCache.Delete(oldestKey)
	}
}

// ClearPatterns removes all patterns
func (fe *filterEngineImpl) ClearPatterns() {
	fe.patternsMutex.Lock()
	defer fe.patternsMutex.Unlock()

	fe.patterns = make(map[string]FilterPattern)

	// Clear cache
	fe.patternCache.Range(func(key, value interface{}) bool {
		fe.patternCache.Delete(key)
		return true
	})
}

// GetCacheStats returns pattern compilation cache statistics
func (fe *filterEngineImpl) GetCacheStats() (hits int, misses int, size int) {
	fe.statsMutex.RLock()
	defer fe.statsMutex.RUnlock()

	hits = int(fe.cacheHits)
	misses = int(fe.cacheMisses)

	// Count cache size
	fe.patternCache.Range(func(key, value interface{}) bool {
		size++
		return true
	})

	return hits, misses, size
}
