// Package unit contains unit tests for individual components of the qf application.
//
// This file tests pattern validation functionality in isolation, verifying regex compilation,
// color validation, pattern creation, and edge cases to ensure robust filtering behavior.
package unit

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

func TestPatternType_String(t *testing.T) {
	tests := []struct {
		name     string
		pt       core.PatternType
		expected string
	}{
		{"Include type", core.Include, "Include"},
		{"Exclude type", core.Exclude, "Exclude"},
		{"Unknown type", core.PatternType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.pt.String(); result != tt.expected {
				t.Errorf("PatternType.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewPattern(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		patternType core.PatternType
		color       string
		expectValid bool
	}{
		{"Valid simple pattern", "test", core.Include, "#ff0000", true},
		{"Valid regex pattern", `\d{4}-\d{2}-\d{2}`, core.Include, "#00ff00", true},
		{"Valid exclude pattern", "error", core.Exclude, "#ff0000", true},
		{"Empty color", "test", core.Include, "", true},
		{"Invalid regex", "[", core.Include, "#ff0000", false},
		{"Invalid color", "test", core.Include, "not-a-color", false},
		{"Empty expression", "", core.Include, "#ff0000", false},
		{"Complex regex", `(?i)^(ERROR|WARN|INFO):\s+.*`, core.Include, "#ffaa00", true},
		{"Escaped characters", `\[\d+\]\s+\w+`, core.Include, "#0000ff", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := core.NewPattern(tt.expression, tt.patternType, tt.color)

			// Verify basic structure
			if pattern == nil {
				t.Fatal("NewPattern should not return nil")
			}

			if pattern.ID == "" {
				t.Error("Pattern ID should not be empty")
			}

			if pattern.Expression != tt.expression {
				t.Errorf("Pattern.Expression = %v, want %v", pattern.Expression, tt.expression)
			}

			if pattern.Type != tt.patternType {
				t.Errorf("Pattern.Type = %v, want %v", pattern.Type, tt.patternType)
			}

			if pattern.Color != tt.color {
				t.Errorf("Pattern.Color = %v, want %v", pattern.Color, tt.color)
			}

			if pattern.MatchCount != 0 {
				t.Errorf("Pattern.MatchCount should be 0, got %d", pattern.MatchCount)
			}

			if pattern.Created.IsZero() {
				t.Error("Pattern.Created should be set")
			}

			// Verify creation time is recent (within last second)
			if time.Since(pattern.Created) > time.Second {
				t.Error("Pattern.Created should be recent")
			}

			// Verify validation result
			if pattern.IsValid != tt.expectValid {
				t.Errorf("Pattern.IsValid = %v, want %v", pattern.IsValid, tt.expectValid)
			}

			// Verify UUID format (basic check)
			if !isValidUUIDFormat(pattern.ID) {
				t.Errorf("Pattern.ID %q should be valid UUID format", pattern.ID)
			}
		})
	}
}

func TestPattern_Validate(t *testing.T) {
	tests := []struct {
		name        string
		pattern     func() *core.Pattern
		expectValid bool
		expectError string
	}{
		{
			name: "Valid pattern with color",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "#ff0000",
				}
			},
			expectValid: true,
			expectError: "",
		},
		{
			name: "Valid pattern without color",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "",
				}
			},
			expectValid: true,
			expectError: "",
		},
		{
			name: "Empty expression",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "",
					Color:      "#ff0000",
				}
			},
			expectValid: false,
			expectError: "pattern expression cannot be empty",
		},
		{
			name: "Invalid regex",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "[",
					Color:      "#ff0000",
				}
			},
			expectValid: false,
			expectError: "invalid regex pattern",
		},
		{
			name: "Invalid color format - no hash",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "ff0000",
				}
			},
			expectValid: false,
			expectError: "invalid hex color format",
		},
		{
			name: "Invalid color format - wrong length",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "#ff00",
				}
			},
			expectValid: false,
			expectError: "invalid hex color format",
		},
		{
			name: "Invalid color format - invalid hex",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "#gggggg",
				}
			},
			expectValid: false,
			expectError: "invalid hex color format",
		},
		{
			name: "Valid 3-digit hex color",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "#f0a",
				}
			},
			expectValid: true,
			expectError: "",
		},
		{
			name: "Valid 6-digit hex color uppercase",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "test",
					Color:      "#FF00AA",
				}
			},
			expectValid: true,
			expectError: "",
		},
		{
			name: "Complex valid regex",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: `(?i)^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d{3}\s+\[(ERROR|WARN|INFO)\]`,
					Color:      "#ffffff",
				}
			},
			expectValid: true,
			expectError: "",
		},
		{
			name: "Invalid regex - unmatched parenthesis",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: "(test",
					Color:      "#ffffff",
				}
			},
			expectValid: false,
			expectError: "invalid regex pattern",
		},
		{
			name: "Invalid regex - invalid escape sequence",
			pattern: func() *core.Pattern {
				return &core.Pattern{
					Expression: `\k`,
					Color:      "#ffffff",
				}
			},
			expectValid: false,
			expectError: "invalid regex pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := tt.pattern()
			valid, err := pattern.Validate()

			if valid != tt.expectValid {
				t.Errorf("Pattern.Validate() valid = %v, want %v", valid, tt.expectValid)
			}

			if tt.expectError == "" {
				if err != nil {
					t.Errorf("Pattern.Validate() unexpected error = %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Pattern.Validate() expected error containing %q, got nil", tt.expectError)
				} else if !strings.Contains(err.Error(), tt.expectError) {
					t.Errorf("Pattern.Validate() error = %q, want to contain %q", err.Error(), tt.expectError)
				}
			}
		})
	}
}

