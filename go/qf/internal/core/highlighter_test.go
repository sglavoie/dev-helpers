package core

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestNewHighlightEngine verifies that a new HighlightEngine is created correctly
func TestNewHighlightEngine(t *testing.T) {
	he := NewHighlightEngine()

	if he == nil {
		t.Fatal("NewHighlightEngine returned nil")
	}

	if he.colorMap == nil {
		t.Error("colorMap should be initialized")
	}

	if he.priorityColors == nil {
		t.Error("priorityColors should be initialized")
	}

	if he.compiledColors == nil {
		t.Error("compiledColors should be initialized")
	}

	if he.resetSequence == "" {
		t.Error("resetSequence should not be empty")
	}

	if len(he.fallbackColors) == 0 {
		t.Error("fallbackColors should be initialized with default colors")
	}
}

// TestTerminalCapabilityDetection verifies terminal capability detection
func TestTerminalCapabilityDetection(t *testing.T) {
	tests := []struct {
		name             string
		envVars          map[string]string
		expectedCapacity TerminalColorCapability
	}{
		{
			name: "NO_COLOR disables colors",
			envVars: map[string]string{
				"NO_COLOR": "1",
			},
			expectedCapacity: NoColor,
		},
		{
			name: "TERM=dumb disables colors",
			envVars: map[string]string{
				"TERM": "dumb",
			},
			expectedCapacity: NoColor,
		},
		{
			name: "COLORTERM=truecolor enables true color",
			envVars: map[string]string{
				"COLORTERM": "truecolor",
				"TERM":      "xterm-256color",
			},
			expectedCapacity: TrueColor,
		},
		{
			name: "COLORTERM=24bit enables true color",
			envVars: map[string]string{
				"COLORTERM": "24bit",
			},
			expectedCapacity: TrueColor,
		},
		{
			name: "TERM=xterm-256color enables 256 colors",
			envVars: map[string]string{
				"TERM": "xterm-256color",
			},
			expectedCapacity: Color256,
		},
		{
			name: "TERM=linux enables basic colors",
			envVars: map[string]string{
				"TERM": "linux",
			},
			expectedCapacity: BasicColors,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment for all relevant variables
			originalEnv := map[string]string{
				"NO_COLOR":  os.Getenv("NO_COLOR"),
				"COLORTERM": os.Getenv("COLORTERM"),
				"TERM":      os.Getenv("TERM"),
			}

			// Clear all environment variables first
			os.Unsetenv("NO_COLOR")
			os.Unsetenv("COLORTERM")
			os.Unsetenv("TERM")

			// Set test environment
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Create new engine with test environment
			he := NewHighlightEngine()
			capability := he.GetTerminalCapability()

			if capability != tt.expectedCapacity {
				t.Errorf("Expected capability %v, got %v", tt.expectedCapacity, capability)
			}

			// Restore original environment
			for key, value := range originalEnv {
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}
		})
	}
}

// TestColorAssignment verifies color assignment functionality
func TestColorAssignment(t *testing.T) {
	he := NewHighlightEngine()
	he.SetColorCapability(TrueColor) // Force true color for consistent testing

	// Test specific hex color assignment
	patternID := "test-pattern-1"
	hexColor := "#ff0000"

	he.AssignColor(patternID, hexColor, NormalPriority)

	// Verify color was assigned
	assignedColors, _, _ := he.GetColorStats()
	if assignedColors != 1 {
		t.Errorf("Expected 1 assigned color, got %d", assignedColors)
	}

	// Test auto-assignment
	patternID2 := "test-pattern-2"
	he.AssignColor(patternID2, "", HighPriority)

	assignedColors, _, _ = he.GetColorStats()
	if assignedColors != 2 {
		t.Errorf("Expected 2 assigned colors, got %d", assignedColors)
	}

	// Test color removal
	he.RemoveColor(patternID)
	assignedColors, _, _ = he.GetColorStats()
	if assignedColors != 1 {
		t.Errorf("Expected 1 assigned color after removal, got %d", assignedColors)
	}
}

