package core

import (
	"strings"
	"testing"
)

func TestFilterSet_NewFilterSet(t *testing.T) {
	fs := NewFilterSet("test-session")

	if fs.Name != "test-session" {
		t.Errorf("Expected name 'test-session', got %s", fs.Name)
	}

	if len(fs.Include) != 0 {
		t.Errorf("Expected empty Include slice, got %d patterns", len(fs.Include))
	}

	if len(fs.Exclude) != 0 {
		t.Errorf("Expected empty Exclude slice, got %d patterns", len(fs.Exclude))
	}

	if !fs.IsEmpty() {
		t.Error("Expected FilterSet to be empty")
	}
}

func TestFilterSet_AddPattern(t *testing.T) {
	fs := NewFilterSet("test-session")

	// Create valid patterns
	includePattern := NewPattern("ERROR", Include, "#ff0000")
	excludePattern := NewPattern("DEBUG", Exclude, "#888888")

	// Test adding include pattern
	err := fs.AddPattern(*includePattern)
	if err != nil {
		t.Fatalf("Failed to add include pattern: %v", err)
	}

	if fs.GetIncludeCount() != 1 {
		t.Errorf("Expected 1 include pattern, got %d", fs.GetIncludeCount())
	}

	// Test adding exclude pattern
	err = fs.AddPattern(*excludePattern)
	if err != nil {
		t.Fatalf("Failed to add exclude pattern: %v", err)
	}

	if fs.GetExcludeCount() != 1 {
		t.Errorf("Expected 1 exclude pattern, got %d", fs.GetExcludeCount())
	}

	if fs.GetTotalPatternCount() != 2 {
		t.Errorf("Expected 2 total patterns, got %d", fs.GetTotalPatternCount())
	}
}

func TestFilterSet_AddDuplicatePattern(t *testing.T) {
	fs := NewFilterSet("test-session")

	pattern1 := NewPattern("ERROR", Include, "#ff0000")
	pattern2 := NewPattern("ERROR", Include, "#00ff00")

	// Add first pattern
	err := fs.AddPattern(*pattern1)
	if err != nil {
		t.Fatalf("Failed to add first pattern: %v", err)
	}

	// Try to add duplicate pattern (same expression and type)
	err = fs.AddPattern(*pattern2)
	if err == nil {
		t.Error("Expected error when adding duplicate pattern")
	}

	if fs.GetTotalPatternCount() != 1 {
		t.Errorf("Expected 1 total pattern, got %d", fs.GetTotalPatternCount())
	}
}

func TestFilterSet_RemovePattern(t *testing.T) {
	fs := NewFilterSet("test-session")

	pattern := NewPattern("ERROR", Include, "#ff0000")
	err := fs.AddPattern(*pattern)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	// Remove the pattern
	err = fs.RemovePattern(pattern.ID)
	if err != nil {
		t.Fatalf("Failed to remove pattern: %v", err)
	}

	if fs.GetTotalPatternCount() != 0 {
		t.Errorf("Expected 0 patterns after removal, got %d", fs.GetTotalPatternCount())
	}

	// Try to remove non-existent pattern
	err = fs.RemovePattern("non-existent-id")
	if err == nil {
		t.Error("Expected error when removing non-existent pattern")
	}
}

func TestFilterSet_FindPatternByID(t *testing.T) {
	fs := NewFilterSet("test-session")

	pattern := NewPattern("ERROR", Include, "#ff0000")
	err := fs.AddPattern(*pattern)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	// Find existing pattern
	found := fs.FindPatternByID(pattern.ID)
	if found == nil {
		t.Error("Expected to find pattern, but got nil")
	} else if found.ID != pattern.ID {
		t.Errorf("Expected pattern ID %s, got %s", pattern.ID, found.ID)
	}

	// Try to find non-existent pattern
	notFound := fs.FindPatternByID("non-existent-id")
	if notFound != nil {
		t.Error("Expected nil for non-existent pattern, but got result")
	}
}

func TestFilterSet_Validation(t *testing.T) {
	// Test empty name validation
	fs := NewFilterSet("")
	err := fs.Validate()
	if err == nil {
		t.Error("Expected validation error for empty name")
	}

	// Test invalid character in name
	fs = NewFilterSet("invalid/name")
	err = fs.Validate()
	if err == nil {
		t.Error("Expected validation error for invalid character in name")
	}

	// Test valid FilterSet
	fs = NewFilterSet("valid-name")
	pattern := NewPattern("ERROR", Include, "#ff0000")
	err = fs.AddPattern(*pattern)
	if err != nil {
		t.Fatalf("Failed to add valid pattern: %v", err)
	}

	err = fs.Validate()
	if err != nil {
		t.Errorf("Expected no validation error for valid FilterSet, got: %v", err)
	}
}

