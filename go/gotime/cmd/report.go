package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/filters"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

var (
	reportDays            int
	reportMonth           bool
	reportYear            bool
	reportToday           bool
	reportYesterday       bool
	reportBetween         string
	reportKeywords        string
	reportExcludeKeywords string
	reportTags            string
	reportExcludeTags     string
	reportJSON            bool
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate time tracking reports",
	Long: `Generate detailed time tracking reports with various filtering options.
By default, shows a weekly report (Sunday to Saturday).

Examples:
  gt report                              # Weekly report
  gt report --today                      # Today's report
  gt report --days 30                    # Last 30 days
  gt report --month                      # Current month
  gt report --keywords coding,meeting    # Only "coding" OR "meeting" entries
  gt report --exclude-keywords meeting   # Exclude "meeting" entries
  gt report --tags golang,cli            # Entries with "golang" OR "cli" tags
  gt report --exclude-tags meeting,work  # Exclude entries with "meeting" OR "work" tags
  gt report --between 2025-08-01,2025-08-07  # Custom date range`,
	RunE:    runReport,
	Aliases: []string{"rep", "r"},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().IntVar(&reportDays, "days", 0, "report for last N days")
	reportCmd.Flags().BoolVar(&reportMonth, "month", false, "report for current month")
	reportCmd.Flags().BoolVar(&reportYear, "year", false, "report for current year")
	reportCmd.Flags().BoolVar(&reportToday, "today", false, "report for today")
	reportCmd.Flags().BoolVar(&reportYesterday, "yesterday", false, "report for yesterday")
	reportCmd.Flags().StringVar(&reportBetween, "between", "", "report between dates (YYYY-MM-DD,YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportKeywords, "keywords", "", "filter by keywords (comma-separated)")
	reportCmd.Flags().StringVar(&reportExcludeKeywords, "exclude-keywords", "", "exclude entries with specified keywords (comma-separated)")
	reportCmd.Flags().StringVar(&reportTags, "tags", "", "filter by tags (comma-separated)")
	reportCmd.Flags().StringVar(&reportExcludeTags, "exclude-tags", "", "exclude entries with specified tags (comma-separated)")
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "output report as JSON")

	reportCmd.MarkFlagsMutuallyExclusive("exclude-keywords", "keywords")
	reportCmd.MarkFlagsMutuallyExclusive("exclude-tags", "tags")
}

func runReport(cmd *cobra.Command, args []string) error {
	// Validate flags (mutual exclusivity is handled by Cobra)

	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create filter
	filter := filters.NewFilter()

	// Set time range
	timeRangeCount := 0
	if reportDays > 0 {
		filter.TimeRange = filters.TimeRangeDays
		filter.DaysBack = reportDays
		timeRangeCount++
	}
	if reportMonth {
		filter.TimeRange = filters.TimeRangeMonth
		timeRangeCount++
	}
	if reportYear {
		filter.TimeRange = filters.TimeRangeYear
		timeRangeCount++
	}
	if reportToday {
		filter.TimeRange = filters.TimeRangeToday
		timeRangeCount++
	}
	if reportYesterday {
		filter.TimeRange = filters.TimeRangeYesterday
		timeRangeCount++
	}
	if reportBetween != "" {
		if err := ParseDateRange(filter, reportBetween); err != nil {
			return err
		}
		timeRangeCount++
	}

	if timeRangeCount > 1 {
		return fmt.Errorf("cannot specify multiple time range filters")
	}

	// Set content filters
	if reportKeywords != "" {
		filter.SetKeywords(reportKeywords)
		filter.ExcludeKeywords = false
	} else if reportExcludeKeywords != "" {
		filter.SetKeywords(reportExcludeKeywords)
		filter.ExcludeKeywords = true
	}

	if reportTags != "" {
		filter.SetTags(reportTags)
		filter.ExcludeTags = false
	} else if reportExcludeTags != "" {
		filter.SetTags(reportExcludeTags)
		filter.ExcludeTags = true
	}

	// Apply filters
	entries := filter.Apply(cfg.Entries)

	// Generate report
	return GenerateReport(entries, filter)
}

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

