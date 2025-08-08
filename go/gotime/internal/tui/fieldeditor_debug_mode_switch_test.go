package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestFieldEditorInternalStateAfterModeSwitch(t *testing.T) {
	// This test examines the internal state of the field editor after mode switching
	// to identify where field values are getting lost

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original_keyword",
		Tags:      []string{"original_tag"},
		StartTime: time.Date(2025, 8, 8, 14, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	t.Log("=== STEP 1: CREATE INITIAL MODEL ===")
	model := NewFieldEditorModel(entry)
	t.Logf("Initial mode: %s", model.inputMode.String())
	t.Logf("Entry pointer address: %p", model.entry)
	t.Logf("Entry keyword: %s", model.entry.Keyword)

	// Display initial field values
	t.Log("Initial field values:")
	for i, field := range model.fields {
		t.Logf("  [%d] %s: '%s'", i, field.name, field.input.Value())
	}

	t.Log("\n=== STEP 2: USER MODIFIES KEYWORD FIELD ===")
	// User modifies the keyword field
	keywordFieldIndex := -1
	for i, field := range model.fields {
		if field.name == "keyword" {
			keywordFieldIndex = i
			break
		}
	}

	if keywordFieldIndex == -1 {
		t.Fatal("Could not find keyword field")
	}

	// Simulate user typing in keyword field
	model.fields[keywordFieldIndex].input.SetValue("user_modified_keyword")
	t.Logf("Set keyword field to: %s", model.fields[keywordFieldIndex].input.Value())

	// Also modify tags for completeness
	tagsFieldIndex := -1
	for i, field := range model.fields {
		if field.name == "tags" {
			tagsFieldIndex = i
			break
		}
	}
	model.fields[tagsFieldIndex].input.SetValue("modified, tags")

	t.Log("Field values after user modification:")
	for i, field := range model.fields {
		t.Logf("  [%d] %s: '%s'", i, field.name, field.input.Value())
	}

	t.Log("\n=== STEP 3: USER SWITCHES MODE WITH SHIFT+TAB ===")
	shiftTabMsg := tea.KeyMsg{Type: tea.KeyShiftTab}

	// Switch mode twice to get to Duration + End Time mode
	updatedModel, _ := model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)
	t.Logf("After 1st Shift+Tab: %s", model.inputMode.String())

	updatedModel, _ = model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)
	t.Logf("After 2nd Shift+Tab: %s", model.inputMode.String())

	t.Logf("Entry pointer address after mode switch: %p", model.entry)
	t.Logf("Entry keyword after mode switch: %s", model.entry.Keyword)

	t.Log("Field values after mode switch:")
	for i, field := range model.fields {
		t.Logf("  [%d] %s: '%s'", i, field.name, field.input.Value())
	}

	t.Log("\n=== STEP 4: VERIFY FIELD VALUES PRESERVED ===")
	for _, field := range model.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() == "user_modified_keyword" {
				t.Log("✅ Keyword field value preserved")
			} else {
				t.Errorf("❌ Keyword field value NOT preserved: expected 'user_modified_keyword', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() == "modified, tags" {
				t.Log("✅ Tags field value preserved")
			} else {
				t.Errorf("❌ Tags field value NOT preserved: expected 'modified, tags', got '%s'", field.input.Value())
			}
		}
	}

	t.Log("\n=== STEP 5: SIMULATE ENTER TO VALIDATE AND SAVE ===")
	// Set valid duration and end_time values so validation passes
	for i, field := range model.fields {
		switch field.name {
		case "duration":
			model.fields[i].input.SetValue("02:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 16:00:00")
		}
	}

	t.Log("Field values before validation:")
	fieldValuesBeforeValidation := make(map[string]string)
	for _, field := range model.fields {
		fieldValuesBeforeValidation[field.name] = field.input.Value()
		t.Logf("  %s: '%s'", field.name, field.input.Value())
	}

	t.Logf("Entry keyword BEFORE validation: %s", model.entry.Keyword)
	t.Logf("Entry tags BEFORE validation: %v", model.entry.Tags)

	// Call validateAndApplyChanges directly
	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Validation failed: %v", err)
		return
	}

	t.Logf("Entry keyword AFTER validation: %s", model.entry.Keyword)
	t.Logf("Entry tags AFTER validation: %v", model.entry.Tags)

	t.Log("\n=== STEP 6: FINAL VERIFICATION ===")
	if model.entry.Keyword != "user_modified_keyword" {
		t.Errorf("❌ BUG CONFIRMED: Entry keyword not updated! Expected 'user_modified_keyword', got '%s'", model.entry.Keyword)

		// Debug: Let's manually check what validateAndApplyChanges sees
		t.Log("DEBUG: Manual field inspection:")
		for _, field := range model.fields {
			t.Logf("  Field %s input value: '%s'", field.name, field.input.Value())
		}

	} else {
		t.Log("✅ Entry keyword updated correctly")
	}

	if len(model.entry.Tags) < 2 || model.entry.Tags[0] != "modified" || model.entry.Tags[1] != "tags" {
		t.Errorf("❌ Entry tags not updated correctly: %v", model.entry.Tags)
	} else {
		t.Log("✅ Entry tags updated correctly")
	}
}
