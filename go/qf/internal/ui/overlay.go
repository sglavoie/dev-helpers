// Package ui provides the OverlayModel component for modal dialogs and overlays.
// This includes pattern testing with live preview, confirmations, help, and error dialogs.
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

// OverlayType represents different types of overlays
type OverlayType int

const (
	OverlayNone OverlayType = iota
	OverlayFileOpen
	OverlayPatternTest
	OverlayConfirmation
	OverlayError
	OverlayHelp
)

// PatternTestState tracks the state of pattern testing
type PatternTestState struct {
	Pattern         string
	PatternType     core.PatternType
	IsValid         bool
	ValidationError string
	Matches         []PatternMatch
	MatchCount      int
	ProcessingTime  time.Duration
	SelectedType    int // 0=Include, 1=Exclude
}

// PatternMatch represents a match in the preview content
type PatternMatch struct {
	LineNumber int
	Start      int
	End        int
	Text       string
	Context    string
}

// ConfirmationState tracks confirmation dialog state
type ConfirmationState struct {
	Title    string
	Message  string
	Buttons  []string
	Selected int
	Callback func(result string) tea.Msg
}

// HelpState tracks help overlay state
type HelpState struct {
	Context   string
	Sections  []HelpSection
	Selected  int
	ScrollPos int
}

// HelpSection represents a section in the help overlay
type HelpSection struct {
	Title       string
	Keybindings []HelpKeybinding
}

// HelpKeybinding represents a single keybinding in help
type HelpKeybinding struct {
	Key         string
	Description string
}

// OverlayModel manages modal overlays and dialogs with rich interactive features
type OverlayModel struct {
	// Core overlay state
	overlayType OverlayType
	visible     bool
	width       int
	height      int
	termWidth   int
	termHeight  int

	// Generic content
	title       string
	content     string
	inputBuffer string
	cursorPos   int

	// Pattern testing state
	patternTest    PatternTestState
	previewContent []string          // Sample content for testing patterns
	filterEngine   core.FilterEngine // For live pattern matching

	// Confirmation dialog state
	confirmation ConfirmationState

	// Help overlay state
	help HelpState

	// File open state
	fileOpenCallback func(result string) tea.Msg

	// Styling
	styles OverlayStyles
}

// OverlayStyles contains lipgloss styles for the overlay
type OverlayStyles struct {
	Base        lipgloss.Style
	Title       lipgloss.Style
	Border      lipgloss.Style
	Input       lipgloss.Style
	InputFocus  lipgloss.Style
	Button      lipgloss.Style
	ButtonFocus lipgloss.Style
	Preview     lipgloss.Style
	Match       lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Help        lipgloss.Style
	Backdrop    lipgloss.Style
}

// NewOverlayModel creates a new OverlayModel with default styling and configuration
func NewOverlayModel(filterEngine core.FilterEngine) *OverlayModel {
	m := &OverlayModel{
		overlayType:  OverlayNone,
		visible:      false,
		width:        80,
		height:       20,
		termWidth:    100,
		termHeight:   30,
		filterEngine: filterEngine,
		patternTest: PatternTestState{
			PatternType:  core.Include,
			SelectedType: 0,
		},
		styles: createOverlayStyles(),
	}

	// Initialize help content
	m.initializeHelp()

	return m
}

// createOverlayStyles initializes the lipgloss styles for the overlay
func createOverlayStyles() OverlayStyles {
	return OverlayStyles{
		Base: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("15")).
			Padding(1, 2).
			Margin(1),

		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true).
			Align(lipgloss.Center),

		Border: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("12")),

		Input: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1),

		InputFocus: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("39")).
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1),

		Button: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 2).
			Margin(0, 1),

		ButtonFocus: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("39")).
			Background(lipgloss.Color("39")).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Padding(0, 2).
			Margin(0, 1),

		Preview: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Background(lipgloss.Color("234")).
			Foreground(lipgloss.Color("15")).
			Padding(1).
			Margin(1, 0),

		Match: lipgloss.NewStyle().
			Background(lipgloss.Color("11")).
			Foreground(lipgloss.Color("0")).
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true),

		Backdrop: lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("240")),
	}
}

