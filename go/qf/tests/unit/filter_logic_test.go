// Package unit contains comprehensive unit tests for the filtering logic components.
//
// This file tests the core filtering engine functionality with extensive coverage
// of all filtering scenarios, performance requirements, and edge cases.
package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// TestFilterEngineInterfaceCompliance verifies the implementation satisfies the interface
func TestFilterEngineInterfaceCompliance(t *testing.T) {
	engine := core.NewFilterEngine()
	if engine == nil {
		t.Fatal("NewFilterEngine should return a non-nil instance")
	}

	// Verify interface compliance by attempting to use all methods
	_ = engine.GetPatterns()
	_, _, _ = engine.GetCacheStats()
}

// TestNewFilterEngine tests the constructor with various configurations
func TestNewFilterEngine(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		engine := core.NewFilterEngine()

		// Should be usable immediately
		patterns := engine.GetPatterns()
		if len(patterns) != 0 {
			t.Errorf("New engine should have 0 patterns, got %d", len(patterns))
		}

		// Cache stats should be available
		hits, misses, size := engine.GetCacheStats()
		if hits != 0 || misses != 0 || size != 0 {
			t.Errorf("New engine cache should be empty, got hits=%d, misses=%d, size=%d",
				hits, misses, size)
		}
	})

	t.Run("Custom Configuration Options", func(t *testing.T) {
		engine := core.NewFilterEngine(
			core.WithDebounceDelay(100*time.Millisecond),
			core.WithCacheSize(500),
			core.WithMaxWorkers(8),
		)

		// Engine should still be functional
		if engine == nil {
			t.Fatal("Engine with custom options should not be nil")
		}

		// Should accept patterns normally
		pattern := core.FilterPattern{
			ID:         "test-config",
			Expression: "test",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("Engine with custom config should accept patterns: %v", err)
		}
	})
}

// TestPatternManagement tests all pattern management operations
func TestPatternManagement(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("AddPattern Success", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "add-test-1",
			Expression: "error.*message",
			Type:       core.FilterInclude,
			MatchCount: 0,
			Color:      "#FF0000",
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed for valid pattern: %v", err)
		}

		// Verify pattern was added
		patterns := engine.GetPatterns()
		if len(patterns) != 1 {
			t.Fatalf("Expected 1 pattern after adding, got %d", len(patterns))
		}

		// Verify pattern properties
		added := patterns[0]
		if added.ID != pattern.ID {
			t.Errorf("Expected ID %s, got %s", pattern.ID, added.ID)
		}
		if added.Expression != pattern.Expression {
			t.Errorf("Expected expression %s, got %s", pattern.Expression, added.Expression)
		}
		if added.Type != pattern.Type {
			t.Errorf("Expected type %v, got %v", pattern.Type, added.Type)
		}
		if !added.IsValid {
			t.Error("Added pattern should be marked as valid")
		}
	})

	t.Run("AddPattern Duplicate ID", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "duplicate-test",
			Expression: "test1",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		// Add first pattern
		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("First AddPattern should succeed: %v", err)
		}

		// Try to add pattern with same ID
		pattern.Expression = "test2"
		err = engine.AddPattern(pattern)
		if err == nil {
			t.Error("AddPattern should fail for duplicate ID")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("Error should mention duplicate ID, got: %v", err)
		}

		// Verify only first pattern exists
		patterns := engine.GetPatterns()
		if len(patterns) != 1 {
			t.Fatalf("Expected 1 pattern, got %d", len(patterns))
		}
		if patterns[0].Expression != "test1" {
			t.Error("Original pattern should be preserved")
		}
	})

	t.Run("UpdatePattern Success", func(t *testing.T) {
		engine.ClearPatterns()

		// Add initial pattern
		original := core.FilterPattern{
			ID:         "update-test",
			Expression: "original",
			Type:       core.FilterInclude,
			Color:      "red",
			Created:    time.Now(),
		}

		err := engine.AddPattern(original)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Update the pattern
		updated := core.FilterPattern{
			ID:         "update-test",
			Expression: "updated.*pattern",
			Type:       core.FilterExclude,
			Color:      "blue",
			Created:    time.Now().Add(time.Hour),
		}

		err = engine.UpdatePattern("update-test", updated)
		if err != nil {
			t.Fatalf("UpdatePattern should succeed: %v", err)
		}

		// Verify update
		patterns := engine.GetPatterns()
		if len(patterns) != 1 {
			t.Fatalf("Expected 1 pattern after update, got %d", len(patterns))
		}

		result := patterns[0]
		if result.Expression != updated.Expression {
			t.Errorf("Expected expression %s, got %s", updated.Expression, result.Expression)
		}
		if result.Type != updated.Type {
			t.Errorf("Expected type %v, got %v", updated.Type, result.Type)
		}
		if result.Color != updated.Color {
			t.Errorf("Expected color %s, got %s", updated.Color, result.Color)
		}
		if !result.IsValid {
			t.Error("Updated pattern should be marked as valid")
		}
	})

	t.Run("UpdatePattern Non-existent", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "non-existent",
			Expression: "test",
			Type:       core.FilterInclude,
		}

		err := engine.UpdatePattern("non-existent", pattern)
		if err == nil {
			t.Error("UpdatePattern should fail for non-existent pattern")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention pattern not found, got: %v", err)
		}
	})

	t.Run("RemovePattern Success", func(t *testing.T) {
		engine.ClearPatterns()

		// Add pattern
		pattern := core.FilterPattern{
			ID:         "remove-test",
			Expression: "test",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Verify pattern exists
		patterns := engine.GetPatterns()
		if len(patterns) != 1 {
			t.Fatal("Pattern should exist before removal")
		}

		// Remove pattern
		err = engine.RemovePattern("remove-test")
		if err != nil {
			t.Fatalf("RemovePattern should succeed: %v", err)
		}

		// Verify pattern was removed
		patterns = engine.GetPatterns()
		if len(patterns) != 0 {
			t.Errorf("Expected 0 patterns after removal, got %d", len(patterns))
		}
	})

	t.Run("RemovePattern Non-existent", func(t *testing.T) {
		engine.ClearPatterns()

		err := engine.RemovePattern("non-existent")
		if err == nil {
			t.Error("RemovePattern should fail for non-existent pattern")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention pattern not found, got: %v", err)
		}
	})

	t.Run("ClearPatterns", func(t *testing.T) {
		engine.ClearPatterns()

		// Add multiple patterns
		for i := 0; i < 5; i++ {
			pattern := core.FilterPattern{
				ID:         fmt.Sprintf("clear-test-%d", i),
				Expression: fmt.Sprintf("test%d", i),
				Type:       core.FilterInclude,
				Created:    time.Now(),
			}

			err := engine.AddPattern(pattern)
			if err != nil {
				t.Fatalf("AddPattern %d should succeed: %v", i, err)
			}
		}

		// Verify patterns exist
		patterns := engine.GetPatterns()
		if len(patterns) != 5 {
			t.Fatalf("Expected 5 patterns before clear, got %d", len(patterns))
		}

		// Clear all patterns
		engine.ClearPatterns()

		// Verify all patterns removed
		patterns = engine.GetPatterns()
		if len(patterns) != 0 {
			t.Errorf("Expected 0 patterns after clear, got %d", len(patterns))
		}

		// Cache should also be cleared
		_, _, size := engine.GetCacheStats()
		if size != 0 {
			t.Errorf("Cache should be empty after clear, got size=%d", size)
		}
	})
}

