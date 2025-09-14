package core

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewPattern(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		patternType PatternType
		color       string
		expectValid bool
	}{
		{
			name:        "valid include pattern",
			expression:  "error",
			patternType: Include,
			color:       "#ff0000",
			expectValid: true,
		},
		{
			name:        "valid exclude pattern",
			expression:  "debug.*info",
			patternType: Exclude,
			color:       "#00ff00",
			expectValid: true,
		},
		{
			name:        "invalid regex pattern",
			expression:  "[unclosed",
			patternType: Include,
			color:       "#ff0000",
			expectValid: false,
		},
		{
			name:        "empty expression",
			expression:  "",
			patternType: Include,
			color:       "#ff0000",
			expectValid: false,
		},
		{
			name:        "invalid hex color",
			expression:  "test",
			patternType: Include,
			color:       "not-a-color",
			expectValid: false,
		},
		{
			name:        "empty color",
			expression:  "test",
			patternType: Include,
			color:       "",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := NewPattern(tt.expression, tt.patternType, tt.color)

			// Check basic fields are set
			if pattern.ID == "" {
				t.Error("Expected non-empty ID")
			}
			if pattern.Expression != tt.expression {
				t.Errorf("Expected expression %q, got %q", tt.expression, pattern.Expression)
			}
			if pattern.Type != tt.patternType {
				t.Errorf("Expected type %v, got %v", tt.patternType, pattern.Type)
			}
			if pattern.Color != tt.color {
				t.Errorf("Expected color %q, got %q", tt.color, pattern.Color)
			}
			if pattern.MatchCount != 0 {
				t.Errorf("Expected MatchCount 0, got %d", pattern.MatchCount)
			}
			if pattern.Created.IsZero() {
				t.Error("Expected Created timestamp to be set")
			}

			// Check validation result
			if pattern.IsValid != tt.expectValid {
				t.Errorf("Expected IsValid %v, got %v", tt.expectValid, pattern.IsValid)
			}
		})
	}
}

