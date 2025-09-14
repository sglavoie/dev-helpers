// Package ui provides the ViewerModel component for the qf application.
//
// The ViewerModel handles the main content display area, showing filtered
// file content with line numbers, pattern highlighting, and vim-style navigation.
// It supports both streaming mode for large files and efficient rendering
// for responsive user experience.
package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

// ViewerModel implements the content viewer component for the qf application.
// It displays filtered file content with line numbers, highlighting, and navigation.
type ViewerModel struct {
	// Component identification
	componentType string
	focused       bool

	// Content state
	currentTab    *file.FileTab    // Currently displayed file tab
	displayLines  []DisplayLine    // Processed lines ready for display
	totalLines    int              // Total lines in current view
	filteredStats core.FilterStats // Statistics from last filter operation

	// Viewport state
	width          int // Available width for display
	height         int // Available height for display
	scrollPosition int // Current vertical scroll (0-based)
	cursorLine     int // Current cursor line (1-based in file coordinates)
	topVisible     int // Top visible line in viewport (1-based)
	bottomVisible  int // Bottom visible line in viewport (1-based)

	// Visual state
	showLineNumbers bool                     // Whether to show line numbers
	lineNumberWidth int                      // Width allocated for line numbers
	wrapLines       bool                     // Whether to wrap long lines
	highlightCursor bool                     // Whether to highlight cursor line
	highlights      map[int][]core.Highlight // Pattern highlights per line

	// Search state
	searchActive    bool            // Whether search is active
	searchPattern   string          // Current search pattern
	searchMatches   []SearchMatch   // Found search matches
	currentMatch    int             // Index of current match
	searchDirection SearchDirection // Search direction

	// Performance settings
	maxDisplayLines int           // Maximum lines to render at once
	streamingMode   bool          // Whether to use streaming for large files
	debounceDelay   time.Duration // Debounce delay for rapid updates

	// Styles (Lipgloss)
	baseStyle       lipgloss.Style            // Base container style
	lineNumStyle    lipgloss.Style            // Line number style
	contentStyle    lipgloss.Style            // Content area style
	cursorStyle     lipgloss.Style            // Current line highlight style
	focusedStyle    lipgloss.Style            // Focused border style
	unfocusedStyle  lipgloss.Style            // Unfocused border style
	highlightStyles map[string]lipgloss.Style // Pattern highlight styles

	// Internal state
	lastUpdate     time.Time // Last update timestamp
	pendingUpdates bool      // Whether updates are pending
}

// DisplayLine represents a line ready for display with formatting information
type DisplayLine struct {
	Number     int              // Original line number (1-based)
	Content    string           // Line content
	Highlights []core.Highlight // Pattern highlights
	IsCursor   bool             // Whether this is the cursor line
	IsMatch    bool             // Whether this line contains search matches
}

// NewViewerModel creates a new ViewerModel with default configuration
func NewViewerModel() *ViewerModel {
	vm := &ViewerModel{
		componentType:   "viewer",
		focused:         false,
		width:           80,
		height:          24,
		showLineNumbers: true,
		lineNumberWidth: 4,
		wrapLines:       false,
		highlightCursor: true,
		highlights:      make(map[int][]core.Highlight),
		maxDisplayLines: 1000,
		streamingMode:   false,
		debounceDelay:   150 * time.Millisecond,
		lastUpdate:      time.Now(),
		highlightStyles: make(map[string]lipgloss.Style),
	}

	vm.initializeStyles()
	return vm
}

// initializeStyles sets up the default Lipgloss styles
func (vm *ViewerModel) initializeStyles() {
	// Base styles
	vm.baseStyle = lipgloss.NewStyle().
		Padding(0, 1)

	vm.lineNumStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		Width(vm.lineNumberWidth).
		Align(lipgloss.Right)

	vm.contentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")) // White

	vm.cursorStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("8")). // Gray background
		Foreground(lipgloss.Color("0"))  // Black text

	// Focus styles
	vm.focusedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")) // Blue

	vm.unfocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")) // Gray

	// Pattern highlight styles (cycling colors)
	colors := []string{"9", "10", "11", "12", "13", "14"} // Red, Green, Yellow, Blue, Magenta, Cyan
	for i, color := range colors {
		vm.highlightStyles[fmt.Sprintf("pattern-%d", i)] = lipgloss.NewStyle().
			Background(lipgloss.Color(color)).
			Foreground(lipgloss.Color("0")) // Black text on colored background
	}

	// Default pattern highlight for when no specific color is set
	vm.highlightStyles["default"] = lipgloss.NewStyle().
		Background(lipgloss.Color("11")). // Yellow
		Foreground(lipgloss.Color("0"))   // Black text
}

