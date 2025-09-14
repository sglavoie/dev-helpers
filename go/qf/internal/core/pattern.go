// Package core provides the core business logic for the qf interactive log filter composer.
// This includes pattern management, filtering engine, and data structures for log processing.
package core

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PatternType defines whether a pattern includes or excludes content
type PatternType int

const (
	// Include patterns use OR logic - content matching any include pattern passes
	Include PatternType = iota
	// Exclude patterns use veto logic - content matching any exclude pattern is filtered out
	Exclude
)

// String returns a string representation of the PatternType
func (pt PatternType) String() string {
	switch pt {
	case Include:
		return "Include"
	case Exclude:
		return "Exclude"
	default:
		return "Unknown"
	}
}

// Pattern represents a compiled filter pattern with metadata.
// It contains all information needed to identify, validate, and apply regex-based filters
// to log content, along with usage statistics and visual highlighting information.
type Pattern struct {
	// ID is a UUID for unique identification across sessions
	ID string `json:"id"`

	// Expression is the raw regex pattern string as entered by the user
	Expression string `json:"expression"`

	// Type indicates whether this pattern includes or excludes matching content
	Type PatternType `json:"type"`

	// MatchCount tracks usage statistics for pattern optimization and history
	MatchCount int `json:"match_count"`

	// Color is the hex color code for highlighting matches (e.g., "#ff0000")
	Color string `json:"color"`

	// Created timestamp for pattern lifecycle management
	Created time.Time `json:"created"`

	// IsValid indicates whether the pattern expression compiles to valid regex
	IsValid bool `json:"is_valid"`
}

// NewPattern creates a new Pattern with generated UUID and validation.
// The pattern expression is validated during creation and IsValid field is set accordingly.
func NewPattern(expression string, patternType PatternType, color string) *Pattern {
	pattern := &Pattern{
		ID:         generateUUID(),
		Expression: expression,
		Type:       patternType,
		Color:      color,
		Created:    time.Now(),
		MatchCount: 0,
	}

	// Validate the pattern during creation
	pattern.IsValid, _ = pattern.Validate()

	return pattern
}

// Validate checks if the pattern's regex expression is valid and the color format is correct.
// Returns true and nil if valid, false and error if invalid.
// Uses the standard regexp package for validation.
func (p *Pattern) Validate() (bool, error) {
	// Validate regex expression
	if p.Expression == "" {
		return false, fmt.Errorf("pattern expression cannot be empty")
	}

	_, err := regexp.Compile(p.Expression)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern %q: %w", p.Expression, err)
	}

	// Validate color format (hex color)
	if p.Color != "" {
		if !isValidHexColor(p.Color) {
			return false, fmt.Errorf("invalid hex color format %q: must be in format #RRGGBB or #RGB", p.Color)
		}
	}

	return true, nil
}

// Compile returns a compiled regexp.Regexp for the pattern's expression.
// Returns an error if the pattern cannot be compiled.
// This method should be used by the filtering engine for pattern matching.
func (p *Pattern) Compile() (*regexp.Regexp, error) {
	if p.Expression == "" {
		return nil, fmt.Errorf("cannot compile empty pattern expression")
	}

	compiled, err := regexp.Compile(p.Expression)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern %q: %w", p.Expression, err)
	}

	return compiled, nil
}

// IncrementMatchCount increments the usage statistics for this pattern.
// This is used to track pattern usage for optimization and user interface hints.
func (p *Pattern) IncrementMatchCount() {
	p.MatchCount++
}

// Clone creates a deep copy of the pattern.
// This is useful for pattern updates without modifying the original.
func (p *Pattern) Clone() *Pattern {
	return &Pattern{
		ID:         p.ID,
		Expression: p.Expression,
		Type:       p.Type,
		MatchCount: p.MatchCount,
		Color:      p.Color,
		Created:    p.Created,
		IsValid:    p.IsValid,
	}
}

// generateUUID creates a simple UUID v4 using crypto/rand.
// This provides sufficient uniqueness for pattern identification within the application.
func generateUUID() string {
	// Generate 16 random bytes
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("pattern-%d", time.Now().UnixNano())
	}

	// Set version (4) and variant bits according to RFC 4122
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // Version 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // Variant 10

	// Format as UUID string
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// isValidHexColor validates that a string is a proper hex color format.
// Accepts both #RRGGBB and #RGB formats.
func isValidHexColor(color string) bool {
	if color == "" {
		return true // Empty color is valid (no highlighting)
	}

	if !strings.HasPrefix(color, "#") {
		return false
	}

	hex := color[1:] // Remove the # prefix

	// Check for valid lengths (#RGB or #RRGGBB)
	if len(hex) != 3 && len(hex) != 6 {
		return false
	}

	// Check that all characters are valid hex digits
	for _, char := range hex {
		if !((char >= '0' && char <= '9') ||
			(char >= 'A' && char <= 'F') ||
			(char >= 'a' && char <= 'f')) {
			return false
		}
	}

	return true
}

// Equal compares two patterns for equality (excluding match count)
func (p *Pattern) Equal(other *Pattern) bool {
	if other == nil {
		return false
	}

	return p.ID == other.ID &&
		p.Expression == other.Expression &&
		p.Type == other.Type &&
		p.Color == other.Color &&
		p.Created.Equal(other.Created) &&
		p.IsValid == other.IsValid
}
