package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
)

func TestAddCommandTimezoneIssue(t *testing.T) {
	// Test the specific issue with the add command and timezone handling
	// This reproduces what the user described: entries stored using UTC but durations wrong

	// The issue might be in how we handle local vs UTC time in the add command
	now := time.Now()

	// Simulate what the add command does - creates an entry 1 hour ago in local time
	oneHourAgo := now.Add(-1 * time.Hour)

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: oneHourAgo, // This should be in local timezone
		EndTime:   &now,       // This should also be in local timezone
		Duration:  3600,       // 1 hour in seconds
		Active:    false,      // Completed entry
	}

	t.Logf("Entry start time: %s", entry.StartTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Entry end time: %s", entry.EndTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Stored duration: %d seconds", entry.Duration)

	// Calculate duration using time difference
	actualTimeDiff := int(entry.EndTime.Sub(entry.StartTime).Seconds())
	t.Logf("Actual time difference: %d seconds", actualTimeDiff)

	// Get duration using GetCurrentDuration (should return stored duration for completed entries)
	calculatedDuration := entry.GetCurrentDuration()
	t.Logf("GetCurrentDuration result: %d seconds", calculatedDuration)

	// For completed entries, GetCurrentDuration should return the stored Duration
	if calculatedDuration != entry.Duration {
		t.Errorf("GetCurrentDuration should return stored duration %d for completed entries, got %d",
			entry.Duration, calculatedDuration)
	}

	// The stored duration should match the actual time difference
	if actualTimeDiff != entry.Duration {
		t.Errorf("Stored duration %d should match actual time difference %d",
			entry.Duration, actualTimeDiff)
	}

	// Check if there are any timezone-related discrepancies
	if actualTimeDiff != 3600 {
		t.Errorf("Time difference should be 3600 seconds (1 hour), got %d", actualTimeDiff)
	}
}

func TestFieldEditorTimezoneValidation(t *testing.T) {
	// Test the field editor's timezone handling when parsing user input

	// Create an entry as the field editor would see it
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

	// Create field editor model (just to test it can be created)
	_ = tui.NewFieldEditorModel(entry)

	// Check the formatted duration display
	expectedDurationStr := "01:00:00"

	// We can't access private fields directly, but we can test the formatting functions
	durationSeconds := entry.GetCurrentDuration()
	formattedDuration := formatDurationHMS(durationSeconds)

	t.Logf("Entry duration: %d seconds", durationSeconds)
	t.Logf("Formatted duration: %s", formattedDuration)
	t.Logf("Expected format: %s", expectedDurationStr)

	if formattedDuration != expectedDurationStr {
		t.Errorf("Duration formatting incorrect: expected %s, got %s", expectedDurationStr, formattedDuration)
	}
}

// Helper function to format duration as HH:MM:SS (copied from fieldeditor.go)
func formatDurationHMS(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func TestManualTimeEntryTimezone(t *testing.T) {
	// Test what happens when a user manually enters times in the field editor

	// Simulate user entering specific start and end times
	startTimeStr := "2025-08-08 09:00:00"
	endTimeStr := "2025-08-08 11:30:00"

	// Parse as local time (this is what the field editor should do)
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, time.Local)
	if err != nil {
		t.Fatalf("Failed to parse start time: %v", err)
	}

	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local)
	if err != nil {
		t.Fatalf("Failed to parse end time: %v", err)
	}

	// Calculate expected duration (2.5 hours = 9000 seconds)
	expectedDuration := int(endTime.Sub(startTime).Seconds())

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "manual_test",
		StartTime: startTime,
		EndTime:   &endTime,
		Duration:  expectedDuration,
		Active:    false,
	}

	t.Logf("Start time: %s", startTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("End time: %s", endTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Expected duration: %d seconds (%.2f hours)", expectedDuration, float64(expectedDuration)/3600.0)

	// Test GetCurrentDuration
	actualDuration := entry.GetCurrentDuration()
	t.Logf("Actual duration: %d seconds (%.2f hours)", actualDuration, float64(actualDuration)/3600.0)

	if actualDuration != expectedDuration {
		t.Errorf("Duration mismatch: expected %d, got %d", expectedDuration, actualDuration)
	}

	// Test that the times are in the correct timezone
	_, startOffset := startTime.Zone()
	_, endOffset := endTime.Zone()

	t.Logf("Start time timezone offset: %d seconds", startOffset)
	t.Logf("End time timezone offset: %d seconds", endOffset)

	// Both should have the same timezone offset (local time)
	if startOffset != endOffset {
		t.Errorf("Timezone offset mismatch: start=%d, end=%d", startOffset, endOffset)
	}
}