// TestHighlightGeneration verifies highlight generation from patterns
func TestHighlightGeneration(t *testing.T) {
	he := NewHighlightEngine()

	// Create test patterns
	patterns := []FilterPattern{
		{
			ID:         "include-1",
			Expression: "ERROR",
			Type:       FilterInclude,
			Color:      "#ff0000",
		},
		{
			ID:         "exclude-1",
			Expression: "DEBUG",
			Type:       FilterExclude,
			Color:      "#00ff00",
		},
	}

	// Compile patterns
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regex, err := regexp.Compile(pattern.Expression)
		if err != nil {
			t.Fatalf("Failed to compile pattern %s: %v", pattern.Expression, err)
		}
		compiled[i] = regex
	}

	// Test line with matches
	testLine := "ERROR: This is an error message with DEBUG info"
	highlights := he.GenerateHighlights(testLine, patterns, compiled)

	expectedHighlights := 2 // ERROR and DEBUG matches
	if len(highlights) != expectedHighlights {
		t.Errorf("Expected %d highlights, got %d", expectedHighlights, len(highlights))
	}

	// Verify ERROR highlight
	errorFound := false
	debugFound := false
	for _, highlight := range highlights {
		if highlight.PatternID == "include-1" && highlight.Start == 0 && highlight.End == 5 {
			errorFound = true
			if highlight.Priority != NormalPriority {
				t.Error("Include pattern should have NormalPriority")
			}
		}
		if highlight.PatternID == "exclude-1" && highlight.Priority != HighPriority {
			t.Error("Exclude pattern should have HighPriority")
		}
		if highlight.PatternID == "exclude-1" {
			debugFound = true
		}
	}

	if !errorFound {
		t.Error("ERROR highlight not found")
	}
	if !debugFound {
		t.Error("DEBUG highlight not found")
	}
}

// TestOverlapResolution verifies overlap resolution with priority
func TestOverlapResolution(t *testing.T) {
	he := NewHighlightEngine()

	testCases := []struct {
		name       string
		highlights []HighlightSpan
		expected   int // expected number of highlights after resolution
	}{
		{
			name: "no overlaps",
			highlights: []HighlightSpan{
				{Start: 0, End: 5, Priority: NormalPriority, PatternID: "p1"},
				{Start: 10, End: 15, Priority: NormalPriority, PatternID: "p2"},
			},
			expected: 2,
		},
		{
			name: "high priority overrides normal",
			highlights: []HighlightSpan{
				{Start: 0, End: 10, Priority: NormalPriority, PatternID: "p1"},
				{Start: 5, End: 8, Priority: HighPriority, PatternID: "p2"},
			},
			expected: 2, // High priority gets [5,8), remaining normal gets [0,5) and [8,10) may merge based on implementation
		},
		{
			name: "identical spans different priorities",
			highlights: []HighlightSpan{
				{Start: 0, End: 5, Priority: NormalPriority, PatternID: "p1"},
				{Start: 0, End: 5, Priority: HighPriority, PatternID: "p2"},
			},
			expected: 1, // High priority wins
		},
		{
			name: "partial overlap",
			highlights: []HighlightSpan{
				{Start: 0, End: 7, Priority: NormalPriority, PatternID: "p1"},
				{Start: 5, End: 12, Priority: HighPriority, PatternID: "p2"},
			},
			expected: 2, // Normal priority trimmed, high priority preserved
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := he.resolveOverlaps(tc.highlights)
			if len(resolved) != tc.expected {
				t.Errorf("Expected %d resolved highlights, got %d", tc.expected, len(resolved))
			}

			// Verify no overlaps remain
			for i := 0; i < len(resolved); i++ {
				for j := i + 1; j < len(resolved); j++ {
					if resolved[i].Start < resolved[j].End && resolved[i].End > resolved[j].Start {
						t.Errorf("Overlapping highlights remain: [%d,%d] and [%d,%d]",
							resolved[i].Start, resolved[i].End,
							resolved[j].Start, resolved[j].End)
					}
				}
			}
		})
	}
}

