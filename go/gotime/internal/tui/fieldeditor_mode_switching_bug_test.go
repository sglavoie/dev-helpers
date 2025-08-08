package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestModeSwitchingPreservesFieldValues(t *testing.T) {
	// This test reproduces and verifies the fix for the mode switching bug
	// where field values reset when switching modes

	// Create a test entry
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original",
		Tags:      []string{"tag1"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	// Start with Duration + Start Time mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// User modifies the duration to "02:00:00"
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("updated_keyword")
		case "tags":
			model.fields[i].input.SetValue("updated_tag1, updated_tag2")
		case "duration":
			model.fields[i].input.SetValue("02:00:00") // This should be preserved
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	t.Log("Before switching mode:")
	for _, field := range model.fields {
		t.Logf("  %s: %s", field.name, field.input.Value())
	}

	// Switch to Start + End Time mode (F2)
	newModel := model.switchMode(ModeStartEndTime)

	t.Log("After switching to Start+End mode:")
	for _, field := range newModel.fields {
		t.Logf("  %s: %s", field.name, field.input.Value())
	}

	// Verify that user-modified values are preserved
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "updated_keyword" {
				t.Errorf("Keyword value not preserved: expected 'updated_keyword', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "updated_tag1, updated_tag2" {
				t.Errorf("Tags value not preserved: expected 'updated_tag1, updated_tag2', got '%s'", field.input.Value())
			}
		case "start_time":
			if field.input.Value() != "2025-08-08 10:00:00" {
				t.Errorf("Start time not preserved: expected '2025-08-08 10:00:00', got '%s'", field.input.Value())
			}
		case "end_time":
			// The end time should be computed from the preserved duration and start time
			// Duration was 02:00:00 (2 hours), start was 10:00:00, so end should be 12:00:00
			expectedEndTime := "2025-08-08 12:00:00"
			if field.input.Value() != expectedEndTime {
				t.Errorf("End time not computed correctly: expected '%s', got '%s'", expectedEndTime, field.input.Value())
			}
		}
	}

	// Now switch to Duration + End Time mode (F3)
	finalModel := newModel.switchMode(ModeDurationEndTime)

	t.Log("After switching to Duration+End mode:")
	for _, field := range finalModel.fields {
		t.Logf("  %s: %s", field.name, field.input.Value())
	}

	// Verify values are still preserved
	for _, field := range finalModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "updated_keyword" {
				t.Errorf("Keyword value not preserved after second switch: expected 'updated_keyword', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "updated_tag1, updated_tag2" {
				t.Errorf("Tags value not preserved after second switch: expected 'updated_tag1, updated_tag2', got '%s'", field.input.Value())
			}
		case "duration":
			// Duration should be preserved as "02:00:00" (the original user input)
			if field.input.Value() != "02:00:00" {
				t.Errorf("Duration value not preserved after switching modes: expected '02:00:00', got '%s'", field.input.Value())
			}
		case "end_time":
			expectedEndTime := "2025-08-08 12:00:00"
			if field.input.Value() != expectedEndTime {
				t.Errorf("End time not preserved: expected '%s', got '%s'", expectedEndTime, field.input.Value())
			}
		}
	}
}

func TestSpecificBugReported(t *testing.T) {
	// This reproduces the exact bug scenario reported by the user:
	// "If the duration is modified to be say '02:00:00', it will reset to '01:00:00' when the mode is changed"

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600, // Initially 1 hour
		Active:    false,
	}

	// Start with Duration + Start Time mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// User modifies duration to "02:00:00" (this was resetting to "01:00:00" before the fix)
	for i, field := range model.fields {
		if field.name == "duration" {
			model.fields[i].input.SetValue("02:00:00")
			t.Logf("User set duration to: %s", field.input.Value())
		}
	}

	// Switch to a different mode
	newModel := model.switchMode(ModeStartEndTime)

	// Switch back to Duration + Start Time mode
	backModel := newModel.switchMode(ModeDurationStartTime)

	// Verify the duration is still "02:00:00" and NOT "01:00:00"
	for _, field := range backModel.fields {
		if field.name == "duration" {
			t.Logf("After mode switching, duration is: %s", field.input.Value())
			if field.input.Value() == "01:00:00" {
				t.Error("BUG REPRODUCED: Duration reset to original value instead of preserving user change")
			} else if field.input.Value() != "02:00:00" {
				t.Errorf("Duration not preserved correctly: expected '02:00:00', got '%s'", field.input.Value())
			} else {
				t.Log("SUCCESS: Duration properly preserved during mode switching")
			}
		}
	}
}

func TestStartTimeEndTimePreservation(t *testing.T) {
	// Test that Start Time and End Time are also preserved when switching modes

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    false,
	}

	// Start with Start + End Time mode
	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// User modifies start and end times
	for i, field := range model.fields {
		switch field.name {
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 09:30:00") // Changed from 10:00:00
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 14:15:00") // Specific end time
		}
	}

	// Switch to Duration + Start Time mode
	newModel := model.switchMode(ModeDurationStartTime)

	// Verify start time is preserved
	for _, field := range newModel.fields {
		if field.name == "start_time" {
			expected := "2025-08-08 09:30:00"
			if field.input.Value() != expected {
				t.Errorf("Start time not preserved: expected '%s', got '%s'", expected, field.input.Value())
			}
		}
		if field.name == "duration" {
			// Duration should be computed: 14:15 - 09:30 = 4:45 = 04:45:00
			expected := "04:45:00"
			if field.input.Value() != expected {
				t.Errorf("Duration not computed correctly: expected '%s', got '%s'", expected, field.input.Value())
			}
		}
	}

	// Switch to Duration + End Time mode
	finalModel := newModel.switchMode(ModeDurationEndTime)

	// Verify end time and duration are preserved
	for _, field := range finalModel.fields {
		switch field.name {
		case "end_time":
			expected := "2025-08-08 14:15:00"
			if field.input.Value() != expected {
				t.Errorf("End time not preserved: expected '%s', got '%s'", expected, field.input.Value())
			}
		case "duration":
			expected := "04:45:00"
			if field.input.Value() != expected {
				t.Errorf("Duration not preserved: expected '%s', got '%s'", expected, field.input.Value())
			}
		}
	}
}