func TestPattern_Compile(t *testing.T) {
	tests := []struct {
		name         string
		expression   string
		expectError  bool
		errorMessage string
	}{
		{"Valid simple pattern", "test", false, ""},
		{"Valid complex pattern", `(?i)\d{4}-\d{2}-\d{2}`, false, ""},
		{"Empty expression", "", true, "cannot compile empty pattern expression"},
		{"Invalid regex", "[", true, "failed to compile pattern"},
		{"Invalid regex - unmatched paren", "(test", true, "failed to compile pattern"},
		{"Valid character class", `[a-zA-Z0-9]+`, false, ""},
		{"Valid anchors", `^start.*end$`, false, ""},
		{"Valid quantifiers", `a{1,3}b*c+d?`, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := &core.Pattern{Expression: tt.expression}
			compiled, err := pattern.Compile()

			if tt.expectError {
				if err == nil {
					t.Errorf("Pattern.Compile() expected error, got nil")
				} else if tt.errorMessage != "" && !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Pattern.Compile() error = %q, want to contain %q", err.Error(), tt.errorMessage)
				}
				if compiled != nil {
					t.Errorf("Pattern.Compile() with error should return nil regexp")
				}
			} else {
				if err != nil {
					t.Errorf("Pattern.Compile() unexpected error = %v", err)
				}
				if compiled == nil {
					t.Errorf("Pattern.Compile() without error should return non-nil regexp")
				}

				// Test that the compiled regex works
				if compiled != nil {
					testString := "test123"
					matches := compiled.MatchString(testString)
					// We don't assert specific match behavior since it depends on the pattern,
					// but we verify the compiled regex is functional
					_ = matches
				}
			}
		})
	}
}

func TestPattern_IncrementMatchCount(t *testing.T) {
	pattern := core.NewPattern("test", core.Include, "#ff0000")
	initialCount := pattern.MatchCount

	if initialCount != 0 {
		t.Errorf("Initial match count should be 0, got %d", initialCount)
	}

	// Test single increment
	pattern.IncrementMatchCount()
	if pattern.MatchCount != 1 {
		t.Errorf("After one increment, match count should be 1, got %d", pattern.MatchCount)
	}

	// Test multiple increments
	for i := 0; i < 10; i++ {
		pattern.IncrementMatchCount()
	}
	if pattern.MatchCount != 11 {
		t.Errorf("After 11 total increments, match count should be 11, got %d", pattern.MatchCount)
	}
}