// Init implements tea.Model interface
func (m *OverlayModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model interface
func (m *OverlayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case ResizeMsg:
		// Update terminal dimensions and resize overlay accordingly
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.resizeOverlay()
		return m, nil

	case WindowResizeMsg:
		// Update terminal dimensions and resize overlay accordingly
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.resizeOverlay()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case PatternTestResultMsg:
		// Handle live pattern testing results
		if m.overlayType == OverlayPatternTest {
			m.patternTest.Matches = msg.Matches
			m.patternTest.MatchCount = msg.MatchCount
			m.patternTest.ProcessingTime = msg.ProcessingTime
			m.patternTest.IsValid = msg.IsValid
			m.patternTest.ValidationError = msg.ValidationError
		}
		return m, nil
	}

	return m, nil
}

// resizeOverlay adjusts overlay dimensions based on terminal size
func (m *OverlayModel) resizeOverlay() {
	switch m.overlayType {
	case OverlayPatternTest:
		m.width = min(100, m.termWidth-6)
		m.height = min(25, m.termHeight-4)
	case OverlayHelp:
		m.width = min(120, m.termWidth-4)
		m.height = min(30, m.termHeight-2)
	default:
		m.width = min(80, m.termWidth-8)
		m.height = min(20, m.termHeight-6)
	}
}

// handleKeyPress processes key presses when overlay is visible
func (m *OverlayModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global overlay keys
	switch key {
	case "ctrl+c":
		return m.hide(), tea.Quit
	case "esc":
		// Universal escape to close overlay
		return m.hide(), nil
	}

	switch m.overlayType {
	case OverlayFileOpen:
		return m.handleFileOpenKeys(key)
	case OverlayPatternTest:
		return m.handlePatternTestKeys(key)
	case OverlayConfirmation:
		return m.handleConfirmationKeys(key)
	case OverlayError:
		return m.handleErrorKeys(key)
	case OverlayHelp:
		return m.handleHelpKeys(key)
	default:
		return m, nil
	}
}

// handleFileOpenKeys handles keys for file open dialog
func (m *OverlayModel) handleFileOpenKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		if m.inputBuffer != "" {
			result := m.inputBuffer
			cmd := m.hide()
			if m.fileOpenCallback != nil {
				return cmd, func() tea.Msg { return m.fileOpenCallback(result) }
			}
			return cmd, nil
		}

	case "backspace", "ctrl+h":
		if len(m.inputBuffer) > 0 && m.cursorPos > 0 {
			m.inputBuffer = m.inputBuffer[:m.cursorPos-1] + m.inputBuffer[m.cursorPos:]
			m.cursorPos--
		}

	case "left":
		if m.cursorPos > 0 {
			m.cursorPos--
		}

	case "right":
		if m.cursorPos < len(m.inputBuffer) {
			m.cursorPos++
		}

	case "home", "ctrl+a":
		m.cursorPos = 0

	case "end", "ctrl+e":
		m.cursorPos = len(m.inputBuffer)

	case "ctrl+u":
		// Clear entire input
		m.inputBuffer = ""
		m.cursorPos = 0

	default:
		// Add character to input
		if len(key) == 1 && key >= " " && key <= "~" {
			m.inputBuffer = m.inputBuffer[:m.cursorPos] + key + m.inputBuffer[m.cursorPos:]
			m.cursorPos++
		}
	}

	return m, nil
}

