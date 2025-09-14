package core

import (
	"context"
	"fmt"
	"regexp"
)

// Example_highlightEngine demonstrates the complete highlighting workflow
func Example_highlightEngine() {
	// Create a new highlight engine
	he := NewHighlightEngine()

	// Force true color support for consistent examples
	he.SetColorCapability(TrueColor)

	// Create patterns for demonstration
	errorPattern := FilterPattern{
		ID:         "error-pattern",
		Expression: "ERROR",
		Type:       FilterInclude,
		Color:      "#ff0000", // Red
	}

	warningPattern := FilterPattern{
		ID:         "warning-pattern",
		Expression: "WARNING|WARN",
		Type:       FilterInclude,
		Color:      "#ffaa00", // Orange
	}

	debugPattern := FilterPattern{
		ID:         "debug-pattern",
		Expression: "DEBUG",
		Type:       FilterExclude,
		Color:      "#ff4444", // Bright red for exclusion
	}

	// Assign colors to patterns
	he.AssignColor(errorPattern.ID, errorPattern.Color, NormalPriority)
	he.AssignColor(warningPattern.ID, warningPattern.Color, NormalPriority)
	he.AssignColor(debugPattern.ID, debugPattern.Color, HighPriority)

	// Compile the patterns
	patterns := []FilterPattern{errorPattern, warningPattern, debugPattern}
	compiled := make([]*regexp.Regexp, len(patterns))

	for i, pattern := range patterns {
		regex, err := regexp.Compile(pattern.Expression)
		if err != nil {
			fmt.Printf("Failed to compile pattern: %v\n", err)
			return
		}
		compiled[i] = regex
	}

	// Example log lines
	logLines := []string{
		"INFO: Application started successfully",
		"ERROR: Failed to connect to database",
		"WARNING: Memory usage is high",
		"DEBUG: Processing user request",
		"ERROR: Authentication failed for user",
		"INFO: DEBUG mode enabled",
	}

	fmt.Println("Highlighting demonstration:")
	fmt.Println("===========================")

	for i, line := range logLines {
		// Generate highlights for the line
		highlights := he.GenerateHighlights(line, patterns, compiled)

		// Apply highlighting
		highlighted := he.HighlightLine(line, highlights)

		fmt.Printf("Line %d: %s\n", i+1, highlighted)

		// Show highlight information
		if len(highlights) > 0 {
			fmt.Printf("         Highlights: %d found\n", len(highlights))
			for _, highlight := range highlights {
				priority := "normal"
				if highlight.Priority == HighPriority {
					priority = "high"
				}
				fmt.Printf("           [%d-%d] Pattern: %s, Priority: %s\n",
					highlight.Start, highlight.End, highlight.PatternID, priority)
			}
		}
		fmt.Println()
	}

	// Demonstrate terminal capability detection
	fmt.Println("Terminal Information:")
	fmt.Printf("  Color capability: %s\n", he.GetTerminalCapability())
	fmt.Printf("  Colors enabled: %v\n", he.IsColorEnabled())

	// Show color statistics
	assigned, cached, capability := he.GetColorStats()
	fmt.Printf("  Assigned colors: %d\n", assigned)
	fmt.Printf("  Cached colors: %d\n", cached)
	fmt.Printf("  Capability: %s\n", capability)
}

