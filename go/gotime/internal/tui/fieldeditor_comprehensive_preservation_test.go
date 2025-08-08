package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestComprehensiveFieldPreservation(t *testing.T) {
	// Test field preservation across all mode switches with various user inputs

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original",
		Tags:      []string{"old"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600,
		Active:    false,
	}

	// Test case 1: Duration+Start → Start+End
	t.Run("Duration+Start_to_Start+End", func(t *testing.T) {
		model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

		// User modifies fields
		for i, field := range model.fields {
			switch field.name {
			case "keyword":
				model.fields[i].input.SetValue("modified_keyword_1")
			case "tags":
				model.fields[i].input.SetValue("new, tags, 1")
			case "duration":
				model.fields[i].input.SetValue("03:15:30") // 3 hours 15 min 30 sec
			case "start_time":
				model.fields[i].input.SetValue("2025-08-08 08:45:15")
			}
		}

		// Switch mode
		newModel := model.switchMode(ModeStartEndTime)

		// Verify preservation
		for _, field := range newModel.fields {
			switch field.name {
			case "keyword":
				if field.input.Value() != "modified_keyword_1" {
					t.Errorf("Keyword not preserved: %s", field.input.Value())
				}
			case "tags":
				if field.input.Value() != "new, tags, 1" {
					t.Errorf("Tags not preserved: %s", field.input.Value())
				}
			case "start_time":
				if field.input.Value() != "2025-08-08 08:45:15" {
					t.Errorf("Start time not preserved: %s", field.input.Value())
				}
			case "end_time":
				// Should be calculated: 08:45:15 + 03:15:30 = 12:00:45
				expected := "2025-08-08 12:00:45"
				if field.input.Value() != expected {
					t.Errorf("End time not calculated correctly: expected %s, got %s",
						expected, field.input.Value())
				}
			}
		}
	})

	// Test case 2: Start+End → Duration+End
	t.Run("Start+End_to_Duration+End", func(t *testing.T) {
		model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

		// User modifies fields
		for i, field := range model.fields {
			switch field.name {
			case "keyword":
				model.fields[i].input.SetValue("modified_keyword_2")
			case "start_time":
				model.fields[i].input.SetValue("2025-08-08 14:30:00")
			case "end_time":
				model.fields[i].input.SetValue("2025-08-08 18:45:30")
			}
		}

		// Switch mode
		newModel := model.switchMode(ModeDurationEndTime)

		// Verify preservation
		for _, field := range newModel.fields {
			switch field.name {
			case "keyword":
				if field.input.Value() != "modified_keyword_2" {
					t.Errorf("Keyword not preserved: %s", field.input.Value())
				}
			case "end_time":
				if field.input.Value() != "2025-08-08 18:45:30" {
					t.Errorf("End time not preserved: %s", field.input.Value())
				}
			case "duration":
				// Should be calculated: 18:45:30 - 14:30:00 = 04:15:30
				expected := "04:15:30"
				if field.input.Value() != expected {
					t.Errorf("Duration not calculated correctly: expected %s, got %s",
						expected, field.input.Value())
				}
			}
		}
	})

	// Test case 3: Duration+End → Duration+Start (full cycle)
	t.Run("Duration+End_to_Duration+Start", func(t *testing.T) {
		model := NewFieldEditorModelWithMode(entry, ModeDurationEndTime)

		// User modifies fields
		for i, field := range model.fields {
			switch field.name {
			case "keyword":
				model.fields[i].input.SetValue("modified_keyword_3")
			case "tags":
				model.fields[i].input.SetValue("final, test, tags")
			case "duration":
				model.fields[i].input.SetValue("05:30:45")
			case "end_time":
				model.fields[i].input.SetValue("2025-08-08 20:15:30")
			}
		}

		// Switch mode
		newModel := model.switchMode(ModeDurationStartTime)

		// Verify preservation
		for _, field := range newModel.fields {
			switch field.name {
			case "keyword":
				if field.input.Value() != "modified_keyword_3" {
					t.Errorf("Keyword not preserved: %s", field.input.Value())
				}
			case "tags":
				if field.input.Value() != "final, test, tags" {
					t.Errorf("Tags not preserved: %s", field.input.Value())
				}
			case "duration":
				if field.input.Value() != "05:30:45" {
					t.Errorf("Duration not preserved: %s", field.input.Value())
				}
			case "start_time":
				// Should be calculated: 20:15:30 - 05:30:45 = 14:44:45
				expected := "2025-08-08 14:44:45"
				if field.input.Value() != expected {
					t.Errorf("Start time not calculated correctly: expected %s, got %s",
						expected, field.input.Value())
				}
			}
		}
	})
}

func TestCycleModePreservation(t *testing.T) {
	// Test that cycleMode (used by Shift+Tab) preserves fields correctly

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "cycle_test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 12, 0, 0, 0, time.Local),
		Duration:  7200, // 2 hours
		Active:    false,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// User enters data
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("cycle_modified")
		case "tags":
			model.fields[i].input.SetValue("cycle, test")
		case "duration":
			model.fields[i].input.SetValue("01:45:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 11:15:00")
		}
	}

	// Cycle through all modes and back
	model2 := model.cycleMode()  // Duration+Start → Start+End
	model3 := model2.cycleMode() // Start+End → Duration+End
	model4 := model3.cycleMode() // Duration+End → Duration+Start

	// Verify we're back to original mode
	if model4.inputMode != ModeDurationStartTime {
		t.Errorf("Expected to cycle back to ModeDurationStartTime, got %v", model4.inputMode)
	}

	// Verify all data is preserved after full cycle
	for _, field := range model4.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "cycle_modified" {
				t.Errorf("Keyword not preserved after full cycle: %s", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "cycle, test" {
				t.Errorf("Tags not preserved after full cycle: %s", field.input.Value())
			}
		case "duration":
			if field.input.Value() != "01:45:00" {
				t.Errorf("Duration not preserved after full cycle: %s", field.input.Value())
			}
		case "start_time":
			if field.input.Value() != "2025-08-08 11:15:00" {
				t.Errorf("Start time not preserved after full cycle: %s", field.input.Value())
			}
		}
	}

	t.Log("SUCCESS: All field values preserved through full cycle of mode switches")
}

func TestEdgeCasePreservation(t *testing.T) {
	// Test edge cases like empty fields, special characters, etc.

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "edge_test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  0,
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Test with edge case values
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test with spaces & symbols!")
		case "tags":
			model.fields[i].input.SetValue("tag1, tag2 with spaces, tag3-special")
		case "start_time":
			model.fields[i].input.SetValue("2025-12-31 23:59:59") // Edge case: end of year
		case "end_time":
			model.fields[i].input.SetValue("") // Empty end time (active entry)
		}
	}

	// Switch modes
	newModel := model.switchMode(ModeDurationStartTime)

	// Verify preservation of edge case values
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			expected := "test with spaces & symbols!"
			if field.input.Value() != expected {
				t.Errorf("Special characters in keyword not preserved: %s", field.input.Value())
			}
		case "tags":
			expected := "tag1, tag2 with spaces, tag3-special"
			if field.input.Value() != expected {
				t.Errorf("Complex tags not preserved: %s", field.input.Value())
			}
		case "start_time":
			expected := "2025-12-31 23:59:59"
			if field.input.Value() != expected {
				t.Errorf("Edge case time not preserved: %s", field.input.Value())
			}
		}
	}

	t.Log("SUCCESS: Edge case values preserved correctly")
}
