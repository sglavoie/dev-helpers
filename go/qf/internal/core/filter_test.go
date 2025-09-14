package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Test that our implementation satisfies the FilterEngine interface
var _ FilterEngine = (*filterEngineImpl)(nil)

// TestFilterEngineImplementation runs the contract tests against our implementation
func TestFilterEngineImplementation(t *testing.T) {
	engine := NewFilterEngine()
	runFilterEngineContractTests(t, engine)
}

// TestFilterEngineEdgeCases runs edge case tests against our implementation
func TestFilterEngineEdgeCases(t *testing.T) {
	engine := NewFilterEngine()

	t.Run("Empty Content", func(t *testing.T) {
		engine.ClearPatterns()
		result, err := engine.ApplyFilters(context.Background(), []string{})
		if err != nil {
			t.Fatalf("Failed to handle empty content: %v", err)
		}
		if len(result.MatchedLines) != 0 {
			t.Error("Expected no matches for empty content")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		engine.ClearPatterns()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		lines := make([]string, 1000)
		for i := range lines {
			lines[i] = "test line"
		}

		_, err := engine.ApplyFilters(ctx, lines)
		if err == nil {
			t.Error("Expected error when context is cancelled")
		}
	})

	t.Run("Duplicate Pattern IDs", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := FilterPattern{
			ID:         "duplicate",
			Expression: "test",
			Type:       FilterInclude,
			IsValid:    true,
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("Failed to add first pattern: %v", err)
		}

		// Try to add same ID again
		err = engine.AddPattern(pattern)
		if err == nil {
			t.Error("Expected error when adding duplicate pattern ID")
		}
	})
}

// Duplicate the contract test functions here to test our implementation

func runFilterEngineContractTests(t *testing.T, engine FilterEngine) {
	t.Run("Pattern Management", func(t *testing.T) {
		testPatternManagement(t, engine)
	})

	t.Run("Include Patterns OR Logic", func(t *testing.T) {
		testIncludePatternsORLogic(t, engine)
	})

	t.Run("Exclude Patterns Veto Logic", func(t *testing.T) {
		testExcludePatternsVetoLogic(t, engine)
	})

	t.Run("Empty Includes Show All", func(t *testing.T) {
		testEmptyIncludesShowAll(t, engine)
	})

	t.Run("Pattern Compilation Caching", func(t *testing.T) {
		testPatternCompilationCaching(t, engine)
	})

	t.Run("Invalid Regex Validation", func(t *testing.T) {
		testInvalidRegexValidation(t, engine)
	})

	t.Run("Performance Requirements", func(t *testing.T) {
		testPerformanceRequirements(t, engine)
	})
}

func testPatternManagement(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Test adding patterns
	pattern1 := FilterPattern{
		ID:         "test-1",
		Expression: "error",
		Type:       FilterInclude,
		Color:      "red",
		Created:    time.Now(),
		IsValid:    true,
	}

	err := engine.AddPattern(pattern1)
	if err != nil {
		t.Fatalf("Failed to add valid pattern: %v", err)
	}

	// Test retrieving patterns
	patterns := engine.GetPatterns()
	if len(patterns) != 1 {
		t.Fatalf("Expected 1 pattern, got %d", len(patterns))
	}

	if patterns[0].ID != "test-1" {
		t.Errorf("Expected pattern ID 'test-1', got %s", patterns[0].ID)
	}

	// Test updating patterns
	updatedPattern := pattern1
	updatedPattern.Expression = "warning"
	err = engine.UpdatePattern("test-1", updatedPattern)
	if err != nil {
		t.Fatalf("Failed to update pattern: %v", err)
	}

	patterns = engine.GetPatterns()
	if patterns[0].Expression != "warning" {
		t.Errorf("Pattern was not updated correctly")
	}

	// Test removing patterns
	err = engine.RemovePattern("test-1")
	if err != nil {
		t.Fatalf("Failed to remove pattern: %v", err)
	}

	patterns = engine.GetPatterns()
	if len(patterns) != 0 {
		t.Errorf("Pattern was not removed correctly")
	}
}

