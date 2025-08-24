package cmd

import (
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

func GenerateReport(entries []models.Entry, filter *filters.Filter) error {
	if len(entries) == 0 {
		fmt.Println("No entries found for the specified criteria")
		return nil
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