// handlePatternTestKeys handles keys for pattern test dialog with live preview
func (m *OverlayModel) handlePatternTestKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Add pattern if valid
		if m.patternTest.IsValid && m.patternTest.Pattern != "" {
			patternType := core.Include
			if m.patternTest.SelectedType == 1 {
				patternType = core.Exclude
			}

			result := PatternConfirmedMsg{
				Pattern: m.patternTest.Pattern,
				Type:    patternType,
				Color:   "#39AFFF", // Default blue
			}

			cmd := m.hide()
			return cmd, func() tea.Msg { return result }
		}

	case "tab":
		// Switch between Include/Exclude
		m.patternTest.SelectedType = (m.patternTest.SelectedType + 1) % 2
		if m.patternTest.SelectedType == 0 {
			m.patternTest.PatternType = core.Include
		} else {
			m.patternTest.PatternType = core.Exclude
		}
		return m, nil

	case "backspace", "ctrl+h":
		if len(m.patternTest.Pattern) > 0 && m.cursorPos > 0 {
			m.patternTest.Pattern = m.patternTest.Pattern[:m.cursorPos-1] + m.patternTest.Pattern[m.cursorPos:]
			m.cursorPos--
			m.inputBuffer = m.patternTest.Pattern
			return m, m.testPatternLive()
		}

	case "left":
		if m.cursorPos > 0 {
			m.cursorPos--
		}

	case "right":
		if m.cursorPos < len(m.patternTest.Pattern) {
			m.cursorPos++
		}

	case "home", "ctrl+a":
		m.cursorPos = 0

	case "end", "ctrl+e":
		m.cursorPos = len(m.patternTest.Pattern)

	case "ctrl+u":
		// Clear entire pattern
		m.patternTest.Pattern = ""
		m.inputBuffer = ""
		m.cursorPos = 0
		m.patternTest.Matches = nil
		m.patternTest.MatchCount = 0
		m.patternTest.IsValid = false
		m.patternTest.ValidationError = ""
		return m, nil

	default:
		// Add character to pattern and trigger live preview
		if len(key) == 1 && key >= " " && key <= "~" {
			m.patternTest.Pattern = m.patternTest.Pattern[:m.cursorPos] + key + m.patternTest.Pattern[m.cursorPos:]
			m.cursorPos++
			m.inputBuffer = m.patternTest.Pattern
			return m, m.testPatternLive()
		}
	}

	return m, nil
}

// testPatternLive performs live pattern testing as user types
func (m *OverlayModel) testPatternLive() tea.Cmd {
	if m.patternTest.Pattern == "" {
		return func() tea.Msg {
			return PatternTestResultMsg{
				IsValid:    false,
				Matches:    nil,
				MatchCount: 0,
			}
		}
	}

	return func() tea.Msg {
		// Validate pattern
		compiled, err := regexp.Compile(m.patternTest.Pattern)
		if err != nil {
			return PatternTestResultMsg{
				IsValid:         false,
				ValidationError: err.Error(),
				Matches:         nil,
				MatchCount:      0,
			}
		}

		// Test against preview content
		startTime := time.Now()
		var matches []PatternMatch
		matchCount := 0

		for lineNum, line := range m.previewContent {
			allMatches := compiled.FindAllStringIndex(line, -1)
			for _, match := range allMatches {
				matches = append(matches, PatternMatch{
					LineNumber: lineNum + 1,
					Start:      match[0],
					End:        match[1],
					Text:       line[match[0]:match[1]],
					Context:    line,
				})
				matchCount++
			}
		}

		processingTime := time.Since(startTime)

		return PatternTestResultMsg{
			IsValid:        true,
			Matches:        matches,
			MatchCount:     matchCount,
			ProcessingTime: processingTime,
		}
	}
}