// Example_highlightEngineIntegration demonstrates integration with FilterEngine
func Example_highlightEngineIntegration() {
	// Create both engines
	filterEngine := NewFilterEngine()
	highlightEngine := NewHighlightEngine()

	// Force consistent color capability
	highlightEngine.SetColorCapability(Color256)

	// Create and add patterns to filter engine
	errorPattern := FilterPattern{
		ID:         "error-001",
		Expression: "ERROR|FATAL",
		Type:       FilterInclude,
		Color:      "#cc0000",
	}

	excludePattern := FilterPattern{
		ID:         "exclude-001",
		Expression: "TEMP|TMP",
		Type:       FilterExclude,
		Color:      "#ff0000",
	}

	err := filterEngine.AddPattern(errorPattern)
	if err != nil {
		fmt.Printf("Error adding pattern: %v\n", err)
		return
	}

	err = filterEngine.AddPattern(excludePattern)
	if err != nil {
		fmt.Printf("Error adding pattern: %v\n", err)
		return
	}

	// Assign colors in highlight engine
	highlightEngine.AssignColor(errorPattern.ID, errorPattern.Color, NormalPriority)
	highlightEngine.AssignColor(excludePattern.ID, excludePattern.Color, HighPriority)

	// Sample log data
	logData := []string{
		"INFO: System initialized",
		"ERROR: Database connection failed",
		"WARN: High memory usage",
		"ERROR: TEMP file access denied",
		"FATAL: Critical system error",
		"DEBUG: Processing TMP cleanup",
	}

	// Apply filters
	ctx := context.Background()
	result, err := filterEngine.ApplyFilters(ctx, logData)
	if err != nil {
		fmt.Printf("Filter error: %v\n", err)
		return
	}

	fmt.Println("Filter and Highlight Integration:")
	fmt.Println("=================================")
	fmt.Printf("Total lines: %d, Matched: %d\n", len(logData), len(result.MatchedLines))
	fmt.Println()

	// Get patterns for highlighting
	patterns := filterEngine.GetPatterns()
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		compiled[i], _ = regexp.Compile(pattern.Expression)
	}

	// Highlight the matched lines
	for i, line := range result.MatchedLines {
		highlights := highlightEngine.GenerateHighlights(line, patterns, compiled)
		highlighted := highlightEngine.HighlightLine(line, highlights)

		fmt.Printf("Matched line %d: %s\n", i+1, highlighted)

		// Show which patterns matched
		if lineHighlights, exists := result.MatchHighlights[result.LineNumbers[i]]; exists {
			fmt.Printf("  Filter matches: %d\n", len(lineHighlights))
		}
	}
}

// Example_colorCapabilities demonstrates different terminal color capabilities
func Example_colorCapabilities() {
	he := NewHighlightEngine()

	testColor := "#4080ff" // A nice blue color

	capabilities := []TerminalColorCapability{
		TrueColor,
		Color256,
		BasicColors,
		NoColor,
	}

	fmt.Println("Color Capability Demonstration:")
	fmt.Println("==============================")

	for _, capability := range capabilities {
		he.SetColorCapability(capability)

		ansiCode := he.hexToANSI(testColor)

		fmt.Printf("Capability: %s\n", capability)
		if ansiCode != "" {
			fmt.Printf("  ANSI code: %s\n", ansiCode)
			fmt.Printf("  Example: %sColored Text%s\n", ansiCode, he.GetResetSequence())
		} else {
			fmt.Printf("  ANSI code: (none - colors disabled)\n")
			fmt.Printf("  Example: Colored Text\n")
		}
		fmt.Println()
	}
}

// Example_overlapResolution demonstrates how overlapping highlights are handled
func Example_overlapResolution() {
	he := NewHighlightEngine()

	// Create overlapping highlights
	highlights := []HighlightSpan{
		{Start: 0, End: 10, PatternID: "pattern1", Priority: NormalPriority, Color: "#00ff00"},
		{Start: 5, End: 15, PatternID: "pattern2", Priority: HighPriority, Color: "#ff0000"},
		{Start: 12, End: 20, PatternID: "pattern3", Priority: NormalPriority, Color: "#0000ff"},
	}

	testLine := "This is a sample text line for demonstration purposes"

	fmt.Println("Overlap Resolution Demonstration:")
	fmt.Println("================================")
	fmt.Printf("Original line: %s\n", testLine)
	fmt.Println()

	fmt.Println("Highlights before resolution:")
	for i, h := range highlights {
		priority := "normal"
		if h.Priority == HighPriority {
			priority = "HIGH"
		}
		fmt.Printf("  %d: [%d-%d] %s priority, pattern: %s\n",
			i+1, h.Start, h.End, priority, h.PatternID)
	}
	fmt.Println()

	// Resolve overlaps
	resolved := he.resolveOverlaps(highlights)

	fmt.Println("Highlights after resolution:")
	for i, h := range resolved {
		priority := "normal"
		if h.Priority == HighPriority {
			priority = "HIGH"
		}
		fmt.Printf("  %d: [%d-%d] %s priority, pattern: %s\n",
			i+1, h.Start, h.End, priority, h.PatternID)
	}
	fmt.Println()

	// Show the actual highlighted result (without colors for example output)
	he.SetColorCapability(NoColor)
	highlighted := he.HighlightLine(testLine, resolved)
	fmt.Printf("Result: %s\n", highlighted)
}
