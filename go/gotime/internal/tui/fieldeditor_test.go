package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestFieldEditorEndTimeHandling(t *testing.T) {
	// Test that the field editor properly handles end_time field
	now := time.Now()
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   &now,
		Duration:  3600,
		Active:    false,
	}

	model := NewFieldEditorModel(entry)

	// Check that end_time field exists
	endTimeFieldFound := false
	for _, field := range model.fields {
		if field.name == "end_time" {
			endTimeFieldFound = true
			expectedValue := now.Format("2006-01-02 15:04:05")
			if field.value != expectedValue {
				t.Errorf("Expected end_time value %s, got %s", expectedValue, field.value)
			}
			break
		}
	}

	if !endTimeFieldFound {
		t.Error("end_time field not found in field editor")
	}
}

func TestFieldEditorActiveEntryHandling(t *testing.T) {
	// Test that active entries show empty end_time
	now := time.Now()
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	model := NewFieldEditorModel(entry)

	// Check that end_time field is empty for active entries
	for _, field := range model.fields {
		if field.name == "end_time" {
			if field.value != "" {
				t.Errorf("Expected empty end_time for active entry, got %s", field.value)
			}
			break
		}
	}
}

func TestValidateEndTimeEmpty(t *testing.T) {
	// Test that empty end_time makes entry active (using Start+End mode)
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   nil,
		Duration:  3600,
		Active:    false,
	}

	// Use Start+End mode which has end_time field
	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Set fields for validation
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("") // Empty end time
		}
	}

	// Validate changes
	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that entry is now active with nil end time
	if !model.entry.Active {
		t.Error("Entry should be active when end_time is empty")
	}
	if model.entry.EndTime != nil {
		t.Error("EndTime should be nil when end_time field is empty")
	}
	if model.entry.Duration != 0 {
		t.Error("Duration should be reset to 0 for active entries")
	}
}

func TestValidateEndTimeWithValue(t *testing.T) {
	// Test that setting end_time makes entry completed and calculates duration
	startTime := time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local)
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: startTime,
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Use Start+End mode which has end_time field
	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Set end_time to 2 hours after start
	endTime := startTime.Add(2 * time.Hour)
	endTimeStr := endTime.Format("2006-01-02 15:04:05")

	// Set all required fields
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("")
		case "start_time":
			model.fields[i].input.SetValue(startTime.Format("2006-01-02 15:04:05"))
		case "end_time":
			model.fields[i].input.SetValue(endTimeStr)
		}
	}

	// Validate changes
	err := model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that entry is now completed with correct duration
	if model.entry.Active {
		t.Error("Entry should not be active when end_time is set")
	}
	if model.entry.EndTime == nil {
		t.Error("EndTime should not be nil when end_time field is set")
	}
	expectedDuration := 7200 // 2 hours
	if model.entry.Duration != expectedDuration {
		t.Errorf("Expected duration %d, got %d", expectedDuration, model.entry.Duration)
	}
}

func TestValidateEndTimeBeforeStartTime(t *testing.T) {
	// Test validation error when end_time is before start_time
	startTime := time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local)
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: startTime,
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Use Start+End mode which has end_time field
	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Set end_time to before start_time
	endTime := startTime.Add(-1 * time.Hour)
	endTimeStr := endTime.Format("2006-01-02 15:04:05")

	// Set all fields including invalid end time
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("")
		case "start_time":
			model.fields[i].input.SetValue(startTime.Format("2006-01-02 15:04:05"))
		case "end_time":
			model.fields[i].input.SetValue(endTimeStr)
		}
	}

	// Validate changes - should return error
	err := model.validateAndApplyChanges()
	if err == nil {
		t.Error("Expected error when end_time is before start_time")
		return
	}
	expectedError := "end time must be after start time"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}
