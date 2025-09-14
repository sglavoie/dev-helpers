package core

import (
	"fmt"
	"regexp"
	"testing"
)

// TestPatternManagerIntegrationWithPattern tests integration with existing Pattern struct
func TestPatternManagerIntegrationWithPattern(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(5))

	// Create patterns using the existing Pattern struct
	patterns := []*Pattern{
		NewPattern("error.*occurred", Include, "#ff0000"),
		NewPattern("warning.*detected", Exclude, "#ffaa00"),
		NewPattern("info.*message", Include, "#00ff00"),
	}

	// Test that PatternManager can work with Pattern.Compile()
	for _, pattern := range patterns {
		if !pattern.IsValid {
			t.Errorf("Expected pattern to be valid: %s", pattern.Expression)
			continue
		}

		// Get compiled regex from pattern manager
		compiled1, cached1 := pm.Get(pattern.Expression)
		if compiled1 == nil {
			t.Errorf("Expected compiled regex for pattern: %s", pattern.Expression)
			continue
		}
		if cached1 {
			t.Errorf("Expected first access to not be cached for pattern: %s", pattern.Expression)
		}

		// Compare with Pattern.Compile()
		compiled2, err := pattern.Compile()
		if err != nil {
			t.Errorf("Expected Pattern.Compile() to succeed for: %s", pattern.Expression)
			continue
		}

		// Test that both compiled regexes produce the same results
		testStrings := []string{
			"error occurred while processing",
			"warning detected in module",
			"info message from system",
			"debug trace information",
		}

		for _, testStr := range testStrings {
			match1 := compiled1.MatchString(testStr)
			match2 := compiled2.MatchString(testStr)
			if match1 != match2 {
				t.Errorf("Compiled regexes produce different results for pattern %s on string %s",
					pattern.Expression, testStr)
			}
		}

		// Second access should be cached
		compiled3, cached2 := pm.Get(pattern.Expression)
		if !cached2 {
			t.Errorf("Expected second access to be cached for pattern: %s", pattern.Expression)
		}
		if compiled3 != compiled1 {
			t.Errorf("Expected same compiled regex instance for cached access")
		}
	}
}

// TestPatternManagerWithFilterSet demonstrates integration with FilterSet
func TestPatternManagerWithFilterSet(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(10))
	filterSet := NewFilterSet("integration-test")

	// Add patterns to FilterSet
	patterns := []Pattern{
		*NewPattern("error", Include, "#ff0000"),
		*NewPattern("warning", Include, "#ffaa00"),
		*NewPattern("debug", Exclude, "#cccccc"),
	}

	for _, pattern := range patterns {
		err := filterSet.AddPattern(pattern)
		if err != nil {
			t.Errorf("Failed to add pattern to FilterSet: %v", err)
		}
	}

	// Use PatternManager to cache compiled versions
	cachedPatterns := make(map[string]*regexp.Regexp)

	for _, pattern := range filterSet.Include {
		compiled, _ := pm.Get(pattern.Expression)
		if compiled != nil {
			cachedPatterns[pattern.ID] = compiled
		}
	}

	for _, pattern := range filterSet.Exclude {
		compiled, _ := pm.Get(pattern.Expression)
		if compiled != nil {
			cachedPatterns[pattern.ID] = compiled
		}
	}

	if len(cachedPatterns) != 3 {
		t.Errorf("Expected 3 cached patterns, got %d", len(cachedPatterns))
	}

	// Test filtering logic with cached patterns
	testLines := []string{
		"error in processing",
		"warning about memory",
		"debug trace here",
		"info message",
	}

	for _, line := range testLines {
		includeMatch := false
		excludeMatch := false

		// Check include patterns
		for _, pattern := range filterSet.Include {
			if compiled := cachedPatterns[pattern.ID]; compiled != nil {
				if compiled.MatchString(line) {
					includeMatch = true
					break
				}
			}
		}

		// Check exclude patterns
		for _, pattern := range filterSet.Exclude {
			if compiled := cachedPatterns[pattern.ID]; compiled != nil {
				if compiled.MatchString(line) {
					excludeMatch = true
					break
				}
			}
		}

		// Apply filtering logic
		shouldInclude := includeMatch && !excludeMatch

		t.Logf("Line: '%s' - Include: %v, Exclude: %v, Result: %v",
			line, includeMatch, excludeMatch, shouldInclude)
	}
}