func TestFilterSet_DeepCopy(t *testing.T) {
	fs := NewFilterSet("original")
	fs.Description = "Original description"

	pattern := NewPattern("ERROR", Include, "#ff0000")
	err := fs.AddPattern(*pattern)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	// Create deep copy
	copy := fs.DeepCopy()

	// Verify basic fields
	if copy.Name != fs.Name {
		t.Errorf("Expected copy name %s, got %s", fs.Name, copy.Name)
	}

	if copy.Description != fs.Description {
		t.Errorf("Expected copy description %s, got %s", fs.Description, copy.Description)
	}

	// Verify patterns were copied
	if len(copy.Include) != len(fs.Include) {
		t.Errorf("Expected %d include patterns in copy, got %d", len(fs.Include), len(copy.Include))
	}

	// Modify original - copy should remain unchanged
	fs.Name = "modified"
	if copy.Name == fs.Name {
		t.Error("Deep copy was not independent of original")
	}
}

func TestFilterSet_JSON(t *testing.T) {
	fs := NewFilterSet("test-json")
	fs.Description = "Test JSON serialization"

	pattern := NewPattern("ERROR", Include, "#ff0000")
	err := fs.AddPattern(*pattern)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	// Test ToJSON
	jsonStr, err := fs.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	if jsonStr == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Test FromJSON
	newFs := NewFilterSet("empty")
	err = newFs.FromJSON(jsonStr)
	if err != nil {
		t.Fatalf("Failed to create from JSON: %v", err)
	}

	if newFs.Name != fs.Name {
		t.Errorf("Expected name %s after JSON roundtrip, got %s", fs.Name, newFs.Name)
	}

	if newFs.Description != fs.Description {
		t.Errorf("Expected description %s after JSON roundtrip, got %s", fs.Description, newFs.Description)
	}

	if len(newFs.Include) != len(fs.Include) {
		t.Errorf("Expected %d include patterns after JSON roundtrip, got %d", len(fs.Include), len(newFs.Include))
	}
}

func TestFilterSet_String(t *testing.T) {
	fs := NewFilterSet("test-string")
	fs.Description = "Test description"

	pattern := NewPattern("ERROR", Include, "#ff0000")
	err := fs.AddPattern(*pattern)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	str := fs.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Check that key information is included
	if !strings.Contains(str, "test-string") {
		t.Error("String representation should contain FilterSet name")
	}

	if !strings.Contains(str, "Test description") {
		t.Error("String representation should contain description")
	}

	if !strings.Contains(str, "ERROR") {
		t.Error("String representation should contain pattern expression")
	}
}

func TestFilterSet_ClearMethods(t *testing.T) {
	fs := NewFilterSet("test-clear")

	// Add patterns
	includePattern := NewPattern("ERROR", Include, "#ff0000")
	excludePattern := NewPattern("DEBUG", Exclude, "#888888")

	err := fs.AddPattern(*includePattern)
	if err != nil {
		t.Fatalf("Failed to add include pattern: %v", err)
	}

	err = fs.AddPattern(*excludePattern)
	if err != nil {
		t.Fatalf("Failed to add exclude pattern: %v", err)
	}

	// Test ClearIncludePatterns
	fs.ClearIncludePatterns()
	if fs.GetIncludeCount() != 0 {
		t.Errorf("Expected 0 include patterns after clear, got %d", fs.GetIncludeCount())
	}
	if fs.GetExcludeCount() != 1 {
		t.Errorf("Expected 1 exclude pattern to remain, got %d", fs.GetExcludeCount())
	}

	// Re-add include pattern
	err = fs.AddPattern(*includePattern)
	if err != nil {
		t.Fatalf("Failed to re-add include pattern: %v", err)
	}

	// Test ClearExcludePatterns
	fs.ClearExcludePatterns()
	if fs.GetExcludeCount() != 0 {
		t.Errorf("Expected 0 exclude patterns after clear, got %d", fs.GetExcludeCount())
	}
	if fs.GetIncludeCount() != 1 {
		t.Errorf("Expected 1 include pattern to remain, got %d", fs.GetIncludeCount())
	}

	// Test ClearAllPatterns
	fs.ClearAllPatterns()
	if !fs.IsEmpty() {
		t.Error("Expected FilterSet to be empty after ClearAllPatterns")
	}
}
