package core

import (
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestNewPatternManager(t *testing.T) {
	tests := []struct {
		name          string
		options       []PatternManagerOption
		expectedMax   int
		expectedStats bool
	}{
		{
			name:          "default configuration",
			options:       nil,
			expectedMax:   100,
			expectedStats: true,
		},
		{
			name:          "custom max size",
			options:       []PatternManagerOption{WithMaxSize(50)},
			expectedMax:   50,
			expectedStats: true,
		},
		{
			name:          "stats disabled",
			options:       []PatternManagerOption{WithStatsEnabled(false)},
			expectedMax:   100,
			expectedStats: false,
		},
		{
			name: "multiple options",
			options: []PatternManagerOption{
				WithMaxSize(25),
				WithStatsEnabled(false),
			},
			expectedMax:   25,
			expectedStats: false,
		},
		{
			name:          "invalid max size ignored",
			options:       []PatternManagerOption{WithMaxSize(-1)},
			expectedMax:   100,
			expectedStats: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPatternManager(tt.options...)

			if pm.GetMaxSize() != tt.expectedMax {
				t.Errorf("Expected max size %d, got %d", tt.expectedMax, pm.GetMaxSize())
			}

			if pm.IsStatsEnabled() != tt.expectedStats {
				t.Errorf("Expected stats enabled %v, got %v", tt.expectedStats, pm.IsStatsEnabled())
			}

			if pm.Size() != 0 {
				t.Errorf("Expected empty cache, got size %d", pm.Size())
			}
		})
	}
}

func TestPatternManagerGet(t *testing.T) {
	pm := NewPatternManager()

	tests := []struct {
		name           string
		pattern        string
		expectCompiled bool
		expectCached   bool
	}{
		{
			name:           "valid pattern first time",
			pattern:        "test.*pattern",
			expectCompiled: true,
			expectCached:   false, // First time, so not cached
		},
		{
			name:           "same pattern second time",
			pattern:        "test.*pattern",
			expectCompiled: true,
			expectCached:   true, // Should be cached now
		},
		{
			name:           "different valid pattern",
			pattern:        "error|warning",
			expectCompiled: true,
			expectCached:   false,
		},
		{
			name:           "invalid pattern",
			pattern:        "[unclosed",
			expectCompiled: false,
			expectCached:   false,
		},
		{
			name:           "empty pattern",
			pattern:        "",
			expectCompiled: false,
			expectCached:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, cached := pm.Get(tt.pattern)

			if tt.expectCompiled {
				if compiled == nil {
					t.Error("Expected compiled regex, got nil")
				} else {
					// Test that the compiled regex works
					testMatch := compiled.MatchString("test some pattern here")
					expectedMatch := regexp.MustCompile(tt.pattern).MatchString("test some pattern here")
					if testMatch != expectedMatch {
						t.Error("Compiled regex behavior differs from expected")
					}
				}
			} else {
				if compiled != nil {
					t.Error("Expected nil compiled regex")
				}
			}

			if cached != tt.expectCached {
				t.Errorf("Expected cached %v, got %v", tt.expectCached, cached)
			}
		})
	}
}

func TestPatternManagerPut(t *testing.T) {
	pm := NewPatternManager()

	pattern := "test.*put"
	compiled := regexp.MustCompile(pattern)

	// Put the pattern in cache
	pm.Put(pattern, compiled)

	// Verify it's cached
	if pm.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", pm.Size())
	}

	if !pm.Contains(pattern) {
		t.Error("Expected pattern to be in cache")
	}

	// Get it back and verify it's cached
	retrieved, cached := pm.Get(pattern)
	if !cached {
		t.Error("Expected pattern to be cached")
	}
	if retrieved == nil {
		t.Error("Expected non-nil compiled regex")
	}

	// Test edge cases
	pm.Put("", compiled) // Empty pattern should be ignored
	pm.Put(pattern, nil) // Nil compiled should be ignored

	// Size should still be 1
	if pm.Size() != 1 {
		t.Errorf("Expected cache size 1 after edge cases, got %d", pm.Size())
	}
}

func TestPatternManagerLRUEviction(t *testing.T) {
	// Create manager with small cache size
	pm := NewPatternManager(WithMaxSize(3))

	patterns := []string{
		"pattern1",
		"pattern2",
		"pattern3",
		"pattern4", // This should evict pattern1
	}

	// Add patterns to fill and exceed cache
	for _, pattern := range patterns {
		pm.Get(pattern)
	}

	// Cache should have exactly 3 patterns
	if pm.Size() != 3 {
		t.Errorf("Expected cache size 3, got %d", pm.Size())
	}

	// pattern1 should be evicted (least recently used)
	if pm.Contains("pattern1") {
		t.Error("Expected pattern1 to be evicted")
	}

	// Other patterns should still be cached
	expectedPatterns := []string{"pattern2", "pattern3", "pattern4"}
	for _, pattern := range expectedPatterns {
		if !pm.Contains(pattern) {
			t.Errorf("Expected pattern %s to be cached", pattern)
		}
	}

	// Access pattern2 to make it most recently used
	pm.Get("pattern2")

	// Add another pattern - pattern3 should be evicted now
	pm.Get("pattern5")

	if pm.Contains("pattern3") {
		t.Error("Expected pattern3 to be evicted after accessing pattern2")
	}

	if !pm.Contains("pattern2") {
		t.Error("Expected pattern2 to still be cached after recent access")
	}
}