// Bubble Tea interface methods

// Init initializes the ViewerModel component
func (vm *ViewerModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the component state
func (vm *ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle key messages when focused
	if keyMsg, ok := msg.(tea.KeyMsg); ok && vm.focused {
		if cmd := vm.handleKeyPress(keyMsg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Handle file-related messages
	if cmd := vm.handleFileMessages(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Handle UI state messages
	vm.handleUIStateMessages(msg)

	// Handle mode and focus messages
	if cmd := vm.handleModeAndFocusMessages(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Return batched commands
	if len(cmds) > 0 {
		return vm, tea.Batch(cmds...)
	}
	return vm, nil
}

// handleFileMessages handles file-related messages
func (vm *ViewerModel) handleFileMessages(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case FileOpenMsg:
		if msg.Success && msg.TabID != "" {
			return vm.handleFileOpen(msg)
		}
	case ContentUpdateMsg:
		return vm.handleContentUpdate(msg)
	case FilterUpdateMsg:
		return vm.handleFilterUpdate(msg)
	}
	return nil
}

// handleUIStateMessages handles UI state messages that don't return commands
func (vm *ViewerModel) handleUIStateMessages(msg tea.Msg) {
	switch msg := msg.(type) {
	case ViewportUpdateMsg:
		vm.handleViewportUpdate(msg)
	case FocusMsg:
		vm.handleFocus(msg)
	case ResizeMsg:
		vm.handleResize(msg)
	}
}

// handleModeAndFocusMessages handles mode transitions and search messages
func (vm *ViewerModel) handleModeAndFocusMessages(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case SearchMsg:
		return vm.handleSearch(msg)
	case SearchResultMsg:
		return vm.handleSearchResults(msg)
	case ModeTransitionMsg:
		return vm.handleModeTransition(msg)
	}
	return nil
}

// View renders the ViewerModel component
func (vm *ViewerModel) View() string {
	if vm.currentTab == nil {
		return vm.renderEmptyState()
	}

	// Calculate visible area
	contentHeight := vm.height - 2 // Account for border
	vm.updateVisibleRange(contentHeight)

	// Render content lines
	var lines []string
	visibleLines := vm.getVisibleLines()

	for _, line := range visibleLines {
		renderedLine := vm.renderLine(line)
		lines = append(lines, renderedLine)
	}

	// Fill remaining space if needed
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	// Join lines and apply container style
	content := strings.Join(lines, "\n")

	// Return content without borders - parent handles borders
	return content
}

// Message handling methods

// handleKeyPress processes keyboard input for navigation and commands
func (vm *ViewerModel) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	// Vim-style navigation
	case "j", "down":
		vm.moveCursorDown(1)
	case "k", "up":
		vm.moveCursorUp(1)
	case "h", "left":
		// Could be used for horizontal scrolling in future
	case "l", "right":
		// Could be used for horizontal scrolling in future
	case "G":
		vm.goToEnd()
	case "g":
		// Handle 'gg' sequence for going to top
		return vm.handleGSequence()
	case "ctrl+d":
		vm.pageDown()
	case "ctrl+u":
		vm.pageUp()

	// Search navigation
	case "n":
		vm.nextMatch()
	case "N":
		vm.prevMatch()

	// Line-specific navigation
	case "0", "home":
		// Go to beginning of line (future feature)
	case "$", "end":
		// Go to end of line (future feature)

	// Return focus message for mode transitions if needed
	case "i", "a", "o":
		return func() tea.Msg {
			return NewModeTransitionMsg(ModeInsert, ModeNormal, "viewer_edit_request")
		}
	}

	// Generate viewport update message
	return func() tea.Msg {
		return ViewportUpdateMsg{
			TabID:          vm.getTabID(),
			ScrollPosition: vm.scrollPosition,
			CursorLine:     vm.cursorLine,
			ViewportHeight: vm.height - 2,
			Source:         "viewer",
		}
	}
}

// handleGSequence handles the 'g' key sequence for 'gg' command
func (vm *ViewerModel) handleGSequence() tea.Cmd {
	// This would need to be implemented with a state machine for proper 'gg' handling
	// For now, treat single 'g' as go to top
	vm.goToTop()
	return func() tea.Msg {
		return ViewportUpdateMsg{
			TabID:          vm.getTabID(),
			ScrollPosition: vm.scrollPosition,
			CursorLine:     vm.cursorLine,
			ViewportHeight: vm.height - 2,
			Source:         "viewer",
		}
	}
}

// handleFileOpen processes file open messages
func (vm *ViewerModel) handleFileOpen(msg FileOpenMsg) tea.Cmd {
	// Create a FileTab from the file open message
	tab := file.NewFileTab(msg.FilePath)
	tab.ID = msg.TabID

	// Convert content to Lines
	for i, content := range msg.Content {
		line := file.Line{
			Number:      i + 1,
			Content:     content,
			Offset:      0, // Would be calculated properly in real implementation
			Highlighted: false,
		}
		tab.Content = append(tab.Content, line)
	}

	tab.IsLoaded = true
	vm.currentTab = tab
	vm.resetViewport()
	vm.refreshDisplayLines()

	// Return status update
	return func() tea.Msg {
		return NewStatusUpdateMsg(
			fmt.Sprintf("Opened %s (%d lines)", tab.GetDisplayName(), tab.GetLineCount()),
			StatusSuccess,
			"file_open",
		)
	}
}

// handleContentUpdate processes content update messages with filtered results
func (vm *ViewerModel) handleContentUpdate(msg ContentUpdateMsg) tea.Cmd {
	if vm.currentTab == nil || vm.currentTab.ID != msg.TabID {
		return nil
	}

	// Update highlights
	vm.highlights = msg.Highlights
	vm.filteredStats = msg.Stats

	// Refresh display with new highlighting
	vm.refreshDisplayLines()

	// Return status update
	return func() tea.Msg {
		return NewStatusUpdateMsg(
			fmt.Sprintf("Filtered: %d/%d lines (%d patterns)",
				msg.Stats.MatchedLines,
				msg.Stats.TotalLines,
				msg.Stats.PatternsUsed,
			),
			StatusInfo,
			"content_filter",
		)
	}
}

// handleFilterUpdate processes filter update messages
func (vm *ViewerModel) handleFilterUpdate(msg FilterUpdateMsg) tea.Cmd {
	// Filter update requires reapplying filters to content
	// This would typically trigger the filter engine to process content
	// For now, just refresh the display
	vm.refreshDisplayLines()
	return nil
}

// handleSearch processes search messages
func (vm *ViewerModel) handleSearch(msg SearchMsg) tea.Cmd {
	if vm.currentTab == nil || vm.currentTab.ID != msg.TabID {
		return nil
	}

	vm.searchActive = true
	vm.searchPattern = msg.Pattern
	vm.searchDirection = msg.Direction

	// Perform the search
	matches := vm.performSearch(msg.Pattern, msg.CaseSensitive)
	vm.searchMatches = matches

	// Find current match
	if len(matches) > 0 {
		vm.currentMatch = vm.findNearestMatch(vm.cursorLine, msg.Direction)
		if vm.currentMatch >= 0 {
			match := matches[vm.currentMatch]
			vm.goToLine(match.LineNumber)
		}
	}

	// Return search results
	return func() tea.Msg {
		return SearchResultMsg{
			Pattern:      msg.Pattern,
			Matches:      matches,
			CurrentMatch: vm.currentMatch,
			TabID:        msg.TabID,
			Total:        len(matches),
		}
	}
}

// handleSearchResults processes search result messages
func (vm *ViewerModel) handleSearchResults(msg SearchResultMsg) tea.Cmd {
	vm.searchMatches = msg.Matches
	vm.currentMatch = msg.CurrentMatch

	// Update status
	return func() tea.Msg {
		return NewStatusUpdateMsg(
			fmt.Sprintf("Search: %d matches for '%s'", msg.Total, msg.Pattern),
			StatusInfo,
			"search",
		)
	}
}

// handleViewportUpdate processes viewport update messages
func (vm *ViewerModel) handleViewportUpdate(msg ViewportUpdateMsg) {
	if vm.currentTab != nil && vm.currentTab.ID == msg.TabID {
		vm.scrollPosition = msg.ScrollPosition
		vm.cursorLine = msg.CursorLine
		// Update tab's view state
		vm.currentTab.UpdateViewState(msg.ScrollPosition, msg.CursorLine, msg.ViewportHeight)
	}
}

// handleFocus processes focus change messages
func (vm *ViewerModel) handleFocus(msg FocusMsg) {
	vm.focused = (msg.Component == vm.componentType)
}

// handleResize processes terminal resize messages
func (vm *ViewerModel) handleResize(msg ResizeMsg) {
	vm.width = msg.Width
	vm.height = msg.Height
	// Adjust line number width based on content
	if vm.currentTab != nil {
		vm.adjustLineNumberWidth(vm.currentTab.GetLineCount())
	}
}

// handleModeTransition processes mode transition messages
func (vm *ViewerModel) handleModeTransition(msg ModeTransitionMsg) tea.Cmd {
	// Viewer typically stays in Normal mode
	// Could handle special display modes here in the future
	return nil
}

// Navigation methods

// moveCursorDown moves cursor down by the specified number of lines
func (vm *ViewerModel) moveCursorDown(lines int) {
	if vm.currentTab == nil {
		return
	}

	maxLine := vm.currentTab.GetLineCount()
	vm.cursorLine += lines
	if vm.cursorLine > maxLine {
		vm.cursorLine = maxLine
	}

	vm.adjustScrollForCursor()
	vm.refreshDisplayLines()
}

// moveCursorUp moves cursor up by the specified number of lines
func (vm *ViewerModel) moveCursorUp(lines int) {
	vm.cursorLine -= lines
	if vm.cursorLine < 1 {
		vm.cursorLine = 1
	}

	vm.adjustScrollForCursor()
	vm.refreshDisplayLines()
}

// goToTop moves cursor to the first line
func (vm *ViewerModel) goToTop() {
	vm.cursorLine = 1
	vm.scrollPosition = 0
	vm.refreshDisplayLines()
}

// goToEnd moves cursor to the last line
func (vm *ViewerModel) goToEnd() {
	if vm.currentTab == nil {
		return
	}

	vm.cursorLine = vm.currentTab.GetLineCount()
	vm.adjustScrollForCursor()
	vm.refreshDisplayLines()
}

// goToLine moves cursor to a specific line number
func (vm *ViewerModel) goToLine(lineNum int) {
	if vm.currentTab == nil {
		return
	}

	maxLine := vm.currentTab.GetLineCount()
	if lineNum < 1 {
		lineNum = 1
	} else if lineNum > maxLine {
		lineNum = maxLine
	}

	vm.cursorLine = lineNum
	vm.adjustScrollForCursor()
	vm.refreshDisplayLines()
}

// pageDown scrolls down by a page
func (vm *ViewerModel) pageDown() {
	pageSize := vm.height - 4 // Account for borders and some overlap
	vm.moveCursorDown(pageSize)
}

// pageUp scrolls up by a page
func (vm *ViewerModel) pageUp() {
	pageSize := vm.height - 4 // Account for borders and some overlap
	vm.moveCursorUp(pageSize)
}

// Search methods

// nextMatch moves to the next search match
func (vm *ViewerModel) nextMatch() {
	if !vm.searchActive || len(vm.searchMatches) == 0 {
		return
	}

	vm.currentMatch++
	if vm.currentMatch >= len(vm.searchMatches) {
		vm.currentMatch = 0 // Wrap around
	}

	match := vm.searchMatches[vm.currentMatch]
	vm.goToLine(match.LineNumber)
}

// prevMatch moves to the previous search match
func (vm *ViewerModel) prevMatch() {
	if !vm.searchActive || len(vm.searchMatches) == 0 {
		return
	}

	vm.currentMatch--
	if vm.currentMatch < 0 {
		vm.currentMatch = len(vm.searchMatches) - 1 // Wrap around
	}

	match := vm.searchMatches[vm.currentMatch]
	vm.goToLine(match.LineNumber)
}

// performSearch searches for a pattern in the current file content
func (vm *ViewerModel) performSearch(pattern string, caseSensitive bool) []SearchMatch {
	if vm.currentTab == nil {
		return nil
	}

	var matches []SearchMatch
	flags := ""
	if !caseSensitive {
		flags = "(?i)" // Case insensitive flag
	}

	regex, err := regexp.Compile(flags + pattern)
	if err != nil {
		return matches // Invalid pattern
	}

	for _, line := range vm.currentTab.Content {
		found := regex.FindAllStringIndex(line.Content, -1)
		for _, match := range found {
			matches = append(matches, SearchMatch{
				LineNumber: line.Number,
				Start:      match[0],
				End:        match[1],
				Context:    line.Content,
			})
		}
	}

	return matches
}

// findNearestMatch finds the nearest search match to the current cursor position
func (vm *ViewerModel) findNearestMatch(cursorLine int, direction SearchDirection) int {
	if len(vm.searchMatches) == 0 {
		return -1
	}

	if direction == SearchForward {
		for i, match := range vm.searchMatches {
			if match.LineNumber > cursorLine {
				return i
			}
		}
		return 0 // Wrap to first match
	}

	// SearchBackward
	for i := len(vm.searchMatches) - 1; i >= 0; i-- {
		match := vm.searchMatches[i]
		if match.LineNumber < cursorLine {
			return i
		}
	}
	return len(vm.searchMatches) - 1 // Wrap to last match
}

// Rendering methods

// renderEmptyState renders the empty state when no file is loaded
func (vm *ViewerModel) renderEmptyState() string {
	emptyMsg := "No file loaded"
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		Width(vm.width).
		Height(vm.height).
		Align(lipgloss.Center, lipgloss.Center)

	// Return empty state without borders - parent handles borders
	return style.Render(emptyMsg)
}

// renderLine renders a single display line with highlighting and formatting
func (vm *ViewerModel) renderLine(line DisplayLine) string {
	var parts []string

	// Add line number if enabled
	if vm.showLineNumbers {
		lineNumStr := fmt.Sprintf("%*d", vm.lineNumberWidth, line.Number)
		parts = append(parts, vm.lineNumStyle.Render(lineNumStr))
	}

	// Process content with highlights
	content := vm.applyHighlights(line.Content, line.Highlights)

	// Apply cursor highlighting if this is the cursor line
	if line.IsCursor && vm.highlightCursor {
		content = vm.cursorStyle.Render(content)
	} else {
		content = vm.contentStyle.Render(content)
	}

	parts = append(parts, content)
	return strings.Join(parts, " ")
}

// applyHighlights applies pattern highlighting to line content
func (vm *ViewerModel) applyHighlights(content string, highlights []core.Highlight) string {
	if len(highlights) == 0 {
		return content
	}

	// Sort highlights by start position
	// (In a real implementation, this would be more sophisticated)
	result := content

	// For simplicity, just apply the first highlight with default style
	// A full implementation would handle overlapping highlights
	for _, highlight := range highlights {
		if highlight.Start >= 0 && highlight.End <= len(content) {
			before := content[:highlight.Start]
			highlighted := content[highlight.Start:highlight.End]
			after := content[highlight.End:]

			style := vm.highlightStyles["default"]
			if customStyle, exists := vm.highlightStyles[highlight.Color]; exists {
				style = customStyle
			}

			result = before + style.Render(highlighted) + after
			break // Only apply first highlight for now
		}
	}

	return result
}

// Helper methods

// getVisibleLines returns the lines that should be visible in the current viewport
func (vm *ViewerModel) getVisibleLines() []DisplayLine {
	if vm.currentTab == nil || len(vm.displayLines) == 0 {
		return []DisplayLine{}
	}

	contentHeight := vm.height - 2 // Account for border
	start := vm.scrollPosition
	end := start + contentHeight

	if start >= len(vm.displayLines) {
		return []DisplayLine{}
	}

	if end > len(vm.displayLines) {
		end = len(vm.displayLines)
	}

	return vm.displayLines[start:end]
}

// refreshDisplayLines rebuilds the display lines from current tab content
func (vm *ViewerModel) refreshDisplayLines() {
	if vm.currentTab == nil {
		vm.displayLines = []DisplayLine{}
		return
	}

	vm.displayLines = []DisplayLine{}

	for _, line := range vm.currentTab.Content {
		displayLine := DisplayLine{
			Number:     line.Number,
			Content:    line.Content,
			Highlights: vm.highlights[line.Number],
			IsCursor:   line.Number == vm.cursorLine,
			IsMatch:    vm.lineHasSearchMatch(line.Number),
		}
		vm.displayLines = append(vm.displayLines, displayLine)
	}

	vm.totalLines = len(vm.displayLines)
}

// lineHasSearchMatch checks if a line contains any search matches
func (vm *ViewerModel) lineHasSearchMatch(lineNum int) bool {
	for _, match := range vm.searchMatches {
		if match.LineNumber == lineNum {
			return true
		}
	}
	return false
}

// updateVisibleRange calculates and updates the visible line range
func (vm *ViewerModel) updateVisibleRange(contentHeight int) {
	vm.topVisible = vm.scrollPosition + 1
	vm.bottomVisible = vm.topVisible + contentHeight - 1

	if vm.bottomVisible > vm.totalLines {
		vm.bottomVisible = vm.totalLines
	}
}

// adjustScrollForCursor adjusts scroll position to keep cursor visible
func (vm *ViewerModel) adjustScrollForCursor() {
	contentHeight := vm.height - 2

	// If cursor is above the visible area, scroll up
	if vm.cursorLine < vm.scrollPosition+1 {
		vm.scrollPosition = vm.cursorLine - 1
		if vm.scrollPosition < 0 {
			vm.scrollPosition = 0
		}
	}

	// If cursor is below the visible area, scroll down
	if vm.cursorLine > vm.scrollPosition+contentHeight {
		vm.scrollPosition = vm.cursorLine - contentHeight
	}
}

// resetViewport resets the viewport to the top of the file
func (vm *ViewerModel) resetViewport() {
	vm.scrollPosition = 0
	vm.cursorLine = 1
	vm.topVisible = 1
	vm.bottomVisible = vm.height - 2
}

// adjustLineNumberWidth adjusts the line number width based on total lines
func (vm *ViewerModel) adjustLineNumberWidth(totalLines int) {
	if totalLines <= 0 {
		vm.lineNumberWidth = 4
		return
	}

	// Calculate width needed for line numbers
	width := len(fmt.Sprintf("%d", totalLines))
	if width < 4 {
		width = 4 // Minimum width
	}
	vm.lineNumberWidth = width

	// Update line number style
	vm.lineNumStyle = vm.lineNumStyle.Width(vm.lineNumberWidth)
}

// getTabID returns the current tab ID or empty string if no tab is loaded
func (vm *ViewerModel) getTabID() string {
	if vm.currentTab == nil {
		return ""
	}
	return vm.currentTab.ID
}

// MessageHandler interface implementation

// HandleMessage processes UI messages (implements MessageHandler)
func (vm *ViewerModel) HandleMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	return vm.Update(msg)
}