// TestPatternValidation tests regex pattern validation
func TestPatternValidation(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("Valid Patterns", func(t *testing.T) {
		validPatterns := []string{
			"simple",
			"error.*message",
			"^start.*end$",
			"(?i)case-insensitive",
			"\\d{4}-\\d{2}-\\d{2}",
			"[A-Za-z]+",
			"(group1|group2)",
			"nested\\(parens\\)",
			"\\w+@\\w+\\.\\w+",
			".*", // Match all
		}

		for _, pattern := range validPatterns {
			err := engine.ValidatePattern(pattern)
			if err != nil {
				t.Errorf("Pattern %q should be valid, got error: %v", pattern, err)
			}
		}
	})

	t.Run("Invalid Patterns", func(t *testing.T) {
		invalidPatterns := []string{
			"",          // Empty pattern
			"[unclosed", // Unclosed bracket
			"(unclosed", // Unclosed parenthesis
			"*invalid",  // Invalid quantifier
			"?invalid",  // Invalid quantifier
			"+invalid",  // Invalid quantifier
			"\\k<name>", // Invalid escape sequence
			"(?P<>)",    // Invalid named group
			"[z-a]",     // Invalid character range
		}

		for _, pattern := range invalidPatterns {
			err := engine.ValidatePattern(pattern)
			if err == nil {
				t.Errorf("Pattern %q should be invalid", pattern)
			}
		}
	})

	t.Run("AddPattern Validation", func(t *testing.T) {
		engine.ClearPatterns()

		// Try to add invalid pattern
		invalidPattern := core.FilterPattern{
			ID:         "invalid-test",
			Expression: "[unclosed",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(invalidPattern)
		if err == nil {
			t.Error("AddPattern should fail for invalid regex")
		}

		// Should be ValidationError
		if !strings.Contains(err.Error(), "validation failed") {
			t.Errorf("Expected ValidationError, got: %v", err)
		}

		// Pattern should not be added
		patterns := engine.GetPatterns()
		if len(patterns) != 0 {
			t.Error("Invalid pattern should not be added")
		}
	})
}

// TestIncludePatternORLogic tests the OR logic for include patterns
func TestIncludePatternORLogic(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("Single Include Pattern", func(t *testing.T) {
		engine.ClearPatterns()

		// Add single include pattern
		pattern := core.FilterPattern{
			ID:         "include-single",
			Expression: "error",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{
			"This is an error message",   // Should match
			"This is a warning message",  // Should not match
			"Another error occurred",     // Should match
			"Info: processing completed", // Should not match
			"Fatal error in system",      // Should match
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should match lines containing "error"
		expectedCount := 3
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify correct lines matched
		expectedContent := []string{
			"This is an error message",
			"Another error occurred",
			"Fatal error in system",
		}

		for i, expected := range expectedContent {
			if i >= len(result.MatchedLines) {
				t.Errorf("Missing expected match: %s", expected)
				continue
			}
			if result.MatchedLines[i] != expected {
				t.Errorf("Expected match %d: %s, got: %s", i, expected, result.MatchedLines[i])
			}
		}
	})

	t.Run("Multiple Include Patterns OR Logic", func(t *testing.T) {
		engine.ClearPatterns()

		// Add multiple include patterns
		patterns := []core.FilterPattern{
			{
				ID:         "include-error",
				Expression: "(?i)error",
				Type:       core.FilterInclude,
				Color:      "red",
				Created:    time.Now(),
			},
			{
				ID:         "include-warning",
				Expression: "(?i)warning",
				Type:       core.FilterInclude,
				Color:      "yellow",
				Created:    time.Now(),
			},
			{
				ID:         "include-fatal",
				Expression: "(?i)fatal",
				Type:       core.FilterInclude,
				Color:      "red",
				Created:    time.Now(),
			},
		}

		for _, p := range patterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		lines := []string{
			"This is an error message",  // Should match (error)
			"This is a warning message", // Should match (warning)
			"This is an info message",   // Should not match
			"Fatal system failure",      // Should match (fatal)
			"Debug information",         // Should not match
			"Another error occurred",    // Should match (error)
			"Warning: low disk space",   // Should match (warning)
			"Normal operation",          // Should not match
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should match lines containing error, warning, or fatal (OR logic)
		expectedCount := 5
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify OR logic: any line matching any pattern should be included
		matchedLines := map[string]bool{}
		for _, line := range result.MatchedLines {
			matchedLines[line] = true
		}

		// Lines that should be matched
		expectedMatches := []string{
			"This is an error message",
			"This is a warning message",
			"Fatal system failure",
			"Another error occurred",
			"Warning: low disk space",
		}

		for _, expected := range expectedMatches {
			if !matchedLines[expected] {
				t.Errorf("Expected line to be matched: %s", expected)
			}
		}

		// Lines that should not be matched
		expectedNonMatches := []string{
			"This is an info message",
			"Debug information",
			"Normal operation",
		}

		for _, notExpected := range expectedNonMatches {
			if matchedLines[notExpected] {
				t.Errorf("Line should not be matched: %s", notExpected)
			}
		}
	})

	t.Run("Complex Regex Include Patterns", func(t *testing.T) {
		engine.ClearPatterns()

		// Add complex regex patterns
		patterns := []core.FilterPattern{
			{
				ID:         "timestamp-error",
				Expression: "\\d{4}-\\d{2}-\\d{2}.*error",
				Type:       core.FilterInclude,
				Created:    time.Now(),
			},
			{
				ID:         "case-insensitive-warning",
				Expression: "(?i)warning",
				Type:       core.FilterInclude,
				Created:    time.Now(),
			},
		}

		for _, p := range patterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		lines := []string{
			"2023-01-01 12:00:00 error occurred", // Should match (timestamp + error)
			"2023-01-01 12:00:00 info logged",    // Should not match
			"WARNING: system overload",           // Should match (case-insensitive warning)
			"warning: low memory",                // Should match (case-insensitive warning)
			"Error without timestamp",            // Should not match
			"2023-01-01 warning detected",        // Should match (both patterns)
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		expectedCount := 4
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}
	})
}

// TestExcludePatternVetoLogic tests the veto logic for exclude patterns
func TestExcludePatternVetoLogic(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("Exclude Only Patterns", func(t *testing.T) {
		engine.ClearPatterns()

		// Add exclude pattern only
		pattern := core.FilterPattern{
			ID:         "exclude-debug",
			Expression: "(?i)debug",
			Type:       core.FilterExclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{
			"This is an error message",  // Should be included
			"This is a debug message",   // Should be excluded
			"This is a warning message", // Should be included
			"DEBUG: processing item",    // Should be excluded (case insensitive)
			"Normal log entry",          // Should be included
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should include all lines except debug lines
		expectedCount := 3
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify no debug lines are included
		for _, line := range result.MatchedLines {
			if strings.Contains(strings.ToLower(line), "debug") {
				t.Errorf("Debug line should be excluded: %s", line)
			}
		}
	})

	t.Run("Include and Exclude Combination", func(t *testing.T) {
		engine.ClearPatterns()

		// Add include pattern (match all)
		includePattern := core.FilterPattern{
			ID:         "include-all",
			Expression: ".*",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		// Add exclude patterns
		excludePatterns := []core.FilterPattern{
			{
				ID:         "exclude-debug",
				Expression: "(?i)debug",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
			{
				ID:         "exclude-trace",
				Expression: "(?i)trace",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
		}

		err := engine.AddPattern(includePattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed for include: %v", err)
		}

		for _, p := range excludePatterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for exclude %s: %v", p.ID, err)
			}
		}

		lines := []string{
			"This is an error message",  // Should match
			"This is a debug message",   // Should be excluded (debug)
			"This is a warning message", // Should match
			"TRACE: method entry",       // Should be excluded (trace)
			"Normal log entry",          // Should match
			"DEBUG and TRACE together",  // Should be excluded (debug)
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should match all except debug and trace lines
		expectedCount := 3
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify excluded patterns are not present
		for _, line := range result.MatchedLines {
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "debug") || strings.Contains(lowerLine, "trace") {
				t.Errorf("Excluded line should not be present: %s", line)
			}
		}
	})

	t.Run("Veto Logic Priority", func(t *testing.T) {
		engine.ClearPatterns()

		// Add include and exclude patterns that could conflict
		includePattern := core.FilterPattern{
			ID:         "include-error",
			Expression: "error",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		excludePattern := core.FilterPattern{
			ID:         "exclude-debug-error",
			Expression: "debug.*error",
			Type:       core.FilterExclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(includePattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed for include: %v", err)
		}

		err = engine.AddPattern(excludePattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed for exclude: %v", err)
		}

		lines := []string{
			"This is an error message",      // Should match (include, no exclude)
			"debug error occurred",          // Should be excluded (veto takes priority)
			"Another error happened",        // Should match (include, no exclude)
			"debug info, then error logged", // Should be excluded (veto takes priority)
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Exclude patterns should take priority (veto logic)
		expectedCount := 2
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify veto logic worked
		for _, line := range result.MatchedLines {
			if strings.Contains(strings.ToLower(line), "debug") {
				t.Errorf("Excluded line should not be present due to veto logic: %s", line)
			}
		}
	})

	t.Run("Multiple Exclude Patterns", func(t *testing.T) {
		engine.ClearPatterns()

		// Multiple exclude patterns should all apply
		excludePatterns := []core.FilterPattern{
			{
				ID:         "exclude-debug",
				Expression: "debug",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
			{
				ID:         "exclude-trace",
				Expression: "trace",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
			{
				ID:         "exclude-verbose",
				Expression: "verbose",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
		}

		for _, p := range excludePatterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		lines := []string{
			"info: processing started",      // Should be included
			"debug: entering function",      // Should be excluded
			"error: operation failed",       // Should be included
			"trace: variable value = 42",    // Should be excluded
			"warning: memory usage high",    // Should be included
			"verbose: detailed information", // Should be excluded
			"normal log message",            // Should be included
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		expectedCount := 4
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify all exclude patterns were applied
		for _, line := range result.MatchedLines {
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "debug") ||
				strings.Contains(lowerLine, "trace") ||
				strings.Contains(lowerLine, "verbose") {
				t.Errorf("Excluded line should not be present: %s", line)
			}
		}
	})
}

// TestEmptyIncludesBehavior tests the "show all minus excludes" behavior
func TestEmptyIncludesBehavior(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("No Patterns Show All", func(t *testing.T) {
		engine.ClearPatterns()

		lines := []string{
			"error message",
			"warning message",
			"info message",
			"debug message",
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should show all lines when no patterns
		if len(result.MatchedLines) != len(lines) {
			t.Errorf("Expected %d matches (all lines), got %d", len(lines), len(result.MatchedLines))
		}

		// Verify all lines are preserved
		for i, expected := range lines {
			if i >= len(result.MatchedLines) {
				t.Errorf("Missing line %d: %s", i, expected)
				continue
			}
			if result.MatchedLines[i] != expected {
				t.Errorf("Line %d: expected %s, got %s", i, expected, result.MatchedLines[i])
			}
		}
	})

	t.Run("Only Exclude Patterns", func(t *testing.T) {
		engine.ClearPatterns()

		// Add only exclude patterns (no includes)
		excludePatterns := []core.FilterPattern{
			{
				ID:         "exclude-debug",
				Expression: "debug",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
			{
				ID:         "exclude-trace",
				Expression: "trace",
				Type:       core.FilterExclude,
				Created:    time.Now(),
			},
		}

		for _, p := range excludePatterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		lines := []string{
			"error message",     // Should be included
			"warning message",   // Should be included
			"info message",      // Should be included
			"debug message",     // Should be excluded
			"trace information", // Should be excluded
			"normal log",        // Should be included
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should show all lines except excluded ones
		expectedCount := 4
		if len(result.MatchedLines) != expectedCount {
			t.Errorf("Expected %d matches, got %d", expectedCount, len(result.MatchedLines))
		}

		// Verify excluded lines are not present
		for _, line := range result.MatchedLines {
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "debug") || strings.Contains(lowerLine, "trace") {
				t.Errorf("Excluded line should not be present: %s", line)
			}
		}

		// Verify non-excluded lines are present
		expectedLines := []string{
			"error message",
			"warning message",
			"info message",
			"normal log",
		}

		matchedSet := make(map[string]bool)
		for _, line := range result.MatchedLines {
			matchedSet[line] = true
		}

		for _, expected := range expectedLines {
			if !matchedSet[expected] {
				t.Errorf("Expected line should be present: %s", expected)
			}
		}
	})

	t.Run("Empty Includes vs Non-Empty Includes", func(t *testing.T) {
		engine.ClearPatterns()

		lines := []string{
			"error occurred",
			"warning issued",
			"info logged",
			"debug traced",
		}

		// Test with exclude only (empty includes)
		excludePattern := core.FilterPattern{
			ID:         "exclude-debug",
			Expression: "debug",
			Type:       core.FilterExclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(excludePattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		ctx := context.Background()
		result1, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should show 3 lines (all except debug)
		if len(result1.MatchedLines) != 3 {
			t.Errorf("Empty includes should show 3 lines, got %d", len(result1.MatchedLines))
		}

		// Now add include pattern
		includePattern := core.FilterPattern{
			ID:         "include-error",
			Expression: "error",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err = engine.AddPattern(includePattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		result2, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should show only 1 line (error, but not debug due to exclude)
		if len(result2.MatchedLines) != 1 {
			t.Errorf("Non-empty includes should show 1 line, got %d", len(result2.MatchedLines))
		}

		if len(result2.MatchedLines) > 0 && !strings.Contains(result2.MatchedLines[0], "error") {
			t.Error("Should match error line when include pattern present")
		}
	})
}

// TestPatternCompilationCaching tests the caching mechanism
func TestPatternCompilationCaching(t *testing.T) {
	engine := core.NewFilterEngine(core.WithCacheSize(10))

	t.Run("Cache Miss Then Hit", func(t *testing.T) {
		engine.ClearPatterns()

		// Get initial cache stats
		_, _, initialSize := engine.GetCacheStats()
		if initialSize != 0 {
			t.Error("Initial cache should be empty")
		}

		pattern := core.FilterPattern{
			ID:         "cache-test-1",
			Expression: "error.*message",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{"error in message", "warning message", "error message here"}
		ctx := context.Background()

		// First application - should cause cache miss
		result1, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("First ApplyFilters should succeed: %v", err)
		}

		// Check cache stats after first use
		hits1, misses1, size1 := engine.GetCacheStats()
		if misses1 <= 0 {
			t.Error("Should have at least one cache miss after first use")
		}
		if size1 <= 0 {
			t.Error("Cache should have entries after first use")
		}

		// Second application - should use cached compilation
		result2, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("Second ApplyFilters should succeed: %v", err)
		}

		// Check cache stats after second use
		hits2, misses2, size2 := engine.GetCacheStats()
		if hits2 <= hits1 {
			t.Error("Should have more cache hits after second use")
		}
		if misses2 != misses1 {
			t.Error("Should not have additional misses on second use")
		}

		// Results should be identical
		if len(result1.MatchedLines) != len(result2.MatchedLines) {
			t.Error("Cached and non-cached results should be identical")
		}

		t.Logf("Cache stats progression: hits %d->%d, misses %d->%d, size %d->%d",
			hits1, hits2, misses1, misses2, size1, size2)
	})

	t.Run("Cache Eviction", func(t *testing.T) {
		engine.ClearPatterns()

		// Use small cache size to test eviction
		smallCacheEngine := core.NewFilterEngine(core.WithCacheSize(3))

		// Add more patterns than cache size
		patterns := []core.FilterPattern{
			{ID: "cache-1", Expression: "pattern1", Type: core.FilterInclude, Created: time.Now()},
			{ID: "cache-2", Expression: "pattern2", Type: core.FilterInclude, Created: time.Now()},
			{ID: "cache-3", Expression: "pattern3", Type: core.FilterInclude, Created: time.Now()},
			{ID: "cache-4", Expression: "pattern4", Type: core.FilterInclude, Created: time.Now()},
			{ID: "cache-5", Expression: "pattern5", Type: core.FilterInclude, Created: time.Now()},
		}

		for _, p := range patterns {
			err := smallCacheEngine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		lines := []string{"test pattern1 pattern2 pattern3 pattern4 pattern5"}
		ctx := context.Background()

		// Apply filters multiple times to trigger caching
		for i := 0; i < 3; i++ {
			_, err := smallCacheEngine.ApplyFilters(ctx, lines)
			if err != nil {
				t.Fatalf("ApplyFilters iteration %d should succeed: %v", i, err)
			}
		}

		// Cache should not exceed configured size
		_, _, size := smallCacheEngine.GetCacheStats()
		if size > 3 {
			t.Errorf("Cache size should not exceed limit of 3, got %d", size)
		}
	})

	t.Run("Pattern Update Clears Cache", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "cache-update-test",
			Expression: "original",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{"original text", "updated text"}
		ctx := context.Background()

		// Use pattern to cache it
		result1, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("First ApplyFilters should succeed: %v", err)
		}

		if len(result1.MatchedLines) != 1 || !strings.Contains(result1.MatchedLines[0], "original") {
			t.Error("Should match original pattern")
		}

		// Update pattern
		updatedPattern := pattern
		updatedPattern.Expression = "updated"

		err = engine.UpdatePattern("cache-update-test", updatedPattern)
		if err != nil {
			t.Fatalf("UpdatePattern should succeed: %v", err)
		}

		// Apply filters again - should use new pattern
		result2, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("Second ApplyFilters should succeed: %v", err)
		}

		if len(result2.MatchedLines) != 1 || !strings.Contains(result2.MatchedLines[0], "updated") {
			t.Error("Should match updated pattern")
		}

		// Results should be different, confirming cache was cleared
		if len(result1.MatchedLines) == len(result2.MatchedLines) && result1.MatchedLines[0] == result2.MatchedLines[0] {
			t.Error("Results should be different after pattern update")
		}
	})

	t.Run("Pattern Removal Clears Cache", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "cache-remove-test",
			Expression: "test",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{"test message", "other message"}
		ctx := context.Background()

		// Use pattern to cache it
		_, err = engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Get cache size before removal
		_, _, sizeBefore := engine.GetCacheStats()

		// Remove pattern
		err = engine.RemovePattern("cache-remove-test")
		if err != nil {
			t.Fatalf("RemovePattern should succeed: %v", err)
		}

		// Cache should be smaller or same (pattern entry removed)
		_, _, sizeAfter := engine.GetCacheStats()
		if sizeAfter > sizeBefore {
			t.Error("Cache size should not increase after pattern removal")
		}
	})
}

// TestPerformanceRequirements tests the <150ms for 10K lines requirement
func TestPerformanceRequirements(t *testing.T) {
	engine := core.NewFilterEngine(core.WithMaxWorkers(8))

	t.Run("10K Lines Performance", func(t *testing.T) {
		engine.ClearPatterns()

		// Add realistic patterns
		patterns := []core.FilterPattern{
			{ID: "perf-error", Expression: "ERROR", Type: core.FilterInclude, Created: time.Now()},
			{ID: "perf-warn", Expression: "WARN", Type: core.FilterInclude, Created: time.Now()},
			{ID: "perf-fatal", Expression: "FATAL", Type: core.FilterInclude, Created: time.Now()},
			{ID: "perf-debug", Expression: "DEBUG", Type: core.FilterExclude, Created: time.Now()},
			{ID: "perf-trace", Expression: "TRACE", Type: core.FilterExclude, Created: time.Now()},
		}

		for _, p := range patterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		// Generate 10K test lines
		lines := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			switch i % 10 {
			case 0, 1:
				lines[i] = fmt.Sprintf("ERROR: Something went wrong at line %d", i)
			case 2, 3:
				lines[i] = fmt.Sprintf("WARN: Potential issue at line %d", i)
			case 4:
				lines[i] = fmt.Sprintf("FATAL: Critical failure at line %d", i)
			case 5, 6:
				lines[i] = fmt.Sprintf("INFO: Processing item %d", i)
			case 7, 8:
				lines[i] = fmt.Sprintf("DEBUG: Debug info for %d", i)
			case 9:
				lines[i] = fmt.Sprintf("TRACE: Trace information %d", i)
			}
		}

		ctx := context.Background()
		start := time.Now()

		result, err := engine.ApplyFilters(ctx, lines)

		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Performance requirement: <150ms for 10K lines
		maxDuration := 150 * time.Millisecond
		if elapsed > maxDuration {
			t.Errorf("Performance requirement failed: took %v, expected <%v for %d lines",
				elapsed, maxDuration, len(lines))
		}

		// Verify result quality
		if len(result.MatchedLines) == 0 {
			t.Error("Expected some matches in performance test")
		}

		// Should match ERROR, WARN, FATAL (50% of lines) but exclude DEBUG, TRACE
		// Lines: 0,1=ERROR, 2,3=WARN, 4=FATAL, 5,6=INFO, 7,8=DEBUG, 9=TRACE
		// Include patterns match ERROR+WARN+FATAL = 50% of lines
		// Since we have include patterns, INFO is not included
		// DEBUG and TRACE are excluded regardless
		// So we expect exactly 50% to match (ERROR+WARN+FATAL only)
		expectedRangeMin := 4500 // Conservative estimate
		expectedRangeMax := 5500 // Conservative estimate
		actualMatches := len(result.MatchedLines)

		if actualMatches < expectedRangeMin || actualMatches > expectedRangeMax {
			t.Errorf("Expected matches in range %d-%d, got %d",
				expectedRangeMin, expectedRangeMax, actualMatches)
		}

		// Verify stats are populated
		if result.Stats.TotalLines != 10000 {
			t.Errorf("Expected TotalLines=10000, got %d", result.Stats.TotalLines)
		}

		if result.Stats.MatchedLines != len(result.MatchedLines) {
			t.Errorf("Stats.MatchedLines should equal actual matches")
		}

		if result.Stats.ProcessingTime == 0 {
			t.Error("Expected ProcessingTime to be recorded in stats")
		}

		if result.Stats.PatternsUsed != 5 {
			t.Errorf("Expected PatternsUsed=5, got %d", result.Stats.PatternsUsed)
		}

		t.Logf("Performance test completed: %v for %d lines (%d matches, %.1f%% matched)",
			elapsed, len(lines), len(result.MatchedLines),
			float64(len(result.MatchedLines))/float64(len(lines))*100)
	})

	t.Run("Scalability Test", func(t *testing.T) {
		engine.ClearPatterns()

		// Add moderate number of patterns
		for i := 0; i < 10; i++ {
			pattern := core.FilterPattern{
				ID:         fmt.Sprintf("scale-test-%d", i),
				Expression: fmt.Sprintf("pattern%d", i),
				Type:       core.FilterInclude,
				Created:    time.Now(),
			}

			err := engine.AddPattern(pattern)
			if err != nil {
				t.Fatalf("AddPattern should succeed for pattern %d: %v", i, err)
			}
		}

		// Test with different line counts to verify scalability
		lineCounts := []int{1000, 5000, 10000}

		for _, lineCount := range lineCounts {
			lines := make([]string, lineCount)
			for i := 0; i < lineCount; i++ {
				// Some lines match patterns, some don't
				if i%3 == 0 {
					lines[i] = fmt.Sprintf("Message with pattern%d", i%10)
				} else {
					lines[i] = fmt.Sprintf("Normal message %d", i)
				}
			}

			ctx := context.Background()
			start := time.Now()

			result, err := engine.ApplyFilters(ctx, lines)

			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("ApplyFilters should succeed for %d lines: %v", lineCount, err)
			}

			// Performance should scale roughly linearly
			expectedMaxDuration := time.Duration(float64(lineCount)/10000*150) * time.Millisecond
			if elapsed > expectedMaxDuration {
				t.Errorf("Performance degraded for %d lines: took %v, expected <%v",
					lineCount, elapsed, expectedMaxDuration)
			}

			t.Logf("Scalability test: %d lines processed in %v (%d matches)",
				lineCount, elapsed, len(result.MatchedLines))
		}
	})
}

// TestContextCancellation tests context cancellation scenarios
func TestContextCancellation(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("Immediate Cancellation", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "cancel-test",
			Expression: "test",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		lines := []string{"test line 1", "test line 2", "other line"}

		_, err = engine.ApplyFilters(ctx, lines)
		if err == nil {
			t.Error("Expected error when context is cancelled immediately")
		}

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	})

	t.Run("Timeout Cancellation", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "timeout-test",
			Expression: ".*", // Match everything to ensure processing happens
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
		defer cancel()

		// Create enough lines that processing might take longer than timeout
		lines := make([]string, 10000)
		for i := range lines {
			lines[i] = fmt.Sprintf("test line %d with lots of text to make processing slower", i)
		}

		_, err = engine.ApplyFilters(ctx, lines)

		// Should either complete quickly or timeout
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("Expected context.DeadlineExceeded, context.Canceled, or nil, got: %v", err)
		}

		// If it timed out, that's expected behavior
		if err == context.DeadlineExceeded {
			t.Log("Context timeout occurred as expected")
		}
	})

	t.Run("Cancellation During Processing", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "process-cancel-test",
			Expression: ".*",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Create context that we'll cancel during processing
		ctx, cancel := context.WithCancel(context.Background())

		// Create many lines to ensure processing takes some time
		lines := make([]string, 50000)
		for i := range lines {
			lines[i] = fmt.Sprintf("line %d", i)
		}

		// Start processing in goroutine
		errChan := make(chan error, 1)
		go func() {
			_, err := engine.ApplyFilters(ctx, lines)
			errChan <- err
		}()

		// Cancel after a short delay
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Wait for result
		select {
		case err := <-errChan:
			if err != nil && err != context.Canceled {
				t.Errorf("Expected context.Canceled or nil, got: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Processing did not complete within timeout")
		}
	})

	t.Run("Valid Context Success", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "valid-context-test",
			Expression: "match",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Create valid context with reasonable timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		lines := []string{
			"this should match",
			"this should not",
			"another match here",
		}

		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed with valid context: %v", err)
		}

		expectedMatches := 2
		if len(result.MatchedLines) != expectedMatches {
			t.Errorf("Expected %d matches, got %d", expectedMatches, len(result.MatchedLines))
		}
	})
}

// TestErrorHandling tests comprehensive error scenarios
func TestErrorHandling(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("ValidationError Structure", func(t *testing.T) {
		engine.ClearPatterns()

		invalidPattern := core.FilterPattern{
			ID:         "validation-error-test",
			Expression: "[unclosed",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(invalidPattern)
		if err == nil {
			t.Fatal("Expected ValidationError for invalid pattern")
		}

		// Check error message contains expected information
		errMsg := err.Error()
		if !strings.Contains(errMsg, "validation failed") {
			t.Errorf("Error should mention validation failure: %s", errMsg)
		}

		if !strings.Contains(errMsg, "validation-error-test") {
			t.Errorf("Error should mention pattern ID: %s", errMsg)
		}

		if !strings.Contains(errMsg, "[unclosed") {
			t.Errorf("Error should mention pattern expression: %s", errMsg)
		}
	})

	t.Run("Nil Context Handling", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "nil-context-test",
			Expression: "test",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{"test line"}

		// ApplyFilters with nil context should handle gracefully
		// Note: In Go, passing nil context is generally not recommended,
		// but the implementation should handle it gracefully
		defer func() {
			if r := recover(); r != nil {
				// It's acceptable for nil context to cause a panic in some implementations
				t.Logf("ApplyFilters panicked with nil context (this is acceptable): %v", r)
			}
		}()

		// This may panic or return error depending on implementation
		// The important thing is it doesn't crash unexpectedly
		_, err = engine.ApplyFilters(nil, lines)
		// We don't assert on the error since nil context behavior may vary
		if err != nil {
			t.Logf("ApplyFilters with nil context returned error (this is acceptable): %v", err)
		}
	})

	t.Run("Empty Pattern Expression", func(t *testing.T) {
		engine.ClearPatterns()

		emptyPattern := core.FilterPattern{
			ID:         "empty-pattern-test",
			Expression: "",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(emptyPattern)
		if err == nil {
			t.Error("Expected error for empty pattern expression")
		}

		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("Error should mention empty pattern: %v", err)
		}
	})

	t.Run("Invalid Pattern Type Handling", func(t *testing.T) {
		// This tests the FilterPatternType enum bounds
		engine.ClearPatterns()

		// Test string representation of pattern types
		includeType := core.FilterInclude
		excludeType := core.FilterExclude

		if includeType.String() != "Include" {
			t.Errorf("Expected Include string, got: %s", includeType.String())
		}

		if excludeType.String() != "Exclude" {
			t.Errorf("Expected Exclude string, got: %s", excludeType.String())
		}

		// Test invalid type (outside enum range)
		invalidType := core.FilterPatternType(999)
		if invalidType.String() != "Unknown" {
			t.Errorf("Expected Unknown string for invalid type, got: %s", invalidType.String())
		}
	})

	t.Run("Concurrent Access Safety", func(t *testing.T) {
		engine.ClearPatterns()

		// Test concurrent pattern additions
		const numGoroutines = 10
		const patternsPerGoroutine = 10

		errChan := make(chan error, numGoroutines*patternsPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < patternsPerGoroutine; j++ {
					pattern := core.FilterPattern{
						ID:         fmt.Sprintf("concurrent-%d-%d", goroutineID, j),
						Expression: fmt.Sprintf("pattern%d", j),
						Type:       core.FilterInclude,
						Created:    time.Now(),
					}

					err := engine.AddPattern(pattern)
					errChan <- err
				}
			}(i)
		}

		// Collect all errors
		for i := 0; i < numGoroutines*patternsPerGoroutine; i++ {
			err := <-errChan
			if err != nil {
				t.Errorf("Concurrent AddPattern failed: %v", err)
			}
		}

		// Verify all patterns were added
		patterns := engine.GetPatterns()
		expectedCount := numGoroutines * patternsPerGoroutine
		if len(patterns) != expectedCount {
			t.Errorf("Expected %d patterns after concurrent adds, got %d",
				expectedCount, len(patterns))
		}
	})

	t.Run("Large Pattern Count", func(t *testing.T) {
		engine.ClearPatterns()

		// Test with large number of patterns
		const largePatternCount = 1000

		// Add many patterns
		for i := 0; i < largePatternCount; i++ {
			pattern := core.FilterPattern{
				ID:         fmt.Sprintf("large-test-%d", i),
				Expression: fmt.Sprintf("pattern%d", i),
				Type:       core.FilterInclude,
				Created:    time.Now(),
			}

			err := engine.AddPattern(pattern)
			if err != nil {
				t.Fatalf("AddPattern %d should succeed: %v", i, err)
			}
		}

		// Verify all patterns were added
		patterns := engine.GetPatterns()
		if len(patterns) != largePatternCount {
			t.Errorf("Expected %d patterns, got %d", largePatternCount, len(patterns))
		}

		// Apply filters should still work
		lines := []string{"test pattern500", "no match", "pattern999 here"}
		ctx := context.Background()

		start := time.Now()
		result, err := engine.ApplyFilters(ctx, lines)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("ApplyFilters should succeed with many patterns: %v", err)
		}

		// Should match lines with patterns
		if len(result.MatchedLines) == 0 {
			t.Error("Expected some matches with large pattern count")
		}

		// Performance should still be reasonable
		if elapsed > 1*time.Second {
			t.Errorf("Performance degraded with large pattern count: %v", elapsed)
		}

		t.Logf("Large pattern count test: %d patterns, %v processing time",
			largePatternCount, elapsed)
	})
}

// TestEdgeCasesAndBoundaryConditions tests various edge cases
func TestEdgeCasesAndBoundaryConditions(t *testing.T) {
	engine := core.NewFilterEngine()

	t.Run("Empty Input Lines", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "empty-lines-test",
			Expression: "test",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Test with empty slice
		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, []string{})
		if err != nil {
			t.Fatalf("ApplyFilters should succeed with empty lines: %v", err)
		}

		if len(result.MatchedLines) != 0 {
			t.Errorf("Expected 0 matches for empty input, got %d", len(result.MatchedLines))
		}

		if result.Stats.TotalLines != 0 {
			t.Errorf("Expected TotalLines=0, got %d", result.Stats.TotalLines)
		}

		// Test with nil slice
		result, err = engine.ApplyFilters(ctx, nil)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed with nil lines: %v", err)
		}

		if len(result.MatchedLines) != 0 {
			t.Errorf("Expected 0 matches for nil input, got %d", len(result.MatchedLines))
		}
	})

	t.Run("Lines With Special Characters", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "special-chars-test",
			Expression: "\\$\\{.*\\}", // Match ${...}
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		lines := []string{
			"Normal line",
			"${VAR_NAME}",          // Should match
			"Text ${ANOTHER} text", // Should match
			"$VAR without braces",  // Should not match
			"${NESTED_${VAR}}",     // Should match
			"",                     // Empty line
			"\t\n\r",               // Whitespace line
			"Unicode: 你好世界",        // Unicode
			"Emoji: 🚀💻🌟",           // Emoji
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Should match lines with ${...} pattern
		expectedMatches := 3
		if len(result.MatchedLines) != expectedMatches {
			t.Errorf("Expected %d matches, got %d", expectedMatches, len(result.MatchedLines))
		}

		// Verify matches
		expectedLines := []string{"${VAR_NAME}", "Text ${ANOTHER} text", "${NESTED_${VAR}}"}
		for i, expected := range expectedLines {
			if i >= len(result.MatchedLines) || result.MatchedLines[i] != expected {
				t.Errorf("Expected match %d: %s, got: %s", i, expected,
					getStringOrEmpty(result.MatchedLines, i))
			}
		}
	})

	t.Run("Very Long Lines", func(t *testing.T) {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         "long-lines-test",
			Expression: "needle",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			t.Fatalf("AddPattern should succeed: %v", err)
		}

		// Create very long lines
		longLine1 := strings.Repeat("x", 100000) + "needle" + strings.Repeat("y", 100000)
		longLine2 := strings.Repeat("z", 200000) // No needle
		longLine3 := "needle" + strings.Repeat("a", 150000)

		lines := []string{longLine1, longLine2, longLine3}

		ctx := context.Background()
		start := time.Now()
		result, err := engine.ApplyFilters(ctx, lines)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("ApplyFilters should succeed with long lines: %v", err)
		}

		// Should match 2 lines (those with "needle")
		if len(result.MatchedLines) != 2 {
			t.Errorf("Expected 2 matches, got %d", len(result.MatchedLines))
		}

		// Performance should still be reasonable
		if elapsed > 5*time.Second {
			t.Errorf("Performance degraded with long lines: %v", elapsed)
		}

		t.Logf("Long lines test: processed %d characters in %v",
			len(longLine1)+len(longLine2)+len(longLine3), elapsed)
	})

	t.Run("Pattern With All Quantifiers", func(t *testing.T) {
		engine.ClearPatterns()

		// Test various regex quantifiers
		patterns := []struct {
			id          string
			expression  string
			testLine    string
			shouldMatch bool
		}{
			{"star", "a*b", "b", true},         // * quantifier
			{"plus", "a+b", "ab", true},        // + quantifier
			{"question", "a?b", "b", true},     // ? quantifier
			{"exact", "a{3}b", "aaab", true},   // {n} quantifier
			{"range", "a{2,4}b", "aaab", true}, // {n,m} quantifier
			{"min", "a{2,}b", "aaaab", true},   // {n,} quantifier
			{"greedy", "a.*b", "axxxb", true},  // Greedy matching
			{"lazy", "a.*?b", "axxxb", true},   // Lazy matching
		}

		for _, test := range patterns {
			engine.ClearPatterns()

			pattern := core.FilterPattern{
				ID:         test.id,
				Expression: test.expression,
				Type:       core.FilterInclude,
				Created:    time.Now(),
			}

			err := engine.AddPattern(pattern)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", test.id, err)
			}

			lines := []string{test.testLine, "no match"}
			ctx := context.Background()
			result, err := engine.ApplyFilters(ctx, lines)
			if err != nil {
				t.Fatalf("ApplyFilters should succeed for %s: %v", test.id, err)
			}

			hasMatch := len(result.MatchedLines) > 0
			if hasMatch != test.shouldMatch {
				t.Errorf("Pattern %s (%s) against %s: expected match=%v, got match=%v",
					test.id, test.expression, test.testLine, test.shouldMatch, hasMatch)
			}
		}
	})

	t.Run("Highlight Generation", func(t *testing.T) {
		engine.ClearPatterns()

		patterns := []core.FilterPattern{
			{
				ID:         "highlight-1",
				Expression: "error",
				Type:       core.FilterInclude,
				Color:      "#FF0000",
				Created:    time.Now(),
			},
			{
				ID:         "highlight-2",
				Expression: "\\d{4}",
				Type:       core.FilterInclude,
				Color:      "#00FF00",
				Created:    time.Now(),
			},
		}

		for _, p := range patterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed for %s: %v", p.ID, err)
			}
		}

		lines := []string{
			"error occurred in 2023", // Should have 2 highlights
			"warning in 1999",        // Should have 1 highlight
			"normal message",         // Should have 0 highlights
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// First line should have highlights
		if highlights, exists := result.MatchHighlights[0]; exists {
			if len(highlights) != 2 {
				t.Errorf("Expected 2 highlights for line 0, got %d", len(highlights))
			}

			// Verify highlight details
			for _, highlight := range highlights {
				if highlight.Start >= highlight.End {
					t.Errorf("Invalid highlight range: start=%d, end=%d",
						highlight.Start, highlight.End)
				}

				if highlight.PatternID == "" {
					t.Error("Highlight should have PatternID")
				}

				if highlight.Color == "" {
					t.Error("Highlight should have Color")
				}
			}
		} else {
			t.Error("Expected highlights for first line")
		}
	})

	t.Run("Statistics Accuracy", func(t *testing.T) {
		engine.ClearPatterns()

		// Add patterns to generate cache activity
		patterns := []core.FilterPattern{
			{ID: "stats-1", Expression: "error", Type: core.FilterInclude, Created: time.Now()},
			{ID: "stats-2", Expression: "warn", Type: core.FilterInclude, Created: time.Now()},
			{ID: "stats-3", Expression: "debug", Type: core.FilterExclude, Created: time.Now()},
		}

		for _, p := range patterns {
			err := engine.AddPattern(p)
			if err != nil {
				t.Fatalf("AddPattern should succeed: %v", err)
			}
		}

		lines := []string{
			"error message",
			"warn message",
			"info message",
			"debug message",
		}

		ctx := context.Background()
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			t.Fatalf("ApplyFilters should succeed: %v", err)
		}

		// Verify statistics
		stats := result.Stats

		if stats.TotalLines != len(lines) {
			t.Errorf("TotalLines should be %d, got %d", len(lines), stats.TotalLines)
		}

		if stats.MatchedLines != len(result.MatchedLines) {
			t.Errorf("MatchedLines should be %d, got %d",
				len(result.MatchedLines), stats.MatchedLines)
		}

		if stats.ProcessingTime == 0 {
			t.Error("ProcessingTime should be greater than 0")
		}

		if stats.PatternsUsed != len(patterns) {
			t.Errorf("PatternsUsed should be %d, got %d", len(patterns), stats.PatternsUsed)
		}

		// Cache stats should be available
		if stats.CacheHits < 0 || stats.CacheMisses < 0 {
			t.Error("Cache stats should be non-negative")
		}

		// Total cache accesses should equal cache hits + misses
		totalAccesses := stats.CacheHits + stats.CacheMisses
		if totalAccesses == 0 {
			t.Error("Expected some cache activity")
		}

		t.Logf("Statistics: total=%d, matched=%d, time=%v, patterns=%d, cache_hits=%d, cache_misses=%d",
			stats.TotalLines, stats.MatchedLines, stats.ProcessingTime,
			stats.PatternsUsed, stats.CacheHits, stats.CacheMisses)
	})
}