func TestPatternManagerLRUOrder(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(3))

	// Add patterns in order
	patterns := []string{"first", "second", "third"}
	for _, pattern := range patterns {
		pm.Get(pattern)
	}

	// Get patterns in LRU order (most recent first)
	cachedPatterns := pm.GetPatterns()

	// Should be in reverse order of insertion (third, second, first)
	expected := []string{"third", "second", "first"}
	if len(cachedPatterns) != len(expected) {
		t.Errorf("Expected %d patterns, got %d", len(expected), len(cachedPatterns))
	}

	for i, pattern := range expected {
		if cachedPatterns[i] != pattern {
			t.Errorf("Expected pattern %d to be %s, got %s", i, pattern, cachedPatterns[i])
		}
	}

	// Access "first" to move it to front
	pm.Get("first")

	// Now order should be: first, third, second
	cachedPatterns = pm.GetPatterns()
	expected = []string{"first", "third", "second"}

	for i, pattern := range expected {
		if cachedPatterns[i] != pattern {
			t.Errorf("After accessing 'first', expected pattern %d to be %s, got %s", i, pattern, cachedPatterns[i])
		}
	}
}

func TestPatternManagerRemove(t *testing.T) {
	pm := NewPatternManager()

	// Add some patterns
	patterns := []string{"remove1", "remove2", "remove3"}
	for _, pattern := range patterns {
		pm.Get(pattern)
	}

	initialSize := pm.Size()
	if initialSize != 3 {
		t.Errorf("Expected initial size 3, got %d", initialSize)
	}

	// Remove existing pattern
	removed := pm.Remove("remove2")
	if !removed {
		t.Error("Expected Remove to return true for existing pattern")
	}

	if pm.Size() != 2 {
		t.Errorf("Expected size 2 after removal, got %d", pm.Size())
	}

	if pm.Contains("remove2") {
		t.Error("Expected removed pattern to not be in cache")
	}

	// Try to remove non-existing pattern
	removed = pm.Remove("nonexistent")
	if removed {
		t.Error("Expected Remove to return false for non-existing pattern")
	}

	if pm.Size() != 2 {
		t.Errorf("Expected size to remain 2 after removing non-existing pattern, got %d", pm.Size())
	}
}

func TestPatternManagerClear(t *testing.T) {
	pm := NewPatternManager()

	// Add some patterns
	patterns := []string{"clear1", "clear2", "clear3"}
	for _, pattern := range patterns {
		pm.Get(pattern)
	}

	if pm.Size() != 3 {
		t.Errorf("Expected size 3 before clear, got %d", pm.Size())
	}

	// Clear the cache
	pm.Clear()

	if pm.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", pm.Size())
	}

	// Verify no patterns are cached
	for _, pattern := range patterns {
		if pm.Contains(pattern) {
			t.Errorf("Expected pattern %s to be cleared", pattern)
		}
	}
}

func TestPatternManagerStats(t *testing.T) {
	pm := NewPatternManager()

	initialStats := pm.Stats()
	if initialStats.Hits != 0 || initialStats.Misses != 0 {
		t.Error("Expected initial stats to be zero")
	}

	// First access should be a miss
	pm.Get("stats_test")
	stats := pm.Stats()
	if stats.Hits != 0 || stats.Misses != 1 {
		t.Errorf("Expected 0 hits, 1 miss after first access, got %d hits, %d misses", stats.Hits, stats.Misses)
	}

	// Second access should be a hit
	pm.Get("stats_test")
	stats = pm.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("Expected 1 hit, 1 miss after second access, got %d hits, %d misses", stats.Hits, stats.Misses)
	}

	// Test hit rate calculation
	expectedHitRate := 1.0 / 2.0 // 1 hit out of 2 total
	if stats.HitRate() != expectedHitRate {
		t.Errorf("Expected hit rate %f, got %f", expectedHitRate, stats.HitRate())
	}

	// Invalid pattern should increase misses
	pm.Get("[invalid")
	stats = pm.Stats()
	if stats.Hits != 1 || stats.Misses != 2 {
		t.Errorf("Expected 1 hit, 2 misses after invalid pattern, got %d hits, %d misses", stats.Hits, stats.Misses)
	}

	// Test stats reset
	pm.ResetStats()
	stats = pm.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("Expected stats to be reset to zero")
	}

	if stats.HitRate() != 0.0 {
		t.Errorf("Expected hit rate 0.0 after reset, got %f", stats.HitRate())
	}
}

