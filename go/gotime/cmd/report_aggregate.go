package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// getWeekStart returns midnight on the Sunday that opens t's week.
func getWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	daysBack := weekday
	weekStart := t.AddDate(0, 0, -daysBack)
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
}

// sortedKeys returns a map's keys in ascending order.
func sortedKeys[V any](data map[string]V) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// aggregateByKeyword groups entries by keyword and returns one summary per
// keyword, sorted by keyword. An active entry contributes its elapsed time only
// when includeActive is set; otherwise only its recorded duration counts.
func aggregateByKeyword(entries []models.Entry, includeActive bool) []KeywordSummaryJSON {
	keywordMap := make(map[string][]models.Entry)
	for _, entry := range entries {
		keywordMap[entry.Keyword] = append(keywordMap[entry.Keyword], entry)
	}

	var summaries []KeywordSummaryJSON
	for _, keyword := range sortedKeys(keywordMap) {
		keywordEntries := keywordMap[keyword]
		keywordDuration := 0
		tagSet := make(map[string]bool)

		for _, entry := range keywordEntries {
			if includeActive {
				keywordDuration += entry.GetCurrentDuration()
			} else {
				keywordDuration += entry.Duration
			}

			for _, tag := range entry.Tags {
				tagSet[tag] = true
			}
		}

		// Collect unique tags
		var tags []string
		for tag := range tagSet {
			tags = append(tags, tag)
		}
		sort.Strings(tags)

		summaries = append(summaries, KeywordSummaryJSON{
			Keyword:  keyword,
			Duration: keywordDuration,
			Entries:  len(keywordEntries),
			Tags:     tags,
		})
	}

	return summaries
}

// weeklyKeywordData buckets entries into the current week by keyword and day of
// the week, returning the per-keyword daily durations (Sunday = 0, Saturday = 6)
// and the total for each day. Entries outside the current week are skipped.
func weeklyKeywordData(entries []models.Entry) (map[string][7]int, [7]int) {
	weekStart := getWeekStart(time.Now())
	keywordData := make(map[string][7]int)
	dailyTotals := [7]int{}

	for _, entry := range entries {
		daysDiff := int(entry.StartTime.Sub(weekStart).Hours() / 24)
		if daysDiff < 0 || daysDiff > 6 {
			continue
		}

		duration := entry.Duration
		if entry.Active {
			duration = entry.GetCurrentDuration()
		}

		dayData := keywordData[entry.Keyword]
		dayData[daysDiff] += duration
		keywordData[entry.Keyword] = dayData

		dailyTotals[daysDiff] += duration
	}

	return keywordData, dailyTotals
}

// formatTags renders a tag list for a table cell, with a dash standing in for an
// entry that carries no tag.
func formatTags(tags []string) string {
	if joined := strings.Join(tags, ", "); joined != "" {
		return joined
	}
	return "-"
}

// formatDurationCompact formats duration in compact HH:MM:SS format for weekly reports
func formatDurationCompact(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}
