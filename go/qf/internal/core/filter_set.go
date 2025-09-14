// Package core provides the core business logic for the qf Interactive Log Filter Composer.
package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FilterSet represents a collection of include and exclude patterns that work together
// to filter log content. Include patterns use OR logic (any match includes the line),
// while exclude patterns use veto logic (any match excludes the line).
//
// The filtering logic follows these rules:
// 1. If Include patterns exist: line must match at least one Include pattern
// 2. If no Include patterns exist: all lines are included by default
// 3. If any Exclude pattern matches: line is excluded (veto logic)
// 4. Exclude patterns always override Include patterns
type FilterSet struct {
	// Include contains patterns that include matching lines (OR logic)
	Include []Pattern `json:"include"`

	// Exclude contains patterns that exclude matching lines (veto logic)
	Exclude []Pattern `json:"exclude"`

	// Name is the session identifier for this FilterSet
	Name string `json:"name"`

	// Description provides optional documentation for this FilterSet
	Description string `json:"description,omitempty"`
}

// NewFilterSet creates a new empty FilterSet with the given name
func NewFilterSet(name string) *FilterSet {
	return &FilterSet{
		Include:     make([]Pattern, 0),
		Exclude:     make([]Pattern, 0),
		Name:        name,
		Description: "",
	}
}

// Validate performs comprehensive validation of the FilterSet
func (fs *FilterSet) Validate() error {
	// Validate name format
	if err := fs.validateName(); err != nil {
		return err
	}

	// Check for unique pattern IDs across both Include and Exclude
	if err := fs.validateUniquePatternIDs(); err != nil {
		return err
	}

	// Validate all Include patterns
	for i, pattern := range fs.Include {
		if valid, err := pattern.Validate(); err != nil || !valid {
			return fmt.Errorf("invalid include pattern at index %d (ID: %s): %w", i, pattern.ID, err)
		}
	}

	// Validate all Exclude patterns
	for i, pattern := range fs.Exclude {
		if valid, err := pattern.Validate(); err != nil || !valid {
			return fmt.Errorf("invalid exclude pattern at index %d (ID: %s): %w", i, pattern.ID, err)
		}
	}

	// Check for duplicate patterns within Include
	if err := fs.validateNoDuplicatePatterns(fs.Include, "include"); err != nil {
		return err
	}

	// Check for duplicate patterns within Exclude
	if err := fs.validateNoDuplicatePatterns(fs.Exclude, "exclude"); err != nil {
		return err
	}

	return nil
}

// validateName ensures the FilterSet name meets requirements
func (fs *FilterSet) validateName() error {
	if strings.TrimSpace(fs.Name) == "" {
		return fmt.Errorf("FilterSet name cannot be empty or whitespace-only")
	}

	// Check for invalid characters that might cause issues with file systems or JSON
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, char := range invalidChars {
		if strings.Contains(fs.Name, char) {
			return fmt.Errorf("FilterSet name contains invalid character: %s", char)
		}
	}

	return nil
}

// validateUniquePatternIDs ensures all pattern IDs are unique across Include and Exclude
func (fs *FilterSet) validateUniquePatternIDs() error {
	seen := make(map[string]bool)

	// Check Include patterns
	for _, pattern := range fs.Include {
		if pattern.ID == "" {
			return fmt.Errorf("include pattern has empty ID")
		}
		if seen[pattern.ID] {
			return fmt.Errorf("duplicate pattern ID found: %s", pattern.ID)
		}
		seen[pattern.ID] = true
	}

	// Check Exclude patterns
	for _, pattern := range fs.Exclude {
		if pattern.ID == "" {
			return fmt.Errorf("exclude pattern has empty ID")
		}
		if seen[pattern.ID] {
			return fmt.Errorf("duplicate pattern ID found: %s", pattern.ID)
		}
		seen[pattern.ID] = true
	}

	return nil
}

// validateNoDuplicatePatterns checks for duplicate patterns within a slice
func (fs *FilterSet) validateNoDuplicatePatterns(patterns []Pattern, patternType string) error {
	seen := make(map[string]bool)

	for i, pattern := range patterns {
		key := fmt.Sprintf("%s|%s", pattern.Expression, pattern.Type)
		if seen[key] {
			return fmt.Errorf("duplicate %s pattern found at index %d: expression=%s, type=%s",
				patternType, i, pattern.Expression, pattern.Type)
		}
		seen[key] = true
	}

	return nil
}

// AddPattern adds a pattern to the FilterSet with duplicate prevention
func (fs *FilterSet) AddPattern(pattern Pattern) error {
	// Validate the pattern
	if valid, err := pattern.Validate(); err != nil || !valid {
		return fmt.Errorf("cannot add invalid pattern: %w", err)
	}

	// Check for duplicate ID
	if fs.FindPatternByID(pattern.ID) != nil {
		return fmt.Errorf("pattern with ID %s already exists", pattern.ID)
	}

	// Check for duplicate pattern (same expression and type)
	targetSlice := &fs.Include
	if pattern.Type == Exclude {
		targetSlice = &fs.Exclude
	}

	for _, existing := range *targetSlice {
		if existing.Expression == pattern.Expression && existing.Type == pattern.Type {
			return fmt.Errorf("duplicate pattern: expression=%s, type=%s", pattern.Expression, pattern.Type)
		}
	}

	// Add to appropriate slice
	*targetSlice = append(*targetSlice, pattern)

	return nil
}

