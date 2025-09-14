// Package core provides the core business logic for the qf interactive log filter composer.
// This module implements pattern highlighting and match display functionality for terminal output.
package core

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// TerminalColorCapability defines the color capabilities of the terminal
type TerminalColorCapability int

const (
	// NoColor indicates terminal doesn't support colors
	NoColor TerminalColorCapability = iota
	// BasicColors indicates support for 8 basic colors (3-bit)
	BasicColors
	// Color256 indicates support for 256 colors (8-bit)
	Color256
	// TrueColor indicates support for 16.7 million colors (24-bit)
	TrueColor
)

// String returns a string representation of the terminal color capability
func (tcc TerminalColorCapability) String() string {
	switch tcc {
	case NoColor:
		return "NoColor"
	case BasicColors:
		return "BasicColors"
	case Color256:
		return "Color256"
	case TrueColor:
		return "TrueColor"
	default:
		return "Unknown"
	}
}

// ColorPriority defines priority levels for highlighting
type ColorPriority int

const (
	// NormalPriority is the default priority for include patterns
	NormalPriority ColorPriority = iota
	// HighPriority is used for exclude patterns (takes precedence)
	HighPriority
)

// HighlightSpan represents a highlighted section within a line
type HighlightSpan struct {
	Start     int           // Start position in line
	End       int           // End position in line
	Color     string        // Color in hex format (#RRGGBB or #RGB)
	PatternID string        // ID of the pattern that caused this highlight
	Priority  ColorPriority // Priority level for overlap resolution
}

// HighlightEngine coordinates pattern highlighting and color management
type HighlightEngine struct {
	// Terminal capability detection
	colorCapability TerminalColorCapability

	// Color management
	colorMutex     sync.RWMutex
	colorMap       map[string]string          // Pattern ID -> ANSI color code
	priorityColors map[ColorPriority][]string // Priority -> available colors

	// Configuration
	enableColors   bool
	fallbackColors []string // Basic 8 colors for fallback

	// Performance optimizations
	compiledColors map[string]string // Hex color -> ANSI sequence cache
	resetSequence  string            // ANSI reset sequence
}

// NewHighlightEngine creates a new HighlightEngine with automatic terminal detection
func NewHighlightEngine() *HighlightEngine {
	he := &HighlightEngine{
		colorMap:       make(map[string]string),
		priorityColors: make(map[ColorPriority][]string),
		compiledColors: make(map[string]string),
		resetSequence:  "\033[0m",
		fallbackColors: []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white", "bright_red"},
	}

	// Detect terminal capabilities
	he.colorCapability = he.detectTerminalCapability()
	he.enableColors = he.colorCapability != NoColor

	// Initialize priority colors
	he.initializePriorityColors()

	return he
}

// detectTerminalCapability detects the terminal's color capabilities
func (he *HighlightEngine) detectTerminalCapability() TerminalColorCapability {
	// Check if colors are explicitly disabled
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return NoColor
	}

	// Check for true color support
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return TrueColor
	}

	// Check TERM environment variable for capability hints
	term := os.Getenv("TERM")
	switch {
	case strings.Contains(term, "256color"):
		return Color256
	case strings.Contains(term, "color") || term == "xterm" || term == "xterm-color":
		return Color256 // Most modern terminals support 256 colors
	case term == "linux" || term == "console":
		return BasicColors
	case term == "" || term == "dumb":
		return NoColor
	default:
		// Default to basic colors for unknown terminals
		return BasicColors
	}
}

// initializePriorityColors sets up color palettes for different priority levels
func (he *HighlightEngine) initializePriorityColors() {
	// Normal priority colors (for include patterns)
	he.priorityColors[NormalPriority] = []string{
		"#00ff00", "#0080ff", "#ff8000", "#ff0080", "#8000ff", "#00ff80",
		"#80ff00", "#ff0040", "#4000ff", "#00ffff", "#ff8080", "#80ff80",
	}

	// High priority colors (for exclude patterns) - more intense/darker
	he.priorityColors[HighPriority] = []string{
		"#ff0000", "#cc0000", "#990000", "#ff3300", "#cc3300", "#ff6600",
		"#ffaa00", "#ff4444", "#cc4444", "#ff7777", "#cc7777", "#ffaaaa",
	}
}

// GetTerminalCapability returns the detected terminal color capability
func (he *HighlightEngine) GetTerminalCapability() TerminalColorCapability {
	return he.colorCapability
}

