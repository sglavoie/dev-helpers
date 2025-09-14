// Package ui provides the FilterPaneModel component for managing include/exclude patterns
// in the qf Interactive Log Filter Composer. This component implements vim-style modal
// interface with real-time pattern management and validation.
package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// FilterPaneModel represents the filter pattern management component.
// It displays include and exclude patterns with vim-style navigation and editing.
type FilterPaneModel struct {
	// State management
	filterSet FilterSet // Current filter configuration
	mode      Mode      // Current mode (Normal/Insert)
	focused   bool      // Whether this component has focus
	width     int       // Available width for rendering
	height    int       // Available height for rendering

	// Navigation and selection
	cursor       int                    // Current cursor position in pattern list
	selectedType core.FilterPatternType // Currently selected pattern type (Include/Exclude)
	includeCount int                    // Number of include patterns
	excludeCount int                    // Number of exclude patterns

	// Input handling
	inputBuffer  string // Text being entered in insert mode
	errorMessage string // Current error message to display
	errorTimeout int    // Countdown for error display

	// Visual state
	showHelp     bool   // Whether to show keybinding help
	lastKeyPress string // Last key pressed (for status display)

	// Component identification
	componentType string // Component type identifier
}

// NewFilterPaneModel creates a new FilterPaneModel with default configuration
func NewFilterPaneModel() *FilterPaneModel {
	return &FilterPaneModel{
		filterSet:     FilterSet{Name: "default", Include: []core.FilterPattern{}, Exclude: []core.FilterPattern{}},
		mode:          ModeNormal,
		focused:       false,
		width:         80,
		height:        24,
		cursor:        0,
		selectedType:  core.FilterInclude,
		inputBuffer:   "",
		errorMessage:  "",
		errorTimeout:  0,
		showHelp:      false,
		lastKeyPress:  "",
		componentType: "filter_pane",
	}
}

// Init implements tea.Model interface - initializes the component
func (m FilterPaneModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model interface - handles messages and user input
func (m FilterPaneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		cmd = m.handleKeyPress(msg)

	case FilterUpdateMsg:
		// Update our filter set if the update came from another component
		if msg.Source != "filter_pane" {
			m.filterSet = msg.FilterSet
			m.updateCounts()
		}

	case ModeTransitionMsg:
		// Handle mode transitions from other components or global state
		if msg.NewMode != m.mode {
			m.mode = msg.NewMode
			if m.mode == ModeNormal {
				// Clear input buffer when leaving insert mode
				m.inputBuffer = ""
			}
		}

	case ErrorMsg:
		// Display error messages from other components
		if msg.Source != "filter_pane" {
			m.errorMessage = msg.Message
			m.errorTimeout = 5 // Show for 5 update cycles
		}

	case FocusMsg:
		// Handle focus changes
		m.focused = msg.Component == "filter_pane"
	}

	// Update error message timeout
	if m.errorTimeout > 0 {
		m.errorTimeout--
		if m.errorTimeout == 0 {
			m.errorMessage = ""
		}
	}

	return m, cmd
}

// handleKeyPress processes keyboard input based on current mode
func (m *FilterPaneModel) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	m.lastKeyPress = msg.String()

	switch m.mode {
	case ModeNormal:
		return m.handleNormalModeKey(msg)
	case ModeInsert:
		return m.handleInsertModeKey(msg)
	default:
		return nil
	}
}

