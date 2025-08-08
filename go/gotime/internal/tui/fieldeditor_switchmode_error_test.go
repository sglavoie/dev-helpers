package tui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestSwitchModeWithExistingError(t *testing.T) {
	// Test the core issue: when switchMode is called on a model that has an error,
	// the new model should have the error cleared

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set invalid data and create an error
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

	// Trigger validation error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected error for empty keyword")
	}

	originalError := model.err
	t.Log("Created error in original model:", originalError.Error())

	// Now call switchMode directly (this simulates what happens when F1/F2/F3 is pressed)
	newModel := model.switchMode(ModeStartEndTime)

	// The new model should NOT have the error from the original model
	if newModel.err != nil {
		t.Errorf("Expected new model to have no error after mode switch, but got: %v", newModel.err)
	}

	// Verify mode actually changed
	if newModel.inputMode != ModeStartEndTime {
		t.Errorf("Expected mode to be ModeStartEndTime, got %v", newModel.inputMode)
	}

	t.Log("Mode switch properly cleared error state")
}

func TestUpdateMethodModeSwitchErrorClearing(t *testing.T) {
	// Test that the Update method properly clears errors when processing F1/F2/F3 keys

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Create error state by trying to validate with empty keyword
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("")
		}
	}

	// Simulate pressing Enter to create error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected validation error for setup")
	}

	t.Log("Error created:", model.err.Error())

	// Now let's simulate the switchMode being called from Update method
	// We'll test each mode switch case directly

	// Test F2 -> ModeStartEndTime
	newModel := model.switchMode(ModeStartEndTime)
	if newModel.err != nil {
		t.Errorf("F2 mode switch should clear error, but got: %v", newModel.err)
	}
	if newModel.inputMode != ModeStartEndTime {
		t.Errorf("Expected ModeStartEndTime, got %v", newModel.inputMode)
	}

	// Test F3 -> ModeDurationEndTime (starting from error state again)
	// Create a fresh model with error state
	modelWithError := model
	modelWithError.err = fmt.Errorf("test error")
	newModel = modelWithError.switchMode(ModeDurationEndTime)
	if newModel.err != nil {
		t.Errorf("F3 mode switch should clear error, but got: %v", newModel.err)
	}
	if newModel.inputMode != ModeDurationEndTime {
		t.Errorf("Expected ModeDurationEndTime, got %v", newModel.inputMode)
	}

	// Test F1 -> ModeDurationStartTime
	modelWithError.err = fmt.Errorf("test error")
	newModel = modelWithError.switchMode(ModeDurationStartTime)
	if newModel.err != nil {
		t.Errorf("F1 mode switch should clear error, but got: %v", newModel.err)
	}
	if newModel.inputMode != ModeDurationStartTime {
		t.Errorf("Expected ModeDurationStartTime, got %v", newModel.inputMode)
	}

	t.Log("All mode switches properly cleared error state")
}

func TestRealWorldScenario(t *testing.T) {
	// Test the exact scenario the user described:
	// 1. Get validation error
	// 2. Switch mode
	// 3. Fill valid data
	// 4. Submit should work

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Step 1: Create validation error
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Empty keyword
		case "duration":
			model.fields[i].input.SetValue("01:00:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Try to submit - should fail
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected validation error")
	}
	if model.done {
		t.Fatal("Should not be done with validation error")
	}
	if cmd != nil {
		t.Error("Should not have quit command with validation error")
	}

	t.Log("Step 1 - Validation error created:", model.err.Error())

	// Step 2: Switch mode (this should clear the error)
	model = model.switchMode(ModeStartEndTime)

	if model.err != nil {
		t.Errorf("Error should be cleared after mode switch, but got: %v", model.err)
	}
	if model.inputMode != ModeStartEndTime {
		t.Errorf("Expected ModeStartEndTime, got %v", model.inputMode)
	}

	t.Log("Step 2 - Mode switched and error cleared")

	// Step 3: Set valid data in new mode
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("valid_keyword") // Fix the error!
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 12:00:00")
		}
	}

	t.Log("Step 3 - Valid data entered")

	// Step 4: Submit should now succeed
	updatedModel, cmd = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err != nil {
		t.Errorf("Expected no error after fixing data, but got: %v", model.err)
	}
	if !model.done {
		t.Error("Expected model to be done after successful validation")
	}
	if cmd == nil {
		t.Error("Expected quit command after successful submission")
	}

	// Verify the data was applied correctly
	if model.entry.Keyword != "valid_keyword" {
		t.Errorf("Expected keyword 'valid_keyword', got '%s'", model.entry.Keyword)
	}

	t.Log("Step 4 - SUCCESS: Submission worked after mode switch and error fix!")
}