// handleConfirmationKeys handles keys for confirmation dialog
func (m *OverlayModel) handleConfirmationKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n", "q":
		// Default to first button (usually "No" or "Cancel")
		result := "cancel"
		if len(m.confirmation.Buttons) > 0 {
			result = strings.ToLower(m.confirmation.Buttons[0])
		}
		cmd := m.hide()
		if m.confirmation.Callback != nil {
			return cmd, func() tea.Msg { return m.confirmation.Callback(result) }
		}
		return cmd, nil

	case "enter", "y":
		// Use selected button
		result := "yes"
		if len(m.confirmation.Buttons) > m.confirmation.Selected {
			result = strings.ToLower(m.confirmation.Buttons[m.confirmation.Selected])
		}
		cmd := m.hide()
		if m.confirmation.Callback != nil {
			return cmd, func() tea.Msg { return m.confirmation.Callback(result) }
		}
		return cmd, nil

	case "left", "h", "shift+tab":
		if len(m.confirmation.Buttons) > 0 {
			m.confirmation.Selected = (m.confirmation.Selected - 1 + len(m.confirmation.Buttons)) % len(m.confirmation.Buttons)
		}

	case "right", "l", "tab":
		if len(m.confirmation.Buttons) > 0 {
			m.confirmation.Selected = (m.confirmation.Selected + 1) % len(m.confirmation.Buttons)
		}

	case "1", "2", "3", "4", "5":
		// Direct button selection
		if buttonIdx := int(key[0] - '1'); buttonIdx < len(m.confirmation.Buttons) {
			m.confirmation.Selected = buttonIdx
			result := strings.ToLower(m.confirmation.Buttons[buttonIdx])
			cmd := m.hide()
			if m.confirmation.Callback != nil {
				return cmd, func() tea.Msg { return m.confirmation.Callback(result) }
			}
			return cmd, nil
		}
	}

	return m, nil
}

// handleErrorKeys handles keys for error dialog
func (m *OverlayModel) handleErrorKeys(key string) (tea.Model, tea.Cmd) {
	// Any key closes error dialog
	switch key {
	case "ctrl+c":
		return m.hide(), tea.Quit
	default:
		return m.hide(), nil
	}
}

// handleHelpKeys handles keys for help overlay
func (m *OverlayModel) handleHelpKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.help.Selected > 0 {
			m.help.Selected--
		}
		return m, nil

	case "down", "j":
		if m.help.Selected < len(m.help.Sections)-1 {
			m.help.Selected++
		}
		return m, nil

	case "page_up":
		m.help.ScrollPos = max(0, m.help.ScrollPos-5)
		return m, nil

	case "page_down":
		m.help.ScrollPos = min(len(m.help.Sections)-1, m.help.ScrollPos+5)
		return m, nil

	case "home":
		m.help.Selected = 0
		m.help.ScrollPos = 0
		return m, nil

	case "end":
		m.help.Selected = len(m.help.Sections) - 1
		m.help.ScrollPos = max(0, len(m.help.Sections)-5)
		return m, nil

	default:
		return m, nil
	}
}

// View implements tea.Model interface with enhanced lipgloss styling
func (m *OverlayModel) View() string {
	if !m.visible {
		return ""
	}

	// Render overlay content
	var overlayContent string
	switch m.overlayType {
	case OverlayFileOpen:
		overlayContent = m.renderFileOpenDialog()
	case OverlayPatternTest:
		overlayContent = m.renderPatternTestDialog()
	case OverlayConfirmation:
		overlayContent = m.renderConfirmationDialog()
	case OverlayError:
		overlayContent = m.renderErrorDialog()
	case OverlayHelp:
		overlayContent = m.renderHelpDialog()
	default:
		return ""
	}

	// Center the overlay
	return m.centerOverlay(overlayContent)
}

// centerOverlay centers the overlay content on the terminal
func (m *OverlayModel) centerOverlay(content string) string {
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	contentWidth := 0
	for _, line := range lines {
		if len(line) > contentWidth {
			contentWidth = len(line)
		}
	}

	// Calculate centering with safety checks
	verticalOffset := max(0, (m.termHeight-contentHeight)/2)
	horizontalOffset := max(0, (m.termWidth-contentWidth)/2)

	// Add vertical padding
	var result []string
	for i := 0; i < verticalOffset; i++ {
		result = append(result, "")
	}

	// Add horizontal padding and content
	for _, line := range lines {
		paddedLine := strings.Repeat(" ", horizontalOffset) + line
		result = append(result, paddedLine)
	}

	return strings.Join(result, "\n")
}

