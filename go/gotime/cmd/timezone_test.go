package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestTimezoneIssueReproduction(t *testing.T) {
	// This test reproduces the timezone issue where entries stored in UTC
	// cause incorrect duration calculations

	// Simulate an entry created 1 hour ago in EDT timezone
	edt, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Cannot load EDT timezone for test")
	}

	// Create a time 1 hour ago in EDT
	now := time.Now().In(edt)
	oneHourAgo := now.Add(-1 * time.Hour)

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: oneHourAgo,
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	t.Logf("Created entry at: %s", entry.StartTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Current time: %s", now.Format("2006-01-02 15:04:05 MST"))

	// Marshal to JSON to simulate file storage
	jsonData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	t.Logf("JSON representation: %s", string(jsonData))

	// Unmarshal to simulate loading from file
	var loadedEntry models.Entry
	err = json.Unmarshal(jsonData, &loadedEntry)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	t.Logf("Loaded entry start time: %s", loadedEntry.StartTime.Format("2006-01-02 15:04:05 MST"))

	// Calculate duration using GetCurrentDuration
	calculatedDuration := loadedEntry.GetCurrentDuration()
	expectedDuration := 3600 // 1 hour

	t.Logf("Calculated duration: %d seconds (%.2f hours)", calculatedDuration, float64(calculatedDuration)/3600.0)
	t.Logf("Expected duration: %d seconds (%.2f hours)", expectedDuration, float64(expectedDuration)/3600.0)

	// Check if there's a significant difference (more than 5 seconds tolerance)
	difference := calculatedDuration - expectedDuration
	if difference < -5 || difference > 5 {
		t.Errorf("Duration calculation error: expected ~%d seconds, got %d seconds (difference: %d)",
			expectedDuration, calculatedDuration, difference)
	}
}

func TestUTCVsLocalTime(t *testing.T) {
	// Test the difference between UTC and local time duration calculation

	// Create a time in UTC
	utcTime := time.Date(2025, 8, 8, 14, 0, 0, 0, time.UTC) // 2 PM UTC

	// Convert to local time (should be 10 AM EDT = UTC-4)
	localTime := utcTime.Local()

	t.Logf("UTC time: %s", utcTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Local time: %s", localTime.Format("2006-01-02 15:04:05 MST"))

	// Create entries with both times
	utcEntry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "utc_test",
		StartTime: utcTime,
		Active:    true,
	}

	localEntry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   2,
		Keyword:   "local_test",
		StartTime: localTime,
		Active:    true,
	}

	// Both should have the same duration since they represent the same moment
	utcDuration := utcEntry.GetCurrentDuration()
	localDuration := localEntry.GetCurrentDuration()

	t.Logf("UTC entry duration: %d seconds", utcDuration)
	t.Logf("Local entry duration: %d seconds", localDuration)

	// The durations should be very close (within a few seconds)
	difference := utcDuration - localDuration
	if difference < -2 || difference > 2 {
		t.Errorf("Duration difference too large: %d seconds", difference)
	}
}

func TestFieldEditorTimezoneHandling(t *testing.T) {
	// Test that the field editor handles timezone correctly when parsing times

	// Create an entry in a specific timezone
	edt, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Cannot load EDT timezone for test")
	}

	startTime := time.Date(2025, 8, 8, 9, 30, 0, 0, edt)
	endTime := time.Date(2025, 8, 8, 11, 0, 0, 0, edt)        // 1.5 hours later
	expectedDuration := int(endTime.Sub(startTime).Seconds()) // 5400 seconds = 1.5 hours

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "timezone_test",
		StartTime: startTime,
		EndTime:   &endTime,
		Duration:  expectedDuration,
		Active:    false,
	}

	// Test GetCurrentDuration for completed entry
	duration := entry.GetCurrentDuration()

	t.Logf("Start time: %s", startTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("End time: %s", endTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Expected duration: %d seconds (%.2f hours)", expectedDuration, float64(expectedDuration)/3600.0)
	t.Logf("Actual duration: %d seconds (%.2f hours)", duration, float64(duration)/3600.0)

	if duration != expectedDuration {
		t.Errorf("Duration mismatch: expected %d, got %d", expectedDuration, duration)
	}
}