// handleNormalModeKey processes keyboard input in Normal mode
func (m *FilterPaneModel) handleNormalModeKey(msg tea.KeyMsg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg.String() {
	case "q", "ctrl+c":
		// Quit application (propagate up)
		return tea.Quit

	case "?":
		// Toggle help display
		m.showHelp = !m.showHelp

	case "tab":
		// Switch focus to next component
		cmds = append(cmds, func() tea.Msg {
			return FocusMsg{Component: "viewer"}
		})

	case "j", "down":
		// Navigate down in pattern list
		m.moveCursorDown()

	case "k", "up":
		// Navigate up in pattern list
		m.moveCursorUp()

	case "h", "left":
		// Switch to Include patterns
		m.selectedType = core.FilterInclude
		m.cursor = 0

	case "l", "right":
		// Switch to Exclude patterns
		m.selectedType = core.FilterExclude
		m.cursor = 0

	case "i":
		// Enter insert mode to add new pattern
		m.mode = ModeInsert
		m.inputBuffer = ""
		cmds = append(cmds, func() tea.Msg {
			return ModeTransitionMsg{NewMode: ModeInsert, PrevMode: ModeNormal, Context: "filter_pane_add_pattern", Timestamp: time.Now()}
		})

	case "e":
		// Edit selected pattern (enter insert mode with current pattern)
		if selectedPattern := m.getSelectedPattern(); selectedPattern != nil {
			m.mode = ModeInsert
			m.inputBuffer = selectedPattern.Expression
			cmds = append(cmds, func() tea.Msg {
				return ModeTransitionMsg{NewMode: ModeInsert, PrevMode: ModeNormal, Context: "filter_pane_edit_pattern", Timestamp: time.Now()}
			})
		}

	case "d", "x":
		// Delete selected pattern
		if m.deleteSelectedPattern() {
			cmds = append(cmds, m.emitFilterUpdate())
		}

	case "D":
		// Delete all patterns of selected type
		if m.clearSelectedType() {
			cmds = append(cmds, m.emitFilterUpdate())
		}

	case "t":
		// Toggle pattern type (Include/Exclude) for selected pattern
		if m.toggleSelectedPatternType() {
			cmds = append(cmds, m.emitFilterUpdate())
		}

	case "c":
		// Clear all patterns
		if m.clearAllPatterns() {
			cmds = append(cmds, m.emitFilterUpdate())
		}

	case "r":
		// Refresh/validate all patterns
		cmds = append(cmds, m.validateAllPatterns())
	}

	return tea.Batch(cmds...)
}

// handleInsertModeKey processes keyboard input in Insert mode
func (m *FilterPaneModel) handleInsertModeKey(msg tea.KeyMsg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg.String() {
	case "esc":
		// Exit insert mode without saving
		m.mode = ModeNormal
		m.inputBuffer = ""
		cmds = append(cmds, func() tea.Msg {
			return ModeTransitionMsg{NewMode: ModeNormal, PrevMode: ModeInsert, Context: "escape_key_pressed", Timestamp: time.Now()}
		})

	case "enter":
		// Save pattern and exit insert mode
		if m.savePattern() {
			cmds = append(cmds, m.emitFilterUpdate())
		}
		m.mode = ModeNormal
		m.inputBuffer = ""
		cmds = append(cmds, func() tea.Msg {
			return ModeTransitionMsg{NewMode: ModeNormal, PrevMode: ModeInsert, Context: "pattern_saved", Timestamp: time.Now()}
		})

	case "ctrl+c":
		// Cancel and exit insert mode
		m.mode = ModeNormal
		m.inputBuffer = ""
		cmds = append(cmds, func() tea.Msg {
			return ModeTransitionMsg{NewMode: ModeNormal, PrevMode: ModeInsert, Context: "input_cancelled", Timestamp: time.Now()}
		})

	case "backspace":
		// Remove last character from input buffer
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}

	case "ctrl+u":
		// Clear entire input buffer
		m.inputBuffer = ""

	case "ctrl+w":
		// Delete last word from input buffer
		words := strings.Fields(m.inputBuffer)
		if len(words) > 0 {
			words = words[:len(words)-1]
			m.inputBuffer = strings.Join(words, " ")
		}

	default:
		// Add typed character to input buffer
		if len(msg.Runes) > 0 {
			m.inputBuffer += string(msg.Runes)
		}
	}

	return tea.Batch(cmds...)
}

// View implements tea.Model interface - renders the component
func (m FilterPaneModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	var content strings.Builder

	// Header
	title := m.renderTitle()
	content.WriteString(title + "\n")

	// Pattern lists
	includeSection := m.renderIncludePatterns()
	excludeSection := m.renderExcludePatterns()

	// Main content area based on selected type
	if m.selectedType == core.FilterInclude {
		content.WriteString(includeSection)
	} else {
		content.WriteString(excludeSection)
	}

	// Input area (in insert mode)
	if m.mode == ModeInsert {
		inputArea := m.renderInputArea()
		content.WriteString("\n" + inputArea)
	}

	// Status line
	statusLine := m.renderStatusLine()
	content.WriteString("\n" + statusLine)

	// Help (if enabled)
	if m.showHelp {
		helpText := m.renderHelp()
		content.WriteString("\n" + helpText)
	}

	// Apply container styling
	containerStyle := m.getContainerStyle()
	return containerStyle.Render(content.String())
}

// renderTitle creates the pane title with current mode and stats
func (m FilterPaneModel) renderTitle() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("4")).
		Padding(0, 1)

	if !m.focused {
		titleStyle = titleStyle.
			Foreground(lipgloss.Color("7")).
			Background(lipgloss.Color("8"))
	}

	title := fmt.Sprintf("Filters [%s] - Include: %d, Exclude: %d",
		m.mode.String(), m.includeCount, m.excludeCount)

	return titleStyle.Render(title)
}