// renderFileOpenDialog renders the file open dialog
func (m *OverlayModel) renderFileOpenDialog() string {
	// Title
	title := m.styles.Title.Render("📂 Open File")

	// Instructions
	instructions := "Enter the path to the file you want to open:"

	// Input field with cursor
	inputText := m.inputBuffer
	if m.cursorPos <= len(inputText) {
		if m.cursorPos == len(inputText) {
			inputText += "█"
		} else {
			inputText = inputText[:m.cursorPos] + "█" + inputText[m.cursorPos:]
		}
	}

	input := m.styles.InputFocus.Width(m.width - 6).Render(inputText)

	// Help text
	helpText := m.styles.Help.Render("Enter: Open file | Esc: Cancel | Ctrl+U: Clear")

	// Combine sections
	sections := []string{
		title,
		"",
		instructions,
		"",
		input,
		"",
		helpText,
	}

	content := strings.Join(sections, "\n")
	return m.styles.Base.Width(m.width).Render(content)
}

// renderPatternTestDialog renders the enhanced pattern test dialog with live preview
func (m *OverlayModel) renderPatternTestDialog() string {
	// Title
	title := m.styles.Title.Render("🔍 Test Pattern")

	// Pattern type selector
	includeBtn := "Include"
	excludeBtn := "Exclude"
	if m.patternTest.SelectedType == 0 {
		includeBtn = m.styles.ButtonFocus.Render(includeBtn)
		excludeBtn = m.styles.Button.Render(excludeBtn)
	} else {
		includeBtn = m.styles.Button.Render(includeBtn)
		excludeBtn = m.styles.ButtonFocus.Render(excludeBtn)
	}
	typeSelector := lipgloss.JoinHorizontal(lipgloss.Left, "Type: ", includeBtn, excludeBtn)

	// Input field with cursor
	inputText := m.patternTest.Pattern
	if m.cursorPos <= len(inputText) {
		if m.cursorPos == len(inputText) {
			inputText += "█"
		} else {
			inputText = inputText[:m.cursorPos] + "█" + inputText[m.cursorPos:]
		}
	}

	inputStyle := m.styles.InputFocus
	if !m.patternTest.IsValid && m.patternTest.Pattern != "" {
		inputStyle = m.styles.Input.Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("9"))
	}
	input := inputStyle.Width(m.width - 6).Render(inputText)

	// Validation status
	var validationStatus string
	if m.patternTest.Pattern == "" {
		validationStatus = m.styles.Help.Render("Enter a regex pattern to test...")
	} else if m.patternTest.IsValid {
		validationStatus = m.styles.Success.Render(fmt.Sprintf("✓ Valid pattern - %d matches found", m.patternTest.MatchCount))
		if m.patternTest.ProcessingTime > 0 {
			validationStatus += m.styles.Help.Render(fmt.Sprintf(" (%.2fms)", float64(m.patternTest.ProcessingTime.Nanoseconds())/1e6))
		}
	} else {
		validationStatus = m.styles.Error.Render("✗ " + m.patternTest.ValidationError)
	}

	// Live preview of matches
	var preview string
	if m.patternTest.IsValid && len(m.patternTest.Matches) > 0 {
		previewLines := []string{}
		maxPreview := min(5, len(m.patternTest.Matches))

		for i := 0; i < maxPreview; i++ {
			match := m.patternTest.Matches[i]
			lineText := match.Context

			// Highlight the match
			if match.Start >= 0 && match.End <= len(lineText) {
				highlighted := lineText[:match.Start] +
					m.styles.Match.Render(lineText[match.Start:match.End]) +
					lineText[match.End:]
				previewLines = append(previewLines, fmt.Sprintf("%3d: %s", match.LineNumber, highlighted))
			} else {
				previewLines = append(previewLines, fmt.Sprintf("%3d: %s", match.LineNumber, lineText))
			}
		}

		if len(m.patternTest.Matches) > maxPreview {
			previewLines = append(previewLines, m.styles.Help.Render(fmt.Sprintf("... and %d more matches", len(m.patternTest.Matches)-maxPreview)))
		}

		preview = m.styles.Preview.Width(m.width - 4).Render(strings.Join(previewLines, "\n"))
	} else if m.patternTest.Pattern != "" && m.patternTest.IsValid {
		preview = m.styles.Preview.Width(m.width - 4).Render(m.styles.Help.Render("No matches found in preview content"))
	}

	// Help text
	helpText := m.styles.Help.Render("Tab: Toggle Include/Exclude | Enter: Add Pattern | Esc: Cancel")

	// Combine all sections
	sections := []string{
		title,
		"",
		typeSelector,
		"",
		"Pattern:",
		input,
		"",
		validationStatus,
	}

	if preview != "" {
		sections = append(sections, "", "Live Preview:", preview)
	}

	sections = append(sections, "", helpText)

	content := strings.Join(sections, "\n")
	return m.styles.Base.Width(m.width).Render(content)
}

