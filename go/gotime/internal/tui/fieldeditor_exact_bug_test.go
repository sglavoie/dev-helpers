package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestExactBugScenario(t *testing.T) {
	// This test replicates the EXACT scenario described by the user:
	// 1. Create entry with missing keyword -> error
	// 2. Switch mode
	// 3. Fill valid form -> still fails

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Step 1: Fill form with missing keyword
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // MISSING - this will cause error
		case "duration":
			model.fields[i].input.SetValue("01:30:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Try to submit -> should fail with "keyword cannot be empty"
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	// Verify error state
	if model.err == nil {
		t.Fatal("Expected 'keyword cannot be empty' error, but got none")
	}
	if !strings.Contains(model.err.Error(), "keyword cannot be empty") {
		t.Errorf("Expected 'keyword cannot be empty', got: %s", model.err.Error())
	}
	if model.done {
		t.Fatal("Model should not be done after validation error")
	}
	if cmd != nil {
		t.Error("Should not have quit command after validation error")
	}

	t.Log("Step 1 ✓ - Got expected validation error:", model.err.Error())

	// Step 2: User switches mode (let's say F2 for Start+End mode)
	// We need to construct a proper KeyMsg for F2
	// Based on bubble tea, F2 should have String() == "f2"

	// Let's directly test what happens in the Update method with mode switching
	// First, let's verify the error is there before mode switch
	if model.err == nil {
		t.Fatal("Error should still be present before mode switch")
	}

	// Call switchMode directly (this is what happens in Update when F2 is pressed)
	newModel := model.switchMode(ModeStartEndTime)

	// This should clear the error!
	if newModel.err != nil {
		t.Errorf("FOUND THE BUG! Error should be cleared after mode switch, but got: %v", newModel.err)
		t.Errorf("This means switchMode is NOT properly clearing the error state!")
	}

	// Update our model reference
	model = newModel

	t.Log("Step 2 ✓ - Switched to Start+End mode, error cleared:", model.err == nil)

	// Step 3: User fills in valid data in the new mode
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("valid_keyword") // NOW FILLED!
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 12:00:00")
		}
	}

	t.Log("Step 3 ✓ - Filled form with valid data")

	// Step 4: User submits again -> this SHOULD work now
	updatedModel, cmd = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	// This should succeed!
	if model.err != nil {
		t.Errorf("VALIDATION STILL FAILING! Expected success after fixing keyword, but got error: %v", model.err)
		t.Errorf("This indicates the bug is still present!")

		// Let's debug what's happening
		t.Logf("Debug - Current field values:")
		for _, field := range model.fields {
			t.Logf("  %s: '%s'", field.name, field.input.Value())
		}

		// Let's try calling validateAndApplyChanges directly to see what fails
		directErr := model.validateAndApplyChanges()
		if directErr != nil {
			t.Logf("Direct validation error: %v", directErr)
		}
	} else {
		t.Log("Step 4 ✓ - Form submitted successfully!")
	}

	if !model.done {
		t.Error("Model should be done after successful validation")
	}

	if cmd == nil {
		t.Error("Should have quit command after successful submission")
	}

	// Verify the entry was properly updated
	if model.entry.Keyword != "valid_keyword" {
		t.Errorf("Entry keyword should be 'valid_keyword', got '%s'", model.entry.Keyword)
	}

	t.Log("SUCCESS - Bug scenario resolved!")
}

func TestModeSwitchingClearsError(t *testing.T) {
	// This is a focused test specifically on error clearing during mode switches

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	// Start with Duration + Start mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Artificially create an error to simulate the state after a validation failure
	model.err = fmt.Errorf("test validation error")

	if model.err == nil {
		t.Fatal("Error should be present for test setup")
	}

	// Test each mode switch clears the error
	testCases := []struct {
		targetMode InputMode
		modeName   string
	}{
		{ModeStartEndTime, "Start+End Time"},
		{ModeDurationEndTime, "Duration+End Time"},
		{ModeDurationStartTime, "Duration+Start Time"},
	}

	for _, tc := range testCases {
		t.Run("Switch_to_"+tc.modeName, func(t *testing.T) {
			// Set error state
			modelWithError := model
			modelWithError.err = fmt.Errorf("test error before switch")

			// Switch mode
			newModel := modelWithError.switchMode(tc.targetMode)

			// Error should be cleared
			if newModel.err != nil {
				t.Errorf("Mode switch to %s should clear error, but got: %v", tc.modeName, newModel.err)
			}

			// Mode should be correct
			if newModel.inputMode != tc.targetMode {
				t.Errorf("Expected mode %v, got %v", tc.targetMode, newModel.inputMode)
			}
		})
	}
}

// Helper function to create a KeyMsg that simulates function key presses
func createFunctionKeyMsg(key string) tea.Msg {
	// This is a simplified mock - in real usage, bubble tea would create the appropriate KeyMsg
	return mockFunctionKey{key: key}
}

type mockFunctionKey struct {
	key string
}

func (m mockFunctionKey) String() string {
	return m.key
}