// SetColorCapability allows manual override of terminal capability detection
func (he *HighlightEngine) SetColorCapability(capability TerminalColorCapability) {
	he.colorCapability = capability
	he.enableColors = capability != NoColor
}

// AssignColor assigns a color to a pattern based on its type and priority
func (he *HighlightEngine) AssignColor(patternID string, hexColor string, priority ColorPriority) {
	he.colorMutex.Lock()
	defer he.colorMutex.Unlock()

	if !he.enableColors {
		return
	}

	// If a specific hex color is provided, use it
	if hexColor != "" && isValidHexColor(hexColor) {
		ansiColor := he.hexToANSI(hexColor)
		he.colorMap[patternID] = ansiColor
		return
	}

	// Auto-assign color based on priority
	colors := he.priorityColors[priority]
	if len(colors) == 0 {
		return
	}

	// Use pattern ID hash to consistently assign colors
	colorIndex := he.hashStringToIndex(patternID, len(colors))
	selectedColor := colors[colorIndex]

	ansiColor := he.hexToANSI(selectedColor)
	he.colorMap[patternID] = ansiColor
}

// RemoveColor removes color assignment for a pattern
func (he *HighlightEngine) RemoveColor(patternID string) {
	he.colorMutex.Lock()
	defer he.colorMutex.Unlock()

	delete(he.colorMap, patternID)
}

// HighlightLine applies highlighting to a line of text based on provided highlights
func (he *HighlightEngine) HighlightLine(line string, highlights []HighlightSpan) string {
	if !he.enableColors || len(highlights) == 0 {
		return line
	}

	// Resolve overlapping highlights
	resolvedHighlights := he.resolveOverlaps(highlights)
	if len(resolvedHighlights) == 0 {
		return line
	}

	// Sort highlights by start position
	sort.Slice(resolvedHighlights, func(i, j int) bool {
		return resolvedHighlights[i].Start < resolvedHighlights[j].Start
	})

	var result strings.Builder
	pos := 0

	for _, highlight := range resolvedHighlights {
		// Add text before highlight
		if highlight.Start > pos {
			result.WriteString(line[pos:highlight.Start])
		}

		// Add highlighted text
		color := he.getPatternColor(highlight.PatternID, highlight.Color)
		if color != "" {
			result.WriteString(color)
			result.WriteString(line[highlight.Start:highlight.End])
			result.WriteString(he.resetSequence)
		} else {
			result.WriteString(line[highlight.Start:highlight.End])
		}

		pos = highlight.End
	}

	// Add remaining text
	if pos < len(line) {
		result.WriteString(line[pos:])
	}

	return result.String()
}

// GenerateHighlights finds all pattern matches in a line and creates highlight spans
func (he *HighlightEngine) GenerateHighlights(line string, patterns []FilterPattern, compiledRegexes []*regexp.Regexp) []HighlightSpan {
	var highlights []HighlightSpan

	if !he.enableColors {
		return highlights
	}

	for i, pattern := range patterns {
		if i >= len(compiledRegexes) {
			continue
		}

		regex := compiledRegexes[i]
		if regex == nil {
			continue
		}

		matches := regex.FindAllStringIndex(line, -1)
		priority := NormalPriority
		if pattern.Type == FilterExclude {
			priority = HighPriority
		}

		for _, match := range matches {
			highlights = append(highlights, HighlightSpan{
				Start:     match[0],
				End:       match[1],
				Color:     pattern.Color,
				PatternID: pattern.ID,
				Priority:  priority,
			})
		}
	}

	return highlights
}