// renderConfirmationDialog renders a confirmation dialog with styled buttons
func (m *OverlayModel) renderConfirmationDialog() string {
	// Title with appropriate icon
	icon := "❓"
	if strings.Contains(strings.ToLower(m.confirmation.Title), "delete") {
		icon = "⚠️"
	} else if strings.Contains(strings.ToLower(m.confirmation.Title), "quit") {
		icon = "🚪"
	}
	title := m.styles.Title.Render(icon + " " + m.confirmation.Title)

	// Message
	message := m.confirmation.Message

	// Buttons
	var buttons []string
	for i, buttonText := range m.confirmation.Buttons {
		if i == m.confirmation.Selected {
			buttons = append(buttons, m.styles.ButtonFocus.Render(fmt.Sprintf("%d. %s", i+1, buttonText)))
		} else {
			buttons = append(buttons, m.styles.Button.Render(fmt.Sprintf("%d. %s", i+1, buttonText)))
		}
	}
	buttonRow := lipgloss.JoinHorizontal(lipgloss.Left, buttons...)

	// Help text
	helpText := m.styles.Help.Render("Arrow keys: Select | Enter/Y: Confirm | Esc/N: Cancel | 1-5: Direct select")

	// Combine sections
	sections := []string{
		title,
		"",
		message,
		"",
		buttonRow,
		"",
		helpText,
	}

	content := strings.Join(sections, "\n")
	return m.styles.Base.Width(m.width).Render(content)
}

// renderErrorDialog renders an error dialog with proper styling
func (m *OverlayModel) renderErrorDialog() string {
	// Title with error icon
	title := m.styles.Title.Render("❌ " + m.title)
	if m.title == "" {
		title = m.styles.Title.Render("❌ Error")
	}

	// Content with word wrapping
	contentWidth := m.width - 4
	var contentLines []string
	for _, line := range strings.Split(m.content, "\n") {
		if len(line) <= contentWidth {
			contentLines = append(contentLines, line)
		} else {
			// Word wrap long lines
			words := strings.Fields(line)
			currentLine := ""
			for _, word := range words {
				if len(currentLine)+len(word)+1 <= contentWidth {
					if currentLine != "" {
						currentLine += " "
					}
					currentLine += word
				} else {
					if currentLine != "" {
						contentLines = append(contentLines, currentLine)
					}
					currentLine = word
				}
			}
			if currentLine != "" {
				contentLines = append(contentLines, currentLine)
			}
		}
	}

	// Apply error styling to content
	var styledContent []string
	for _, line := range contentLines {
		styledContent = append(styledContent, m.styles.Error.Render(line))
	}
	content := strings.Join(styledContent, "\n")

	// Help text
	helpText := m.styles.Help.Render("Press any key to close")

	// Combine sections
	sections := []string{
		title,
		"",
		content,
		"",
		helpText,
	}

	result := strings.Join(sections, "\n")
	return m.styles.Base.Width(m.width).Render(result)
}