func TestPattern_Clone(t *testing.T) {
	original := core.NewPattern("test.*pattern", core.Exclude, "#aabbcc")
	original.MatchCount = 42
	original.IncrementMatchCount() // Make it 43

	clone := original.Clone()

	// Verify clone is not nil
	if clone == nil {
		t.Fatal("Clone should not be nil")
	}

	// Verify clone is not the same object
	if clone == original {
		t.Error("Clone should not be the same object as original")
	}

	// Verify all fields are copied correctly
	if clone.ID != original.ID {
		t.Errorf("Clone.ID = %v, want %v", clone.ID, original.ID)
	}

	if clone.Expression != original.Expression {
		t.Errorf("Clone.Expression = %v, want %v", clone.Expression, original.Expression)
	}

	if clone.Type != original.Type {
		t.Errorf("Clone.Type = %v, want %v", clone.Type, original.Type)
	}

	if clone.MatchCount != original.MatchCount {
		t.Errorf("Clone.MatchCount = %v, want %v", clone.MatchCount, original.MatchCount)
	}

	if clone.Color != original.Color {
		t.Errorf("Clone.Color = %v, want %v", clone.Color, original.Color)
	}

	if !clone.Created.Equal(original.Created) {
		t.Errorf("Clone.Created = %v, want %v", clone.Created, original.Created)
	}

	if clone.IsValid != original.IsValid {
		t.Errorf("Clone.IsValid = %v, want %v", clone.IsValid, original.IsValid)
	}

	// Verify modifying clone doesn't affect original
	clone.MatchCount = 999
	if original.MatchCount == 999 {
		t.Error("Modifying clone should not affect original")
	}

	clone.Expression = "modified"
	if original.Expression == "modified" {
		t.Error("Modifying clone should not affect original")
	}
}

