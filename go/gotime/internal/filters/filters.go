package filters

import (
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// TimeRange represents different time filtering options
type TimeRange int

const (
	TimeRangeWeek TimeRange = iota
	TimeRangeToday
	TimeRangeYesterday
	TimeRangeMonth
	TimeRangeYear
	TimeRangeDays
	TimeRangeBetween
)

// Filter represents filtering criteria for entries
type Filter struct {
	TimeRange  TimeRange
	DaysBack   int
	StartDate  *time.Time
	EndDate    *time.Time
	Keyword    string
	Tags       []string
	InvertTags bool
	ActiveOnly bool
	NoActive   bool
}

// NewFilter creates a new filter with default values (current week)
func NewFilter() *Filter {
	return &Filter{
		TimeRange: TimeRangeWeek,
	}
}

// Apply applies the filter to a slice of entries
func (f *Filter) Apply(entries []models.Entry) []models.Entry {
	var filtered []models.Entry

	for _, entry := range entries {
		if f.matchesEntry(&entry) {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

func (f *Filter) matchesEntry(entry *models.Entry) bool {
	// Check active status filters
	if f.ActiveOnly && !entry.Active {
		return false
	}
	if f.NoActive && entry.Active {
		return false
	}

	// Check keyword filter
	if f.Keyword != "" && entry.Keyword != f.Keyword {
		return false
	}

	// Check tag filters
	if len(f.Tags) > 0 {
		hasMatchingTag := entry.HasAnyTag(f.Tags)
		if f.InvertTags && hasMatchingTag {
			return false
		}
		if !f.InvertTags && !hasMatchingTag {
			return false
		}
	}

	// Check time range
	if !f.matchesTimeRange(entry) {
		return false
	}

	return true
}

func (f *Filter) matchesTimeRange(entry *models.Entry) bool {
	now := time.Now()
	entryDate := entry.StartTime

	switch f.TimeRange {
	case TimeRangeToday:
		return isSameDay(entryDate, now)

	case TimeRangeYesterday:
		yesterday := now.AddDate(0, 0, -1)
		return isSameDay(entryDate, yesterday)

	case TimeRangeWeek:
		weekStart := getWeekStart(now)
		weekEnd := weekStart.AddDate(0, 0, 7)
		return entryDate.After(weekStart) || entryDate.Equal(weekStart) &&
			entryDate.Before(weekEnd)

	case TimeRangeMonth:
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)
		return entryDate.After(monthStart) || entryDate.Equal(monthStart) &&
			entryDate.Before(monthEnd)

	case TimeRangeYear:
		yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		yearEnd := yearStart.AddDate(1, 0, 0)
		return entryDate.After(yearStart) || entryDate.Equal(yearStart) &&
			entryDate.Before(yearEnd)

	case TimeRangeDays:
		daysBack := now.AddDate(0, 0, -f.DaysBack)
		return entryDate.After(daysBack) || entryDate.Equal(daysBack)

	case TimeRangeBetween:
		if f.StartDate != nil && entryDate.Before(*f.StartDate) {
			return false
		}
		if f.EndDate != nil && entryDate.After(*f.EndDate) {
			return false
		}
		return true
	}

	return true
}

// SetTags sets the tag filter from a comma-separated string
func (f *Filter) SetTags(tagsStr string) {
	if tagsStr == "" {
		f.Tags = nil
		return
	}

	tags := strings.Split(tagsStr, ",")
	f.Tags = make([]string, len(tags))
	for i, tag := range tags {
		f.Tags[i] = strings.TrimSpace(tag)
	}
}

// SetDateRange sets a custom date range
func (f *Filter) SetDateRange(start, end time.Time) {
	f.TimeRange = TimeRangeBetween
	f.StartDate = &start
	f.EndDate = &end
}

// Helper functions

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func getWeekStart(t time.Time) time.Time {
	// Find the most recent Sunday (start of week)
	weekday := int(t.Weekday())
	daysBack := weekday // Sunday = 0, so this gives us days since Sunday

	weekStart := t.AddDate(0, 0, -daysBack)
	// Set to start of day
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
}
