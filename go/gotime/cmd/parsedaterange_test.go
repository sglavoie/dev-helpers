package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/filters"
)

func TestParseDateRangeTimezone(t *testing.T) {
	// Test that ParseDateRange interprets dates as local time, not UTC

	filter := filters.NewFilter()
	rangeStr := "2025-08-08,2025-08-10"

	err := ParseDateRange(filter, rangeStr)
	if err != nil {
		t.Fatalf("Failed to parse date range: %v", err)
	}

	// Check that dates were parsed as local time
	expectedStartDate, _ := time.ParseInLocation("2006-01-02", "2025-08-08", time.Local)
	expectedEndDate, _ := time.ParseInLocation("2006-01-02", "2025-08-10", time.Local)
	expectedEndDate = expectedEndDate.Add(24*time.Hour - time.Second) // End of day

	t.Logf("Expected start date: %s", expectedStartDate.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Actual start date: %s", filter.StartDate.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Expected end date: %s", expectedEndDate.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Actual end date: %s", filter.EndDate.Format("2006-01-02 15:04:05 MST"))

	// Verify the dates match
	if !filter.StartDate.Equal(expectedStartDate) {
		t.Errorf("Start date mismatch: expected %v, got %v", expectedStartDate, filter.StartDate)
	}

	if !filter.EndDate.Equal(expectedEndDate) {
		t.Errorf("End date mismatch: expected %v, got %v", expectedEndDate, filter.EndDate)
	}

	// Check timezone consistency
	_, startOffset := filter.StartDate.Zone()
	_, endOffset := filter.EndDate.Zone()

	t.Logf("Start date timezone offset: %d seconds", startOffset)
	t.Logf("End date timezone offset: %d seconds", endOffset)

	if startOffset != endOffset {
		t.Errorf("Timezone offset mismatch: start=%d, end=%d", startOffset, endOffset)
	}
}

func TestParseDateRangeUTCvsLocal(t *testing.T) {
	// Test the difference between UTC and local date parsing

	dateStr := "2025-08-08"

	// Parse as UTC (old buggy behavior)
	utcDate, _ := time.Parse("2006-01-02", dateStr)

	// Parse as local time (fixed behavior)
	localDate, _ := time.ParseInLocation("2006-01-02", dateStr, time.Local)

	t.Logf("Date string: %s", dateStr)
	t.Logf("Parsed as UTC: %s", utcDate.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Parsed as Local: %s", localDate.Format("2006-01-02 15:04:05 MST"))

	// Calculate the difference
	difference := int(localDate.Sub(utcDate).Seconds())
	t.Logf("Time difference: %d seconds (%.2f hours)", difference, float64(difference)/3600.0)

	// The difference should be the negative of the timezone offset
	// (because local time is behind UTC in EDT)
	_, localOffset := localDate.Zone()
	expectedDifference := -localOffset

	t.Logf("Local timezone offset: %d seconds", localOffset)
	t.Logf("Expected difference: %d seconds", expectedDifference)

	if difference != expectedDifference {
		t.Errorf("Expected difference %d seconds, got %d seconds", expectedDifference, difference)
	}
}

func TestParseDateRangeInvalidInput(t *testing.T) {
	// Test error handling for invalid date range input

	filter := filters.NewFilter()

	testCases := []struct {
		input       string
		expectError bool
		errorMsg    string
	}{
		{"2025-08-08", true, "must be in format YYYY-MM-DD,YYYY-MM-DD"},
		{"2025-08-08,2025-08-10,2025-08-12", true, "must be in format YYYY-MM-DD,YYYY-MM-DD"},
		{"invalid,2025-08-10", true, "invalid start date"},
		{"2025-08-08,invalid", true, "invalid end date"},
		{"2025-08-08,2025-08-10", false, ""},
	}

	for _, tc := range testCases {
		err := ParseDateRange(filter, tc.input)

		if tc.expectError {
			if err == nil {
				t.Errorf("Expected error for input '%s', but got nil", tc.input)
			} else if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
				t.Errorf("Expected error containing '%s' for input '%s', got '%s'",
					tc.errorMsg, tc.input, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
			}
		}
	}
}
