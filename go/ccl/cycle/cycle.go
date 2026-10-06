package cycle

import (
	"fmt"
	"strings"
	"time"
)

// Clock holds an hour and minute within a day.
type Clock struct {
	Hour, Minute int
}

// CycleInfo holds verbose information about the current usage cycle.
type CycleInfo struct {
	CycleStart    time.Time
	CycleEnd      time.Time
	Expected      float64   // 0.0–100.0
	WorkDaysTotal int
	WorkDaysDone  int       // fully completed work day segments before "now"
	IsWorkDay     bool      // whether "now" falls in a work day segment
	NextBoundary  time.Time // start of the next segment
}

var weekdayAbbr = map[string]time.Weekday{
	"Sun": time.Sunday, "Mon": time.Monday, "Tue": time.Tuesday,
	"Wed": time.Wednesday, "Thu": time.Thursday, "Fri": time.Friday,
	"Sat": time.Saturday,
}

// ParseResetTime parses a reset time string like "Thu 2:59 PM" into its components.
func ParseResetTime(s string) (time.Weekday, Clock, error) {
	t, err := time.Parse("Mon 3:04 PM", s)
	if err != nil {
		return 0, Clock{}, fmt.Errorf("invalid reset time %q: expected format like \"Thu 2:59 PM\"", s)
	}
	abbr := strings.SplitN(s, " ", 2)[0]
	wd, ok := weekdayAbbr[abbr]
	if !ok {
		return 0, Clock{}, fmt.Errorf("unrecognized weekday abbreviation %q", abbr)
	}
	return wd, Clock{Hour: t.Hour(), Minute: t.Minute()}, nil
}

// FindCycleStart returns the most recent occurrence of (resetWeekday, resetClock) that is <= now.
func FindCycleStart(now time.Time, resetWeekday time.Weekday, resetClock Clock) time.Time {
	// Start from the reset time today.
	candidate := time.Date(now.Year(), now.Month(), now.Day(),
		resetClock.Hour, resetClock.Minute, 0, 0, now.Location())
	// If candidate is in the future, step back one day.
	if candidate.After(now) {
		candidate = candidate.AddDate(0, 0, -1)
	}
	// Walk backward until we reach the target weekday.
	for candidate.Weekday() != resetWeekday {
		candidate = candidate.AddDate(0, 0, -1)
	}
	return candidate
}

// segmentTag returns the weekday that labels the segment starting at segStart.
// Segments are end-based: the segment [segStart, segStart+1day) is tagged by
// the weekday of (segStart + 1 day). This means "Thursday's timeslot counts
// toward Friday's quota."
func segmentTag(segStart time.Time) time.Weekday {
	return segStart.AddDate(0, 0, 1).Weekday()
}

func contains(days []time.Weekday, wd time.Weekday) bool {
	for _, d := range days {
		if d == wd {
			return true
		}
	}
	return false
}

// ExpectedUsage returns the expected usage percentage (0.0–100.0) for now.
func ExpectedUsage(now time.Time, workDays []time.Weekday, resetWeekday time.Weekday, resetClock Clock) float64 {
	return GetCycleInfo(now, workDays, resetWeekday, resetClock).Expected
}

// GetCycleInfo returns the full cycle status for the status command.
func GetCycleInfo(now time.Time, workDays []time.Weekday, resetWeekday time.Weekday, resetClock Clock) CycleInfo {
	cycleStart := FindCycleStart(now, resetWeekday, resetClock)
	cycleEnd := cycleStart.AddDate(0, 0, 7)

	perDay := 100.0 / float64(len(workDays))

	workDaysDone := 0   // completed work-day segments before the current segment
	isWorkDay := false
	var nextBoundary time.Time
	expected := 0.0

	for i := 0; i < 7; i++ {
		segStart := cycleStart.AddDate(0, 0, i)
		segEnd := cycleStart.AddDate(0, 0, i+1)

		if now.Before(segStart) || !now.Before(segEnd) {
			// Not the current segment — count it if it's a work day and it's before now.
			if segEnd.Before(now) || segEnd.Equal(now) {
				if contains(workDays, segmentTag(segStart)) {
					workDaysDone++
				}
			}
			continue
		}

		// now is in [segStart, segEnd)
		tag := segmentTag(segStart)
		isWorkDay = contains(workDays, tag)
		nextBoundary = segEnd

		if isWorkDay {
			elapsed := now.Sub(segStart)
			total := segEnd.Sub(segStart)
			fraction := elapsed.Seconds() / total.Seconds()
			expected = (float64(workDaysDone) + fraction) * perDay
		} else {
			expected = float64(workDaysDone) * perDay
		}
		break
	}

	return CycleInfo{
		CycleStart:    cycleStart,
		CycleEnd:      cycleEnd,
		Expected:      expected,
		WorkDaysTotal: len(workDays),
		WorkDaysDone:  workDaysDone,
		IsWorkDay:     isWorkDay,
		NextBoundary:  nextBoundary,
	}
}
