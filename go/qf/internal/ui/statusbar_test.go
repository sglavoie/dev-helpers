// Package ui_test provides unit tests for the StatusBar component
package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// TestStatusBarCreation tests basic StatusBar creation and initialization
func TestStatusBarCreation(t *testing.T) {
	statusBar := NewStatusBarModel()

	if statusBar == nil {
		t.Fatal("NewStatusBarModel should not return nil")
	}

	if statusBar.GetComponentType() != "statusbar" {
		t.Errorf("Expected component type 'statusbar', got '%s'", statusBar.GetComponentType())
	}

	if statusBar.GetCurrentMode() != ModeNormal {
		t.Errorf("Expected initial mode ModeNormal, got %v", statusBar.GetCurrentMode())
	}

	if statusBar.IsShowingError() {
		t.Error("New StatusBar should not show error initially")
	}
}

// TestStatusBarMessageSupport tests which messages are supported
func TestStatusBarMessageSupport(t *testing.T) {
	statusBar := NewStatusBarModel()

	testCases := []struct {
		msg      tea.Msg
		expected bool
	}{
		{ModeTransitionMsg{}, true},
		{FilterUpdateMsg{}, true},
		{FileOpenMsg{}, true},
		{ErrorMsg{}, true},
		{StatusUpdateMsg{}, true},
		{tea.WindowSizeMsg{}, true},
		{tea.KeyMsg{}, true},
		{tea.QuitMsg{}, false},
	}

	for _, tc := range testCases {
		result := statusBar.IsMessageSupported(tc.msg)
		if result != tc.expected {
			t.Errorf("Message %T support: expected %t, got %t", tc.msg, tc.expected, result)
		}
	}
}

// TestStatusBarModeTransition tests mode transitions
func TestStatusBarModeTransition(t *testing.T) {
	statusBar := NewStatusBarModel()

	// Test transition to Insert mode
	modeMsg := ModeTransitionMsg{
		NewMode:   ModeInsert,
		PrevMode:  ModeNormal,
		Context:   "test",
		Timestamp: time.Now(),
	}

	updatedModel, cmd := statusBar.Update(modeMsg)
	if cmd != nil {
		t.Error("Mode transition should not return command")
	}

	updatedStatusBar, ok := updatedModel.(StatusBarModel)
	if !ok {
		t.Fatal("Updated model should be StatusBarModel")
	}

	if updatedStatusBar.GetCurrentMode() != ModeInsert {
		t.Errorf("Expected mode ModeInsert after transition, got %v", updatedStatusBar.GetCurrentMode())
	}
}

// TestStatusBarErrorHandling tests error message handling
func TestStatusBarErrorHandling(t *testing.T) {
	statusBar := NewStatusBarModel()

	// Test error message
	errorMsg := ErrorMsg{
		Message:     "Test error",
		Context:     "test_context",
		Source:      "test_source",
		Recoverable: true,
		Timestamp:   time.Now(),
	}

	updatedModel, _ := statusBar.Update(errorMsg)
	updatedStatusBar := updatedModel.(StatusBarModel)

	if !updatedStatusBar.IsShowingError() {
		t.Error("StatusBar should show error after ErrorMsg")
	}

	// Test error clearing
	updatedStatusBar.ClearError()
	if updatedStatusBar.IsShowingError() {
		t.Error("StatusBar should not show error after ClearError()")
	}
}

// TestStatusBarFilterUpdate tests filter statistics updates
func TestStatusBarFilterUpdate(t *testing.T) {
	statusBar := NewStatusBarModel()

	filterSet := FilterSet{
		Name: "test-session",
		Include: []core.FilterPattern{
			{ID: "1", Expression: "ERROR", Type: core.FilterInclude, IsValid: true},
			{ID: "2", Expression: "WARN", Type: core.FilterInclude, IsValid: true},
		},
		Exclude: []core.FilterPattern{
			{ID: "3", Expression: "DEBUG", Type: core.FilterExclude, IsValid: true},
		},
	}

	filterMsg := FilterUpdateMsg{
		FilterSet: filterSet,
		Source:    "test",
		Timestamp: time.Now(),
	}

	updatedModel, _ := statusBar.Update(filterMsg)
	updatedStatusBar := updatedModel.(StatusBarModel)

	// Check that filter stats were updated
	if updatedStatusBar.includePatterns != 2 {
		t.Errorf("Expected 2 include patterns, got %d", updatedStatusBar.includePatterns)
	}
	if updatedStatusBar.excludePatterns != 1 {
		t.Errorf("Expected 1 exclude pattern, got %d", updatedStatusBar.excludePatterns)
	}
	if updatedStatusBar.totalPatterns != 3 {
		t.Errorf("Expected 3 total patterns, got %d", updatedStatusBar.totalPatterns)
	}
}