func testIncludePatternsORLogic(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Add multiple include patterns
	patterns := []FilterPattern{
		{
			ID:         "include-1",
			Expression: "error",
			Type:       FilterInclude,
			IsValid:    true,
		},
		{
			ID:         "include-2",
			Expression: "warning",
			Type:       FilterInclude,
			IsValid:    true,
		},
	}

	for _, p := range patterns {
		if err := engine.AddPattern(p); err != nil {
			t.Fatalf("Failed to add pattern %s: %v", p.ID, err)
		}
	}

	// Test content that should match OR logic
	lines := []string{
		"This is an error message",  // Should match include-1
		"This is a warning message", // Should match include-2
		"This is an info message",   // Should not match any
		"Another error occurred",    // Should match include-1
		"Debug information",         // Should not match any
	}

	ctx := context.Background()
	result, err := engine.ApplyFilters(ctx, lines)
	if err != nil {
		t.Fatalf("Failed to apply filters: %v", err)
	}

	// Should match lines 0, 1, 3 (OR logic)
	expectedMatches := 3
	if len(result.MatchedLines) != expectedMatches {
		t.Errorf("Expected %d matches, got %d", expectedMatches, len(result.MatchedLines))
	}

	expectedLines := []string{
		"This is an error message",
		"This is a warning message",
		"Another error occurred",
	}

	for i, expectedLine := range expectedLines {
		if i >= len(result.MatchedLines) || !strings.Contains(result.MatchedLines[i], strings.Split(expectedLine, " ")[2]) {
			t.Errorf("Expected line containing '%s' not found in results", strings.Split(expectedLine, " ")[2])
		}
	}
}

func testExcludePatternsVetoLogic(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Add include and exclude patterns
	includePattern := FilterPattern{
		ID:         "include-all",
		Expression: ".*", // Match everything
		Type:       FilterInclude,
		IsValid:    true,
	}

	excludePattern := FilterPattern{
		ID:         "exclude-debug",
		Expression: "(?i)debug", // Case-insensitive regex
		Type:       FilterExclude,
		IsValid:    true,
	}

	if err := engine.AddPattern(includePattern); err != nil {
		t.Fatalf("Failed to add include pattern: %v", err)
	}

	if err := engine.AddPattern(excludePattern); err != nil {
		t.Fatalf("Failed to add exclude pattern: %v", err)
	}

	lines := []string{
		"This is an error message",
		"This is a debug message", // Should be excluded (veto)
		"This is a warning message",
		"Debug information here", // Should be excluded (veto)
		"Normal log entry",
	}

	ctx := context.Background()
	result, err := engine.ApplyFilters(ctx, lines)
	if err != nil {
		t.Fatalf("Failed to apply filters: %v", err)
	}

	// Should match all lines except those with "debug" (veto logic)
	expectedMatches := 3
	if len(result.MatchedLines) != expectedMatches {
		t.Errorf("Expected %d matches, got %d", expectedMatches, len(result.MatchedLines))
	}

	// Verify debug lines were excluded
	for _, line := range result.MatchedLines {
		if strings.Contains(strings.ToLower(line), "debug") {
			t.Errorf("Debug line was not excluded: %s", line)
		}
	}
}

func testEmptyIncludesShowAll(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Add only exclude patterns (no includes)
	excludePattern := FilterPattern{
		ID:         "exclude-test",
		Expression: "test",
		Type:       FilterExclude,
		IsValid:    true,
	}

	if err := engine.AddPattern(excludePattern); err != nil {
		t.Fatalf("Failed to add exclude pattern: %v", err)
	}

	lines := []string{
		"This is an error message",
		"This is a test message", // Should be excluded
		"This is a warning message",
		"Another test line", // Should be excluded
		"Normal log entry",
	}

	ctx := context.Background()
	result, err := engine.ApplyFilters(ctx, lines)
	if err != nil {
		t.Fatalf("Failed to apply filters: %v", err)
	}

	// Empty includes = show all (minus excludes)
	expectedMatches := 3 // All lines except those with "test"
	if len(result.MatchedLines) != expectedMatches {
		t.Errorf("Expected %d matches, got %d", expectedMatches, len(result.MatchedLines))
	}

	// Verify test lines were excluded
	for _, line := range result.MatchedLines {
		if strings.Contains(strings.ToLower(line), "test") {
			t.Errorf("Test line was not excluded: %s", line)
		}
	}
}