func GenerateReport(entries []models.Entry, filter *filters.Filter) error {
	if len(entries) == 0 {
		if reportJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No entries found for the specified criteria")
		}
		return nil
	}

	if reportJSON {
		return generateJSONReport(entries, filter)
	}

	// Print report header
	printReportHeader(filter)

	// Special handling for weekly reports
	if filter.TimeRange == filters.TimeRangeWeek {
		return generateWeeklyReport(entries)
	}

	// Separate active and completed entries
	var completedEntries []models.Entry
	var activeEntries []models.Entry

	for _, entry := range entries {
		if entry.Active {
			activeEntries = append(activeEntries, entry)
		} else {
			completedEntries = append(completedEntries, entry)
		}
	}

	// Generate completed entries summary
	if len(completedEntries) > 0 {
		fmt.Println("COMPLETED ENTRIES")
		GenerateKeywordSummary(completedEntries, false)
		fmt.Println()
	}

	// Show active entries
	if len(activeEntries) > 0 {
		fmt.Println("ACTIVE ENTRIES")
		GenerateActiveEntriesTable(activeEntries)
		fmt.Println()
	}

	// Grand total
	PrintGrandTotal(completedEntries, activeEntries)

	return nil
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

	// Separate active and completed entries
	var completedEntries []models.Entry
	var activeEntries []models.Entry

	for _, entry := range entries {
		if entry.Active {
			activeEntries = append(activeEntries, entry)
		} else {
			completedEntries = append(completedEntries, entry)
		}
	}

	// Generate completed entries summary
	if len(completedEntries) > 0 {
		report.CompletedEntries = generateKeywordSummaryJSON(completedEntries)
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

func getReportTitle(filter *filters.Filter) string {
	now := time.Now()
	switch filter.TimeRange {
	case filters.TimeRangeToday:
		return fmt.Sprintf("Today's Report (%s)", now.Format("Jan 2, 2006"))
	case filters.TimeRangeYesterday:
		yesterday := now.AddDate(0, 0, -1)
		return fmt.Sprintf("Yesterday's Report (%s)", yesterday.Format("Jan 2, 2006"))
	case filters.TimeRangeWeek:
		weekStart := getWeekStart(now)
		weekEnd := weekStart.AddDate(0, 0, 6)
		return fmt.Sprintf("Weekly Report (%s - %s)",
			weekStart.Format("Jan 2"), weekEnd.Format("Jan 2, 2006"))
	case filters.TimeRangeMonth:
		return fmt.Sprintf("Monthly Report (%s)", now.Format("January 2006"))
	case filters.TimeRangeYear:
		return fmt.Sprintf("Yearly Report (%d)", now.Year())
	case filters.TimeRangeDays:
		return fmt.Sprintf("Last %d Days Report", filter.DaysBack)
	case filters.TimeRangeBetween:
		if filter.StartDate != nil && filter.EndDate != nil {
			return fmt.Sprintf("Custom Report (%s - %s)",
				filter.StartDate.Format("Jan 2"), filter.EndDate.Format("Jan 2, 2006"))
		}
	}
	return "Time Tracking Report"
}

func getTimeRangeString(filter *filters.Filter) string {
	switch filter.TimeRange {
	case filters.TimeRangeToday:
		return "today"
	case filters.TimeRangeYesterday:
		return "yesterday"
	case filters.TimeRangeWeek:
		return "week"
	case filters.TimeRangeMonth:
		return "month"
	case filters.TimeRangeYear:
		return "year"
	case filters.TimeRangeDays:
		return fmt.Sprintf("last_%d_days", filter.DaysBack)
	case filters.TimeRangeBetween:
		return "custom"
	}
	return "unknown"
}

func generateKeywordSummaryJSON(entries []models.Entry) []KeywordSummaryJSON {
	// Group by keyword
	keywordMap := make(map[string][]models.Entry)
	for _, entry := range entries {
		keywordMap[entry.Keyword] = append(keywordMap[entry.Keyword], entry)
	}

	// Sort keywords
	var keywords []string
	for keyword := range keywordMap {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)

	var summaries []KeywordSummaryJSON
	for _, keyword := range keywords {
		keywordEntries := keywordMap[keyword]
		keywordDuration := 0
		tagSet := make(map[string]bool)

		for _, entry := range keywordEntries {
			keywordDuration += entry.Duration
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

	// Determine time periods based on filter type
	var periodStart time.Time
	var periodCount int
	var periodLabels []string

	now := time.Now()

	switch filter.TimeRange {
	case filters.TimeRangeWeek:
		// 7 days: Sun-Sat
		periodStart = getWeekStart(now)
		periodCount = 7
		periodLabels = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	case filters.TimeRangeToday:
		// 1 period: today
		periodStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		periodCount = 1
		periodLabels = []string{now.Format("Jan 2")}

	case filters.TimeRangeYesterday:
		// 1 period: yesterday
		yesterday := now.AddDate(0, 0, -1)
		periodStart = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		periodCount = 1
		periodLabels = []string{yesterday.Format("Jan 2")}

	case filters.TimeRangeMonth:
		// Days in current month
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		periodStart = firstOfMonth
		periodCount = lastOfMonth.Day()
		for i := 1; i <= periodCount; i++ {
			day := firstOfMonth.AddDate(0, 0, i-1)
			periodLabels = append(periodLabels, day.Format("Jan 2"))
		}

	case filters.TimeRangeYear:
		// 12 months
		yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		periodStart = yearStart
		periodCount = 12
		for i := 0; i < 12; i++ {
			month := yearStart.AddDate(0, i, 0)
			periodLabels = append(periodLabels, month.Format("Jan"))
		}

	case filters.TimeRangeDays:
		// Last N days
		periodCount = filter.DaysBack
		periodStart = now.AddDate(0, 0, -periodCount+1)
		periodStart = time.Date(periodStart.Year(), periodStart.Month(), periodStart.Day(), 0, 0, 0, 0, periodStart.Location())
		for i := 0; i < periodCount; i++ {
			day := periodStart.AddDate(0, 0, i)
			periodLabels = append(periodLabels, day.Format("Jan 2"))
		}

	case filters.TimeRangeBetween:
		// Custom date range - create daily periods
		if filter.StartDate != nil && filter.EndDate != nil {
			periodStart = time.Date(filter.StartDate.Year(), filter.StartDate.Month(), filter.StartDate.Day(), 0, 0, 0, 0, filter.StartDate.Location())
			endDate := time.Date(filter.EndDate.Year(), filter.EndDate.Month(), filter.EndDate.Day(), 0, 0, 0, 0, filter.EndDate.Location())
			periodCount = int(endDate.Sub(periodStart).Hours()/24) + 1
			for i := 0; i < periodCount; i++ {
				day := periodStart.AddDate(0, 0, i)
				periodLabels = append(periodLabels, day.Format("Jan 2"))
			}
		} else {
			// Fallback to single period
			periodStart = now
			periodCount = 1
			periodLabels = []string{now.Format("Jan 2")}
		}

	default:
		// Default to single period
		periodStart = now
		periodCount = 1
		periodLabels = []string{now.Format("Jan 2")}
	}

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

	// Build result
	keywords := make([]string, 0, len(keywordData))
	for keyword := range keywordData {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)

	grandTotal := 0
	var timeSeriesKeywords []TimeSeriesKeywordJSON

	for _, keyword := range keywords {
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
	now := time.Now()
	weekStart := getWeekStart(now)

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

		if _, exists := keywordData[entry.Keyword]; !exists {
			keywordData[entry.Keyword] = [7]int{}
		}

		dayData := keywordData[entry.Keyword]
		dayData[daysDiff] += duration
		keywordData[entry.Keyword] = dayData
		dailyTotals[daysDiff] += duration
	}

	// Sort keywords
	keywords := make([]string, 0, len(keywordData))
	for keyword := range keywordData {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)

	grandTotal := 0
	var weeklyKeywords []WeeklyKeywordJSON

	for _, keyword := range keywords {
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
	// Group by keyword
	keywordMap := make(map[string][]models.Entry)
	for _, entry := range entries {
		keywordMap[entry.Keyword] = append(keywordMap[entry.Keyword], entry)
	}

	// Create summary table
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Keyword", "Duration", "Entries", "Tags"})

	// Sort keywords
	var keywords []string
	for keyword := range keywordMap {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)

	totalDuration := 0
	totalEntries := 0

	for _, keyword := range keywords {
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

		tagsStr := strings.Join(tags, ", ")
		if tagsStr == "" {
			tagsStr = "-"
		}

		t.AppendRow(table.Row{
			keyword,
			formatDuration(keywordDuration),
			fmt.Sprintf("%d", len(keywordEntries)),
			tagsStr,
		})

		totalDuration += keywordDuration
		totalEntries += len(keywordEntries)
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
		tagsStr := strings.Join(entry.Tags, ", ")
		if tagsStr == "" {
			tagsStr = "-"
		}

		t.AppendRow(table.Row{
			entry.ShortID,
			entry.Keyword,
			formatDuration(entry.GetCurrentDuration()),
			tagsStr,
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

func getWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	daysBack := weekday
	weekStart := t.AddDate(0, 0, -daysBack)
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
}

func generateWeeklyReport(entries []models.Entry) error {
	// Get the week start (Sunday)
	now := time.Now()
	weekStart := getWeekStart(now)

	// Create a map to store duration by keyword and day
	// keywordData[keyword][dayOfWeek] = duration
	keywordData := make(map[string][7]int) // Sunday = 0, Saturday = 6
	dailyTotals := [7]int{}

	// Process all entries
	for _, entry := range entries {
		// Determine which day of the week this entry belongs to
		daysDiff := int(entry.StartTime.Sub(weekStart).Hours() / 24)
		if daysDiff < 0 || daysDiff > 6 {
			continue // Skip entries outside the current week
		}

		// Get the duration for this entry
		duration := entry.Duration
		if entry.Active {
			duration = entry.GetCurrentDuration()
		}

		// Initialize keyword data if it doesn't exist
		if _, exists := keywordData[entry.Keyword]; !exists {
			keywordData[entry.Keyword] = [7]int{}
		}

		// Add duration to the appropriate day and keyword
		dayData := keywordData[entry.Keyword]
		dayData[daysDiff] += duration
		keywordData[entry.Keyword] = dayData

		// Add to daily total
		dailyTotals[daysDiff] += duration
	}

	// Create the table
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)

	// Header row: Keyword | Sun | Mon | Tue | Wed | Thu | Fri | Sat | Total
	header := []interface{}{"KEYWORD", "SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT", "TOTAL"}
	t.AppendHeader(table.Row(header))

	// Sort keywords alphabetically
	keywords := make([]string, 0, len(keywordData))
	for keyword := range keywordData {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)

	grandTotal := 0

	// Add rows for each keyword
	for _, keyword := range keywords {
		dayData := keywordData[keyword]
		row := []interface{}{keyword}
		keywordTotal := 0

		// Add duration for each day
		for day := 0; day < 7; day++ {
			duration := dayData[day]
			keywordTotal += duration
			if duration > 0 {
				row = append(row, formatDurationCompact(duration))
			} else {
				row = append(row, "-")
			}
		}

		// Add keyword total
		row = append(row, formatDurationCompact(keywordTotal))
		t.AppendRow(table.Row(row))
		grandTotal += keywordTotal
	}

	// Add separator and total row
	if len(keywords) > 0 {
		t.AppendSeparator()
		totalRow := []interface{}{"TOTAL"}

		// Add daily totals
		for day := 0; day < 7; day++ {
			if dailyTotals[day] > 0 {
				totalRow = append(totalRow, formatDurationCompact(dailyTotals[day]))
			} else {
				totalRow = append(totalRow, "-")
			}
		}

		// Add grand total
		totalRow = append(totalRow, formatDurationCompact(grandTotal))
		t.AppendRow(table.Row(totalRow))
	}

	fmt.Println(t.Render())

	return nil
}

// formatDurationCompact formats duration in compact HH:MM:SS format for weekly reports
func formatDurationCompact(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}
