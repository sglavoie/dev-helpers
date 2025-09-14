package core

import (
	"fmt"
	"log"
)

// ExamplePatternManager demonstrates basic usage of PatternManager
func ExamplePatternManager() {
	// Create a new pattern manager with custom settings
	pm := NewPatternManager(
		WithMaxSize(50),        // Cache up to 50 patterns
		WithStatsEnabled(true), // Enable statistics tracking
	)

	// Common log patterns
	patterns := []string{
		`ERROR.*`,
		`WARN.*`,
		`INFO.*`,
		`DEBUG.*`,
		`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}.*`,
	}

	// First access - cache misses
	fmt.Println("=== First Access (Cache Misses) ===")
	for _, pattern := range patterns {
		compiled, cached := pm.Get(pattern)
		if compiled != nil {
			fmt.Printf("Pattern: %s - Cached: %v\n", pattern, cached)
		}
	}

	// Second access - cache hits
	fmt.Println("\n=== Second Access (Cache Hits) ===")
	for _, pattern := range patterns {
		compiled, cached := pm.Get(pattern)
		if compiled != nil {
			fmt.Printf("Pattern: %s - Cached: %v\n", pattern, cached)
		}
	}

	// Show cache statistics
	stats := pm.Stats()
	fmt.Printf("\n=== Cache Statistics ===\n")
	fmt.Printf("Hits: %d\n", stats.Hits)
	fmt.Printf("Misses: %d\n", stats.Misses)
	fmt.Printf("Hit Rate: %.2f%%\n", stats.HitRate()*100)
	fmt.Printf("Current Size: %d\n", stats.Size)
	fmt.Printf("Max Size: %d\n", stats.MaxSize)

	// Output:
	// === First Access (Cache Misses) ===
	// Pattern: ERROR.* - Cached: false
	// Pattern: WARN.* - Cached: false
	// Pattern: INFO.* - Cached: false
	// Pattern: DEBUG.* - Cached: false
	// Pattern: \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}.* - Cached: false
	//
	// === Second Access (Cache Hits) ===
	// Pattern: ERROR.* - Cached: true
	// Pattern: WARN.* - Cached: true
	// Pattern: INFO.* - Cached: true
	// Pattern: DEBUG.* - Cached: true
	// Pattern: \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}.* - Cached: true
	//
	// === Cache Statistics ===
	// Hits: 5
	// Misses: 5
	// Hit Rate: 50.00%
	// Current Size: 5
	// Max Size: 50
}

// ExamplePatternManager_withFilterEngine demonstrates integration with filtering
func ExamplePatternManager_withFilterEngine() {
	pm := NewPatternManager(WithMaxSize(10))

	// Simulate log filtering workflow
	logLines := []string{
		"2024-01-01 10:00:00 INFO Application started",
		"2024-01-01 10:00:05 ERROR Database connection failed",
		"2024-01-01 10:00:10 WARN Memory usage high",
		"2024-01-01 10:00:15 DEBUG Processing request",
		"2024-01-01 10:00:20 ERROR Authentication failed",
	}

	// Define filter patterns
	includePatterns := []string{
		"ERROR.*",
		"WARN.*",
	}

	excludePatterns := []string{
		".*Authentication.*", // Exclude sensitive auth errors
	}

	fmt.Println("=== Log Filtering Example ===")

	// Compile patterns using PatternManager
	var compiledIncludes, compiledExcludes []interface {
		MatchString(string) bool
	}

	for _, pattern := range includePatterns {
		if compiled, _ := pm.Get(pattern); compiled != nil {
			compiledIncludes = append(compiledIncludes, compiled)
		}
	}

	for _, pattern := range excludePatterns {
		if compiled, _ := pm.Get(pattern); compiled != nil {
			compiledExcludes = append(compiledExcludes, compiled)
		}
	}

	// Apply filtering logic
	for _, line := range logLines {
		// Check include patterns (OR logic)
		includeMatch := len(compiledIncludes) == 0 // If no includes, include all
		for _, regex := range compiledIncludes {
			if regex.MatchString(line) {
				includeMatch = true
				break
			}
		}

		// Check exclude patterns (veto logic)
		excludeMatch := false
		for _, regex := range compiledExcludes {
			if regex.MatchString(line) {
				excludeMatch = true
				break
			}
		}

		// Apply filter logic
		if includeMatch && !excludeMatch {
			fmt.Printf("✓ %s\n", line)
		} else {
			fmt.Printf("✗ %s\n", line)
		}
	}

	// Show final cache stats
	stats := pm.Stats()
	fmt.Printf("\n=== Final Cache Stats ===\n")
	fmt.Printf("Patterns Cached: %d\n", stats.Size)
	fmt.Printf("Cache Efficiency: %.1f%% hit rate\n", stats.HitRate()*100)

	// Output:
	// === Log Filtering Example ===
	// ✗ 2024-01-01 10:00:00 INFO Application started
	// ✓ 2024-01-01 10:00:05 ERROR Database connection failed
	// ✓ 2024-01-01 10:00:10 WARN Memory usage high
	// ✗ 2024-01-01 10:00:15 DEBUG Processing request
	// ✗ 2024-01-01 10:00:20 ERROR Authentication failed
	//
	// === Final Cache Stats ===
	// Patterns Cached: 3
	// Cache Efficiency: 0.0% hit rate
}