func testPatternCompilationCaching(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Get initial cache stats
	initialHits, initialMisses, _ := engine.GetCacheStats()

	// Add a pattern (should cause cache miss)
	pattern := FilterPattern{
		ID:         "cache-test",
		Expression: "error.*message",
		Type:       FilterInclude,
		IsValid:    true,
	}

	if err := engine.AddPattern(pattern); err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	lines := []string{"error in message", "warning message", "error message here"}
	ctx := context.Background()

	// First application (should use cached compilation)
	_, err := engine.ApplyFilters(ctx, lines)
	if err != nil {
		t.Fatalf("Failed to apply filters: %v", err)
	}

	// Second application (should use cached compilation)
	_, err = engine.ApplyFilters(ctx, lines)
	if err != nil {
		t.Fatalf("Failed to apply filters on second run: %v", err)
	}

	// Check cache stats improved
	hits, misses, size := engine.GetCacheStats()

	if misses <= initialMisses {
		t.Error("Expected cache misses to increase after adding pattern")
	}

	if hits <= initialHits {
		t.Error("Expected cache hits to increase after repeated pattern usage")
	}

	if size <= 0 {
		t.Error("Expected cache to have entries")
	}
}

func testInvalidRegexValidation(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Test invalid regex patterns
	invalidPatterns := []string{
		"[unclosed",    // Unclosed bracket
		"(unclosed",    // Unclosed parenthesis
		"*invalid",     // Invalid quantifier
		"?invalid",     // Invalid quantifier
		"+invalid",     // Invalid quantifier
		"\\k<invalid>", // Invalid escape
	}

	for _, invalid := range invalidPatterns {
		// Test validation without adding
		err := engine.ValidatePattern(invalid)
		if err == nil {
			t.Errorf("Expected validation error for invalid pattern: %s", invalid)
		}

		// Test adding invalid pattern
		pattern := FilterPattern{
			ID:         "invalid-test",
			Expression: invalid,
			Type:       FilterInclude,
			IsValid:    false,
		}

		err = engine.AddPattern(pattern)
		if err == nil {
			t.Errorf("Expected ValidationError when adding invalid pattern: %s", invalid)
		} else if !strings.Contains(err.Error(), "validation failed") {
			t.Errorf("Expected ValidationError, got: %T %v", err, err)
		}
	}
}

func testPerformanceRequirements(t *testing.T, engine FilterEngine) {
	engine.ClearPatterns()

	// Add some realistic patterns
	patterns := []FilterPattern{
		{ID: "perf-1", Expression: "ERROR", Type: FilterInclude, IsValid: true},
		{ID: "perf-2", Expression: "WARNING", Type: FilterInclude, IsValid: true},
		{ID: "perf-3", Expression: "DEBUG", Type: FilterExclude, IsValid: true},
		{ID: "perf-4", Expression: "TRACE", Type: FilterExclude, IsValid: true},
	}

	for _, p := range patterns {
		if err := engine.AddPattern(p); err != nil {
			t.Fatalf("Failed to add pattern %s: %v", p.ID, err)
		}
	}

	// Generate 10K test lines
	lines := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		switch i % 5 {
		case 0:
			lines[i] = fmt.Sprintf("ERROR: Something went wrong at line %d", i)
		case 1:
			lines[i] = fmt.Sprintf("WARNING: Potential issue at line %d", i)
		case 2:
			lines[i] = fmt.Sprintf("INFO: Processing item %d", i)
		case 3:
			lines[i] = fmt.Sprintf("DEBUG: Debugging info for %d", i)
		case 4:
			lines[i] = fmt.Sprintf("TRACE: Trace information %d", i)
		}
	}

	ctx := context.Background()
	start := time.Now()

	result, err := engine.ApplyFilters(ctx, lines)

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to apply filters: %v", err)
	}

	// Performance requirement: <150ms for 10K lines
	maxDuration := 150 * time.Millisecond
	if elapsed > maxDuration {
		t.Errorf("Performance requirement failed: took %v, expected <%v", elapsed, maxDuration)
	}

	// Verify result quality
	if len(result.MatchedLines) == 0 {
		t.Error("Expected some matches in performance test")
	}

	// Verify stats are populated
	if result.Stats.TotalLines != 10000 {
		t.Errorf("Expected TotalLines=10000, got %d", result.Stats.TotalLines)
	}

	if result.Stats.ProcessingTime == 0 {
		t.Error("Expected ProcessingTime to be recorded in stats")
	}

	t.Logf("Performance test completed: %v for %d lines (%d matches)",
		elapsed, len(lines), len(result.MatchedLines))
}