// resolveOverlaps handles overlapping highlights with priority rules
func (he *HighlightEngine) resolveOverlaps(highlights []HighlightSpan) []HighlightSpan {
	if len(highlights) <= 1 {
		return highlights
	}

	// Sort by priority first (highest first), then by start position
	sort.Slice(highlights, func(i, j int) bool {
		if highlights[i].Priority == highlights[j].Priority {
			return highlights[i].Start < highlights[j].Start
		}
		return highlights[i].Priority > highlights[j].Priority
	})

	var result []HighlightSpan

	for _, current := range highlights {
		// Check if current overlaps with any existing highlights
		overlapped := false

		for i := 0; i < len(result); i++ {
			existing := &result[i]

			// Check for overlap
			if current.Start < existing.End && current.End > existing.Start {
				overlapped = true

				if current.Priority > existing.Priority {
					// Current has higher priority - it can override existing
					if current.Start <= existing.Start && current.End >= existing.End {
						// Current completely covers existing - replace it
						result[i] = current
						break
					} else if current.Start <= existing.Start {
						// Current starts before existing - trim existing start
						existing.Start = current.End
						if existing.Start >= existing.End {
							// Existing is completely overridden
							result = append(result[:i], result[i+1:]...)
							i-- // Adjust index after removal
						}
						// Add current
						result = append(result, current)
						break
					} else if current.End >= existing.End {
						// Current ends after existing - trim existing end
						existing.End = current.Start
						if existing.Start >= existing.End {
							// Existing is completely overridden
							result[i] = current
						} else {
							result = append(result, current)
						}
						break
					} else {
						// Current is in the middle of existing - split existing
						newSpan := HighlightSpan{
							Start:     current.End,
							End:       existing.End,
							Color:     existing.Color,
							PatternID: existing.PatternID,
							Priority:  existing.Priority,
						}
						existing.End = current.Start
						if existing.Start >= existing.End {
							// First part is empty, replace with current
							result[i] = current
						} else {
							result = append(result, current)
						}
						if newSpan.Start < newSpan.End {
							result = append(result, newSpan)
						}
						break
					}
				} else {
					// Existing has higher or equal priority - current is blocked
					if current.Start >= existing.Start && current.End <= existing.End {
						// Current is completely covered - skip it
						break
					} else if current.Start < existing.Start {
						// Current starts before existing - keep only the non-overlapping part
						current.End = existing.Start
						if current.Start < current.End {
							result = append(result, current)
						}
						break
					} else if current.End > existing.End {
						// Current extends past existing - keep only the non-overlapping part
						current.Start = existing.End
						// Continue checking against other spans
						overlapped = false
					} else {
						// Complex partial overlap - skip current
						break
					}
				}
			}
		}

		if !overlapped {
			result = append(result, current)
		}
	}

	// Sort final result by start position
	sort.Slice(result, func(i, j int) bool {
		return result[i].Start < result[j].Start
	})

	return result
}

// getPatternColor retrieves the ANSI color code for a pattern
func (he *HighlightEngine) getPatternColor(patternID, hexColor string) string {
	he.colorMutex.RLock()
	defer he.colorMutex.RUnlock()

	// Check if pattern has assigned color
	if ansiColor, exists := he.colorMap[patternID]; exists {
		return ansiColor
	}

	// Fallback to hex color if provided
	if hexColor != "" && isValidHexColor(hexColor) {
		return he.hexToANSI(hexColor)
	}

	return ""
}

// hexToANSI converts a hex color to the appropriate ANSI escape sequence
func (he *HighlightEngine) hexToANSI(hexColor string) string {
	if !he.enableColors {
		return ""
	}

	// Check cache first
	if cached, exists := he.compiledColors[hexColor]; exists {
		return cached
	}

	var ansiCode string

	switch he.colorCapability {
	case TrueColor:
		ansiCode = he.hexToTrueColor(hexColor)
	case Color256:
		ansiCode = he.hexTo256Color(hexColor)
	case BasicColors:
		ansiCode = he.hexToBasicColor(hexColor)
	default:
		return ""
	}

	// Cache the result
	he.compiledColors[hexColor] = ansiCode
	return ansiCode
}

// hexToTrueColor converts hex color to 24-bit ANSI sequence
func (he *HighlightEngine) hexToTrueColor(hexColor string) string {
	r, g, b, err := parseHexColor(hexColor)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

// hexTo256Color converts hex color to 256-color ANSI sequence
func (he *HighlightEngine) hexTo256Color(hexColor string) string {
	r, g, b, err := parseHexColor(hexColor)
	if err != nil {
		return ""
	}

	// Convert RGB to 256-color palette index
	colorIndex := he.rgbTo256(r, g, b)
	return fmt.Sprintf("\033[38;5;%dm", colorIndex)
}

// hexToBasicColor converts hex color to basic 8-color ANSI sequence
func (he *HighlightEngine) hexToBasicColor(hexColor string) string {
	r, g, b, err := parseHexColor(hexColor)
	if err != nil {
		return ""
	}

	// Map to closest basic color
	basicColor := he.rgbToBasic(r, g, b)
	return he.basicColorToANSI(basicColor)
}

// parseHexColor parses a hex color string to RGB values
func parseHexColor(hexColor string) (r, g, b int, err error) {
	if !strings.HasPrefix(hexColor, "#") {
		return 0, 0, 0, fmt.Errorf("hex color must start with #")
	}

	hex := hexColor[1:]

	if len(hex) == 3 {
		// Expand #RGB to #RRGGBB
		hex = fmt.Sprintf("%c%c%c%c%c%c", hex[0], hex[0], hex[1], hex[1], hex[2], hex[2])
	}

	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color length")
	}

	rgb, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, err
	}

	r = int((rgb >> 16) & 0xFF)
	g = int((rgb >> 8) & 0xFF)
	b = int(rgb & 0xFF)

	return r, g, b, nil
}