// TestPatternManagerCacheEfficiency tests cache efficiency over time
func TestPatternManagerCacheEfficiency(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(5))

	patterns := []string{
		"error.*occurred",
		"warning.*detected",
		"info.*message",
		"debug.*trace",
		"fatal.*exception",
	}

	// First round - all should be cache misses
	for _, pattern := range patterns {
		_, cached := pm.Get(pattern)
		if cached {
			t.Errorf("Expected cache miss on first access for: %s", pattern)
		}
	}

	stats := pm.Stats()
	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits after first round, got %d", stats.Hits)
	}
	if stats.Misses != int64(len(patterns)) {
		t.Errorf("Expected %d misses after first round, got %d", len(patterns), stats.Misses)
	}

	// Second round - all should be cache hits
	for _, pattern := range patterns {
		_, cached := pm.Get(pattern)
		if !cached {
			t.Errorf("Expected cache hit on second access for: %s", pattern)
		}
	}

	stats = pm.Stats()
	if stats.Hits != int64(len(patterns)) {
		t.Errorf("Expected %d hits after second round, got %d", len(patterns), stats.Hits)
	}

	// Test hit rate
	expectedHitRate := float64(len(patterns)) / float64(len(patterns)*2) // 5/(5+5) = 0.5
	if stats.HitRate() != expectedHitRate {
		t.Errorf("Expected hit rate %f, got %f", expectedHitRate, stats.HitRate())
	}

	t.Logf("Cache efficiency test: %d patterns, %.2f%% hit rate",
		len(patterns), stats.HitRate()*100)
}

// TestPatternManagerMemoryUsage tests memory behavior with large number of patterns
func TestPatternManagerMemoryUsage(t *testing.T) {
	const maxSize = 10
	const totalPatterns = 50

	pm := NewPatternManager(WithMaxSize(maxSize))

	// Generate many patterns
	for i := 0; i < totalPatterns; i++ {
		pattern := fmt.Sprintf("pattern_%d_.*", i)
		pm.Get(pattern)
	}

	// Cache should not exceed max size
	if pm.Size() > maxSize {
		t.Errorf("Cache size %d exceeds max size %d", pm.Size(), maxSize)
	}

	// Should have exactly maxSize patterns
	if pm.Size() != maxSize {
		t.Errorf("Expected cache size %d, got %d", maxSize, pm.Size())
	}

	// Most recently used patterns should still be cached
	recentPatterns := []string{
		fmt.Sprintf("pattern_%d_.*", totalPatterns-1),
		fmt.Sprintf("pattern_%d_.*", totalPatterns-2),
		fmt.Sprintf("pattern_%d_.*", totalPatterns-3),
	}

	for _, pattern := range recentPatterns {
		if !pm.Contains(pattern) {
			t.Errorf("Expected recent pattern to be cached: %s", pattern)
		}
	}

	// Oldest patterns should be evicted
	oldPatterns := []string{
		fmt.Sprintf("pattern_%d_.*", 0),
		fmt.Sprintf("pattern_%d_.*", 1),
		fmt.Sprintf("pattern_%d_.*", 2),
	}

	for _, pattern := range oldPatterns {
		if pm.Contains(pattern) {
			t.Errorf("Expected old pattern to be evicted: %s", pattern)
		}
	}

	t.Logf("Memory usage test: %d total patterns generated, %d cached (max: %d)",
		totalPatterns, pm.Size(), maxSize)
}

// BenchmarkPatternManagerVsDirectCompile compares performance of cached vs direct compilation
func BenchmarkPatternManagerVsDirectCompile(b *testing.B) {
	pm := NewPatternManager(WithMaxSize(100))
	patterns := []string{
		"error.*occurred",
		"warning.*detected",
		"info.*message",
		"debug.*trace",
		"fatal.*exception",
	}

	// Pre-populate cache
	for _, pattern := range patterns {
		pm.Get(pattern)
	}

	b.Run("PatternManager_CacheHit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pattern := patterns[i%len(patterns)]
			pm.Get(pattern)
		}
	})

	b.Run("DirectCompile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pattern := patterns[i%len(patterns)]
			regexp.Compile(pattern)
		}
	})

	b.Run("PatternManager_CacheMiss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pattern := fmt.Sprintf("new_pattern_%d", i)
			pm.Get(pattern)
		}
	})
}

// TestPatternManagerThreadSafetyIntegration tests thread safety with Pattern operations
func TestPatternManagerThreadSafetyIntegration(t *testing.T) {
	pm := NewPatternManager(WithMaxSize(20))

	// Create patterns using Pattern struct
	patterns := make([]*Pattern, 10)
	for i := range patterns {
		patterns[i] = NewPattern(fmt.Sprintf("thread_test_%d.*", i), Include, "#ff0000")
	}

	// Test concurrent access with Pattern operations
	const numGoroutines = 5
	const numOps = 100

	done := make(chan bool, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			for i := 0; i < numOps; i++ {
				pattern := patterns[i%len(patterns)]

				// Mix of PatternManager and Pattern operations
				switch i % 4 {
				case 0:
					pm.Get(pattern.Expression)
				case 1:
					pattern.Compile() // Direct Pattern compilation
				case 2:
					pm.Contains(pattern.Expression)
				case 3:
					pm.Stats()
				}
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines
	for g := 0; g < numGoroutines; g++ {
		<-done
	}

	// Verify final state
	stats := pm.Stats()
	size := pm.Size()

	if size < 0 || size > pm.GetMaxSize() {
		t.Errorf("Invalid final cache size: %d (max: %d)", size, pm.GetMaxSize())
	}

	t.Logf("Thread safety test: %d goroutines, %d ops each, final size: %d, hits: %d, misses: %d",
		numGoroutines, numOps, size, stats.Hits, stats.Misses)
}
