package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestExactUserWorkflowWithSave(t *testing.T) {
	// This reproduces the exact issue the user described:
	// 1. Run gt set
	// 2. Select an entry to edit
	// 3. Switch mode (e.g., to Duration + End Time)
	// 4. Edit the keyword field
	// 5. Press Enter to save
	// 6. Check if the keyword was actually updated in the entry

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original_keyword",
		Tags:      []string{"original_tag"},
		StartTime: time.Date(2025, 8, 8, 14, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	t.Log("=== INITIAL ENTRY STATE ===")
	t.Logf("Keyword: %s", entry.Keyword)
	t.Logf("Tags: %v", entry.Tags)

	// Step 1-2: User runs gt set and selects entry (this creates the model)
	model := NewFieldEditorModel(entry)

	t.Log("\n=== INITIAL MODEL ===")
	t.Logf("Mode: %s", model.inputMode.String())
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Step 3: User switches to Duration + End Time mode using Shift+Tab
	t.Log("\n=== STEP 3: USER SWITCHES TO DURATION+END MODE ===")

	// Simulate multiple Shift+Tab presses to get to Duration + End Time mode
	// Duration+Start -> Start+End -> Duration+End
	shiftTabMsg := tea.KeyMsg{Type: tea.KeyShiftTab}

	// First Shift+Tab: Duration+Start -> Start+End
	updatedModel, _ := model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)
	t.Logf("After 1st Shift+Tab: %s", model.inputMode.String())

	// Second Shift+Tab: Start+End -> Duration+End
	updatedModel, _ = model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)
	t.Logf("After 2nd Shift+Tab: %s", model.inputMode.String())

	if model.inputMode != ModeDurationEndTime {
		t.Fatalf("Expected Duration+End mode, got %s", model.inputMode.String())
	}

	t.Log("=== AFTER MODE SWITCH ===")
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Step 4: User edits the keyword field
	t.Log("\n=== STEP 4: USER EDITS KEYWORD FIELD ===")
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("user_modified_keyword")
			t.Logf("Keyword changed to: %s", model.fields[i].input.Value())
			break
		}
	}

	t.Log("=== AFTER USER EDITS KEYWORD ===")
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Also set valid values for duration and end_time to make validation pass
	for i, field := range model.fields {
		switch field.name {
		case "duration":
			model.fields[i].input.SetValue("02:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 16:00:00")
		}
	}

	// Step 5: User presses Enter to save
	t.Log("\n=== STEP 5: USER PRESSES ENTER TO SAVE ===")
	t.Logf("Entry keyword BEFORE save: %s", model.entry.Keyword)

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	t.Logf("Entry keyword AFTER save: %s", model.entry.Keyword)

	if model.err != nil {
		t.Errorf("Validation error: %v", model.err)
		return
	}

	if !model.done {
		t.Error("Model should be done after successful save")
	}

	if cmd == nil {
		t.Error("Should have quit command after save")
	}

	// Step 6: Check if the keyword was actually updated
	t.Log("\n=== STEP 6: VERIFICATION ===")
	if model.entry.Keyword != "user_modified_keyword" {
		t.Errorf("❌ BUG CONFIRMED: Keyword was NOT saved! Expected 'user_modified_keyword', got '%s'", model.entry.Keyword)
		t.Error("This is the exact issue the user reported!")
	} else {
		t.Logf("✅ SUCCESS: Keyword was saved correctly: %s", model.entry.Keyword)
	}

	// Also check tags to make sure they weren't lost
	if len(model.entry.Tags) == 0 || model.entry.Tags[0] != "original_tag" {
		t.Errorf("❌ Tags were lost during save: %v", model.entry.Tags)
	} else {
		t.Logf("✅ Tags preserved: %v", model.entry.Tags)
	}
}

func TestFieldValuesAfterModeSwitch(t *testing.T) {
	// Test specifically that field input values are correct after mode switching

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test_keyword",
		Tags:      []string{"test_tag"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  7200, // 2 hours
		Active:    false,
	}

	model := NewFieldEditorModel(entry)

	// Switch to Duration + End Time mode
	shiftTabMsg := tea.KeyMsg{Type: tea.KeyShiftTab}
	updatedModel, _ := model.Update(shiftTabMsg) // Duration+Start -> Start+End
	model = updatedModel.(FieldEditorModel)
	updatedModel, _ = model.Update(shiftTabMsg) // Start+End -> Duration+End
	model = updatedModel.(FieldEditorModel)

	t.Log("=== AFTER SWITCHING TO DURATION+END MODE ===")
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// User modifies keyword
	keywordModified := false
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("modified_in_duration_end_mode")
			keywordModified = true
			break
		}
	}

	if !keywordModified {
		t.Fatal("Could not find keyword field to modify")
	}

	// Verify the field has the new value
	keywordFieldValue := ""
	for _, field := range model.fields {
		if field.name == "keyword" {
			keywordFieldValue = field.input.Value()
			break
		}
	}

	if keywordFieldValue != "modified_in_duration_end_mode" {
		t.Errorf("Keyword field value not set correctly: got '%s'", keywordFieldValue)
	}

	// Now call validateAndApplyChanges directly to see what happens
	t.Log("\n=== CALLING validateAndApplyChanges DIRECTLY ===")
	t.Logf("Entry keyword before validation: %s", model.entry.Keyword)

	// Set valid duration and end_time for validation to pass
	for i, field := range model.fields {
		switch field.name {
		case "duration":
			model.fields[i].input.SetValue("01:30:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 12:00:00")
		}
	}

	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Validation failed: %v", err)

		// Debug: show what values validation is seeing
		t.Log("DEBUG: Field values seen by validation:")
		for _, field := range model.fields {
			t.Logf("  %s: '%s'", field.name, field.input.Value())
		}
		return
	}

	t.Logf("Entry keyword after validation: %s", model.entry.Keyword)

	if model.entry.Keyword != "modified_in_duration_end_mode" {
		t.Errorf("❌ VALIDATION BUG: Keyword not applied to entry! Expected 'modified_in_duration_end_mode', got '%s'", model.entry.Keyword)
	} else {
		t.Log("✅ Validation correctly applied keyword to entry")
	}
}