// renderHelpDialog renders the help overlay
func (m *OverlayModel) renderHelpDialog() string {
	// Title
	title := m.styles.Title.Render("📚 Help - qf Interactive Log Filter Composer")

	// Context info
	contextInfo := ""
	if m.help.Context != "" {
		contextInfo = m.styles.Help.Render(fmt.Sprintf("Context: %s", m.help.Context))
	}

	// Render help sections
	var sections []string
	for i, section := range m.help.Sections {
		// Skip sections outside visible area
		if i < m.help.ScrollPos || i >= m.help.ScrollPos+10 {
			continue
		}

		// Section title
		sectionTitle := section.Title
		if i == m.help.Selected {
			sectionTitle = m.styles.ButtonFocus.Render("▶ " + sectionTitle)
		} else {
			sectionTitle = m.styles.Button.Render("  " + sectionTitle)
		}
		sections = append(sections, sectionTitle)

		// Keybindings for selected section or all if not interactive
		if i == m.help.Selected || len(m.help.Sections) < 10 {
			for _, kb := range section.Keybindings {
				keyStyle := m.styles.Success.Render(fmt.Sprintf("%-15s", kb.Key))
				descStyle := kb.Description
				sections = append(sections, fmt.Sprintf("  %s %s", keyStyle, descStyle))
			}
			sections = append(sections, "")
		}
	}

	// Help navigation
	helpNav := m.styles.Help.Render("↑↓: Navigate sections | PgUp/PgDn: Scroll | Home/End: Go to top/bottom | Esc: Close")

	// Combine all sections
	allSections := []string{title}
	if contextInfo != "" {
		allSections = append(allSections, "", contextInfo)
	}
	allSections = append(allSections, "", strings.Join(sections, "\n"), helpNav)

	content := strings.Join(allSections, "\n")
	return m.styles.Base.Width(m.width).Render(content)
}

// ShowFileOpenDialog shows the file open dialog
func (m *OverlayModel) ShowFileOpenDialog(callback func(string) tea.Msg) *OverlayModel {
	m.overlayType = OverlayFileOpen
	m.visible = true
	m.title = "Open File"
	m.content = ""
	m.inputBuffer = ""
	m.cursorPos = 0
	m.fileOpenCallback = callback
	m.resizeOverlay()
	return m
}

// ShowPatternTestDialog shows the enhanced pattern test dialog with live preview
func (m *OverlayModel) ShowPatternTestDialog(previewContent []string) *OverlayModel {
	m.overlayType = OverlayPatternTest
	m.visible = true
	m.previewContent = previewContent
	m.patternTest = PatternTestState{
		Pattern:      "",
		PatternType:  core.Include,
		SelectedType: 0,
		IsValid:      false,
		Matches:      nil,
		MatchCount:   0,
	}
	m.cursorPos = 0
	m.inputBuffer = ""
	m.resizeOverlay()
	return m
}

// ShowConfirmationDialog shows a confirmation dialog
func (m *OverlayModel) ShowConfirmationDialog(title, message string, buttons []string, callback func(string) tea.Msg) *OverlayModel {
	m.overlayType = OverlayConfirmation
	m.visible = true
	m.confirmation = ConfirmationState{
		Title:    title,
		Message:  message,
		Buttons:  buttons,
		Selected: 0,
		Callback: callback,
	}
	m.resizeOverlay()
	return m
}

// ShowErrorDialog shows an error dialog
func (m *OverlayModel) ShowErrorDialog(title, content string) *OverlayModel {
	m.overlayType = OverlayError
	m.visible = true
	m.title = title
	m.content = content
	m.resizeOverlay()
	return m
}

// ShowHelpDialog shows the help overlay
func (m *OverlayModel) ShowHelpDialog(context string) *OverlayModel {
	m.overlayType = OverlayHelp
	m.visible = true
	m.help.Context = context
	m.help.Selected = 0
	m.help.ScrollPos = 0
	m.resizeOverlay()
	return m
}

// hide hides the overlay and clears state
func (m *OverlayModel) hide() tea.Model {
	m.overlayType = OverlayNone
	m.visible = false
	m.title = ""
	m.content = ""
	m.inputBuffer = ""
	m.cursorPos = 0

	// Clear pattern test state
	m.patternTest = PatternTestState{}
	m.previewContent = nil

	// Clear confirmation state
	m.confirmation = ConfirmationState{}

	// Clear help state
	m.help = HelpState{}

	// Clear file open callback
	m.fileOpenCallback = nil

	return m
}

// IsVisible returns whether the overlay is currently visible
func (m *OverlayModel) IsVisible() bool {
	return m.visible
}

