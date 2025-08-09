package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/filters"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

var (
	listActiveOnly bool
	listNoActive   bool
	listKeyword    string
	listTags       string
	listInvertTags bool
	listDays       int
	listWeek       bool
	listMonth      bool
	listYear       bool
	listYesterday  bool
	listBetween    string
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List time tracking entries",
	Long: `List time tracking entries with various filtering options.
By default, shows today's entries with active entries highlighted.

Examples:
  gt list                                # List today's entries
  gt list --week                         # List current week's entries
  gt list --active                       # Show only active entries
  gt list --no-active                   # Show only stopped entries
  gt list --keyword coding              # Show entries for "coding" keyword
  gt list --tags golang,cli             # Show entries with "golang" or "cli" tags
  gt list --invert-tags meeting         # Show entries without "meeting" tag
  gt list --days 7                      # Show last 7 days
  gt list --between 2025-08-01,2025-08-07  # Custom date range`,
	RunE:    runList,
	Aliases: []string{"ls", "l"},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Activity filters
	listCmd.Flags().BoolVar(&listActiveOnly, "active", false, "show only active entries")
	listCmd.Flags().BoolVar(&listNoActive, "no-active", false, "show only stopped entries")

	// Content filters
	listCmd.Flags().StringVar(&listKeyword, "keyword", "", "filter by keyword")
	listCmd.Flags().StringVar(&listTags, "tags", "", "filter by tags (comma-separated)")
	listCmd.Flags().BoolVar(&listInvertTags, "invert-tags", false, "exclude entries with specified tags")

	// Time filters
	listCmd.Flags().IntVar(&listDays, "days", 0, "show last N days")
	listCmd.Flags().BoolVar(&listWeek, "week", false, "show current week")
	listCmd.Flags().BoolVar(&listMonth, "month", false, "show current month")
	listCmd.Flags().BoolVar(&listYear, "year", false, "show current year")
	listCmd.Flags().BoolVar(&listYesterday, "yesterday", false, "show yesterday's entries")
	listCmd.Flags().StringVar(&listBetween, "between", "", "show entries between dates (YYYY-MM-DD,YYYY-MM-DD)")
}

func runList(cmd *cobra.Command, args []string) error {
	// Validate flags
	if listActiveOnly && listNoActive {
		return fmt.Errorf("cannot use both --active and --no-active flags")
	}

	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create and configure filter
	filter := filters.NewFilter()

	// Set default to today if no time filter is specified
	timeRangeCount := 0
	if listDays > 0 {
		filter.TimeRange = filters.TimeRangeDays
		filter.DaysBack = listDays
		timeRangeCount++
	}
	if listWeek {
		filter.TimeRange = filters.TimeRangeWeek
		timeRangeCount++
	}
	if listMonth {
		filter.TimeRange = filters.TimeRangeMonth
		timeRangeCount++
	}
	if listYear {
		filter.TimeRange = filters.TimeRangeYear
		timeRangeCount++
	}
	if listYesterday {
		filter.TimeRange = filters.TimeRangeYesterday
		timeRangeCount++
	}
	if listBetween != "" {
		if err := ParseDateRange(filter, listBetween); err != nil {
			return err
		}
		timeRangeCount++
	}

	if timeRangeCount > 1 {
		return fmt.Errorf("cannot specify multiple time range filters")
	}

	// If no time filter specified, default to today
	if timeRangeCount == 0 {
		filter.TimeRange = filters.TimeRangeToday
	}

	// Set content filters
	filter.Keyword = listKeyword
	filter.SetTags(listTags)
	filter.InvertTags = listInvertTags
	filter.ActiveOnly = listActiveOnly
	filter.NoActive = listNoActive
	filter.IncludeStashed = true // List command should show stashed entries

	// Apply filters
	entries := filter.Apply(cfg.Entries)

	if len(entries) == 0 {
		fmt.Println("No entries match the specified criteria")
		return nil
	}

	// Create table
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "Started", "Status"})

	for _, entry := range entries {
		var status string
		var currentDuration int
		
		// Calculate current duration
		if entry.Active {
			currentDuration = entry.GetCurrentDuration()
		} else {
			currentDuration = entry.Duration
		}
		
		// Enhanced status display
		if entry.Stashed {
			stashedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")). // Orange
				Bold(true)
			status = stashedStyle.Render("⏸️ Stashed")
		} else if entry.Active {
			// Check for long-running warnings
			if currentDuration > 28800 { // > 8 hours
				warningStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("220")). // Yellow
					Bold(true)
				status = warningStyle.Render("⚠️ Long Run")
			} else if currentDuration > 14400 { // > 4 hours
				cautionStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("214")). // Orange
					Bold(true)
				status = cautionStyle.Render("🔶 Running")
			} else {
				runningStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("46")). // Green
					Bold(true)
				status = runningStyle.Render("🟢 Running")
			}
		} else {
			stoppedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")). // Gray
				Bold(false)
			status = stoppedStyle.Render("⏹️ Stopped")
		}

		// Enhanced duration and tag display using shared utilities
		durationStr := formatDurationWithWarning(currentDuration, entry.Active)
		tagsStr := formatTagsWithColors(entry.Tags)

		startedStr := entry.StartTime.Format("Jan 2 3:04 PM")

		row := table.Row{
			entry.ShortID,
			entry.Keyword,
			durationStr,
			tagsStr,
			startedStr,
			status,
		}

		t.AppendRow(row)
	}

	if IsVerbose() {
		// Output entries as pretty-printed JSON
		if err := outputEntriesAsJSON(entries); err != nil {
			return fmt.Errorf("failed to output JSON: %w", err)
		}

		// Add summary at the bottom
		activeCount := 0
		stoppedCount := 0
		for _, entry := range entries {
			if entry.Active {
				activeCount++
			} else {
				stoppedCount++
			}
		}
		fmt.Printf("\nSummary: %d total (%d active, %d stopped)\n",
			len(entries), activeCount, stoppedCount)
	} else {
		// Normal table output
		fmt.Println(t.Render())
	}

	return nil
}

// outputEntriesAsJSON outputs the entries as pretty-printed JSON using jq
func outputEntriesAsJSON(entries []models.Entry) error {
	// First, try to marshal the entries to JSON
	jsonData, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to marshal entries to JSON: %w", err)
	}

	// Try to use jq for pretty-printing
	jqCmd := exec.Command("jq", ".")
	jqCmd.Stdin = strings.NewReader(string(jsonData))
	jqCmd.Stdout = os.Stdout
	jqCmd.Stderr = os.Stderr

	if err := jqCmd.Run(); err != nil {
		// If jq fails, fall back to Go's JSON pretty-printing
		var prettyJSON interface{}
		if err := json.Unmarshal(jsonData, &prettyJSON); err != nil {
			return fmt.Errorf("failed to unmarshal JSON for pretty-printing: %w", err)
		}

		prettyData, err := json.MarshalIndent(prettyJSON, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to pretty-print JSON: %w", err)
		}

		fmt.Println(string(prettyData))
	}

	return nil
}

