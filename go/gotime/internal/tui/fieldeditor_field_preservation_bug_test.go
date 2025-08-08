package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestFieldPreservationBugReproduction(t *testing.T) {
	// This test reproduces the exact issue described:
	// When mode is switched, field values are not preserved

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	// Start in Duration + Start Time mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// User types new values in fields
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("modified_keyword")
		case "tags":
			model.fields[i].input.SetValue("tag1, tag2")
		case "duration":
			model.fields[i].input.SetValue("02:30:00") // User changes from 1 hour to 2.5 hours
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 09:00:00") // User changes start time
		}
	}

	// Verify user input is set correctly
	for _, field := range model.fields {
		switch field.name {
		case "duration":
			if field.input.Value() != "02:30:00" {
				t.Errorf("Expected duration to be '02:30:00', but got '%s'", field.input.Value())
			}
		case "start_time":
			if field.input.Value() != "2025-08-08 09:00:00" {
				t.Errorf("Expected start time to be '2025-08-08 09:00:00', but got '%s'", field.input.Value())
			}
		}
	}

	t.Log("Before mode switch:")
	for _, field := range model.fields {
		t.Logf("  %s: '%s'", field.name, field.input.Value())
	}

	// Switch to Start + End Time mode
	newModel := model.switchMode(ModeStartEndTime)

	t.Log("After mode switch:")
	for _, field := range newModel.fields {
		t.Logf("  %s: '%s'", field.name, field.input.Value())
	}

	// Check that user input was preserved
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "modified_keyword" {
				t.Errorf("Keyword not preserved: expected 'modified_keyword', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "tag1, tag2" {
				t.Errorf("Tags not preserved: expected 'tag1, tag2', got '%s'", field.input.Value())
			}
		case "start_time":
			if field.input.Value() != "2025-08-08 09:00:00" {
				t.Errorf("FIELD PRESERVATION BUG: Start time not preserved: expected '2025-08-08 09:00:00', got '%s'", field.input.Value())
			}
		case "end_time":
			// End time should be calculated from the preserved start time and duration
			// Start: 09:00:00, Duration: 2.5 hours -> End: 11:30:00
			expected := "2025-08-08 11:30:00"
			if field.input.Value() != expected {
				t.Errorf("End time not calculated correctly from preserved values: expected '%s', got '%s'",
					expected, field.input.Value())
			}
		}
	}
}

func TestComputedFieldsOnlyUpdated(t *testing.T) {
	// Test that only computed/calculated fields are updated, not user input fields

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 14, 0, 0, 0, time.Local), // 2 PM
		Active:    false,
	}

	// Test Duration + End Time mode (start time is computed)
	model := NewFieldEditorModelWithMode(entry, ModeDurationEndTime)

	// User enters duration and end time
	for i, field := range model.fields {
		switch field.name {
		case "duration":
			model.fields[i].input.SetValue("03:00:00") // 3 hours
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 17:00:00") // 5 PM
		}
	}

	// Manually call updateComputedFieldValues to see what it does
	originalDuration := ""
	originalEndTime := ""
	for _, field := range model.fields {
		switch field.name {
		case "duration":
			originalDuration = field.input.Value()
		case "end_time":
			originalEndTime = field.input.Value()
		}
	}

	// This should NOT overwrite user input, only update computed fields
	model.updateComputedFieldValues()

	// Verify user input was NOT overwritten
	for _, field := range model.fields {
		switch field.name {
		case "duration":
			if field.input.Value() != originalDuration {
				t.Errorf("BUG: Duration field was overwritten! Expected '%s', got '%s'",
					originalDuration, field.input.Value())
			}
		case "end_time":
			if field.input.Value() != originalEndTime {
				t.Errorf("BUG: End time field was overwritten! Expected '%s', got '%s'",
					originalEndTime, field.input.Value())
			}
		}
	}
}

func TestSpecificScenarioFromUser(t *testing.T) {
	// Test the exact scenario the user described for gt set command

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original",
		Tags:      []string{"old"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// User modifies several fields
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("updated_keyword")
		case "tags":
			model.fields[i].input.SetValue("new, tags")
		case "duration":
			model.fields[i].input.SetValue("02:00:00") // Change to 2 hours
		}
	}

	// User switches mode using Shift+Tab (which calls cycleMode -> switchMode)
	newModel := model.cycleMode()

	// ALL user changes should be preserved
	preserved := true
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "updated_keyword" {
				t.Errorf("Keyword not preserved after mode switch")
				preserved = false
			}
		case "tags":
			if field.input.Value() != "new, tags" {
				t.Errorf("Tags not preserved after mode switch")
				preserved = false
			}
		case "start_time":
			// Start time should be preserved (it exists in both modes)
			if field.input.Value() != "2025-08-08 10:00:00" {
				t.Errorf("Start time not preserved after mode switch: got '%s'", field.input.Value())
				preserved = false
			}
		}
	}

	if !preserved {
		t.Error("USER BUG CONFIRMED: Field values are not preserved when switching modes")
	} else {
		t.Log("SUCCESS: All field values properly preserved during mode switch")
	}
}