func TestPatternValidate(t *testing.T) {
	tests := []struct {
		name        string
		pattern     Pattern
		expectValid bool
		expectError string
	}{
		{
			name: "valid pattern with color",
			pattern: Pattern{
				Expression: "test.*pattern",
				Color:      "#ff0000",
			},
			expectValid: true,
		},
		{
			name: "valid pattern without color",
			pattern: Pattern{
				Expression: "simple",
				Color:      "",
			},
			expectValid: true,
		},
		{
			name: "invalid regex",
			pattern: Pattern{
				Expression: "[unclosed",
				Color:      "#ff0000",
			},
			expectValid: false,
			expectError: "invalid regex pattern",
		},
		{
			name: "empty expression",
			pattern: Pattern{
				Expression: "",
				Color:      "#ff0000",
			},
			expectValid: false,
			expectError: "cannot be empty",
		},
		{
			name: "invalid hex color",
			pattern: Pattern{
				Expression: "test",
				Color:      "invalid",
			},
			expectValid: false,
			expectError: "invalid hex color format",
		},
		{
			name: "short hex color",
			pattern: Pattern{
				Expression: "test",
				Color:      "#f00",
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := tt.pattern.Validate()

			if valid != tt.expectValid {
				t.Errorf("Expected valid %v, got %v", tt.expectValid, valid)
			}

			if tt.expectError != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.expectError)
				} else if !strings.Contains(err.Error(), tt.expectError) {
					t.Errorf("Expected error containing %q, got %q", tt.expectError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestPatternCompile(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		expectError bool
	}{
		{
			name:        "valid regex",
			expression:  "test.*pattern",
			expectError: false,
		},
		{
			name:        "simple string",
			expression:  "error",
			expectError: false,
		},
		{
			name:        "complex regex",
			expression:  "^(INFO|WARN|ERROR)\\s+\\d{4}-\\d{2}-\\d{2}",
			expectError: false,
		},
		{
			name:        "invalid regex",
			expression:  "[unclosed",
			expectError: true,
		},
		{
			name:        "empty expression",
			expression:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := &Pattern{Expression: tt.expression}
			compiled, err := pattern.Compile()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if compiled != nil {
					t.Error("Expected nil compiled regex on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if compiled == nil {
					t.Error("Expected non-nil compiled regex")
				}

				// Test that compiled regex works
				if compiled != nil {
					testString := "test pattern"
					matches := compiled.MatchString(testString)
					expected := regexp.MustCompile(tt.expression).MatchString(testString)
					if matches != expected {
						t.Errorf("Compiled regex behavior differs from original")
					}
				}
			}
		})
	}
}

func TestPatternIncrementMatchCount(t *testing.T) {
	pattern := &Pattern{MatchCount: 5}

	pattern.IncrementMatchCount()
	if pattern.MatchCount != 6 {
		t.Errorf("Expected MatchCount 6, got %d", pattern.MatchCount)
	}

	pattern.IncrementMatchCount()
	if pattern.MatchCount != 7 {
		t.Errorf("Expected MatchCount 7, got %d", pattern.MatchCount)
	}
}

func TestPatternClone(t *testing.T) {
	original := &Pattern{
		ID:         "test-id",
		Expression: "test.*pattern",
		Type:       Include,
		MatchCount: 10,
		Color:      "#ff0000",
		Created:    time.Now(),
		IsValid:    true,
	}

	cloned := original.Clone()

	// Verify all fields are copied
	if cloned.ID != original.ID {
		t.Errorf("Expected ID %q, got %q", original.ID, cloned.ID)
	}
	if cloned.Expression != original.Expression {
		t.Errorf("Expected Expression %q, got %q", original.Expression, cloned.Expression)
	}
	if cloned.Type != original.Type {
		t.Errorf("Expected Type %v, got %v", original.Type, cloned.Type)
	}
	if cloned.MatchCount != original.MatchCount {
		t.Errorf("Expected MatchCount %d, got %d", original.MatchCount, cloned.MatchCount)
	}
	if cloned.Color != original.Color {
		t.Errorf("Expected Color %q, got %q", original.Color, cloned.Color)
	}
	if !cloned.Created.Equal(original.Created) {
		t.Errorf("Expected Created %v, got %v", original.Created, cloned.Created)
	}
	if cloned.IsValid != original.IsValid {
		t.Errorf("Expected IsValid %v, got %v", original.IsValid, cloned.IsValid)
	}

	// Verify it's a separate instance
	cloned.MatchCount = 999
	if original.MatchCount == 999 {
		t.Error("Clone modification affected original")
	}
}

func TestPatternTypeString(t *testing.T) {
	tests := []struct {
		patternType PatternType
		expected    string
	}{
		{Include, "Include"},
		{Exclude, "Exclude"},
		{PatternType(99), "Unknown"}, // Invalid type
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.patternType.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateUUID(t *testing.T) {
	// Test that UUID generation works
	uuid1 := generateUUID()
	uuid2 := generateUUID()

	if uuid1 == "" {
		t.Error("Expected non-empty UUID")
	}
	if uuid2 == "" {
		t.Error("Expected non-empty UUID")
	}
	if uuid1 == uuid2 {
		t.Error("Expected different UUIDs")
	}

	// Test UUID format (basic validation)
	parts := strings.Split(uuid1, "-")
	if len(parts) != 5 {
		t.Errorf("Expected UUID with 5 parts separated by hyphens, got %d parts", len(parts))
	}

	// Test length of parts (8-4-4-4-12)
	expectedLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLengths[i] {
			t.Errorf("UUID part %d expected length %d, got %d", i, expectedLengths[i], len(part))
		}
	}
}

func TestIsValidHexColor(t *testing.T) {
	tests := []struct {
		color    string
		expected bool
	}{
		{"#ff0000", true},   // Full hex
		{"#FF0000", true},   // Uppercase
		{"#f00", true},      // Short hex
		{"#F00", true},      // Short uppercase
		{"#123abc", true},   // Mixed case
		{"", true},          // Empty (valid for no color)
		{"ff0000", false},   // Missing #
		{"#gg0000", false},  // Invalid hex character
		{"#ff00", false},    // Wrong length
		{"#ff00000", false}, // Too long
		{"#", false},        // Just #
		{"red", false},      // Color name
		{"#xyz", false},     // Invalid characters
	}

	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			result := isValidHexColor(tt.color)
			if result != tt.expected {
				t.Errorf("For color %q, expected %v, got %v", tt.color, tt.expected, result)
			}
		})
	}
}
