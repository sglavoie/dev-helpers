package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/filters"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// ReportJSON represents the JSON structure for a report
type ReportJSON struct {
	Title             string                `json:"title"`
	TimeRange         string                `json:"time_range"`
	CompletedEntries  []KeywordSummaryJSON  `json:"completed_entries,omitempty"` // Deprecated: kept for backwards compatibility
	ActiveEntries     []models.Entry        `json:"active_entries,omitempty"`    // Deprecated: kept for backwards compatibility
	WeeklyData        *WeeklyReportJSON     `json:"weekly_data,omitempty"`       // Deprecated: use TimeSeries instead
	TimeSeries        *TimeSeriesReportJSON `json:"time_series,omitempty"`       // New: unified time-based data
	TotalDuration     int                   `json:"total_duration"`
	CompletedDuration int                   `json:"completed_duration"`
	ActiveDuration    int                   `json:"active_duration"`
	FiltersApplied    *FiltersMetadata      `json:"filters_applied,omitempty"`
}

// FiltersMetadata represents the filters that were applied to generate this report
type FiltersMetadata struct {
	Keywords        []string `json:"keywords,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	ExcludeKeywords bool     `json:"exclude_keywords,omitempty"`
	ExcludeTags     bool     `json:"exclude_tags,omitempty"`
}

// KeywordSummaryJSON represents keyword summary for JSON output
type KeywordSummaryJSON struct {
	Keyword  string   `json:"keyword"`
	Duration int      `json:"duration"`
	Entries  int      `json:"entries"`
	Tags     []string `json:"tags"`
}

// WeeklyReportJSON represents weekly report data
type WeeklyReportJSON struct {
	Keywords    []WeeklyKeywordJSON `json:"keywords"`
	DailyTotals [7]int              `json:"daily_totals"`
	GrandTotal  int                 `json:"grand_total"`
}

// WeeklyKeywordJSON represents a keyword's weekly data
type WeeklyKeywordJSON struct {
	Keyword      string `json:"keyword"`
	DailyData    [7]int `json:"daily_data"`
	KeywordTotal int    `json:"keyword_total"`
}

// TimeSeriesReportJSON represents time-based report data (flexible for different time ranges)
type TimeSeriesReportJSON struct {
	Keywords     []TimeSeriesKeywordJSON `json:"keywords"`
	PeriodTotals []int                   `json:"period_totals"` // Totals for each time period
	PeriodLabels []string                `json:"period_labels"` // Labels for each period (e.g., "Mon", "Oct 28", etc.)
	GrandTotal   int                     `json:"grand_total"`
}

// TimeSeriesKeywordJSON represents a keyword's time-series data
type TimeSeriesKeywordJSON struct {
	Keyword      string `json:"keyword"`
	PeriodData   []int  `json:"period_data"` // Duration for each time period
	KeywordTotal int    `json:"keyword_total"`
}

func generateJSONReport(entries []models.Entry, filter *filters.Filter) error {
	report := ReportJSON{
		Title:     getReportTitle(filter),
		TimeRange: getTimeRangeString(filter),
	}

	// Add filter metadata if any filters were applied
	if len(filter.Keywords) > 0 || len(filter.Tags) > 0 {
		report.FiltersApplied = &FiltersMetadata{
			Keywords:        filter.Keywords,
			Tags:            filter.Tags,
			ExcludeKeywords: filter.ExcludeKeywords,
			ExcludeTags:     filter.ExcludeTags,
		}
	}

	// Generate time series data for ALL report types (unified format)
	timeSeriesData := generateTimeSeriesReportJSON(entries, filter)
	report.TimeSeries = &timeSeriesData
	report.TotalDuration = timeSeriesData.GrandTotal

	// Keep weekly_data for backwards compatibility with existing code
	if filter.TimeRange == filters.TimeRangeWeek {
		weeklyData := generateWeeklyReportJSON(entries)
		report.WeeklyData = &weeklyData
	}

	completedEntries, activeEntries := splitByActivity(entries)

	// Generate completed entries summary
	if len(completedEntries) > 0 {
		report.CompletedEntries = aggregateByKeyword(completedEntries, false)
	}

	// Include active entries
	if len(activeEntries) > 0 {
		report.ActiveEntries = activeEntries
	}

	// Calculate totals
	completedDuration := 0
	for _, entry := range completedEntries {
		completedDuration += entry.Duration
	}
	report.CompletedDuration = completedDuration

	activeDuration := 0
	for _, entry := range activeEntries {
		activeDuration += entry.GetCurrentDuration()
	}
	report.ActiveDuration = activeDuration
	report.TotalDuration = completedDuration + activeDuration

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report to JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

// timeSeriesPeriods describes the buckets a report's time range is split into:
// where they start, how many there are, and how each one is labelled.
func timeSeriesPeriods(filter *filters.Filter) (periodStart time.Time, periodCount int, periodLabels []string) {
	now := time.Now()

	switch filter.TimeRange {
	case filters.TimeRangeWeek:
		// 7 days: Sun-Sat
		return getWeekStart(now), 7, []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	case filters.TimeRangeToday:
		// 1 period: today
		return startOfDay(now), 1, []string{now.Format("Jan 2")}

	case filters.TimeRangeYesterday:
		// 1 period: yesterday
		yesterday := now.AddDate(0, 0, -1)
		return startOfDay(yesterday), 1, []string{yesterday.Format("Jan 2")}

	case filters.TimeRangeMonth:
		// Days in current month
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		return firstOfMonth, lastOfMonth.Day(), dailyLabels(firstOfMonth, lastOfMonth.Day())

	case filters.TimeRangeYear:
		// 12 months
		yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		for i := 0; i < 12; i++ {
			periodLabels = append(periodLabels, yearStart.AddDate(0, i, 0).Format("Jan"))
		}
		return yearStart, 12, periodLabels

	case filters.TimeRangeDays:
		// Last N days
		periodCount = filter.DaysBack
		periodStart = startOfDay(now.AddDate(0, 0, -periodCount+1))
		return periodStart, periodCount, dailyLabels(periodStart, periodCount)

	case filters.TimeRangeBetween:
		// Custom date range - create daily periods
		if filter.StartDate != nil && filter.EndDate != nil {
			periodStart = startOfDay(*filter.StartDate)
			endDate := startOfDay(*filter.EndDate)
			periodCount = int(endDate.Sub(periodStart).Hours()/24) + 1
			return periodStart, periodCount, dailyLabels(periodStart, periodCount)
		}
	}

	// Fallback to a single period
	return now, 1, []string{now.Format("Jan 2")}
}

// startOfDay returns midnight of t's own day, in t's own location.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// dailyLabels labels count consecutive days starting at start.
func dailyLabels(start time.Time, count int) []string {
	labels := make([]string, 0, count)
	for i := 0; i < count; i++ {
		labels = append(labels, start.AddDate(0, 0, i).Format("Jan 2"))
	}
	return labels
}

// generateTimeSeriesReportJSON creates a time-series report for any time range
func generateTimeSeriesReportJSON(entries []models.Entry, filter *filters.Filter) TimeSeriesReportJSON {
	if len(entries) == 0 {
		return TimeSeriesReportJSON{
			Keywords:     []TimeSeriesKeywordJSON{},
			PeriodTotals: []int{},
			PeriodLabels: []string{},
			GrandTotal:   0,
		}
	}

	periodStart, periodCount, periodLabels := timeSeriesPeriods(filter)

	// Initialize data structures
	keywordData := make(map[string][]int)
	periodTotals := make([]int, periodCount)

	// Process entries and assign to appropriate periods
	for _, entry := range entries {
		duration := entry.Duration
		if entry.Active {
			duration = entry.GetCurrentDuration()
		}

		// Calculate which period this entry belongs to
		var periodIndex int

		switch filter.TimeRange {
		case filters.TimeRangeYear:
			// Month-based periods
			monthsDiff := int(entry.StartTime.Month()) - int(periodStart.Month())
			yearsDiff := entry.StartTime.Year() - periodStart.Year()
			periodIndex = monthsDiff + (yearsDiff * 12)

		default:
			// Day-based periods
			daysDiff := int(entry.StartTime.Sub(periodStart).Hours() / 24)
			periodIndex = daysDiff
		}

		// Skip if outside period range
		if periodIndex < 0 || periodIndex >= periodCount {
			continue
		}

		// Initialize keyword data if needed
		if _, exists := keywordData[entry.Keyword]; !exists {
			keywordData[entry.Keyword] = make([]int, periodCount)
		}

		// Add duration to appropriate period
		keywordData[entry.Keyword][periodIndex] += duration
		periodTotals[periodIndex] += duration
	}

	grandTotal := 0
	var timeSeriesKeywords []TimeSeriesKeywordJSON

	for _, keyword := range sortedKeys(keywordData) {
		periodData := keywordData[keyword]
		keywordTotal := 0
		for _, duration := range periodData {
			keywordTotal += duration
		}
		grandTotal += keywordTotal

		timeSeriesKeywords = append(timeSeriesKeywords, TimeSeriesKeywordJSON{
			Keyword:      keyword,
			PeriodData:   periodData,
			KeywordTotal: keywordTotal,
		})
	}

	return TimeSeriesReportJSON{
		Keywords:     timeSeriesKeywords,
		PeriodTotals: periodTotals,
		PeriodLabels: periodLabels,
		GrandTotal:   grandTotal,
	}
}

func generateWeeklyReportJSON(entries []models.Entry) WeeklyReportJSON {
	keywordData, dailyTotals := weeklyKeywordData(entries)

	grandTotal := 0
	var weeklyKeywords []WeeklyKeywordJSON

	for _, keyword := range sortedKeys(keywordData) {
		dayData := keywordData[keyword]
		keywordTotal := 0
		for day := 0; day < 7; day++ {
			keywordTotal += dayData[day]
		}
		grandTotal += keywordTotal

		weeklyKeywords = append(weeklyKeywords, WeeklyKeywordJSON{
			Keyword:      keyword,
			DailyData:    dayData,
			KeywordTotal: keywordTotal,
		})
	}

	return WeeklyReportJSON{
		Keywords:    weeklyKeywords,
		DailyTotals: dailyTotals,
		GrandTotal:  grandTotal,
	}
}
