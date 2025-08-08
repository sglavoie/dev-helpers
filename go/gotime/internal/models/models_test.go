package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTimezoneHandling(t *testing.T) {
	// Test that time.Since works correctly across timezones
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	entry := &Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: oneHourAgo,
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Test GetCurrentDuration
	duration := entry.GetCurrentDuration()
	expectedDuration := 3600 // 1 hour

	// Allow some tolerance for test execution time
	if duration < expectedDuration-5 || duration > expectedDuration+5 {
		t.Errorf("Duration calculation incorrect: expected ~%d, got %d", expectedDuration, duration)
	}

	t.Logf("Duration calculation: expected ~%d, got %d", expectedDuration, duration)
	t.Logf("Start time: %s", oneHourAgo.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Current time: %s", now.Format("2006-01-02 15:04:05 MST"))
}

func TestJSONMarshalUnmarshalTimezone(t *testing.T) {
	// Test that JSON marshaling/unmarshaling preserves timezone information correctly

	// Create an entry with a specific timezone
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Cannot load EST timezone for test")
	}

	startTime := time.Date(2025, 8, 8, 10, 0, 0, 0, est)
	entry := &Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: startTime,
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	t.Logf("JSON: %s", string(jsonData))

	// Unmarshal from JSON
	var unmarshaled Entry
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal entry: %v", err)
	}

	// Check that timezone is preserved
	originalZone, originalOffset := entry.StartTime.Zone()
	unmarshaledZone, unmarshaledOffset := unmarshaled.StartTime.Zone()

	t.Logf("Original: %s %s (offset %d)", entry.StartTime.Format("2006-01-02 15:04:05 MST"), originalZone, originalOffset)
	t.Logf("Unmarshaled: %s %s (offset %d)", unmarshaled.StartTime.Format("2006-01-02 15:04:05 MST"), unmarshaledZone, unmarshaledOffset)

	// The times should be equal even if timezone representation changes
	if !entry.StartTime.Equal(unmarshaled.StartTime) {
		t.Errorf("Times are not equal after JSON round-trip")
	}
}

func TestDurationCalculationAfterJSONRoundTrip(t *testing.T) {
	// Test duration calculation after JSON marshaling/unmarshaling

	// Create an entry 30 minutes ago in local time
	now := time.Now()
	thirtyMinutesAgo := now.Add(-30 * time.Minute)

	entry := &Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: thirtyMinutesAgo,
		EndTime:   nil,
		Duration:  0,
		Active:    true,
	}

	// Get duration before JSON round-trip
	originalDuration := entry.GetCurrentDuration()

	// JSON round-trip
	jsonData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var unmarshaled Entry
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Get duration after JSON round-trip (should be slightly larger due to elapsed time)
	unmarshaledDuration := unmarshaled.GetCurrentDuration()

	expectedDuration := 1800 // 30 minutes

	t.Logf("Original duration: %d seconds", originalDuration)
	t.Logf("Unmarshaled duration: %d seconds", unmarshaledDuration)
	t.Logf("Expected duration: ~%d seconds", expectedDuration)

	// Both should be close to 30 minutes, with unmarshaled being slightly larger
	if originalDuration < expectedDuration-5 || originalDuration > expectedDuration+5 {
		t.Errorf("Original duration incorrect: expected ~%d, got %d", expectedDuration, originalDuration)
	}

	if unmarshaledDuration < originalDuration || unmarshaledDuration > expectedDuration+10 {
		t.Errorf("Unmarshaled duration incorrect: expected >%d and <%d, got %d",
			originalDuration, expectedDuration+10, unmarshaledDuration)
	}
}