func TestPatternManagerStatsDisabled(t *testing.T) {
	pm := NewPatternManager(WithStatsEnabled(false))

	// Access patterns
	pm.Get("test1")
	pm.Get("test1") // Hit
	pm.Get("test2") // Miss

	stats := pm.Stats()
	// When stats are disabled, they should still work but might be 0
	// The key test is that the functionality doesn't break
	if stats.Size != 2 {
		t.Errorf("Expected cache size 2, got %d", stats.Size)
	}
}

func TestPatternManagerSetMaxSize(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(5))

	// Fill cache
	for i := 0; i < 5; i++ {
		pm.Get(fmt.Sprintf("pattern%d", i))
	}

	if pm.Size() != 5 {
		t.Errorf("Expected size 5, got %d", pm.Size())
	}

	// Reduce max size - should trigger eviction
	err := pm.SetMaxSize(3)
	if err != nil {
		t.Errorf("Expected no error setting max size, got %v", err)
	}

	if pm.Size() != 3 {
		t.Errorf("Expected size 3 after reducing max size, got %d", pm.Size())
	}

	if pm.GetMaxSize() != 3 {
		t.Errorf("Expected max size 3, got %d", pm.GetMaxSize())
	}

	// Test invalid size
	err = pm.SetMaxSize(0)
	if err == nil {
		t.Error("Expected error for zero max size")
	}

	err = pm.SetMaxSize(-1)
	if err == nil {
		t.Error("Expected error for negative max size")
	}
}

func TestPatternManagerConcurrentAccess(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(10))

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start multiple goroutines performing concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				pattern := fmt.Sprintf("pattern_%d_%d", id, j%5) // Limited patterns to increase hits

				// Mix of operations
				switch j % 4 {
				case 0:
					pm.Get(pattern)
				case 1:
					compiled := regexp.MustCompile(fmt.Sprintf("test_%d", j))
					pm.Put(pattern, compiled)
				case 2:
					pm.Contains(pattern)
				case 3:
					pm.Remove(pattern)
				}
			}
		}(i)
	}

	// Also perform operations on main thread
	go func() {
		for i := 0; i < numOperations; i++ {
			pm.Stats()
			pm.Size()
			pm.GetPatterns()
			pm.Clear()
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()

	// Verify the manager is still in a valid state
	stats := pm.Stats()
	size := pm.Size()

	if size < 0 || size > pm.GetMaxSize() {
		t.Errorf("Invalid cache size after concurrent access: %d (max: %d)", size, pm.GetMaxSize())
	}

	if stats.Size != size {
		t.Errorf("Stats size %d doesn't match actual size %d", stats.Size, size)
	}
}

func TestPatternManagerConcurrentSamePattern(t *testing.T) {
	pm := NewPatternManager()

	const numGoroutines = 20
	pattern := "concurrent_test_pattern.*"

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make(chan bool, numGoroutines)

	// Multiple goroutines trying to get the same pattern
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			compiled, _ := pm.Get(pattern)
			results <- (compiled != nil)
		}()
	}

	wg.Wait()
	close(results)

	// All should succeed
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected %d successful compilations, got %d", numGoroutines, successCount)
	}

	// Pattern should be cached exactly once
	if pm.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", pm.Size())
	}
}

func TestCacheStatsHitRate(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{
			name:     "no operations",
			hits:     0,
			misses:   0,
			expected: 0.0,
		},
		{
			name:     "all hits",
			hits:     10,
			misses:   0,
			expected: 1.0,
		},
		{
			name:     "all misses",
			hits:     0,
			misses:   5,
			expected: 0.0,
		},
		{
			name:     "mixed results",
			hits:     3,
			misses:   7,
			expected: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CacheStats{
				Hits:   tt.hits,
				Misses: tt.misses,
			}

			hitRate := stats.HitRate()
			if hitRate != tt.expected {
				t.Errorf("Expected hit rate %f, got %f", tt.expected, hitRate)
			}
		})
	}
}

func TestPatternManagerBenchmark(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(100))

	// Pre-populate cache
	patterns := make([]string, 50)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("benchmark_pattern_%d.*", i)
		pm.Get(patterns[i])
	}

	// Measure performance of cache hits vs misses
	start := time.Now()
	for i := 0; i < 1000; i++ {
		pm.Get(patterns[i%len(patterns)]) // Should be cache hits
	}
	cacheHitDuration := time.Since(start)

	start = time.Now()
	for i := 0; i < 100; i++ {
		pm.Get(fmt.Sprintf("new_pattern_%d", i)) // Cache misses
	}
	cacheMissDuration := time.Since(start)

	t.Logf("Cache hits (1000 ops): %v", cacheHitDuration)
	t.Logf("Cache misses (100 ops): %v", cacheMissDuration)

	// Cache hits should be significantly faster per operation
	avgHitTime := cacheHitDuration / 1000
	avgMissTime := cacheMissDuration / 100

	if avgHitTime > avgMissTime {
		t.Errorf("Cache hits (%v) should be faster than cache misses (%v)", avgHitTime, avgMissTime)
	}
}
