package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestCycleModeFunction(t *testing.T) {
	// Test the cycleMode function cycles through modes in the correct order
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	testCases := []struct {
		currentMode  InputMode
		expectedNext InputMode
		description  string
	}{
		{ModeDurationStartTime, ModeStartEndTime, "Duration+Start → Start+End"},
		{ModeStartEndTime, ModeDurationEndTime, "Start+End → Duration+End"},
		{ModeDurationEndTime, ModeDurationStartTime, "Duration+End → Duration+Start (wrap around)"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			model := NewFieldEditorModelWithMode(entry, tc.currentMode)

			newModel := model.cycleMode()

			if newModel.inputMode != tc.expectedNext {
				t.Errorf("Expected mode %v (%s), got %v (%s)",
					tc.expectedNext, tc.expectedNext.String(),
					newModel.inputMode, newModel.inputMode.String())
			}
		})
	}
}

func TestCycleModeFullCycle(t *testing.T) {
	// Test that cycling through all modes returns to the starting mode
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)
	originalMode := model.inputMode

	// Cycle through all modes and come back to start
	model = model.cycleMode() // Duration+Start → Start+End
	model = model.cycleMode() // Start+End → Duration+End
	model = model.cycleMode() // Duration+End → Duration+Start

	if model.inputMode != originalMode {
		t.Errorf("After full cycle, expected to return to %v, got %v", originalMode, model.inputMode)
	}

	t.Log("Full mode cycle completed successfully")
}

func TestRenderModeSelector(t *testing.T) {
	// Test that the mode selector displays all modes with current one highlighted
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	// Test each mode as the selected one
	testCases := []struct {
		mode InputMode
		name string
	}{
		{ModeDurationStartTime, "Duration + Start Time"},
		{ModeStartEndTime, "Start Time + End Time"},
		{ModeDurationEndTime, "Duration + End Time"},
	}

	for _, tc := range testCases {
		t.Run("Selected_"+strings.ReplaceAll(tc.name, " ", "_"), func(t *testing.T) {
			model := NewFieldEditorModelWithMode(entry, tc.mode)

			selector := model.renderModeSelector()

			// Should contain the header
			if !strings.Contains(selector, "Modes:") {
				t.Error("Mode selector should contain 'Modes:' header")
			}

			// Should contain all three mode names
			if !strings.Contains(selector, "Duration + Start Time") {
				t.Error("Mode selector should contain 'Duration + Start Time'")
			}
			if !strings.Contains(selector, "Start Time + End Time") {
				t.Error("Mode selector should contain 'Start Time + End Time'")
			}
			if !strings.Contains(selector, "Duration + End Time") {
				t.Error("Mode selector should contain 'Duration + End Time'")
			}

			// Should contain separators
			if !strings.Contains(selector, "|") {
				t.Error("Mode selector should contain separator '|'")
			}

			t.Logf("Mode selector for %s: %s", tc.name, selector)
		})
	}
}

func TestViewContainsModeSelector(t *testing.T) {
	// Test that the View method includes the mode selector
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)
	view := model.View()

	// Should contain mode information
	if !strings.Contains(view, "Modes:") {
		t.Error("View should contain mode selector with 'Modes:' header")
	}

	// Should contain all mode names
	if !strings.Contains(view, "Duration + Start Time") {
		t.Error("View should show all available modes")
	}
	if !strings.Contains(view, "Start Time + End Time") {
		t.Error("View should show all available modes")
	}
	if !strings.Contains(view, "Duration + End Time") {
		t.Error("View should show all available modes")
	}

	// Should contain the new help text
	if !strings.Contains(view, "Shift+Tab: Switch Mode") {
		t.Error("View should contain updated help text for Shift+Tab")
	}

	// Should NOT contain old F-key instructions
	if strings.Contains(view, "F1:") || strings.Contains(view, "F2:") || strings.Contains(view, "F3:") {
		t.Error("View should not contain old F-key instructions")
	}

	t.Log("View correctly displays mode selector and help text")
}

func TestCycleModePreservesFieldValues(t *testing.T) {
	// Test that cycling modes preserves field values just like switchMode
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{"tag1"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set some field values
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("updated_keyword")
		case "tags":
			model.fields[i].input.SetValue("updated_tag1, updated_tag2")
		case "duration":
			model.fields[i].input.SetValue("02:30:00")
		}
	}

	// Cycle to next mode
	newModel := model.cycleMode()

	// Verify mode changed
	if newModel.inputMode == model.inputMode {
		t.Error("Mode should have changed after cycling")
	}
	if newModel.inputMode != ModeStartEndTime {
		t.Errorf("Expected ModeStartEndTime, got %v", newModel.inputMode)
	}

	// Verify common field values were preserved
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "updated_keyword" {
				t.Errorf("Keyword not preserved: expected 'updated_keyword', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "updated_tag1, updated_tag2" {
				t.Errorf("Tags not preserved: expected 'updated_tag1, updated_tag2', got '%s'", field.input.Value())
			}
		}
	}

	t.Log("Mode cycling properly preserved field values")
}

func TestCycleModeErrorClearing(t *testing.T) {
	// Test that cycling mode clears errors (since it uses switchMode internally)
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Simulate an error state
	model.err = fmt.Errorf("test validation error")

	if model.err == nil {
		t.Fatal("Error should be present for test setup")
	}

	// Cycle mode
	newModel := model.cycleMode()

	// Error should be cleared
	if newModel.err != nil {
		t.Errorf("Error should be cleared after cycling mode, but got: %v", newModel.err)
	}

	t.Log("Mode cycling properly cleared error state")
}