// TestColorConversion verifies color format conversions
func TestColorConversion(t *testing.T) {
	he := NewHighlightEngine()

	testCases := []struct {
		name       string
		capability TerminalColorCapability
		hexColor   string
		expectANSI bool
	}{
		{
			name:       "true color conversion",
			capability: TrueColor,
			hexColor:   "#ff0000",
			expectANSI: true,
		},
		{
			name:       "256 color conversion",
			capability: Color256,
			hexColor:   "#00ff00",
			expectANSI: true,
		},
		{
			name:       "basic color conversion",
			capability: BasicColors,
			hexColor:   "#0000ff",
			expectANSI: true,
		},
		{
			name:       "no color capability",
			capability: NoColor,
			hexColor:   "#ffff00",
			expectANSI: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			he.SetColorCapability(tc.capability)
			ansi := he.hexToANSI(tc.hexColor)

			if tc.expectANSI && ansi == "" {
				t.Error("Expected ANSI color sequence, got empty string")
			}
			if !tc.expectANSI && ansi != "" {
				t.Errorf("Expected no ANSI sequence, got: %s", ansi)
			}

			// For color-enabled cases, verify ANSI format
			if tc.expectANSI && ansi != "" {
				if !strings.HasPrefix(ansi, "\033[") {
					t.Errorf("ANSI sequence should start with escape code: %s", ansi)
				}
				if !strings.HasSuffix(ansi, "m") {
					t.Errorf("ANSI sequence should end with 'm': %s", ansi)
				}
			}
		})
	}
}

// TestHexColorParsing verifies hex color parsing functionality
func TestHexColorParsing(t *testing.T) {
	testCases := []struct {
		hexColor  string
		expectR   int
		expectG   int
		expectB   int
		expectErr bool
	}{
		{"#ff0000", 255, 0, 0, false},
		{"#00ff00", 0, 255, 0, false},
		{"#0000ff", 0, 0, 255, false},
		{"#fff", 255, 255, 255, false},
		{"#000", 0, 0, 0, false},
		{"#12abcd", 18, 171, 205, false},
		{"ff0000", 0, 0, 0, true},  // Missing #
		{"#gghhii", 0, 0, 0, true}, // Invalid hex
		{"#ff00", 0, 0, 0, true},   // Wrong length
		{"", 0, 0, 0, true},        // Empty
	}

	for _, tc := range testCases {
		r, g, b, err := parseHexColor(tc.hexColor)

		if tc.expectErr && err == nil {
			t.Errorf("Expected error for %s, got none", tc.hexColor)
		}
		if !tc.expectErr && err != nil {
			t.Errorf("Unexpected error for %s: %v", tc.hexColor, err)
		}
		if !tc.expectErr {
			if r != tc.expectR || g != tc.expectG || b != tc.expectB {
				t.Errorf("For %s, expected RGB(%d,%d,%d), got RGB(%d,%d,%d)",
					tc.hexColor, tc.expectR, tc.expectG, tc.expectB, r, g, b)
			}
		}
	}
}

// TestLineHighlighting verifies complete line highlighting
func TestLineHighlighting(t *testing.T) {
	he := NewHighlightEngine()
	he.SetColorCapability(TrueColor)
	he.SetColorEnabled(true)

	// Assign colors to patterns
	he.AssignColor("pattern1", "#ff0000", NormalPriority)
	he.AssignColor("pattern2", "#00ff00", HighPriority)

	testLine := "This is a test line with ERROR and WARNING"
	highlights := []HighlightSpan{
		{Start: 25, End: 30, PatternID: "pattern1", Priority: NormalPriority}, // ERROR
		{Start: 35, End: 42, PatternID: "pattern2", Priority: HighPriority},   // WARNING
	}

	highlighted := he.HighlightLine(testLine, highlights)

	// Verify structure - should contain ANSI escape sequences
	if !strings.Contains(highlighted, "\033[") {
		t.Error("Highlighted line should contain ANSI escape sequences")
	}

	// Should contain reset sequences
	resetCount := strings.Count(highlighted, he.GetResetSequence())
	if resetCount != 2 {
		t.Errorf("Expected 2 reset sequences, found %d", resetCount)
	}
}