// GetOverlayType returns the current overlay type
func (m *OverlayModel) GetOverlayType() OverlayType {
	return m.overlayType
}

// SetPreviewContent sets the content for pattern testing preview
func (m *OverlayModel) SetPreviewContent(content []string) {
	m.previewContent = content
}

// GetPatternTestState returns the current pattern test state
func (m *OverlayModel) GetPatternTestState() PatternTestState {
	return m.patternTest
}

// initializeHelp sets up the help content
func (m *OverlayModel) initializeHelp() {
	m.help.Sections = []HelpSection{
		{
			Title: "Navigation",
			Keybindings: []HelpKeybinding{
				{"h/j/k/l", "Move left/down/up/right"},
				{"Arrow keys", "Move in any direction"},
				{"Tab/Shift+Tab", "Switch between panes"},
				{"g/G", "Go to top/bottom"},
				{"Page Up/Down", "Scroll page up/down"},
			},
		},
		{
			Title: "Pattern Management",
			Keybindings: []HelpKeybinding{
				{"i", "Insert mode - add new pattern"},
				{"t", "Test pattern with live preview"},
				{"d", "Delete selected pattern"},
				{"e", "Edit selected pattern"},
				{"Enter", "Toggle pattern on/off"},
				{"Space", "Toggle pattern type (Include/Exclude)"},
			},
		},
		{
			Title: "File Operations",
			Keybindings: []HelpKeybinding{
				{"o", "Open file"},
				{"w", "Save current session"},
				{"W", "Save session as..."},
				{"r", "Reload current file"},
				{"Ctrl+O", "Recent files"},
			},
		},
		{
			Title: "Search & Export",
			Keybindings: []HelpKeybinding{
				{"/", "Search in current file"},
				{"n/N", "Next/previous search match"},
				{"E", "Export filtered results"},
				{"R", "Generate ripgrep command"},
				{"y", "Copy filtered lines to clipboard"},
			},
		},
		{
			Title: "Application",
			Keybindings: []HelpKeybinding{
				{"?", "Show this help"},
				{"q", "Quit application"},
				{"Esc", "Cancel/close current overlay"},
				{"Ctrl+C", "Force quit"},
				{"F1", "Context-sensitive help"},
			},
		},
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Message types for overlay interactions

// PatternTestMsg requests opening the pattern test overlay
type PatternTestMsg struct {
	PreviewContent []string
}

// PatternTestResultMsg contains results from live pattern testing
type PatternTestResultMsg struct {
	IsValid         bool
	ValidationError string
	Matches         []PatternMatch
	MatchCount      int
	ProcessingTime  time.Duration
}

// PatternConfirmedMsg is sent when user confirms a pattern
type PatternConfirmedMsg struct {
	Pattern string
	Type    core.PatternType
	Color   string
}

// ConfirmationMsg requests showing a confirmation dialog
type ConfirmationMsg struct {
	Title    string
	Message  string
	Buttons  []string
	Callback func(result string) tea.Msg
}

// HelpMsg requests showing the help overlay
type HelpMsg struct {
	Context string // Context-specific help
}

// MessageHandler interface implementation

// HandleMessage implements MessageHandler interface
func (m *OverlayModel) HandleMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedModel, cmd := m.Update(msg)
	return updatedModel, cmd
}

// GetComponentType implements MessageHandler interface
func (m *OverlayModel) GetComponentType() string {
	return "overlay"
}

// IsMessageSupported implements MessageHandler interface
func (m *OverlayModel) IsMessageSupported(msg tea.Msg) bool {
	// Overlay handles specific message types
	switch msg.(type) {
	case ResizeMsg:
		return true
	case WindowResizeMsg:
		return true
	case tea.KeyMsg:
		return m.visible // Only when visible
	case PatternTestMsg:
		return true
	case PatternTestResultMsg:
		return m.visible && m.overlayType == OverlayPatternTest
	case ConfirmationMsg:
		return true
	case HelpMsg:
		return true
	default:
		return false
	}
}

// Interface compliance
var _ MessageHandler = (*OverlayModel)(nil)
var _ tea.Model = (*OverlayModel)(nil)