// rgbTo256 converts RGB values to 256-color palette index
func (he *HighlightEngine) rgbTo256(r, g, b int) int {
	// 256-color palette consists of:
	// 0-15: Basic colors
	// 16-231: 216-color cube (6x6x6)
	// 232-255: Grayscale

	// Check if it's grayscale
	if r == g && g == b {
		// Map to grayscale ramp (232-255)
		if r == 0 {
			return 16 // Pure black
		}
		if r == 255 {
			return 231 // Pure white
		}
		gray := (r - 8) / 10
		if gray < 0 {
			gray = 0
		}
		if gray > 23 {
			gray = 23
		}
		return 232 + gray
	}

	// Map to 6x6x6 color cube (16-231)
	// Each component is mapped to 0-5
	cr := r * 5 / 255
	cg := g * 5 / 255
	cb := b * 5 / 255

	return 16 + 36*cr + 6*cg + cb
}

// rgbToBasic maps RGB values to basic 8 colors
func (he *HighlightEngine) rgbToBasic(r, g, b int) string {
	// Check for very dark colors first (sum threshold)
	if r+g+b < 192 { // Raised threshold for better dark color detection
		return "black"
	}

	// Check for very bright colors
	if r > 200 && g > 200 && b > 200 {
		return "white"
	}

	// Check for composite colors first (they have priority)
	if r > 128 && g > 128 && b < 64 {
		return "yellow"
	}
	if r > 128 && b > 128 && g < 64 {
		return "magenta"
	}
	if g > 128 && b > 128 && r < 64 {
		return "cyan"
	}

	// Find dominant color component
	maxComponent := r
	color := "red"

	if g > maxComponent {
		maxComponent = g
		color = "green"
	}
	if b > maxComponent {
		maxComponent = b
		color = "blue"
	}

	return color
}

// basicColorToANSI converts basic color names to ANSI escape sequences
func (he *HighlightEngine) basicColorToANSI(color string) string {
	colorMap := map[string]string{
		"black":      "\033[30m",
		"red":        "\033[31m",
		"green":      "\033[32m",
		"yellow":     "\033[33m",
		"blue":       "\033[34m",
		"magenta":    "\033[35m",
		"cyan":       "\033[36m",
		"white":      "\033[37m",
		"bright_red": "\033[91m",
	}

	if ansi, exists := colorMap[color]; exists {
		return ansi
	}
	return "\033[37m" // Default to white
}

// hashStringToIndex creates a consistent index from a string for color assignment
func (he *HighlightEngine) hashStringToIndex(s string, maxIndex int) int {
	if maxIndex <= 0 {
		return 0
	}

	hash := 0
	for _, char := range s {
		hash = 31*hash + int(char)
	}

	if hash < 0 {
		hash = -hash
	}

	return hash % maxIndex
}

// GetResetSequence returns the ANSI reset sequence
func (he *HighlightEngine) GetResetSequence() string {
	return he.resetSequence
}

// IsColorEnabled returns whether colors are enabled
func (he *HighlightEngine) IsColorEnabled() bool {
	return he.enableColors
}

// SetColorEnabled allows manual control of color output
func (he *HighlightEngine) SetColorEnabled(enabled bool) {
	he.enableColors = enabled && he.colorCapability != NoColor
}

// GetColorStats returns statistics about color usage
func (he *HighlightEngine) GetColorStats() (assignedColors int, cachedColors int, capability string) {
	he.colorMutex.RLock()
	defer he.colorMutex.RUnlock()

	return len(he.colorMap), len(he.compiledColors), he.colorCapability.String()
}

// ClearColorCache clears the compiled color cache to free memory
func (he *HighlightEngine) ClearColorCache() {
	he.colorMutex.Lock()
	defer he.colorMutex.Unlock()

	he.compiledColors = make(map[string]string)
}
