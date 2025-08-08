package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestModeSwitchWithErrorRetry(t *testing.T) {
	// This test reproduces the bug where:
	// 1. User gets validation error (e.g. missing keyword)
	// 2. User switches mode (F1, F2, F3)
	// 3. User fixes the error and submits
	// 4. Form still fails despite valid data

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	// Start with Duration + Start Time mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set invalid input (empty keyword)
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Empty - will cause error
		case "duration":
			model.fields[i].input.SetValue("01:30:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Try to submit (should fail)
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected error for empty keyword, but got none")
	}
	if model.done {
		t.Fatal("Should not be done with validation error")
	}

	t.Log("First submission failed as expected:", model.err.Error())

	// Now switch to Start + End Time mode (F2)
	model = model.switchMode(ModeStartEndTime)

	t.Log("Switched to Start+End Time mode")

	// Set valid data in the new mode
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("valid_keyword") // Fixed the error
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 11:30:00")
		}
	}

	t.Log("Set valid data after mode switch")

	// Try to submit again (should succeed)
	updatedModel, cmd := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err != nil {
		t.Errorf("Expected no error after fixing keyword and switching modes, but got: %v", model.err)
	}
	if !model.done {
		t.Error("Expected model to be done after successful validation, but it wasn't")
	}

	// Verify the entry was updated correctly
	if model.entry.Keyword != "valid_keyword" {
		t.Errorf("Expected keyword to be 'valid_keyword', got '%s'", model.entry.Keyword)
	}

	// Check that cmd indicates completion
	if cmd == nil {
		t.Error("Expected quit command after successful submission, but got nil")
	}

	t.Log("Submission succeeded after mode switch and error fix")
}

func TestMultipleModesSwitchesWithErrors(t *testing.T) {
	// Test multiple mode switches with errors in between

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}

	// Step 1: Error in Duration + Start Time mode
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Empty
		case "duration":
			model.fields[i].input.SetValue("01:00:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)
	if model.err == nil {
		t.Fatal("Expected error in first mode")
	}

	// Step 2: Switch to Start + End Time mode
	model = model.switchMode(ModeStartEndTime)

	// Set valid keyword but invalid end time
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("valid_keyword")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 08:00:00") // Before start time
		}
	}

	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)
	if model.err == nil {
		t.Fatal("Expected error for end time before start time")
	}

	// Step 3: Switch to Duration + End Time mode
	model = model.switchMode(ModeDurationEndTime)

	// Set valid data
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("valid_keyword")
		case "duration":
			model.fields[i].input.SetValue("02:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 12:00:00")
		}
	}

	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err != nil {
		t.Errorf("Expected no error after multiple mode switches and fixes, but got: %v", model.err)
	}
	if !model.done {
		t.Error("Should be done after successful validation")
	}

	t.Log("Successfully handled multiple mode switches with errors")
}

func TestErrorStateClearingOnModeSwitch(t *testing.T) {
	// Test that error state is properly cleared when switching modes

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Create an error condition
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("")
		case "duration":
			model.fields[i].input.SetValue("01:00:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Trigger error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected error for setup")
	}

	previousError := model.err
	t.Log("Created error:", previousError.Error())

	// Switch mode - this should clear the error state
	newModel := model.switchMode(ModeStartEndTime)

	// Check that error state is cleared in new model
	if newModel.err != nil {
		t.Errorf("Expected error to be cleared after mode switch, but got: %v", newModel.err)
	}

	// Check the View doesn't show the old error
	view := newModel.View()
	if containsSubstring(view, previousError.Error()) {
		t.Error("Previous error should not be visible in view after mode switch")
	}

	t.Log("Error state properly cleared on mode switch")
}

// Helper function to check if a string contains a substring (simple implementation)
func containsSubstring(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
