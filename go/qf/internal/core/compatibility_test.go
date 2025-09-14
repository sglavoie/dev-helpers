package core

import (
	"testing"
	"time"
)

// This test verifies that our Pattern struct is compatible with the FilterPattern
// interface defined in the contract tests by testing field compatibility
func TestFilterPatternCompatibility(t *testing.T) {
	// Create a Pattern using our implementation
	pattern := NewPattern("test.*error", Include, "#ff0000")

	// Verify all required fields exist and have correct types
	var _ string = pattern.ID
	var _ string = pattern.Expression
	var _ PatternType = pattern.Type
	var _ int = pattern.MatchCount
	var _ string = pattern.Color
	var _ time.Time = pattern.Created
	var _ bool = pattern.IsValid

	// Test that PatternType values are compatible (0 and 1)
	if Include != 0 {
		t.Errorf("Include should be 0 for compatibility with FilterInclude, got %d", Include)
	}
	if Exclude != 1 {
		t.Errorf("Exclude should be 1 for compatibility with FilterExclude, got %d", Exclude)
	}

	// Test basic functionality expected by FilterEngine interface
	valid, err := pattern.Validate()
	if !valid || err != nil {
		t.Errorf("Pattern validation failed: valid=%v, err=%v", valid, err)
	}

	compiled, err := pattern.Compile()
	if err != nil {
		t.Errorf("Pattern compilation failed: %v", err)
	}
	if compiled == nil {
		t.Error("Expected non-nil compiled regex")
	}

	// Test that UUID is generated
	if pattern.ID == "" {
		t.Error("Expected non-empty pattern ID")
	}

	// Test MatchCount functionality
	initialCount := pattern.MatchCount
	pattern.IncrementMatchCount()
	if pattern.MatchCount != initialCount+1 {
		t.Errorf("MatchCount increment failed: expected %d, got %d", initialCount+1, pattern.MatchCount)
	}

	// Test Clone functionality
	clone := pattern.Clone()
	if clone.ID != pattern.ID || clone.Expression != pattern.Expression {
		t.Error("Clone does not match original pattern")
	}

	t.Log("Pattern struct is compatible with FilterPattern interface requirements")
}
