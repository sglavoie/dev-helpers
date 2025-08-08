package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestFieldEditorTimezoneParsingFix(t *testing.T) {
	// Test that the field editor correctly parses times as local time, not UTC
	// This test reproduces and verifies the fix for the timezone bug

	// Create an entry
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Now(),
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Use Start+End mode to test both start_time and end_time fields
	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Simulate user entering start_time as local time
	startTimeStr := "2025-08-08 10:00:00"
	endTimeStr := "2025-08-08 11:30:00"

	// Set all required fields
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("")
		case "start_time":
			model.fields[i].input.SetValue(startTimeStr)
		case "end_time":
			model.fields[i].input.SetValue(endTimeStr)
		}
	}

	// Apply changes
	err := model.validateAndApplyChanges()
	if err != nil {
		t.Fatalf("Failed to apply changes: %v", err)
	}

	// Check that times were parsed as local time
	expectedStartTime, _ := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, time.Local)
	expectedEndTime, _ := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local)
	expectedDuration := int(expectedEndTime.Sub(expectedStartTime).Seconds()) // 1.5 hours = 5400 seconds

	t.Logf("Expected start time: %s", expectedStartTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Actual start time: %s", model.entry.StartTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Expected end time: %s", expectedEndTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Actual end time: %s", model.entry.EndTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Expected duration: %d seconds (%.2f hours)", expectedDuration, float64(expectedDuration)/3600.0)
	t.Logf("Actual duration: %d seconds (%.2f hours)", model.entry.Duration, float64(model.entry.Duration)/3600.0)

	// Verify the times match
	if !model.entry.StartTime.Equal(expectedStartTime) {
		t.Errorf("Start time mismatch: expected %v, got %v", expectedStartTime, model.entry.StartTime)
	}

	if !model.entry.EndTime.Equal(expectedEndTime) {
		t.Errorf("End time mismatch: expected %v, got %v", expectedEndTime, *model.entry.EndTime)
	}

	// Verify the duration calculation
	if model.entry.Duration != expectedDuration {
		t.Errorf("Duration calculation incorrect: expected %d seconds, got %d seconds",
			expectedDuration, model.entry.Duration)
	}

	// Verify the entry is marked as completed
	if model.entry.Active {
		t.Error("Entry should be marked as completed when end_time is set")
	}
}

func TestFieldEditorUTCvsLocalTimeParsing(t *testing.T) {
	// Test the difference between UTC and local time parsing
	// This demonstrates what would happen with time.Parse vs time.ParseInLocation

	timeStr := "2025-08-08 14:00:00"

	// Parse as UTC (old buggy behavior)
	utcTime, _ := time.Parse("2006-01-02 15:04:05", timeStr)

	// Parse as local time (fixed behavior)
	localTime, _ := time.ParseInLocation("2006-01-02 15:04:05", timeStr, time.Local)

	t.Logf("Time string: %s", timeStr)
	t.Logf("Parsed as UTC: %s", utcTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Parsed as Local: %s", localTime.Format("2006-01-02 15:04:05 MST"))

	// The difference in seconds is the timezone offset
	difference := int(localTime.Sub(utcTime).Seconds())
	t.Logf("Time difference: %d seconds (%.2f hours)", difference, float64(difference)/3600.0)

	// In EDT (UTC-4), the difference should be 4 hours = 14400 seconds
	expectedDifference := 4 * 3600 // 4 hours in seconds
	if difference != expectedDifference {
		t.Logf("Timezone offset: expected %d seconds, got %d seconds", expectedDifference, difference)
		// Note: This might vary based on the system's timezone, so we don't fail the test
	}
}

func TestFieldEditorTimezoneConsistency(t *testing.T) {
	// Test that start_time and end_time are consistently parsed in the same timezone

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Now(),
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Use Start+End mode to test both start_time and end_time fields
	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	// Set both start_time and end_time
	startTimeStr := "2025-08-08 10:00:00"
	endTimeStr := "2025-08-08 12:00:00"

	// Set all required fields
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("test")
		case "tags":
			model.fields[i].input.SetValue("")
		case "start_time":
			model.fields[i].input.SetValue(startTimeStr)
		case "end_time":
			model.fields[i].input.SetValue(endTimeStr)
		}
	}

	// Apply changes
	err := model.validateAndApplyChanges()
	if err != nil {
		t.Fatalf("Failed to apply changes: %v", err)
	}

	// Check that both times have the same timezone
	_, startOffset := model.entry.StartTime.Zone()
	_, endOffset := model.entry.EndTime.Zone()

	t.Logf("Start time: %s (offset: %d)",
		model.entry.StartTime.Format("2006-01-02 15:04:05 MST"), startOffset)
	t.Logf("End time: %s (offset: %d)",
		model.entry.EndTime.Format("2006-01-02 15:04:05 MST"), endOffset)

	if startOffset != endOffset {
		t.Errorf("Timezone offset mismatch: start_time offset %d, end_time offset %d",
			startOffset, endOffset)
	}

	// Duration should be exactly 2 hours (7200 seconds)
	expectedDuration := 7200
	if model.entry.Duration != expectedDuration {
		t.Errorf("Duration calculation incorrect: expected %d seconds, got %d seconds",
			expectedDuration, model.entry.Duration)
	}
}
