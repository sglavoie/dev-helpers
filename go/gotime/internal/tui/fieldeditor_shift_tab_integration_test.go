package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestShiftTabIntegration(t *testing.T) {
	// Integration test: Test mode cycling (what Shift+Tab does) by calling cycleMode directly
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Verify starting mode
	if model.inputMode != ModeDurationStartTime {
		t.Fatalf("Expected starting mode ModeDurationStartTime, got %v", model.inputMode)
	}

	// Simulate first Shift+Tab (calls cycleMode)
	model = model.cycleMode()

	// Verify mode changed
	if model.inputMode != ModeStartEndTime {
		t.Errorf("After first mode cycle, expected ModeStartEndTime, got %v", model.inputMode)
	}

	// Simulate second Shift+Tab
	model = model.cycleMode()

	// Should now be in Duration+End mode
	if model.inputMode != ModeDurationEndTime {
		t.Errorf("After second mode cycle, expected ModeDurationEndTime, got %v", model.inputMode)
	}

	// Simulate third Shift+Tab
	model = model.cycleMode()

	// Should wrap back to Duration+Start mode
	if model.inputMode != ModeDurationStartTime {
		t.Errorf("After third mode cycle, expected ModeDurationStartTime (wrap around), got %v", model.inputMode)
	}

	t.Log("Mode cycling (Shift+Tab functionality) integration test completed successfully")
}

func TestShiftTabWithErrorClearing(t *testing.T) {
	// Test that Shift+Tab clears validation errors
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Create validation error by trying to submit with empty keyword
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Empty - will cause error
		case "duration":
			model.fields[i].input.SetValue("01:00:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Try to submit - should fail
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected validation error for empty keyword")
	}

	t.Log("Created validation error:", model.err.Error())

	// Now cycle mode (what Shift+Tab does) - should clear error
	model = model.cycleMode()

	if model.err != nil {
		t.Errorf("Expected error to be cleared after Shift+Tab, but got: %v", model.err)
	}

	// Verify mode changed
	if model.inputMode != ModeStartEndTime {
		t.Errorf("Expected mode to change to ModeStartEndTime, got %v", model.inputMode)
	}

	t.Log("Shift+Tab successfully cleared validation error and changed mode")
}

func TestViewDisplaysCorrectModeHighlighting(t *testing.T) {
	// Test that the view shows the correct mode highlighting as we cycle
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	// Test each mode
	modes := []InputMode{
		ModeDurationStartTime,
		ModeStartEndTime,
		ModeDurationEndTime,
	}

	for _, mode := range modes {
		model := NewFieldEditorModelWithMode(entry, mode)
		view := model.View()

		// Should contain all mode names
		if !strings.Contains(view, "Duration + Start Time") {
			t.Error("View should always show all available modes")
		}
		if !strings.Contains(view, "Start Time + End Time") {
			t.Error("View should always show all available modes")
		}
		if !strings.Contains(view, "Duration + End Time") {
			t.Error("View should always show all available modes")
		}

		// Should contain the modes header
		if !strings.Contains(view, "Modes:") {
			t.Error("View should contain 'Modes:' header")
		}

		// Should contain mode separators
		if !strings.Contains(view, "|") {
			t.Error("View should contain mode separators")
		}

		// Should contain the new help text
		if !strings.Contains(view, "Shift+Tab: Switch Mode") {
			t.Error("View should show updated help text")
		}

		t.Logf("View for mode %v correctly displays mode selector", mode)
	}
}

func TestF1F2F3KeysRemovedFromCode(t *testing.T) {
	// Since we removed F1/F2/F3 handling from the code, this test just verifies
	// that the functionality works through the new cycleMode method
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	// Test that we can access all three modes through cycling
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	modes := []InputMode{}
	modes = append(modes, model.inputMode)

	// Cycle through modes
	model = model.cycleMode()
	modes = append(modes, model.inputMode)

	model = model.cycleMode()
	modes = append(modes, model.inputMode)

	// Should have seen all three modes
	if len(modes) != 3 {
		t.Errorf("Expected 3 modes, got %d", len(modes))
	}

	// Should have all different modes
	expected := []InputMode{ModeDurationStartTime, ModeStartEndTime, ModeDurationEndTime}
	for i, expectedMode := range expected {
		if modes[i] != expectedMode {
			t.Errorf("Mode %d: expected %v, got %v", i, expectedMode, modes[i])
		}
	}

	t.Log("All three modes accessible through cycling (replacing F1/F2/F3 functionality)")
}