// renderIncludePatterns renders the include patterns section
func (m FilterPaneModel) renderIncludePatterns() string {
	var content strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")). // Green for include
		Underline(m.selectedType == core.FilterInclude)

	content.WriteString(headerStyle.Render("Include Patterns:") + "\n")

	if len(m.filterSet.Include) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("8"))
		content.WriteString(emptyStyle.Render("  (no include patterns)") + "\n")
	} else {
		for i, pattern := range m.filterSet.Include {
			line := m.renderPatternLine(pattern, i, core.FilterInclude)
			content.WriteString(line + "\n")
		}
	}

	return content.String()
}

// renderExcludePatterns renders the exclude patterns section
func (m FilterPaneModel) renderExcludePatterns() string {
	var content strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("9")). // Red for exclude
		Underline(m.selectedType == core.FilterExclude)

	content.WriteString(headerStyle.Render("Exclude Patterns:") + "\n")

	if len(m.filterSet.Exclude) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("8"))
		content.WriteString(emptyStyle.Render("  (no exclude patterns)") + "\n")
	} else {
		for i, pattern := range m.filterSet.Exclude {
			line := m.renderPatternLine(pattern, i, core.FilterExclude)
			content.WriteString(line + "\n")
		}
	}

	return content.String()
}

// renderPatternLine renders a single pattern line with appropriate styling
func (m FilterPaneModel) renderPatternLine(pattern core.FilterPattern, index int, patternType core.FilterPatternType) string {
	// Determine if this line is selected
	isSelected := m.selectedType == patternType && m.cursor == index
	isFocused := m.focused && isSelected

	// Base style
	lineStyle := lipgloss.NewStyle().
		Width(m.width - 4). // Account for padding
		PaddingLeft(2)

	// Apply selection styling
	if isFocused {
		lineStyle = lineStyle.
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("15"))
	} else if isSelected {
		lineStyle = lineStyle.
			Background(lipgloss.Color("8")).
			Foreground(lipgloss.Color("15"))
	}

	// Pattern status indicators
	statusIndicator := "✓"
	if !pattern.IsValid {
		statusIndicator = "✗"
		lineStyle = lineStyle.Foreground(lipgloss.Color("9")) // Red for invalid
	}

	// Format pattern text
	patternText := fmt.Sprintf("%s %s", statusIndicator, pattern.Expression)

	// Add match count if available
	if pattern.MatchCount > 0 {
		patternText += fmt.Sprintf(" (%d matches)", pattern.MatchCount)
	}

	return lineStyle.Render(patternText)
}

// renderInputArea renders the input area for insert mode
func (m FilterPaneModel) renderInputArea() string {
	if m.mode != ModeInsert {
		return ""
	}

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(0, 1).
		Width(m.width - 2)

	prompt := fmt.Sprintf("Add %s pattern: ", strings.ToLower(m.selectedType.String()))
	cursor := "│" // Blinking cursor representation

	content := prompt + m.inputBuffer + cursor

	return inputStyle.Render(content)
}

// renderStatusLine renders the status line with current information
func (m FilterPaneModel) renderStatusLine() string {
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Background(lipgloss.Color("0")).
		Width(m.width).
		Padding(0, 1)

	var statusText string

	if m.errorMessage != "" {
		// Show error message
		statusStyle = statusStyle.Foreground(lipgloss.Color("9"))
		statusText = fmt.Sprintf("ERROR: %s", m.errorMessage)
	} else if m.lastKeyPress != "" {
		// Show last key press and mode info
		statusText = fmt.Sprintf("Mode: %s | Last: %s | Focus: %t",
			m.mode.String(), m.lastKeyPress, m.focused)
	} else {
		// Show general status
		statusText = fmt.Sprintf("Mode: %s | Patterns: %d total",
			m.mode.String(), len(m.filterSet.Include)+len(m.filterSet.Exclude))
	}

	return statusStyle.Render(statusText)
}

// renderHelp renders the keybinding help text
func (m FilterPaneModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(1).
		Margin(1, 0)

	helpText := ""
	if m.mode == ModeNormal {
		helpText = "Normal Mode:\n" +
			"j/k - Navigate patterns  h/l - Switch Include/Exclude\n" +
			"i - Add pattern  e - Edit pattern  d - Delete pattern\n" +
			"t - Toggle pattern type  c - Clear all  r - Refresh\n" +
			"tab - Next component  ? - Toggle help  q - Quit"
	} else {
		helpText = "Insert Mode:\n" +
			"Type to enter pattern  Enter - Save  Esc - Cancel\n" +
			"Ctrl+U - Clear all  Ctrl+W - Delete word  Backspace - Delete char"
	}

	return helpStyle.Render(helpText)
}