// ExamplePatternManager_lruBehavior demonstrates LRU eviction behavior
func ExamplePatternManager_lruBehavior() {
	// Create small cache to demonstrate LRU eviction
	pm := NewPatternManager(WithMaxSize(3))

	patterns := []string{
		"pattern1.*",
		"pattern2.*",
		"pattern3.*",
		"pattern4.*", // This will evict pattern1 (least recently used)
	}

	fmt.Println("=== LRU Eviction Demonstration ===")

	// Add patterns sequentially
	for i, pattern := range patterns {
		pm.Get(pattern)
		fmt.Printf("Added pattern%d, cache size: %d\n", i+1, pm.Size())

		// Show current cached patterns
		cached := pm.GetPatterns()
		fmt.Printf("  Cached patterns: %v\n", cached)
	}

	// Access pattern2 to make it most recently used
	fmt.Println("\nAccessing pattern2.* to move it to front...")
	pm.Get("pattern2.*")

	// Add another pattern - should evict pattern3 now (least recently used)
	fmt.Println("Adding pattern5.* (should evict pattern3.*)")
	pm.Get("pattern5.*")

	cached := pm.GetPatterns()
	fmt.Printf("Final cached patterns: %v\n", cached)
	fmt.Printf("Final cache size: %d (max: %d)\n", pm.Size(), pm.GetMaxSize())

	// Output:
	// === LRU Eviction Demonstration ===
	// Added pattern1, cache size: 1
	//   Cached patterns: [pattern1.*]
	// Added pattern2, cache size: 2
	//   Cached patterns: [pattern2.* pattern1.*]
	// Added pattern3, cache size: 3
	//   Cached patterns: [pattern3.* pattern2.* pattern1.*]
	// Added pattern4, cache size: 3
	//   Cached patterns: [pattern4.* pattern3.* pattern2.*]
	//
	// Accessing pattern2.* to move it to front...
	// Adding pattern5.* (should evict pattern3.*)
	// Final cached patterns: [pattern5.* pattern2.* pattern4.*]
	// Final cache size: 3 (max: 3)
}

// ExamplePatternManager_errorHandling demonstrates error handling and validation
func ExamplePatternManager_errorHandling() {
	pm := NewPatternManager()

	fmt.Println("=== Error Handling Examples ===")

	// Test invalid patterns
	invalidPatterns := []string{
		"[unclosed bracket",
		"*invalid quantifier",
		"(?invalid group",
	}

	for _, pattern := range invalidPatterns {
		compiled, cached := pm.Get(pattern)
		if compiled == nil {
			fmt.Printf("❌ Invalid pattern: %s\n", pattern)
		} else {
			fmt.Printf("✅ Valid pattern: %s (cached: %v)\n", pattern, cached)
		}
	}

	// Test empty pattern
	compiled, _ := pm.Get("")
	if compiled == nil {
		fmt.Println("❌ Empty patterns are rejected")
	}

	// Test valid patterns after errors
	fmt.Println("\nTesting valid patterns:")
	validPatterns := []string{
		"simple",
		"test.*pattern",
		"^start.*end$",
	}

	for _, pattern := range validPatterns {
		compiled, cached := pm.Get(pattern)
		if compiled != nil {
			fmt.Printf("✅ Valid pattern: %s (cached: %v)\n", pattern, cached)
		} else {
			fmt.Printf("❌ Unexpected failure: %s\n", pattern)
		}
	}

	// Show that invalid patterns don't affect cache
	stats := pm.Stats()
	fmt.Printf("\nCache contains %d valid patterns\n", stats.Size)
	fmt.Printf("Total misses (including invalid patterns): %d\n", stats.Misses)

	// Output:
	// === Error Handling Examples ===
	// ❌ Invalid pattern: [unclosed bracket
	// ❌ Invalid pattern: *invalid quantifier
	// ❌ Invalid pattern: (?invalid group
	// ❌ Empty patterns are rejected
	//
	// Testing valid patterns:
	// ✅ Valid pattern: simple (cached: false)
	// ✅ Valid pattern: test.*pattern (cached: false)
	// ✅ Valid pattern: ^start.*end$ (cached: false)
	//
	// Cache contains 3 valid patterns
	// Total misses (including invalid patterns): 6
}

func init() {
	// Suppress log output during examples
	log.SetOutput(&discardWriter{})
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
