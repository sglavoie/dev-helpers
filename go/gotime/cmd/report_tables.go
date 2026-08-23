package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/filters"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func printReportHeader(filter *filters.Filter) {
	now := time.Now()
	var title string

	switch filter.TimeRange {
	case filters.TimeRangeToday:
		title = fmt.Sprintf("TODAY'S REPORT (%s)", now.Format("Jan 2, 2006"))
	case filters.TimeRangeYesterday:
		yesterday := now.AddDate(0, 0, -1)
		title = fmt.Sprintf("YESTERDAY'S REPORT (%s)", yesterday.Format("Jan 2, 2006"))
	case filters.TimeRangeWeek:
		weekStart := getWeekStart(now)
		weekEnd := weekStart.AddDate(0, 0, 6)
		title = fmt.Sprintf("WEEKLY REPORT (%s - %s)",
			weekStart.Format("Jan 2"), weekEnd.Format("Jan 2, 2006"))
	case filters.TimeRangeMonth:
		title = fmt.Sprintf("MONTHLY REPORT (%s)", now.Format("January 2006"))
	case filters.TimeRangeYear:
		title = fmt.Sprintf("YEARLY REPORT (%d)", now.Year())
	case filters.TimeRangeDays:
		title = fmt.Sprintf("LAST %d DAYS REPORT", filter.DaysBack)
	case filters.TimeRangeBetween:
		if filter.StartDate != nil && filter.EndDate != nil {
			title = fmt.Sprintf("CUSTOM REPORT (%s - %s)",
				filter.StartDate.Format("Jan 2"), filter.EndDate.Format("Jan 2, 2006"))
		}
	default:
		title = "TIME TRACKING REPORT"
	}

	fmt.Println(title)
	fmt.Println(strings.Repeat("=", len(title)))
	fmt.Println()
}

func GenerateKeywordSummary(entries []models.Entry, includeActive bool) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Keyword", "Duration", "Entries", "Tags"})

	totalDuration := 0
	totalEntries := 0

	for _, summary := range aggregateByKeyword(entries, includeActive) {
		t.AppendRow(table.Row{
			summary.Keyword,
			formatDuration(summary.Duration),
			fmt.Sprintf("%d", summary.Entries),
			formatTags(summary.Tags),
		})

		totalDuration += summary.Duration
		totalEntries += summary.Entries
	}

	// Add total row
	t.AppendSeparator()
	t.AppendRow(table.Row{
		"TOTAL",
		formatDuration(totalDuration),
		fmt.Sprintf("%d", totalEntries),
		"-",
	})

	fmt.Println(t.Render())
}

func GenerateActiveEntriesTable(entries []models.Entry) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "Started"})

	for _, entry := range entries {
		t.AppendRow(table.Row{
			entry.ShortID,
			entry.Keyword,
			formatDuration(entry.GetCurrentDuration()),
			formatTags(entry.Tags),
			entry.StartTime.Format("3:04 PM"),
		})
	}

	fmt.Println(t.Render())
}

func PrintGrandTotal(completedEntries, activeEntries []models.Entry) {
	completedDuration := 0
	for _, entry := range completedEntries {
		completedDuration += entry.Duration
	}

	activeDuration := 0
	for _, entry := range activeEntries {
		activeDuration += entry.GetCurrentDuration()
	}

	totalDuration := completedDuration + activeDuration

	if len(activeEntries) > 0 {
		fmt.Printf("GRAND TOTAL: %s (%s completed + %s active)\n",
			formatDuration(totalDuration),
			formatDuration(completedDuration),
			formatDuration(activeDuration))
	} else {
		fmt.Printf("TOTAL: %s\n", formatDuration(completedDuration))
	}
}

// generateWeeklyReport renders one row per keyword with a column per day of the
// current week, closed by a row of daily totals.
func generateWeeklyReport(entries []models.Entry) error {
	keywordData, dailyTotals := weeklyKeywordData(entries)

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"KEYWORD", "SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT", "TOTAL"})

	keywords := sortedKeys(keywordData)
	grandTotal := 0

	for _, keyword := range keywords {
		dayData := keywordData[keyword]
		row := []interface{}{keyword}
		keywordTotal := 0

		for day := 0; day < 7; day++ {
			keywordTotal += dayData[day]
			row = append(row, compactOrDash(dayData[day]))
		}

		row = append(row, formatDurationCompact(keywordTotal))
		t.AppendRow(table.Row(row))
		grandTotal += keywordTotal
	}

	if len(keywords) > 0 {
		t.AppendSeparator()
		totalRow := []interface{}{"TOTAL"}
		for day := 0; day < 7; day++ {
			totalRow = append(totalRow, compactOrDash(dailyTotals[day]))
		}
		totalRow = append(totalRow, formatDurationCompact(grandTotal))
		t.AppendRow(table.Row(totalRow))
	}

	fmt.Println(t.Render())

	return nil
}

// compactOrDash renders a duration for a weekly cell, with a dash standing in
// for a day nothing was tracked on.
func compactOrDash(seconds int) string {
	if seconds > 0 {
		return formatDurationCompact(seconds)
	}
	return "-"
}