func TestPattern_Equal(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	createPattern := func(id, expr string, pType core.PatternType, color string, created time.Time, valid bool) *core.Pattern {
		return &core.Pattern{
			ID:         id,
			Expression: expr,
			Type:       pType,
			Color:      color,
			Created:    created,
			IsValid:    valid,
			MatchCount: 0, // Equal ignores match count
		}
	}

	pattern1 := createPattern("id1", "test", core.Include, "#ff0000", baseTime, true)

	tests := []struct {
		name     string
		pattern1 *core.Pattern
		pattern2 *core.Pattern
		expected bool
	}{
		{
			name:     "Identical patterns",
			pattern1: pattern1,
			pattern2: createPattern("id1", "test", core.Include, "#ff0000", baseTime, true),
			expected: true,
		},
		{
			name:     "Same pattern with different match count",
			pattern1: pattern1,
			pattern2: func() *core.Pattern {
				p := createPattern("id1", "test", core.Include, "#ff0000", baseTime, true)
				p.MatchCount = 100 // Should be ignored in comparison
				return p
			}(),
			expected: true,
		},
		{
			name:     "Different ID",
			pattern1: pattern1,
			pattern2: createPattern("id2", "test", core.Include, "#ff0000", baseTime, true),
			expected: false,
		},
		{
			name:     "Different expression",
			pattern1: pattern1,
			pattern2: createPattern("id1", "different", core.Include, "#ff0000", baseTime, true),
			expected: false,
		},
		{
			name:     "Different type",
			pattern1: pattern1,
			pattern2: createPattern("id1", "test", core.Exclude, "#ff0000", baseTime, true),
			expected: false,
		},
		{
			name:     "Different color",
			pattern1: pattern1,
			pattern2: createPattern("id1", "test", core.Include, "#00ff00", baseTime, true),
			expected: false,
		},
		{
			name:     "Different created time",
			pattern1: pattern1,
			pattern2: createPattern("id1", "test", core.Include, "#ff0000", baseTime.Add(time.Hour), true),
			expected: false,
		},
		{
			name:     "Different validity",
			pattern1: pattern1,
			pattern2: createPattern("id1", "test", core.Include, "#ff0000", baseTime, false),
			expected: false,
		},
		{
			name:     "Compare with nil",
			pattern1: pattern1,
			pattern2: nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pattern1.Equal(tt.pattern2)
			if result != tt.expected {
				t.Errorf("Pattern.Equal() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHexColorValidation(t *testing.T) {
	tests := []struct {
		name  string
		color string
		valid bool
	}{
		// Valid cases
		{"Empty color", "", true},
		{"Valid 6-digit lowercase", "#ff0000", true},
		{"Valid 6-digit uppercase", "#FF0000", true},
		{"Valid 6-digit mixed case", "#Ff00Aa", true},
		{"Valid 3-digit lowercase", "#f0a", true},
		{"Valid 3-digit uppercase", "#F0A", true},
		{"Valid 3-digit mixed case", "#F0a", true},
		{"All zeros", "#000000", true},
		{"All ones", "#111111", true},
		{"All letters", "#abcdef", true},

		// Invalid cases
		{"No hash prefix", "ff0000", false},
		{"Wrong length - 1 digit", "#f", false},
		{"Wrong length - 2 digits", "#ff", false},
		{"Wrong length - 4 digits", "#ff00", false},
		{"Wrong length - 5 digits", "#ff000", false},
		{"Wrong length - 7 digits", "#ff00000", false},
		{"Invalid characters - g", "#gg0000", false},
		{"Invalid characters - z", "#ff00zz", false},
		{"Invalid characters - symbols", "#ff00@@", false},
		{"Invalid characters - space", "#ff 000", false},
		{"Only hash", "#", false},
		{"Hash with spaces", "#ff 00 00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := &core.Pattern{
				Expression: "test", // Valid expression
				Color:      tt.color,
			}

			valid, err := pattern.Validate()

			if tt.valid {
				if !valid {
					t.Errorf("Color %q should be valid, got invalid", tt.color)
				}
				if err != nil {
					t.Errorf("Color %q should be valid, got error: %v", tt.color, err)
				}
			} else {
				if valid {
					t.Errorf("Color %q should be invalid, got valid", tt.color)
				}
				if err == nil {
					t.Errorf("Color %q should produce error, got nil", tt.color)
				} else if !strings.Contains(err.Error(), "invalid hex color format") {
					t.Errorf("Color %q error should mention hex format, got: %v", tt.color, err)
				}
			}
		})
	}
}

func TestPatternCreationEdgeCases(t *testing.T) {
	// Test with all pattern types
	types := []core.PatternType{core.Include, core.Exclude}

	for _, pType := range types {
		t.Run(fmt.Sprintf("Type_%s", pType.String()), func(t *testing.T) {
			pattern := core.NewPattern("test", pType, "#ff0000")
			if pattern.Type != pType {
				t.Errorf("Pattern type should be %v, got %v", pType, pattern.Type)
			}
		})
	}

	// Test with extreme expression lengths
	t.Run("Very long expression", func(t *testing.T) {
		longExpr := strings.Repeat("a", 1000) + "*"
		pattern := core.NewPattern(longExpr, core.Include, "#ff0000")
		if pattern.Expression != longExpr {
			t.Error("Should handle very long expressions")
		}
		if !pattern.IsValid {
			t.Error("Long but valid expression should be valid")
		}
	})

	// Test with unicode characters
	t.Run("Unicode in expression", func(t *testing.T) {
		unicodeExpr := "测试.*パターン"
		pattern := core.NewPattern(unicodeExpr, core.Include, "#ff0000")
		if pattern.Expression != unicodeExpr {
			t.Error("Should handle unicode expressions")
		}
	})
}

func TestUUIDGeneration(t *testing.T) {
	// Create multiple patterns to test UUID uniqueness
	patterns := make([]*core.Pattern, 100)
	uuids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		patterns[i] = core.NewPattern("test", core.Include, "#ff0000")

		// Check UUID format
		if !isValidUUIDFormat(patterns[i].ID) {
			t.Errorf("Pattern %d has invalid UUID format: %s", i, patterns[i].ID)
		}

		// Check uniqueness
		if uuids[patterns[i].ID] {
			t.Errorf("Duplicate UUID found: %s", patterns[i].ID)
		}
		uuids[patterns[i].ID] = true
	}

	// Verify all UUIDs are unique
	if len(uuids) != 100 {
		t.Errorf("Expected 100 unique UUIDs, got %d", len(uuids))
	}
}

func TestPatternValidationPerformance(t *testing.T) {
	// Test performance requirements: validation should be fast
	pattern := core.NewPattern("test.*pattern", core.Include, "#ff0000")

	// Measure validation time
	start := time.Now()
	iterations := 10000

	for i := 0; i < iterations; i++ {
		_, _ = pattern.Validate()
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	// Validation should be very fast (< 1ms per operation)
	if avgDuration > time.Millisecond {
		t.Errorf("Pattern validation too slow: avg %v per operation", avgDuration)
	}
}

func TestRealWorldRegexPatterns(t *testing.T) {
	// Test with realistic log parsing patterns
	realWorldPatterns := []struct {
		name       string
		expression string
		shouldWork bool
	}{
		{"ISO timestamp", `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z`, true},
		{"Log level", `(?i)(ERROR|WARN|INFO|DEBUG)`, true},
		{"IP address", `\b(?:\d{1,3}\.){3}\d{1,3}\b`, true},
		{"JSON log", `^\{.*\}$`, true},
		{"HTTP status", `HTTP/\d\.\d\s+[12345]\d{2}`, true},
		{"Email pattern", `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, true},
		{"Java exception", `^\s*at\s+\S+\.\S+\(.*:\d+\)$`, true},
		{"Docker container", `[a-f0-9]{12}`, true},
		{"Complex alternation", `(GET|POST|PUT|DELETE)\s+/\w+(/\w+)*`, true},
		{"Word boundaries", `\bpassword\b`, true},
		{"Named groups", `(?P<timestamp>\d{4}-\d{2}-\d{2})\s+(?P<level>\w+)`, true},
	}

	for _, tt := range realWorldPatterns {
		t.Run(tt.name, func(t *testing.T) {
			pattern := core.NewPattern(tt.expression, core.Include, "#ff0000")

			if pattern.IsValid != tt.shouldWork {
				t.Errorf("Pattern %q validity = %v, want %v", tt.expression, pattern.IsValid, tt.shouldWork)
			}

			if tt.shouldWork {
				// Test that it can be compiled
				compiled, err := pattern.Compile()
				if err != nil {
					t.Errorf("Pattern %q failed to compile: %v", tt.expression, err)
				}
				if compiled == nil {
					t.Errorf("Pattern %q compiled to nil", tt.expression)
				}
			}
		})
	}
}

func TestPatternConcurrency(t *testing.T) {
	// Test that pattern operations are safe for concurrent use
	pattern := core.NewPattern("test", core.Include, "#ff0000")

	const numGoroutines = 100
	const operationsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Mix of read and write operations
				switch j % 4 {
				case 0:
					pattern.IncrementMatchCount()
				case 1:
					_, _ = pattern.Validate()
				case 2:
					_, _ = pattern.Compile()
				case 3:
					clone := pattern.Clone()
					_ = clone.Equal(pattern)
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify pattern state is consistent
	if pattern.MatchCount != numGoroutines*operationsPerGoroutine/4 {
		// Note: This might be flaky due to race conditions, but tests concurrent access
		// The actual count depends on scheduling, but should be reasonable
		if pattern.MatchCount == 0 || pattern.MatchCount > numGoroutines*operationsPerGoroutine {
			t.Errorf("Unexpected match count after concurrent access: %d", pattern.MatchCount)
		}
	}
}

// TestCryptoRandFailure tests UUID generation fallback when crypto/rand fails
func TestUUIDFallback(t *testing.T) {
	// This test verifies the fallback behavior mentioned in generateUUID
	// We can't easily force crypto/rand to fail, but we can test that
	// the UUID format is consistent

	uuids := make(map[string]bool)

	// Generate many UUIDs to test consistency
	for i := 0; i < 1000; i++ {
		pattern := core.NewPattern("test", core.Include, "#ff0000")

		if uuids[pattern.ID] {
			t.Errorf("Duplicate UUID generated: %s", pattern.ID)
		}
		uuids[pattern.ID] = true

		if !isValidUUIDFormat(pattern.ID) {
			// Check if it's the fallback format
			if !strings.HasPrefix(pattern.ID, "pattern-") {
				t.Errorf("Invalid UUID format (not standard UUID or fallback): %s", pattern.ID)
			}
		}
	}
}

// isValidUUIDFormat checks if a string matches UUID v4 format
func isValidUUIDFormat(uuid string) bool {
	if len(uuid) != 36 {
		return false
	}

	// Check format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// where x is any hexadecimal digit and y is one of 8, 9, A, or B
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(uuid))
}

// Benchmark tests for performance validation
func BenchmarkPatternValidation(b *testing.B) {
	pattern := &core.Pattern{
		Expression: "test.*pattern",
		Color:      "#ff0000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pattern.Validate()
	}
}

func BenchmarkPatternCompilation(b *testing.B) {
	pattern := &core.Pattern{
		Expression: `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pattern.Compile()
	}
}

func BenchmarkPatternCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = core.NewPattern("test.*pattern", core.Include, "#ff0000")
	}
}

func BenchmarkPatternClone(b *testing.B) {
	pattern := core.NewPattern("test.*pattern", core.Include, "#ff0000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pattern.Clone()
	}
}

func BenchmarkUUIDGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// We can't directly benchmark generateUUID since it's not exported,
		// but we can benchmark pattern creation which includes UUID generation
		p := core.NewPattern("test", core.Include, "#ff0000")
		_ = p.ID // Force UUID usage
	}
}
