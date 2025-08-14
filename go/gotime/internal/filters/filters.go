package filters

import (
	"fmt"
	"strconv"
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
	TimeRange       TimeRange
	DaysBack        int
	StartDate       *time.Time
	EndDate         *time.Time
	Keywords        []string
	ExcludeKeywords bool
	Tags            []string
	ExcludeTags     bool
	ActiveOnly      bool
	NoActive        bool
	IncludeStashed  bool // Whether to include stashed entries in results
	MinDuration     int  // Minimum duration in seconds
	MaxDuration     int  // Maximum duration in seconds (0 means no limit)
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
	// Handle stashed entries based on IncludeStashed flag
	if entry.Stashed && !f.IncludeStashed {
		return false
	}

	// Check active status filters
	if f.ActiveOnly && !entry.Active {
		return false
	}
	if f.NoActive && entry.Active {
		return false
	}

	// Check keyword filters
	if len(f.Keywords) > 0 {
		hasMatchingKeyword := f.entryHasAnyKeyword(entry, f.Keywords)
		if f.ExcludeKeywords && hasMatchingKeyword {
			return false
		}
		if !f.ExcludeKeywords && !hasMatchingKeyword {
			return false
		}
	}

	// Check tag filters
	if len(f.Tags) > 0 {
		hasMatchingTag := entry.HasAnyTag(f.Tags)
		if f.ExcludeTags && hasMatchingTag {
			return false
		}
		if !f.ExcludeTags && !hasMatchingTag {
			return false
		}
	}

	// Check time range
	if !f.matchesTimeRange(entry) {
		return false
	}

	// Check duration filters
	if !f.matchesDuration(entry) {
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

func (f *Filter) matchesDuration(entry *models.Entry) bool {
	// Get the entry's duration (either current duration if active, or stored duration)
	var entryDuration int
	if entry.Active {
		entryDuration = entry.GetCurrentDuration()
	} else {
		entryDuration = entry.Duration
	}

	// Check minimum duration
	if f.MinDuration > 0 && entryDuration < f.MinDuration {
		return false
	}

	// Check maximum duration (0 means no limit)
	if f.MaxDuration > 0 && entryDuration > f.MaxDuration {
		return false
	}

	return true
}

// SetKeywords sets the keyword filter from a comma-separated string
func (f *Filter) SetKeywords(keywordsStr string) {
	if keywordsStr == "" {
		f.Keywords = nil
		return
	}

	keywords := strings.Split(keywordsStr, ",")
	f.Keywords = make([]string, len(keywords))
	for i, keyword := range keywords {
		f.Keywords[i] = strings.TrimSpace(keyword)
	}
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

// entryHasAnyKeyword checks if the entry has any of the specified keywords
func (f *Filter) entryHasAnyKeyword(entry *models.Entry, keywords []string) bool {
	for _, keyword := range keywords {
		if entry.Keyword == keyword {
			return true
		}
	}
	return false
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

// ParseDuration parses a duration string like "1h", "30m", "2h30m", "3600" (seconds)
// Returns duration in seconds
func ParseDuration(durationStr string) (int, error) {
	if durationStr == "" {
		return 0, nil
	}

	// Try parsing as Go duration first (supports "1h30m", "45m", etc.)
	if duration, err := time.ParseDuration(durationStr); err == nil {
		return int(duration.Seconds()), nil
	}

	// Try parsing as plain seconds
	if seconds, err := strconv.Atoi(durationStr); err == nil {
		return seconds, nil
	}

	return 0, fmt.Errorf("invalid duration format: %s (use formats like '1h', '30m', '1h30m', or seconds)", durationStr)
}