// getContainerStyle returns the main container styling
func (m FilterPaneModel) getContainerStyle() lipgloss.Style {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.width).
		Height(m.height)

	if m.focused {
		containerStyle = containerStyle.BorderForeground(lipgloss.Color("4"))
	} else {
		containerStyle = containerStyle.BorderForeground(lipgloss.Color("8"))
	}

	return containerStyle
}

// Navigation helper methods

// moveCursorDown moves the cursor down in the current pattern list
func (m *FilterPaneModel) moveCursorDown() {
	maxPos := m.getMaxCursorPosition()
	if m.cursor < maxPos {
		m.cursor++
	}
}

// moveCursorUp moves the cursor up in the current pattern list
func (m *FilterPaneModel) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// getMaxCursorPosition returns the maximum cursor position for current pattern type
func (m *FilterPaneModel) getMaxCursorPosition() int {
	if m.selectedType == core.FilterInclude {
		return len(m.filterSet.Include) - 1
	}
	return len(m.filterSet.Exclude) - 1
}

// getSelectedPattern returns the currently selected pattern
func (m *FilterPaneModel) getSelectedPattern() *core.FilterPattern {
	if m.selectedType == core.FilterInclude {
		if m.cursor >= 0 && m.cursor < len(m.filterSet.Include) {
			return &m.filterSet.Include[m.cursor]
		}
	} else {
		if m.cursor >= 0 && m.cursor < len(m.filterSet.Exclude) {
			return &m.filterSet.Exclude[m.cursor]
		}
	}
	return nil
}

// Pattern management methods

// generatePatternID creates a simple unique ID for patterns
func generatePatternID() string {
	return fmt.Sprintf("pattern-%d", time.Now().UnixNano())
}

// savePattern adds or updates a pattern based on current input
func (m *FilterPaneModel) savePattern() bool {
	if strings.TrimSpace(m.inputBuffer) == "" {
		m.errorMessage = "Pattern cannot be empty"
		m.errorTimeout = 5
		return false
	}

	// Validate regex pattern
	_, err := regexp.Compile(m.inputBuffer)
	if err != nil {
		m.errorMessage = "Invalid regex pattern"
		m.errorTimeout = 5
		return false
	}

	// Create new pattern
	pattern := core.FilterPattern{
		ID:         generatePatternID(),
		Expression: m.inputBuffer,
		Type:       m.selectedType,
		Color:      "",
		Created:    time.Now(),
		IsValid:    true,
		MatchCount: 0,
	}

	// Add to filter set
	if m.selectedType == core.FilterInclude {
		m.filterSet.Include = append(m.filterSet.Include, pattern)
	} else {
		m.filterSet.Exclude = append(m.filterSet.Exclude, pattern)
	}

	m.updateCounts()
	return true
}