// Helper function to safely get string from slice or return empty string
func getStringOrEmpty(slice []string, index int) string {
	if index >= 0 && index < len(slice) {
		return slice[index]
	}
	return ""
}

// Benchmarks for performance validation
func BenchmarkFilterEngine_ApplyFilters_1K(b *testing.B) {
	engine := core.NewFilterEngine()
	setupBenchmarkEngine(b, engine)

	lines := generateBenchmarkLines(1000)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			b.Fatalf("ApplyFilters failed: %v", err)
		}
	}
}

func BenchmarkFilterEngine_ApplyFilters_10K(b *testing.B) {
	engine := core.NewFilterEngine()
	setupBenchmarkEngine(b, engine)

	lines := generateBenchmarkLines(10000)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			b.Fatalf("ApplyFilters failed: %v", err)
		}
	}
}

func BenchmarkFilterEngine_PatternCompilation(b *testing.B) {
	engine := core.NewFilterEngine()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		engine.ClearPatterns()

		pattern := core.FilterPattern{
			ID:         fmt.Sprintf("bench-pattern-%d", i),
			Expression: "error.*message.*\\d+",
			Type:       core.FilterInclude,
			Created:    time.Now(),
		}

		err := engine.AddPattern(pattern)
		if err != nil {
			b.Fatalf("AddPattern failed: %v", err)
		}
	}
}

func setupBenchmarkEngine(b *testing.B, engine core.FilterEngine) {
	b.Helper()

	patterns := []core.FilterPattern{
		{ID: "bench-error", Expression: "ERROR", Type: core.FilterInclude, Created: time.Now()},
		{ID: "bench-warn", Expression: "WARN", Type: core.FilterInclude, Created: time.Now()},
		{ID: "bench-debug", Expression: "DEBUG", Type: core.FilterExclude, Created: time.Now()},
	}

	for _, p := range patterns {
		err := engine.AddPattern(p)
		if err != nil {
			b.Fatalf("Setup failed: %v", err)
		}
	}
}

func generateBenchmarkLines(count int) []string {
	lines := make([]string, count)
	for i := 0; i < count; i++ {
		switch i % 4 {
		case 0:
			lines[i] = fmt.Sprintf("ERROR: Something went wrong at %d", i)
		case 1:
			lines[i] = fmt.Sprintf("WARN: Warning message %d", i)
		case 2:
			lines[i] = fmt.Sprintf("INFO: Information %d", i)
		case 3:
			lines[i] = fmt.Sprintf("DEBUG: Debug info %d", i)
		}
	}
	return lines
}
