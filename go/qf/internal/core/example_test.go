package core

import (
	"context"
	"fmt"
	"log"
)

// ExampleFilterEngine demonstrates basic usage of the FilterEngine
func ExampleFilterEngine() {
	// Create a new filter engine
	engine := NewFilterEngine()

	// Add some patterns
	includePattern := FilterPattern{
		ID:         "errors",
		Expression: "(?i)(error|fail)",
		Type:       FilterInclude,
		Color:      "#ff0000",
		IsValid:    true,
	}

	excludePattern := FilterPattern{
		ID:         "debug",
		Expression: "(?i)debug",
		Type:       FilterExclude,
		Color:      "#888888",
		IsValid:    true,
	}

	err := engine.AddPattern(includePattern)
	if err != nil {
		log.Fatalf("Failed to add include pattern: %v", err)
	}

	err = engine.AddPattern(excludePattern)
	if err != nil {
		log.Fatalf("Failed to add exclude pattern: %v", err)
	}

	// Sample log lines
	lines := []string{
		"INFO: Application started successfully",
		"ERROR: Database connection failed",
		"DEBUG: This debug message should be excluded",
		"WARN: High memory usage detected",
		"ERROR: Authentication failed for user",
		"INFO: Processing complete",
		"DEBUG: Another debug error message", // Contains both error and debug
	}

	// Apply filters
	ctx := context.Background()
	result, err := engine.ApplyFilters(ctx, lines)
	if err != nil {
		log.Fatalf("Failed to apply filters: %v", err)
	}

	// Print results
	fmt.Printf("Total lines processed: %d\n", result.Stats.TotalLines)
	fmt.Printf("Lines matched: %d\n", result.Stats.MatchedLines)
	fmt.Printf("Cache hits: %d, misses: %d\n", result.Stats.CacheHits, result.Stats.CacheMisses)
	fmt.Println("\nMatched lines:")
	for i, line := range result.MatchedLines {
		fmt.Printf("  [%d] %s\n", result.LineNumbers[i], line)

		// Show highlights if any
		if highlights, exists := result.MatchHighlights[result.LineNumbers[i]]; exists {
			for _, highlight := range highlights {
				fmt.Printf("    ^-- Match at position %d-%d (pattern: %s)\n",
					highlight.Start, highlight.End, highlight.PatternID)
			}
		}
	}

	// Output:
	// Total lines processed: 7
	// Lines matched: 2
	// Cache hits: 0, misses: 2
	//
	// Matched lines:
	//   [1] ERROR: Database connection failed
	//     ^-- Match at position 0-5 (pattern: errors)
	//     ^-- Match at position 27-31 (pattern: errors)
	//   [4] ERROR: Authentication failed for user
	//     ^-- Match at position 0-5 (pattern: errors)
	//     ^-- Match at position 22-26 (pattern: errors)
}

// ExampleFilterEngine_configuration shows how to configure the FilterEngine
func ExampleFilterEngine_configuration() {
	// Create engine with custom configuration
	engine := NewFilterEngine(
		WithDebounceDelay(100), // 100ms debounce
		WithCacheSize(500),     // Cache up to 500 patterns
		WithMaxWorkers(8),      // Use up to 8 worker goroutines
	)

	// Add a pattern
	pattern := FilterPattern{
		ID:         "info",
		Expression: "INFO:",
		Type:       FilterInclude,
		IsValid:    true,
	}

	err := engine.AddPattern(pattern)
	if err != nil {
		log.Fatalf("Failed to add pattern: %v", err)
	}

	// Validate a pattern before adding
	err = engine.ValidatePattern("(?i)warn.*ing")
	if err != nil {
		log.Printf("Invalid pattern: %v", err)
	} else {
		fmt.Println("Pattern is valid")
	}

	// Get cache statistics
	hits, misses, size := engine.GetCacheStats()
	fmt.Printf("Cache stats - hits: %d, misses: %d, size: %d\n", hits, misses, size)

	// Output:
	// Pattern is valid
	// Cache stats - hits: 0, misses: 0, size: 0
}

// ExampleFilterEngine_emptyIncludesShowAll demonstrates the "empty includes show all" behavior
func ExampleFilterEngine_emptyIncludesShowAll() {
	engine := NewFilterEngine()

	// Add only exclude patterns (no includes)
	excludePattern := FilterPattern{
		ID:         "exclude-spam",
		Expression: "(?i)spam",
		Type:       FilterExclude,
		IsValid:    true,
	}

	err := engine.AddPattern(excludePattern)
	if err != nil {
		log.Fatalf("Failed to add exclude pattern: %v", err)
	}

	lines := []string{
		"Normal log message",
		"SPAM: Unwanted message",
		"Important notification",
		"spam alert detected",
		"Regular application log",
	}

	ctx := context.Background()
	result, err := engine.ApplyFilters(ctx, lines)
	if err != nil {
		log.Fatalf("Failed to apply filters: %v", err)
	}

	fmt.Printf("With empty includes and one exclude, showing %d/%d lines:\n",
		len(result.MatchedLines), len(lines))

	for _, line := range result.MatchedLines {
		fmt.Printf("  %s\n", line)
	}

	// Output:
	// With empty includes and one exclude, showing 3/5 lines:
	//   Normal log message
	//   Important notification
	//   Regular application log
}