// TestStatusBarView tests basic rendering
func TestStatusBarView(t *testing.T) {
	statusBar := NewStatusBarModel()

	// Test basic view
	view := statusBar.View()
	if len(view) == 0 {
		t.Error("View should not be empty")
	}

	// Test minimal view for narrow terminal
	statusBar.terminalWidth = 10
	minimalView := statusBar.View()
	if len(minimalView) == 0 {
		t.Error("Minimal view should not be empty")
	}

	// Test wide terminal
	statusBar.terminalWidth = 120
	wideView := statusBar.View()
	if len(wideView) == 0 {
		t.Error("Wide view should not be empty")
	}
}

// TestStatusBarPublicMethods tests public interface methods
func TestStatusBarPublicMethods(t *testing.T) {
	statusBar := NewStatusBarModel()

	// Test SetMode
	statusBar.SetMode(ModeInsert)
	if statusBar.GetCurrentMode() != ModeInsert {
		t.Error("SetMode should update current mode")
	}

	// Test SetFileInfo
	statusBar.SetFileInfo("/test/file.log", 100, 50, 10, true)
	if statusBar.currentFile != "/test/file.log" {
		t.Error("SetFileInfo should update file information")
	}

	// Test SetFilterStats
	statusBar.SetFilterStats(5, 3, 2, 80, 100)
	if statusBar.totalPatterns != 5 || statusBar.matchedLines != 80 {
		t.Error("SetFilterStats should update statistics")
	}

	// Test performance info
	statusBar.SetPerformanceInfo(50*time.Millisecond, 10, 2)
	if statusBar.filterTime != 50*time.Millisecond {
		t.Error("SetPerformanceInfo should update performance metrics")
	}

	// Test status and error methods
	statusBar.ShowStatus("Test status", 1*time.Second)
	if statusBar.statusMessage != "Test status" {
		t.Error("ShowStatus should set status message")
	}

	statusBar.ShowError("Test error", "test", true)
	if !statusBar.IsShowingError() {
		t.Error("ShowError should display error")
	}

	statusBar.ClearError()
	if statusBar.IsShowingError() {
		t.Error("ClearError should clear error")
	}

	// Test toggle methods
	initialPerf := statusBar.showPerformance
	statusBar.TogglePerformance()
	if statusBar.showPerformance == initialPerf {
		t.Error("TogglePerformance should change performance display state")
	}
}

// TestStatusBarKeyHandling tests keyboard input handling
func TestStatusBarKeyHandling(t *testing.T) {
	statusBar := NewStatusBarModel()

	testKeys := []struct {
		key      string
		testFunc func(StatusBarModel) bool
	}{
		{"?", func(sb StatusBarModel) bool { return sb.showHelp }},
		{"ctrl+p", func(sb StatusBarModel) bool { return sb.showPerformance }},
	}

	for _, tc := range testKeys {
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
		if tc.key == "ctrl+p" {
			keyMsg = tea.KeyMsg{Type: tea.KeyCtrlP}
		}

		initialState := statusBar
		updatedModel, _ := statusBar.Update(keyMsg)
		updatedStatusBar := updatedModel.(StatusBarModel)

		// For help and performance toggles, the state should change
		if tc.key == "?" || tc.key == "ctrl+p" {
			if tc.testFunc(updatedStatusBar) == tc.testFunc(initialState) {
				t.Errorf("Key %s should toggle state", tc.key)
			}
		}
	}
}

// TestStatusBarWindowResize tests window resize handling
func TestStatusBarWindowResize(t *testing.T) {
	statusBar := NewStatusBarModel()

	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 30}
	updatedModel, cmd := statusBar.Update(resizeMsg)

	if cmd != nil {
		t.Error("Window resize should not return command")
	}

	updatedStatusBar := updatedModel.(StatusBarModel)
	if updatedStatusBar.terminalWidth != 120 || updatedStatusBar.terminalHeight != 30 {
		t.Error("Window resize should update terminal dimensions")
	}
}

// Benchmark tests for performance
func BenchmarkStatusBarView(b *testing.B) {
	statusBar := NewStatusBarModel()
	statusBar.terminalWidth = 120

	// Set up some state
	statusBar.SetFileInfo("/test/long/path/to/file.log", 1000, 500, 25, true)
	statusBar.SetFilterStats(10, 6, 4, 750, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = statusBar.View()
	}
}

func BenchmarkStatusBarUpdate(b *testing.B) {
	statusBar := NewStatusBarModel()

	msg := ModeTransitionMsg{
		NewMode:   ModeInsert,
		PrevMode:  ModeNormal,
		Context:   "benchmark",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		statusBar.Update(msg)
	}
}
