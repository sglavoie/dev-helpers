package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestInputModes(t *testing.T) {
	testCases := []struct {
		name string
		mode InputMode
	}{
		{"Duration + Start Time", ModeDurationStartTime},
		{"Start Time + End Time", ModeStartEndTime},
		{"Duration + End Time", ModeDurationEndTime},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &models.Entry{
				ID:        uuid.NewString(),
				ShortID:   1,
				Keyword:   "test",
				Tags:      []string{"tag1", "tag2"},
				StartTime: time.Now(),
				Active:    true,
			}

			model := NewFieldEditorModelWithMode(entry, tc.mode)

			// Verify the correct fields are present
			expectedFields := []string{"keyword", "tags"}
			switch tc.mode {
			case ModeDurationStartTime:
				expectedFields = append(expectedFields, "duration", "start_time")
			case ModeStartEndTime:
				expectedFields = append(expectedFields, "start_time", "end_time")
			case ModeDurationEndTime:
				expectedFields = append(expectedFields, "duration", "end_time")
			}

			if len(model.fields) != len(expectedFields) {
				t.Errorf("Expected %d fields for %s mode, got %d", len(expectedFields), tc.name, len(model.fields))
			}

			// Verify field names match
			for i, expectedField := range expectedFields {
				if i >= len(model.fields) {
					t.Errorf("Missing field %s in %s mode", expectedField, tc.name)
					continue
				}
				if model.fields[i].name != expectedField {
					t.Errorf("Field %d: expected %s, got %s in %s mode", i, expectedField, model.fields[i].name, tc.name)
				}
			}

			// Verify input mode is set correctly
			if model.inputMode != tc.mode {
				t.Errorf("Expected input mode %v, got %v", tc.mode, model.inputMode)
			}
		})
	}
}

func TestDetermineInputMode(t *testing.T) {
	testCases := []struct {
		name         string
		entry        *models.Entry
		expectedMode InputMode
	}{
		{
			name: "Active entry should use Duration + Start Time",
			entry: &models.Entry{
				Active:    true,
				StartTime: time.Now(),
				EndTime:   nil,
			},
			expectedMode: ModeDurationStartTime,
		},
		{
			name: "Completed entry should use Start + End Time",
			entry: func() *models.Entry {
				now := time.Now()
				return &models.Entry{
					Active:    false,
					StartTime: now.Add(-time.Hour),
					EndTime:   &now,
				}
			}(),
			expectedMode: ModeStartEndTime,
		},
		{
			name: "New entry should use Duration + Start Time",
			entry: &models.Entry{
				Active:    false,
				StartTime: time.Now(),
				EndTime:   nil,
			},
			expectedMode: ModeDurationStartTime,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode := determineInputMode(tc.entry)
			if mode != tc.expectedMode {
				t.Errorf("Expected mode %v for %s, got %v", tc.expectedMode, tc.name, mode)
			}
		})
	}
}

func TestModeSwitching(t *testing.T) {
	// Create a test entry
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original",
		Tags:      []string{"tag1"},
		StartTime: time.Now(),
		Active:    true,
	}

	// Start with Duration + Start Time mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set some field values
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("updated")
		case "tags":
			model.fields[i].input.SetValue("tag2, tag3")
		case "duration":
			model.fields[i].input.SetValue("02:30:00")
		}
	}

	// Switch to Start + End Time mode
	newModel := model.switchMode(ModeStartEndTime)

	// Verify the mode changed
	if newModel.inputMode != ModeStartEndTime {
		t.Errorf("Expected mode %v after switch, got %v", ModeStartEndTime, newModel.inputMode)
	}

	// Verify that common field values were preserved
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "updated" {
				t.Errorf("Keyword value not preserved during mode switch: expected 'updated', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "tag2, tag3" {
				t.Errorf("Tags value not preserved during mode switch: expected 'tag2, tag3', got '%s'", field.input.Value())
			}
		}
	}

	// Verify the correct fields exist for the new mode
	expectedFields := []string{"keyword", "tags", "start_time", "end_time"}
	if len(newModel.fields) != len(expectedFields) {
		t.Errorf("Expected %d fields after mode switch, got %d", len(expectedFields), len(newModel.fields))
	}
}

func TestDurationStartTimeValidation(t *testing.T) {
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Now(),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set valid values
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("tag1, tag2")
		case "duration":
			model.fields[i].input.SetValue("02:00:00") // 2 hours
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Validation failed with error: %v", err)
	}

	// Verify calculations
	expectedStartTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-08 10:00:00", time.Local)
	expectedEndTime := expectedStartTime.Add(2 * time.Hour)
	expectedDuration := 7200 // 2 hours in seconds

	if !model.entry.StartTime.Equal(expectedStartTime) {
		t.Errorf("Start time not set correctly: expected %v, got %v", expectedStartTime, model.entry.StartTime)
	}

	if model.entry.EndTime == nil || !model.entry.EndTime.Equal(expectedEndTime) {
		t.Errorf("End time not calculated correctly: expected %v, got %v", expectedEndTime, model.entry.EndTime)
	}

	if model.entry.Duration != expectedDuration {
		t.Errorf("Duration not set correctly: expected %d, got %d", expectedDuration, model.entry.Duration)
	}

	if model.entry.Active {
		t.Error("Entry should be marked as completed")
	}
}

func TestStartEndTimeValidation(t *testing.T) {
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Now(),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Set valid values
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("tag1")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 13:30:00") // 3.5 hours later
		}
	}

	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Validation failed with error: %v", err)
	}

	// Verify calculations
	expectedStartTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-08 10:00:00", time.Local)
	expectedEndTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-08 13:30:00", time.Local)
	expectedDuration := int(expectedEndTime.Sub(expectedStartTime).Seconds()) // 3.5 hours

	if !model.entry.StartTime.Equal(expectedStartTime) {
		t.Errorf("Start time not set correctly: expected %v, got %v", expectedStartTime, model.entry.StartTime)
	}

	if model.entry.EndTime == nil || !model.entry.EndTime.Equal(expectedEndTime) {
		t.Errorf("End time not set correctly: expected %v, got %v", expectedEndTime, model.entry.EndTime)
	}

	if model.entry.Duration != expectedDuration {
		t.Errorf("Duration not calculated correctly: expected %d, got %d", expectedDuration, model.entry.Duration)
	}

	if model.entry.Active {
		t.Error("Entry should be marked as completed")
	}
}

func TestDurationEndTimeValidation(t *testing.T) {
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Now(),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationEndTime)

	// Set valid values
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("")
		case "duration":
			model.fields[i].input.SetValue("01:45:00") // 1 hour 45 minutes
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 14:00:00")
		}
	}

	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Validation failed with error: %v", err)
	}

	// Verify calculations
	expectedEndTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-08-08 14:00:00", time.Local)
	expectedDuration := 6300 // 1 hour 45 minutes in seconds
	expectedStartTime := expectedEndTime.Add(-time.Duration(expectedDuration) * time.Second)

	if !model.entry.StartTime.Equal(expectedStartTime) {
		t.Errorf("Start time not calculated correctly: expected %v, got %v", expectedStartTime, model.entry.StartTime)
	}

	if model.entry.EndTime == nil || !model.entry.EndTime.Equal(expectedEndTime) {
		t.Errorf("End time not set correctly: expected %v, got %v", expectedEndTime, model.entry.EndTime)
	}

	if model.entry.Duration != expectedDuration {
		t.Errorf("Duration not set correctly: expected %d, got %d", expectedDuration, model.entry.Duration)
	}

	if model.entry.Active {
		t.Error("Entry should be marked as completed")
	}
}