// TestColorCaching verifies color caching functionality
func TestColorCaching(t *testing.T) {
	he := NewHighlightEngine()
	he.SetColorCapability(TrueColor)

	hexColor := "#ff0000"

	// First conversion should add to cache
	ansi1 := he.hexToANSI(hexColor)
	_, cachedCount1, _ := he.GetColorStats()

	// Second conversion should use cache
	ansi2 := he.hexToANSI(hexColor)
	_, cachedCount2, _ := he.GetColorStats()

	if ansi1 != ansi2 {
		t.Error("Cached color conversion should return same result")
	}

	if cachedCount2 != cachedCount1 {
		t.Error("Second conversion should use cached result")
	}

	// Test cache clearing
	he.ClearColorCache()
	_, cachedCountAfterClear, _ := he.GetColorStats()
	if cachedCountAfterClear != 0 {
		t.Error("Cache should be empty after clearing")
	}
}

// TestRGB256Conversion verifies RGB to 256-color conversion
func TestRGB256Conversion(t *testing.T) {
	he := NewHighlightEngine()

	testCases := []struct {
		r, g, b  int
		expected int
	}{
		{0, 0, 0, 16},        // Black
		{255, 0, 0, 196},     // Red
		{0, 255, 0, 46},      // Green
		{0, 0, 255, 21},      // Blue
		{255, 255, 255, 231}, // White
		{128, 128, 128, 244}, // Gray (should use grayscale ramp)
	}

	for _, tc := range testCases {
		result := he.rgbTo256(tc.r, tc.g, tc.b)
		if result != tc.expected {
			t.Errorf("RGB(%d,%d,%d) expected 256-color index %d, got %d",
				tc.r, tc.g, tc.b, tc.expected, result)
		}
	}
}

// TestBasicColorMapping verifies RGB to basic color mapping
func TestBasicColorMapping(t *testing.T) {
	he := NewHighlightEngine()

	testCases := []struct {
		r, g, b  int
		expected string
	}{
		{255, 0, 0, "red"},
		{0, 255, 0, "green"},
		{0, 0, 255, "blue"},
		{255, 255, 0, "yellow"},
		{255, 0, 255, "magenta"},
		{0, 255, 255, "cyan"},
		{255, 255, 255, "white"},
		{0, 0, 0, "black"},
		{32, 32, 32, "black"}, // Very dark colors map to black
	}

	for _, tc := range testCases {
		result := he.rgbToBasic(tc.r, tc.g, tc.b)
		if result != tc.expected {
			t.Errorf("RGB(%d,%d,%d) expected basic color %s, got %s",
				tc.r, tc.g, tc.b, tc.expected, result)
		}
	}
}

// TestHashStringToIndex verifies consistent string hashing
func TestHashStringToIndex(t *testing.T) {
	he := NewHighlightEngine()

	testString := "test-pattern-id"
	maxIndex := 10

	// Hash should be consistent
	index1 := he.hashStringToIndex(testString, maxIndex)
	index2 := he.hashStringToIndex(testString, maxIndex)

	if index1 != index2 {
		t.Error("Hash function should be consistent")
	}

	if index1 < 0 || index1 >= maxIndex {
		t.Errorf("Index %d should be within bounds [0, %d)", index1, maxIndex)
	}

	// Different strings should generally produce different indices
	index3 := he.hashStringToIndex("different-string", maxIndex)
	if index1 == index3 {
		// This could happen by chance, but log it
		t.Logf("Same hash for different strings (rare but possible): %d", index1)
	}
}

// TestColorStatsAndManagement verifies color statistics and management
func TestColorStatsAndManagement(t *testing.T) {
	he := NewHighlightEngine()

	// Initial stats
	assigned, cached, capability := he.GetColorStats()
	if assigned != 0 || cached != 0 {
		t.Error("Initial stats should show zero assigned and cached colors")
	}

	if capability == "" {
		t.Error("Capability string should not be empty")
	}

	// Assign some colors
	he.AssignColor("pattern1", "#ff0000", NormalPriority)
	he.AssignColor("pattern2", "#00ff00", HighPriority)

	assigned, _, _ = he.GetColorStats()
	if assigned != 2 {
		t.Errorf("Expected 2 assigned colors, got %d", assigned)
	}

	// Test color enabled/disabled
	if !he.IsColorEnabled() && he.GetTerminalCapability() != NoColor {
		t.Error("Colors should be enabled for color-capable terminals")
	}

	he.SetColorEnabled(false)
	if he.IsColorEnabled() {
		t.Error("Colors should be disabled after SetColorEnabled(false)")
	}
}

