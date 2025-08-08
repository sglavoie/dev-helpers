package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestCrossDayScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		mode      InputMode
		scenario  string
		startTime string
		endTime   string
		duration  string
	}{
		{
			name:      "11 AM to 1 PM same day",
			mode:      ModeStartEndTime,
			scenario:  "Same day 2-hour session",
			startTime: "2025-08-08 11:00:00",
			endTime:   "2025-08-08 13:00:00",
		},
		{
			name:      "9 PM to 3 AM next day",
			mode:      ModeStartEndTime,
			scenario:  "Cross-midnight 6-hour session",
			startTime: "2025-08-07 21:00:00", // 9 PM
			endTime:   "2025-08-08 03:00:00", // 3 AM next day
		},
		{
			name:      "Duration mode crossing midnight",
			mode:      ModeDurationStartTime,
			scenario:  "8-hour session starting at 10 PM",
			startTime: "2025-08-07 22:00:00", // 10 PM
			duration:  "08:00:00",            // 8 hours -> ends at 6 AM next day
		},
		{
			name:     "End time mode crossing midnight",
			mode:     ModeDurationEndTime,
			scenario: "4-hour session ending at 2 AM",
			endTime:  "2025-08-08 02:00:00", // 2 AM
			duration: "04:00:00",            // 4 hours -> starts at 10 PM previous day
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &models.Entry{
				ID:        uuid.NewString(),
				ShortID:   1,
				Keyword:   "",
				Tags:      []string{},
				StartTime: time.Now(),
				Active:    true,
			}

			model := NewFieldEditorModelWithMode(entry, tc.mode)

			// Set field values based on the mode
			for i, field := range model.fields {
				switch field.name {
				case "keyword":
					model.fields[i].input.SetValue("cross_day_test")
				case "tags":
					model.fields[i].input.SetValue("overtime")
				case "start_time":
					if tc.startTime != "" {
						model.fields[i].input.SetValue(tc.startTime)
					}
				case "end_time":
					if tc.endTime != "" {
						model.fields[i].input.SetValue(tc.endTime)
					}
				case "duration":
					if tc.duration != "" {
						model.fields[i].input.SetValue(tc.duration)
					}
				}
			}

			err := model.validateAndApplyChanges()
			if err != nil {
				t.Errorf("%s - Validation failed: %v", tc.scenario, err)
				return
			}

			// Verify the entry makes sense
			if model.entry.StartTime.After(*model.entry.EndTime) {
				t.Errorf("%s - Start time %v is after end time %v",
					tc.scenario, model.entry.StartTime, model.entry.EndTime)
			}

			// Verify duration calculation
			expectedDuration := int(model.entry.EndTime.Sub(model.entry.StartTime).Seconds())
			if model.entry.Duration != expectedDuration {
				t.Errorf("%s - Duration mismatch: expected %d seconds, got %d seconds",
					tc.scenario, expectedDuration, model.entry.Duration)
			}

			t.Logf("%s - Success:", tc.scenario)
			t.Logf("  Start: %s", model.entry.StartTime.Format("2006-01-02 15:04:05 MST"))
			t.Logf("  End:   %s", model.entry.EndTime.Format("2006-01-02 15:04:05 MST"))
			t.Logf("  Duration: %.2f hours (%d seconds)", float64(model.entry.Duration)/3600, model.entry.Duration)

			// Additional validations for specific cases
			switch tc.name {
			case "11 AM to 1 PM same day":
				if model.entry.Duration != 7200 { // 2 hours
					t.Errorf("Expected 2 hours (7200s), got %d seconds", model.entry.Duration)
				}
			case "9 PM to 3 AM next day":
				if model.entry.Duration != 21600 { // 6 hours
					t.Errorf("Expected 6 hours (21600s), got %d seconds", model.entry.Duration)
				}
				// Verify it crosses midnight
				if model.entry.StartTime.Day() == model.entry.EndTime.Day() {
					t.Error("Expected session to cross midnight (different days)")
				}
			case "Duration mode crossing midnight":
				if model.entry.Duration != 28800 { // 8 hours
					t.Errorf("Expected 8 hours (28800s), got %d seconds", model.entry.Duration)
				}
				// Should end at 6 AM next day
				expectedEndHour := 6
				if model.entry.EndTime.Hour() != expectedEndHour {
					t.Errorf("Expected end time hour %d, got %d", expectedEndHour, model.entry.EndTime.Hour())
				}
			case "End time mode crossing midnight":
				if model.entry.Duration != 14400 { // 4 hours
					t.Errorf("Expected 4 hours (14400s), got %d seconds", model.entry.Duration)
				}
				// Should start at 10 PM previous day
				expectedStartHour := 22
				if model.entry.StartTime.Hour() != expectedStartHour {
					t.Errorf("Expected start time hour %d, got %d", expectedStartHour, model.entry.StartTime.Hour())
				}
			}
		})
	}
}

func TestMultiDayScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		mode      InputMode
		scenario  string
		startTime string
		endTime   string
		duration  string
		expected  struct {
			days  int
			hours int
		}
	}{
		{
			name:      "3-day marathon session",
			mode:      ModeStartEndTime,
			scenario:  "Friday 2 PM to Monday 5 PM",
			startTime: "2025-08-07 14:00:00", // Thursday 2 PM
			endTime:   "2025-08-10 17:00:00", // Sunday 5 PM
			expected: struct {
				days  int
				hours int
			}{3, 75}, // 3 days, 3 hours = 75 hours total
		},
		{
			name:      "Weekend duration session",
			mode:      ModeDurationStartTime,
			scenario:  "48-hour session starting Friday evening",
			startTime: "2025-08-08 18:00:00", // Friday 6 PM
			duration:  "48:00:00",            // 48 hours
			expected: struct {
				days  int
				hours int
			}{2, 48}, // Exactly 2 days
		},
		{
			name:     "Long project ending Sunday",
			mode:     ModeDurationEndTime,
			scenario: "36-hour project ending Sunday noon",
			endTime:  "2025-08-10 12:00:00", // Sunday noon
			duration: "36:00:00",            // 36 hours
			expected: struct {
				days  int
				hours int
			}{1, 36}, // 1.5 days
		},
		{
			name:      "Week-long intensive",
			mode:      ModeStartEndTime,
			scenario:  "Monday 9 AM to Friday 6 PM",
			startTime: "2025-08-04 09:00:00", // Monday 9 AM
			endTime:   "2025-08-08 18:00:00", // Friday 6 PM
			expected: struct {
				days  int
				hours int
			}{4, 105}, // 4 days, 9 hours = 105 hours
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &models.Entry{
				ID:        uuid.NewString(),
				ShortID:   1,
				Keyword:   "",
				Tags:      []string{},
				StartTime: time.Now(),
				Active:    true,
			}

			model := NewFieldEditorModelWithMode(entry, tc.mode)

			// Set field values
			for i, field := range model.fields {
				switch field.name {
				case "keyword":
					model.fields[i].input.SetValue("multiday_project")
				case "tags":
					model.fields[i].input.SetValue("intensive, marathon")
				case "start_time":
					if tc.startTime != "" {
						model.fields[i].input.SetValue(tc.startTime)
					}
				case "end_time":
					if tc.endTime != "" {
						model.fields[i].input.SetValue(tc.endTime)
					}
				case "duration":
					if tc.duration != "" {
						model.fields[i].input.SetValue(tc.duration)
					}
				}
			}

			err := model.validateAndApplyChanges()
			if err != nil {
				t.Errorf("%s - Validation failed: %v", tc.scenario, err)
				return
			}

			// Calculate actual days and hours
			totalSeconds := model.entry.Duration
			totalHours := totalSeconds / 3600
			totalDays := int(model.entry.EndTime.Sub(model.entry.StartTime).Hours() / 24)

			t.Logf("%s - Success:", tc.scenario)
			t.Logf("  Start: %s", model.entry.StartTime.Format("Monday, Jan 2, 2006 3:04 PM"))
			t.Logf("  End:   %s", model.entry.EndTime.Format("Monday, Jan 2, 2006 3:04 PM"))
			t.Logf("  Total duration: %d hours (%.1f days)", totalHours, float64(totalHours)/24)
			t.Logf("  Calendar days spanned: %d", totalDays+1)

			// Verify the expected duration
			expectedTotalSeconds := tc.expected.hours * 3600
			if model.entry.Duration != expectedTotalSeconds {
				t.Errorf("%s - Duration mismatch: expected %d hours (%d seconds), got %d hours (%d seconds)",
					tc.scenario, tc.expected.hours, expectedTotalSeconds, totalHours, totalSeconds)
			}

			// Verify it spans the expected number of days
			actualDaySpan := int(model.entry.EndTime.Sub(model.entry.StartTime).Hours()/24) + 1
			expectedDaySpan := tc.expected.days + 1 // Add 1 because we count both start and end days
			if actualDaySpan != expectedDaySpan {
				t.Errorf("%s - Day span mismatch: expected %d days, got %d days",
					tc.scenario, expectedDaySpan, actualDaySpan)
			}

			// Sanity checks
			if model.entry.StartTime.After(*model.entry.EndTime) {
				t.Errorf("%s - Start time is after end time", tc.scenario)
			}

			if model.entry.Duration <= 0 {
				t.Errorf("%s - Duration should be positive, got %d", tc.scenario, model.entry.Duration)
			}

			if model.entry.Active {
				t.Errorf("%s - Multi-day entries should be completed", tc.scenario)
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("Very short duration", func(t *testing.T) {
		entry := &models.Entry{
			ID:        uuid.NewString(),
			ShortID:   1,
			Keyword:   "",
			Tags:      []string{},
			StartTime: time.Now(),
			Active:    true,
		}

		model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

		// Set 1 minute duration
		for i, field := range model.fields {
			switch field.name {
			case "keyword":
				model.fields[i].input.SetValue("quick_task")
			case "duration":
				model.fields[i].input.SetValue("00:01:00")
			case "start_time":
				model.fields[i].input.SetValue("2025-08-08 10:00:00")
			}
		}

		err := model.validateAndApplyChanges()
		if err != nil {
			t.Errorf("Very short duration validation failed: %v", err)
		}

		if model.entry.Duration != 60 {
			t.Errorf("Expected 60 seconds, got %d", model.entry.Duration)
		}
	})

	t.Run("Timezone consistency", func(t *testing.T) {
		entry := &models.Entry{
			ID:        uuid.NewString(),
			ShortID:   1,
			Keyword:   "",
			Tags:      []string{},
			StartTime: time.Now(),
			Active:    true,
		}

		model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

		// Set times that should be parsed as local time
		for i, field := range model.fields {
			switch field.name {
			case "keyword":
				model.fields[i].input.SetValue("timezone_test")
			case "start_time":
				model.fields[i].input.SetValue("2025-08-08 10:00:00")
			case "end_time":
				model.fields[i].input.SetValue("2025-08-08 14:00:00")
			}
		}

		err := model.validateAndApplyChanges()
		if err != nil {
			t.Errorf("Timezone test validation failed: %v", err)
		}

		// Verify both times have the same timezone
		_, startOffset := model.entry.StartTime.Zone()
		_, endOffset := model.entry.EndTime.Zone()

		if startOffset != endOffset {
			t.Errorf("Timezone offset mismatch: start=%d, end=%d", startOffset, endOffset)
		}

		// Duration should be exactly 4 hours
		if model.entry.Duration != 14400 {
			t.Errorf("Expected 14400 seconds (4 hours), got %d", model.entry.Duration)
		}
	})

	t.Run("Leap year February 29", func(t *testing.T) {
		entry := &models.Entry{
			ID:        uuid.NewString(),
			ShortID:   1,
			Keyword:   "",
			Tags:      []string{},
			StartTime: time.Now(),
			Active:    true,
		}

		model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

		// Test on leap year February 29 (2024 was a leap year)
		for i, field := range model.fields {
			switch field.name {
			case "keyword":
				model.fields[i].input.SetValue("leap_year_test")
			case "start_time":
				model.fields[i].input.SetValue("2024-02-29 10:00:00")
			case "end_time":
				model.fields[i].input.SetValue("2024-02-29 15:00:00")
			}
		}

		err := model.validateAndApplyChanges()
		if err != nil {
			t.Errorf("Leap year test validation failed: %v", err)
		}

		// Should be 5 hours
		if model.entry.Duration != 18000 {
			t.Errorf("Expected 18000 seconds (5 hours), got %d", model.entry.Duration)
		}

		// Verify the date is correct
		if model.entry.StartTime.Day() != 29 || model.entry.StartTime.Month() != time.February {
			t.Errorf("Expected February 29, got %s", model.entry.StartTime.Format("January 2"))
		}
	})
}
