package cmd

import (
	"fmt"
	"time"

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

	completedEntries, activeEntries := splitByActivity(entries)

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

// splitByActivity separates the entries still running from the finished ones,
// preserving the order they were given in.
func splitByActivity(entries []models.Entry) (completed, active []models.Entry) {
	for _, entry := range entries {
		if entry.Active {
			active = append(active, entry)
		} else {
			completed = append(completed, entry)
		}
	}
	return completed, active
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