// BenchmarkHighlightGeneration benchmarks highlight generation performance
func BenchmarkHighlightGeneration(b *testing.B) {
	he := NewHighlightEngine()

	patterns := []FilterPattern{
		{ID: "p1", Expression: "ERROR", Type: FilterInclude, Color: "#ff0000"},
		{ID: "p2", Expression: "WARNING", Type: FilterInclude, Color: "#ffaa00"},
		{ID: "p3", Expression: "DEBUG", Type: FilterExclude, Color: "#00ff00"},
	}

	compiled := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		compiled[i] = regexp.MustCompile(pattern.Expression)
	}

	testLine := "ERROR: This is an error message with WARNING and DEBUG information"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		he.GenerateHighlights(testLine, patterns, compiled)
	}
}

// BenchmarkHighlightLine benchmarks line highlighting performance
func BenchmarkHighlightLine(b *testing.B) {
	he := NewHighlightEngine()
	he.SetColorCapability(TrueColor)
	he.AssignColor("p1", "#ff0000", NormalPriority)
	he.AssignColor("p2", "#00ff00", HighPriority)

	testLine := "This is a test line with multiple words to highlight"
	highlights := []HighlightSpan{
		{Start: 10, End: 14, PatternID: "p1", Priority: NormalPriority},
		{Start: 25, End: 33, PatternID: "p2", Priority: HighPriority},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		he.HighlightLine(testLine, highlights)
	}
}

// BenchmarkColorConversion benchmarks color conversion performance
func BenchmarkColorConversion(b *testing.B) {
	he := NewHighlightEngine()
	he.SetColorCapability(TrueColor)

	hexColor := "#ff8040"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		he.hexToANSI(hexColor)
	}
}

// TestIntegrationWithFilterEngine verifies integration with existing filter engine
func TestIntegrationWithFilterEngine(t *testing.T) {
	// Create a filter engine and highlight engine
	fe := NewFilterEngine()
	he := NewHighlightEngine()

	// Add patterns to filter engine
	pattern1 := FilterPattern{
		ID:         "integration-1",
		Expression: "ERROR",
		Type:       FilterInclude,
		Color:      "#ff0000",
		Created:    time.Now(),
	}

	pattern2 := FilterPattern{
		ID:         "integration-2",
		Expression: "WARNING",
		Type:       FilterExclude,
		Color:      "#ffaa00",
		Created:    time.Now(),
	}

	err := fe.AddPattern(pattern1)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	err = fe.AddPattern(pattern2)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	// Assign colors in highlight engine
	he.AssignColor(pattern1.ID, pattern1.Color, NormalPriority)
	he.AssignColor(pattern2.ID, pattern2.Color, HighPriority)

	// Test integration by simulating the highlight workflow
	testLines := []string{
		"ERROR: This is an error message",
		"INFO: This is info with WARNING",
		"DEBUG: Pure debug message",
	}

	patterns := fe.GetPatterns()
	if len(patterns) != 2 {
		t.Fatalf("Expected 2 patterns, got %d", len(patterns))
	}

	// Compile patterns for highlighting
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regex, err := regexp.Compile(pattern.Expression)
		if err != nil {
			t.Fatalf("Failed to compile pattern: %v", err)
		}
		compiled[i] = regex
	}

	// Generate highlights for each line
	for _, line := range testLines {
		highlights := he.GenerateHighlights(line, patterns, compiled)
		highlighted := he.HighlightLine(line, highlights)

		// Verify that highlighting works
		if strings.Contains(line, "ERROR") || strings.Contains(line, "WARNING") {
			// Lines with matches should be different when highlighted (unless colors disabled)
			if he.IsColorEnabled() && highlighted == line {
				t.Errorf("Line with matches should be highlighted: %s", line)
			}
		}
	}
}
