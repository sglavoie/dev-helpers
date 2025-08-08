package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestExactUserWorkflow(t *testing.T) {
	// This test simulates the exact user workflow:
	// 1. Open gt set command
	// 2. User navigates to a field and changes value
	// 3. User presses Shift+Tab to switch modes
	// 4. Check if the changed value is preserved

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "meeting",
		Tags:      []string{"work"},
		StartTime: time.Date(2025, 8, 8, 14, 0, 0, 0, time.Local),
		Duration:  1800, // 30 minutes
		Active:    false,
	}

	// Start with initial model (this is what gt set would create)
	model := NewFieldEditorModel(entry)

	t.Log("=== INITIAL STATE ===")
	t.Logf("Mode: %s", model.inputMode.String())
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Simulate user typing in the duration field (change from 30min to 2hrs)
	t.Log("\n=== USER CHANGES DURATION FIELD ===")
	for i, field := range model.fields {
		if field.name == "duration" {
			// Simulate user clearing field and typing new value
			model.fields[i].input.SetValue("")         // User clears field
			model.fields[i].input.SetValue("02:00:00") // User types new duration
			t.Logf("Duration field changed to: %s", model.fields[i].input.Value())
			break
		}
	}

	// Also change keyword to make it more obvious
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("modified_meeting")
			t.Logf("Keyword field changed to: %s", model.fields[i].input.Value())
			break
		}
	}

	t.Log("\n=== AFTER USER CHANGES ===")
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// User presses Shift+Tab to switch modes
	t.Log("\n=== USER PRESSES SHIFT+TAB ===")

	// Create a mock Shift+Tab key message
	shiftTabMsg := createShiftTabKeyMsg()

	// Process the key message through Update method
	updatedModel, cmd := model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)

	if cmd != nil {
		t.Logf("Command generated: %v", cmd)
	}

	t.Log("\n=== AFTER MODE SWITCH ===")
	t.Logf("New mode: %s", model.inputMode.String())
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Check if user changes were preserved
	t.Log("\n=== VERIFICATION ===")
	keywordPreserved := false
	for _, field := range model.fields {
		if field.name == "keyword" && field.input.Value() == "modified_meeting" {
			keywordPreserved = true
			t.Log("✅ Keyword preserved")
			break
		}
	}
	if !keywordPreserved {
		t.Error("❌ Keyword NOT preserved")
	}

	// Check if start time is preserved (it exists in both old and new modes)
	startTimePreserved := false
	for _, field := range model.fields {
		if field.name == "start_time" && field.input.Value() == "2025-08-08 14:00:00" {
			startTimePreserved = true
			t.Log("✅ Start time preserved")
			break
		}
	}
	if !startTimePreserved {
		t.Error("❌ Start time NOT preserved")
	}
}

func TestTypingAndSwitching(t *testing.T) {
	// Test the specific scenario of typing in a field and immediately switching modes

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "task",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Focus on duration field and start typing
	focusDurationField := func(m *FieldEditorModel) {
		for i, field := range m.fields {
			if field.name == "duration" {
				m.fields[i].input.Focus()
				m.focused = i
				break
			}
		}
	}

	focusDurationField(&model)

	// Simulate user typing character by character (this is more realistic)
	chars := []rune{'0', '2', ':', '3', '0', ':', '0', '0'}

	// Clear the field first
	for i, field := range model.fields {
		if field.name == "duration" {
			model.fields[i].input.SetValue("")
			break
		}
	}

	// Type each character
	for _, char := range chars {
		charMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{char},
		}

		updatedModel, _ := model.Update(charMsg)
		model = updatedModel.(FieldEditorModel)
	}

	t.Log("=== AFTER TYPING '02:30:00' ===")
	for _, field := range model.fields {
		if field.name == "duration" {
			t.Logf("Duration field: '%s'", field.input.Value())
			break
		}
	}

	// Immediately switch modes
	shiftTabMsg := createShiftTabKeyMsg()
	updatedModel, _ := model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)

	t.Log("=== AFTER IMMEDIATE MODE SWITCH ===")
	t.Logf("New mode: %s", model.inputMode.String())
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// The duration value should be preserved and used for calculations
	// We switched to Start+End mode, so we should see start_time and calculated end_time
	// Start: 10:00:00, Duration: 02:30:00 -> End: 12:30:00

	startTimeFound := false
	endTimeFound := false
	expectedEndTime := "2025-08-08 12:30:00"

	for _, field := range model.fields {
		switch field.name {
		case "start_time":
			if field.input.Value() == "2025-08-08 10:00:00" {
				startTimeFound = true
				t.Log("✅ Start time correct")
			} else {
				t.Errorf("❌ Start time wrong: %s", field.input.Value())
			}
		case "end_time":
			if field.input.Value() == expectedEndTime {
				endTimeFound = true
				t.Log("✅ End time calculated correctly from preserved duration")
			} else {
				t.Logf("❌ End time calculation wrong: expected %s, got %s", expectedEndTime, field.input.Value())
			}
		}
	}

	if !startTimeFound || !endTimeFound {
		t.Error("Field preservation or calculation failed")
	} else {
		t.Log("🎉 SUCCESS: Typed duration was preserved and used for calculation")
	}
}

// Helper function to create a proper Shift+Tab key message
func createShiftTabKeyMsg() tea.KeyMsg {
	// This creates a KeyMsg that should result in msg.String() == "shift+tab"
	return tea.KeyMsg{
		Type: tea.KeyShiftTab,
	}
}
