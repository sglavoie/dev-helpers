package cmd

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestAddCommandDurationBug(t *testing.T) {
	// This test reproduces the 3-hour offset bug
	// When user sets a 1-hour duration, it should show 1 hour, not 4 hours

	// Create a test entry as the add command would
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

	// Simulate what happens when user enters "01:00:00" (1 hour) in the field editor
	// This is what parseDurationHMS would return for "01:00:00"
	expectedDurationSeconds := 3600 // 1 hour = 3600 seconds

	// Simulate the field editor's validation logic for duration field
	now := time.Now()
	if entry.Active {
		// For active entries, adjust the start time
		newStartTime := now.Add(-time.Duration(expectedDurationSeconds) * time.Second)
		entry.StartTime = newStartTime
	} else {
		entry.Duration = expectedDurationSeconds
	}

	// Now check what GetCurrentDuration() returns
	actualDuration := entry.GetCurrentDuration()

	// The bug: actualDuration will be much larger than expectedDurationSeconds
	// because it calculates time.Now().Sub(entry.StartTime) which includes
	// the time that has passed since we set the StartTime

	t.Logf("Expected duration: %d seconds (%d hours)", expectedDurationSeconds, expectedDurationSeconds/3600)
	t.Logf("Actual duration: %d seconds (%.2f hours)", actualDuration, float64(actualDuration)/3600.0)

	// The bug manifests as the actual duration being significantly different
	// from the expected duration due to the time elapsed during execution
	difference := actualDuration - expectedDurationSeconds
	t.Logf("Difference: %d seconds (%.2f hours)", difference, float64(difference)/3600.0)

	// If the difference is more than a few seconds, we have the bug
	if difference > 10 { // Allow 10 seconds tolerance for test execution time
		t.Errorf("Duration calculation bug detected: expected ~%d seconds, got %d seconds (difference: %d)",
			expectedDurationSeconds, actualDuration, difference)
	}
}

func TestCompletedEntryDuration(t *testing.T) {
	// Test that completed entries don't have this issue
	startTime := time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local)
	endTime := startTime.Add(1 * time.Hour)

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: startTime,
		EndTime:   &endTime,
		Duration:  3600, // 1 hour
		Active:    false,
	}

	actualDuration := entry.GetCurrentDuration()
	expectedDuration := 3600

	if actualDuration != expectedDuration {
		t.Errorf("Completed entry duration mismatch: expected %d, got %d", expectedDuration, actualDuration)
	}
}

func TestAddCommandDefaults(t *testing.T) {
	// Test that the add command creates entries with reasonable defaults
	now := time.Now()

	// Simulate what the add command creates
	oneHourAgo := now.Add(-1 * time.Hour)
	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: oneHourAgo,
		EndTime:   &now,
		Duration:  3600,
		Active:    false,
	}

	// Verify defaults are reasonable
	if entry.Active {
		t.Error("Add command should create completed entries by default")
	}

	if entry.EndTime == nil {
		t.Error("Add command should set EndTime for completed entries")
	}

	if entry.Duration != 3600 {
		t.Errorf("Expected default duration 3600, got %d", entry.Duration)
	}

	actualDuration := entry.GetCurrentDuration()
	if actualDuration != 3600 {
		t.Errorf("GetCurrentDuration should return %d for completed entry, got %d", 3600, actualDuration)
	}
}

func TestRetrospectiveEntryCreation(t *testing.T) {
	// Test creating a retrospective entry with custom start/end times
	startTime := time.Date(2025, 8, 8, 9, 0, 0, 0, time.Local)
	endTime := time.Date(2025, 8, 8, 11, 30, 0, 0, time.Local)
	expectedDuration := int(endTime.Sub(startTime).Seconds()) // 2.5 hours = 9000 seconds

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "coding",
		Tags:      []string{"golang", "cli"},
		StartTime: startTime,
		EndTime:   &endTime,
		Duration:  expectedDuration,
		Active:    false,
	}

	// Test that GetCurrentDuration returns the stored duration for completed entries
	actualDuration := entry.GetCurrentDuration()
	if actualDuration != expectedDuration {
		t.Errorf("Expected duration %d, got %d", expectedDuration, actualDuration)
	}

	// Test duration matches time difference
	timeDiff := int(endTime.Sub(startTime).Seconds())
	if entry.Duration != timeDiff {
		t.Errorf("Duration %d should match time difference %d", entry.Duration, timeDiff)
	}
}