// GetComponentType returns the component type (implements MessageHandler)
func (vm *ViewerModel) GetComponentType() string {
	return vm.componentType
}

// IsMessageSupported checks if a message type is supported (implements MessageHandler)
func (vm *ViewerModel) IsMessageSupported(msg tea.Msg) bool {
	switch msg.(type) {
	case FileOpenMsg, ContentUpdateMsg, FilterUpdateMsg, SearchMsg, SearchResultMsg,
		ViewportUpdateMsg, FocusMsg, ResizeMsg, ModeTransitionMsg, tea.KeyMsg:
		return true
	default:
		return false
	}
}

// Public methods for external control

// SetFocused sets the focus state of the viewer
func (vm *ViewerModel) SetFocused(focused bool) {
	vm.focused = focused
}

// GetCurrentLine returns the current cursor line number
func (vm *ViewerModel) GetCurrentLine() int {
	return vm.cursorLine
}

// GetTotalLines returns the total number of lines in the current view
func (vm *ViewerModel) GetTotalLines() int {
	return vm.totalLines
}

// GetStats returns the current filter statistics
func (vm *ViewerModel) GetStats() core.FilterStats {
	return vm.filteredStats
}

// LoadFileTab loads content from a FileTab
func (vm *ViewerModel) LoadFileTab(tab *file.FileTab) {
	vm.currentTab = tab
	vm.resetViewport()
	vm.adjustLineNumberWidth(tab.GetLineCount())
	vm.refreshDisplayLines()
}

// SetDimensions updates the component dimensions
func (vm *ViewerModel) SetDimensions(width, height int) {
	vm.width = width
	vm.height = height
}