// deleteSelectedPattern removes the currently selected pattern
func (m *FilterPaneModel) deleteSelectedPattern() bool {
	selectedPattern := m.getSelectedPattern()
	if selectedPattern == nil {
		return false
	}

	// Remove pattern from the appropriate slice
	if m.selectedType == core.FilterInclude {
		if m.cursor >= 0 && m.cursor < len(m.filterSet.Include) {
			m.filterSet.Include = append(m.filterSet.Include[:m.cursor], m.filterSet.Include[m.cursor+1:]...)
		}
	} else {
		if m.cursor >= 0 && m.cursor < len(m.filterSet.Exclude) {
			m.filterSet.Exclude = append(m.filterSet.Exclude[:m.cursor], m.filterSet.Exclude[m.cursor+1:]...)
		}
	}

	// Adjust cursor position
	maxPos := m.getMaxCursorPosition()
	if m.cursor > maxPos {
		m.cursor = maxPos
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	m.updateCounts()
	return true
}

// clearSelectedType removes all patterns of the currently selected type
func (m *FilterPaneModel) clearSelectedType() bool {
	if m.selectedType == core.FilterInclude {
		m.filterSet.Include = []core.FilterPattern{}
	} else {
		m.filterSet.Exclude = []core.FilterPattern{}
	}

	m.cursor = 0
	m.updateCounts()
	return true
}

// clearAllPatterns removes all patterns from the filter set
func (m *FilterPaneModel) clearAllPatterns() bool {
	m.filterSet.Include = []core.FilterPattern{}
	m.filterSet.Exclude = []core.FilterPattern{}
	m.cursor = 0
	m.updateCounts()
	return true
}

// toggleSelectedPatternType switches the selected pattern between Include/Exclude
func (m *FilterPaneModel) toggleSelectedPatternType() bool {
	selectedPattern := m.getSelectedPattern()
	if selectedPattern == nil {
		return false
	}

	// Create new pattern with opposite type
	newType := core.FilterInclude
	if selectedPattern.Type == core.FilterInclude {
		newType = core.FilterExclude
	}

	newPattern := core.FilterPattern{
		ID:         generatePatternID(),
		Expression: selectedPattern.Expression,
		Type:       newType,
		Color:      selectedPattern.Color,
		Created:    time.Now(),
		IsValid:    selectedPattern.IsValid,
		MatchCount: selectedPattern.MatchCount,
	}

	// Remove old pattern and add new one
	if m.selectedType == core.FilterInclude {
		if m.cursor >= 0 && m.cursor < len(m.filterSet.Include) {
			m.filterSet.Include = append(m.filterSet.Include[:m.cursor], m.filterSet.Include[m.cursor+1:]...)
			m.filterSet.Exclude = append(m.filterSet.Exclude, newPattern)
			m.selectedType = core.FilterExclude
			m.cursor = len(m.filterSet.Exclude) - 1
		}
	} else {
		if m.cursor >= 0 && m.cursor < len(m.filterSet.Exclude) {
			m.filterSet.Exclude = append(m.filterSet.Exclude[:m.cursor], m.filterSet.Exclude[m.cursor+1:]...)
			m.filterSet.Include = append(m.filterSet.Include, newPattern)
			m.selectedType = core.FilterInclude
			m.cursor = len(m.filterSet.Include) - 1
		}
	}

	m.updateCounts()
	return true
}

// validateAllPatterns validates all patterns and updates their status
func (m *FilterPaneModel) validateAllPatterns() tea.Cmd {
	// Validate include patterns
	for i := range m.filterSet.Include {
		_, err := regexp.Compile(m.filterSet.Include[i].Expression)
		m.filterSet.Include[i].IsValid = (err == nil)
		if err != nil {
			m.errorMessage = fmt.Sprintf("Include pattern %d invalid: %v", i+1, err)
			m.errorTimeout = 5
		}
	}

	// Validate exclude patterns
	for i := range m.filterSet.Exclude {
		_, err := regexp.Compile(m.filterSet.Exclude[i].Expression)
		m.filterSet.Exclude[i].IsValid = (err == nil)
		if err != nil {
			m.errorMessage = fmt.Sprintf("Exclude pattern %d invalid: %v", i+1, err)
			m.errorTimeout = 5
		}
	}

	return m.emitFilterUpdate()
}

// updateCounts updates the internal pattern counts
func (m *FilterPaneModel) updateCounts() {
	m.includeCount = len(m.filterSet.Include)
	m.excludeCount = len(m.filterSet.Exclude)
}

// emitFilterUpdate creates a command to emit a FilterUpdateMsg
func (m *FilterPaneModel) emitFilterUpdate() tea.Cmd {
	return func() tea.Msg {
		return FilterUpdateMsg{FilterSet: m.filterSet, Source: "filter_pane", Timestamp: time.Now()}
	}
}

// MessageHandler interface implementation

// HandleMessage processes incoming messages
func (m FilterPaneModel) HandleMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.Update(msg)
}

// GetComponentType returns the component type identifier
func (m FilterPaneModel) GetComponentType() string {
	return m.componentType
}

// IsMessageSupported returns whether this component supports a message type
func (m FilterPaneModel) IsMessageSupported(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyMsg,
		tea.WindowSizeMsg,
		FilterUpdateMsg,
		ModeTransitionMsg,
		ErrorMsg,
		FocusMsg:
		return true
	default:
		return false
	}
}

// Public API methods for external component interaction

// SetFilterSet updates the component's filter set
func (m *FilterPaneModel) SetFilterSet(filterSet FilterSet) {
	m.filterSet = filterSet
	m.updateCounts()
}

// GetFilterSet returns the current filter set
func (m FilterPaneModel) GetFilterSet() FilterSet {
	return m.filterSet
}

// SetFocused updates the focused state
func (m *FilterPaneModel) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the component is focused
func (m FilterPaneModel) IsFocused() bool {
	return m.focused
}

// GetMode returns the current mode
func (m FilterPaneModel) GetMode() Mode {
	return m.mode
}

// SetMode updates the current mode
func (m *FilterPaneModel) SetMode(mode Mode) {
	m.mode = mode
	if mode == ModeNormal {
		m.inputBuffer = ""
	}
}