// RemovePattern removes a pattern by ID from the FilterSet
func (fs *FilterSet) RemovePattern(patternID string) error {
	// Try to remove from Include patterns
	for i, pattern := range fs.Include {
		if pattern.ID == patternID {
			fs.Include = append(fs.Include[:i], fs.Include[i+1:]...)
			return nil
		}
	}

	// Try to remove from Exclude patterns
	for i, pattern := range fs.Exclude {
		if pattern.ID == patternID {
			fs.Exclude = append(fs.Exclude[:i], fs.Exclude[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("pattern with ID %s not found", patternID)
}

// FindPatternByID finds a pattern by its ID in either Include or Exclude
func (fs *FilterSet) FindPatternByID(patternID string) *Pattern {
	// Search Include patterns
	for i := range fs.Include {
		if fs.Include[i].ID == patternID {
			return &fs.Include[i]
		}
	}

	// Search Exclude patterns
	for i := range fs.Exclude {
		if fs.Exclude[i].ID == patternID {
			return &fs.Exclude[i]
		}
	}

	return nil
}

// ClearAllPatterns removes all patterns from the FilterSet
func (fs *FilterSet) ClearAllPatterns() {
	fs.Include = make([]Pattern, 0)
	fs.Exclude = make([]Pattern, 0)
}

// ClearIncludePatterns removes all include patterns
func (fs *FilterSet) ClearIncludePatterns() {
	fs.Include = make([]Pattern, 0)
}

// ClearExcludePatterns removes all exclude patterns
func (fs *FilterSet) ClearExcludePatterns() {
	fs.Exclude = make([]Pattern, 0)
}

// GetIncludeCount returns the number of include patterns
func (fs *FilterSet) GetIncludeCount() int {
	return len(fs.Include)
}

// GetExcludeCount returns the number of exclude patterns
func (fs *FilterSet) GetExcludeCount() int {
	return len(fs.Exclude)
}

// GetTotalPatternCount returns the total number of patterns
func (fs *FilterSet) GetTotalPatternCount() int {
	return len(fs.Include) + len(fs.Exclude)
}

// IsEmpty returns true if the FilterSet has no patterns
func (fs *FilterSet) IsEmpty() bool {
	return len(fs.Include) == 0 && len(fs.Exclude) == 0
}

// DeepCopy creates a complete deep copy of the FilterSet
func (fs *FilterSet) DeepCopy() *FilterSet {
	copy := &FilterSet{
		Name:        fs.Name,
		Description: fs.Description,
		Include:     make([]Pattern, len(fs.Include)),
		Exclude:     make([]Pattern, len(fs.Exclude)),
	}

	// Deep copy Include patterns
	for i, pattern := range fs.Include {
		copy.Include[i] = *pattern.Clone()
	}

	// Deep copy Exclude patterns
	for i, pattern := range fs.Exclude {
		copy.Exclude[i] = *pattern.Clone()
	}

	return copy
}

// MarshalJSON implements custom JSON marshaling
func (fs *FilterSet) MarshalJSON() ([]byte, error) {
	type Alias FilterSet
	return json.Marshal(&struct {
		*Alias
		Version string `json:"version"`
	}{
		Alias:   (*Alias)(fs),
		Version: "1.0",
	})
}

// UnmarshalJSON implements custom JSON unmarshaling
func (fs *FilterSet) UnmarshalJSON(data []byte) error {
	type Alias FilterSet
	aux := &struct {
		*Alias
		Version string `json:"version"`
	}{
		Alias: (*Alias)(fs),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Handle version compatibility if needed
	if aux.Version != "" && aux.Version != "1.0" {
		return fmt.Errorf("unsupported FilterSet version: %s", aux.Version)
	}

	return nil
}

// ToJSON converts the FilterSet to JSON string
func (fs *FilterSet) ToJSON() (string, error) {
	data, err := json.MarshalIndent(fs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal FilterSet to JSON: %w", err)
	}
	return string(data), nil
}

// FromJSON populates the FilterSet from JSON string
func (fs *FilterSet) FromJSON(jsonStr string) error {
	if err := json.Unmarshal([]byte(jsonStr), fs); err != nil {
		return fmt.Errorf("failed to unmarshal FilterSet from JSON: %w", err)
	}

	// Validate after unmarshaling
	return fs.Validate()
}

// Equal compares two FilterSets for equality
func (fs *FilterSet) Equal(other *FilterSet) bool {
	if other == nil {
		return false
	}

	if fs.Name != other.Name || fs.Description != other.Description {
		return false
	}

	if len(fs.Include) != len(other.Include) || len(fs.Exclude) != len(other.Exclude) {
		return false
	}

	// Compare Include patterns
	for i, pattern := range fs.Include {
		if !pattern.Equal(&other.Include[i]) {
			return false
		}
	}

	// Compare Exclude patterns
	for i, pattern := range fs.Exclude {
		if !pattern.Equal(&other.Exclude[i]) {
			return false
		}
	}

	return true
}

// String provides a human-readable representation of the FilterSet
func (fs *FilterSet) String() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("FilterSet: %s\n", fs.Name))
	if fs.Description != "" {
		builder.WriteString(fmt.Sprintf("Description: %s\n", fs.Description))
	}

	builder.WriteString(fmt.Sprintf("Include Patterns (%d):\n", len(fs.Include)))
	for i, pattern := range fs.Include {
		builder.WriteString(fmt.Sprintf("  %d. ID=%s, Expression=%q, Color=%s\n",
			i+1, pattern.ID, pattern.Expression, pattern.Color))
	}

	builder.WriteString(fmt.Sprintf("Exclude Patterns (%d):\n", len(fs.Exclude)))
	for i, pattern := range fs.Exclude {
		builder.WriteString(fmt.Sprintf("  %d. ID=%s, Expression=%q, Color=%s\n",
			i+1, pattern.ID, pattern.Expression, pattern.Color))
	}

	return builder.String()
}
