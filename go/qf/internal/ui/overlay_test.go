package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// Mock FilterEngine for testing
type mockFilterEngine struct{}

func (m *mockFilterEngine) AddPattern(pattern core.FilterPattern) error { return nil }
func (m *mockFilterEngine) RemovePattern(patternID string) error        { return nil }
func (m *mockFilterEngine) UpdatePattern(patternID string, pattern core.FilterPattern) error {
	return nil
}
func (m *mockFilterEngine) GetPatterns() []core.FilterPattern       { return nil }
func (m *mockFilterEngine) ClearPatterns()                          {}
func (m *mockFilterEngine) ValidatePattern(expression string) error { return nil }
func (m *mockFilterEngine) ApplyFilters(ctx context.Context, content []string) (core.FilterResult, error) {
	return core.FilterResult{}, nil
}
func (m *mockFilterEngine) GetCacheStats() (hits int, misses int, size int) {
	return 0, 0, 0
}

func TestOverlayModel_Creation(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	if overlay == nil {
		t.Fatal("NewOverlayModel returned nil")
	}

	if overlay.GetOverlayType() != OverlayNone {
		t.Errorf("Expected initial overlay type to be OverlayNone, got %v", overlay.GetOverlayType())
	}

	if overlay.IsVisible() {
		t.Error("Expected overlay to be initially invisible")
	}
}

func TestOverlayModel_PatternTestDialog(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	sampleContent := []string{
		"2023-01-01 10:00:00 INFO Starting application",
		"2023-01-01 10:00:01 DEBUG Loading configuration",
		"2023-01-01 10:00:02 ERROR Failed to connect to database",
	}

	// Show pattern test dialog
	overlay.ShowPatternTestDialog(sampleContent)

	if !overlay.IsVisible() {
		t.Error("Expected overlay to be visible after showing pattern test dialog")
	}

	if overlay.GetOverlayType() != OverlayPatternTest {
		t.Errorf("Expected overlay type to be OverlayPatternTest, got %v", overlay.GetOverlayType())
	}

	// Test that preview content is set
	state := overlay.GetPatternTestState()
	if state.Pattern != "" {
		t.Errorf("Expected initial pattern to be empty, got %q", state.Pattern)
	}
}

func TestOverlayModel_ConfirmationDialog(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Show confirmation dialog
	overlay.ShowConfirmationDialog(
		"Delete Pattern",
		"Are you sure you want to delete this pattern?",
		[]string{"Yes", "No"},
		nil,
	)

	if !overlay.IsVisible() {
		t.Error("Expected overlay to be visible after showing confirmation dialog")
	}

	if overlay.GetOverlayType() != OverlayConfirmation {
		t.Errorf("Expected overlay type to be OverlayConfirmation, got %v", overlay.GetOverlayType())
	}
}

func TestOverlayModel_HelpDialog(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Show help dialog
	overlay.ShowHelpDialog("pattern_management")

	if !overlay.IsVisible() {
		t.Error("Expected overlay to be visible after showing help dialog")
	}

	if overlay.GetOverlayType() != OverlayHelp {
		t.Errorf("Expected overlay type to be OverlayHelp, got %v", overlay.GetOverlayType())
	}
}

func TestOverlayModel_ErrorDialog(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Show error dialog
	overlay.ShowErrorDialog("Validation Error", "The regex pattern contains invalid syntax")

	if !overlay.IsVisible() {
		t.Error("Expected overlay to be visible after showing error dialog")
	}

	if overlay.GetOverlayType() != OverlayError {
		t.Errorf("Expected overlay type to be OverlayError, got %v", overlay.GetOverlayType())
	}
}

func TestOverlayModel_FileOpenDialog(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Show file open dialog
	overlay.ShowFileOpenDialog(nil)

	if !overlay.IsVisible() {
		t.Error("Expected overlay to be visible after showing file open dialog")
	}

	if overlay.GetOverlayType() != OverlayFileOpen {
		t.Errorf("Expected overlay type to be OverlayFileOpen, got %v", overlay.GetOverlayType())
	}
}

func TestOverlayModel_KeyHandling(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Show pattern test dialog
	sampleContent := []string{"test line with error in it"}
	overlay.ShowPatternTestDialog(sampleContent)

	// Test escape key closes overlay
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := overlay.Update(keyMsg)

	overlayModel := updatedModel.(*OverlayModel)
	if overlayModel.IsVisible() {
		t.Error("Expected overlay to be hidden after escape key")
	}
}

func TestOverlayModel_PatternTesting(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	sampleContent := []string{
		"2023-01-01 10:00:00 INFO Starting application",
		"2023-01-01 10:00:02 ERROR Failed to connect",
		"2023-01-01 10:00:04 INFO Connected successfully",
	}

	overlay.ShowPatternTestDialog(sampleContent)

	// Simulate typing "ERROR" pattern
	for _, char := range "ERROR" {
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
		overlay.Update(keyMsg)
	}

	state := overlay.GetPatternTestState()
	if state.Pattern != "ERROR" {
		t.Errorf("Expected pattern to be 'ERROR', got %q", state.Pattern)
	}
}

func TestOverlayModel_Rendering(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Test that invisible overlay renders empty
	view := overlay.View()
	if view != "" {
		t.Error("Expected invisible overlay to render empty string")
	}

	// Test that visible overlay renders content
	overlay.ShowErrorDialog("Test Error", "Test error message")
	view = overlay.View()
	if view == "" {
		t.Error("Expected visible overlay to render content")
	}

	// Check that error dialog contains expected text
	if !strings.Contains(view, "Test Error") || !strings.Contains(view, "Test error message") {
		t.Error("Expected error dialog to contain title and message")
	}
}

func TestOverlayModel_MessageHandling(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Test message support when invisible
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	if overlay.IsMessageSupported(keyMsg) {
		t.Error("Expected invisible overlay to not support key messages")
	}

	// Test message support when visible
	overlay.ShowPatternTestDialog([]string{"test"})
	if !overlay.IsMessageSupported(keyMsg) {
		t.Error("Expected visible overlay to support key messages")
	}

	// Test resize message support
	resizeMsg := ResizeMsg{Width: 100, Height: 50}
	if !overlay.IsMessageSupported(resizeMsg) {
		t.Error("Expected overlay to support resize messages")
	}
}

func TestOverlayModel_TeaModelInterface(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Test that it implements tea.Model interface
	var model tea.Model = overlay
	if model == nil {
		t.Error("OverlayModel should implement tea.Model interface")
	}

	// Test Init method
	cmd := overlay.Init()
	if cmd != nil {
		t.Error("Expected Init to return nil command")
	}
}

func TestOverlayModel_MessageHandlerInterface(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Test that it implements MessageHandler interface
	var handler MessageHandler = overlay
	if handler == nil {
		t.Error("OverlayModel should implement MessageHandler interface")
	}

	// Test GetComponentType method
	componentType := overlay.GetComponentType()
	if componentType != "overlay" {
		t.Errorf("Expected component type 'overlay', got %q", componentType)
	}
}

func TestOverlayModel_StateManagement(t *testing.T) {
	filterEngine := &mockFilterEngine{}
	overlay := NewOverlayModel(filterEngine)

	// Test setting preview content
	testContent := []string{"line 1", "line 2", "line 3"}
	overlay.SetPreviewContent(testContent)
	overlay.ShowPatternTestDialog(testContent)

	// Verify content is accessible
	view := overlay.View()
	if view == "" {
		t.Error("Expected pattern test dialog to render with content")
	}
}
