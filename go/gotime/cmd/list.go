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
	listActiveOnly      bool
	listNoActive        bool
	listKeywords        string
	listExcludeKeywords string
	listTags            string
	listExcludeTags     string
	listDays            int
	listWeek            bool
	listMonth           bool
	listYear            bool
	listYesterday       bool
	listBetween         string
	listMinDuration     string
	listMaxDuration     string
	listFromDate        string
	listToDate          string
	listShowGaps        bool
	listJSON            bool
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
  gt list --keywords coding,meeting     # Show entries for "coding" OR "meeting" keywords
  gt list --exclude-keywords meeting    # Show entries EXCEPT "meeting" keyword
  gt list --tags golang,cli             # Show entries with "golang" OR "cli" tags
  gt list --exclude-tags meeting,work   # Show entries WITHOUT "meeting" OR "work" tags
  gt list --days 7                      # Show last 7 days
  gt list --between 2025-08-01,2025-08-07  # Custom date range
  gt list --from 2025-08-01 --to 2025-08-07 # Date range with separate flags
  gt list --min-duration 1h             # Show entries >= 1 hour
  gt list --max-duration 4h             # Show entries <= 4 hours
  gt list --min-duration 30m --max-duration 2h # Duration range`,
	RunE:    runList,
	Aliases: []string{"ls", "l"},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Activity filters
	listCmd.Flags().BoolVar(&listActiveOnly, "active", false, "show only active entries")
	listCmd.Flags().BoolVar(&listNoActive, "no-active", false, "show only stopped entries")

	// Content filters
	listCmd.Flags().StringVar(&listKeywords, "keywords", "", "filter by keywords (comma-separated)")
	listCmd.Flags().StringVar(&listExcludeKeywords, "exclude-keywords", "", "exclude entries with specified keywords (comma-separated)")
	listCmd.Flags().StringVar(&listTags, "tags", "", "filter by tags (comma-separated)")
	listCmd.Flags().StringVar(&listExcludeTags, "exclude-tags", "", "exclude entries with specified tags (comma-separated)")

	// Time filters
	listCmd.Flags().IntVar(&listDays, "days", 0, "show last N days")
	listCmd.Flags().BoolVar(&listWeek, "week", false, "show current week")
	listCmd.Flags().BoolVar(&listMonth, "month", false, "show current month")
	listCmd.Flags().BoolVar(&listYear, "year", false, "show current year")
	listCmd.Flags().BoolVar(&listYesterday, "yesterday", false, "show yesterday's entries")
	listCmd.Flags().StringVar(&listBetween, "between", "", "show entries between dates (YYYY-MM-DD,YYYY-MM-DD)")
	listCmd.Flags().StringVar(&listFromDate, "from", "", "show entries from date (YYYY-MM-DD)")
	listCmd.Flags().StringVar(&listToDate, "to", "", "show entries to date (YYYY-MM-DD)")

	// Duration filters
	listCmd.Flags().StringVar(&listMinDuration, "min-duration", "", "minimum duration filter (e.g., '1h', '30m', '3600')")
	listCmd.Flags().StringVar(&listMaxDuration, "max-duration", "", "maximum duration filter (e.g., '4h', '2h30m', '14400')")

	// Display options
	listCmd.Flags().BoolVar(&listShowGaps, "show-gaps", false, "show ENDED and GAP columns")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output entries as JSON")

	listCmd.MarkFlagsMutuallyExclusive("active", "no-active")
	listCmd.MarkFlagsMutuallyExclusive("exclude-keywords", "keywords")
	listCmd.MarkFlagsMutuallyExclusive("exclude-tags", "tags")
}

func runList(cmd *cobra.Command, args []string) error {
	// Validate flags (mutual exclusivity is handled by Cobra)
	if listActiveOnly && listNoActive {
		return fmt.Errorf("cannot use both --active and --no-active flags")
	}

	// Validate that --from/--to are not used with --between
	if listBetween != "" && (listFromDate != "" || listToDate != "") {
		return fmt.Errorf("cannot use --between with --from/--to flags")
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
	if listFromDate != "" || listToDate != "" {
		if err := ParseFromToDateRange(filter, listFromDate, listToDate); err != nil {
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
	if listKeywords != "" {
		filter.SetKeywords(listKeywords)
		filter.ExcludeKeywords = false
	} else if listExcludeKeywords != "" {
		filter.SetKeywords(listExcludeKeywords)
		filter.ExcludeKeywords = true
	}

	if listTags != "" {
		filter.SetTags(listTags)
		filter.ExcludeTags = false
	} else if listExcludeTags != "" {
		filter.SetTags(listExcludeTags)
		filter.ExcludeTags = true
	}

	filter.ActiveOnly = listActiveOnly
	filter.NoActive = listNoActive
	filter.IncludeStashed = true // List command should show stashed entries

	// Set duration filters
	if listMinDuration != "" {
		minDur, err := filters.ParseDuration(listMinDuration)
		if err != nil {
			return fmt.Errorf("invalid min-duration: %w", err)
		}
		filter.MinDuration = minDur
	}
	if listMaxDuration != "" {
		maxDur, err := filters.ParseDuration(listMaxDuration)
		if err != nil {
			return fmt.Errorf("invalid max-duration: %w", err)
		}
		filter.MaxDuration = maxDur
	}

	// Apply filters
	entries := filter.Apply(cfg.Entries)

	if len(entries) == 0 {
		fmt.Println("No entries match the specified criteria")
		return nil
	}

	// Sort by start time (oldest first) for chronological display
	SortEntries(entries, ByStartTime, Ascending)

	// Determine if we should show gaps (flag OR config setting)
	showGaps := listShowGaps || cfg.ListShowGaps

	// Create table
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)

	// Build header based on showGaps setting
	if showGaps {
		t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "Started", "Ended", "Gap", "Status"})
	} else {
		t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "Started", "Status"})
	}

	for i, entry := range entries {
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

		// Build row based on showGaps setting
		var row table.Row
		if showGaps {
			// Calculate ENDED column
			var endedStr string
			if entry.EndTime != nil {
				endedStr = entry.EndTime.Format("Jan 2 3:04 PM")
			} else {
				endedStr = "-"
			}

			// Calculate GAP column
			var gapStr string
			if i == 0 {
				// First entry has no previous entry
				gapStr = "-"
			} else {
				prevEntry := entries[i-1]
				if prevEntry.EndTime == nil {
					// Previous entry is active/stashed, no gap can be calculated
					gapStr = "-"
				} else {
					// Calculate gap between previous entry's end and current entry's start
					gap := entry.StartTime.Sub(*prevEntry.EndTime)
					if gap < 0 {
						// Overlapping entries
						gapStr = "overlap"
					} else {
						// Format gap duration (not relative to now, but the actual gap duration)
						gapStr = formatGapDuration(gap)
					}
				}
			}

			row = table.Row{
				entry.ShortID,
				entry.Keyword,
				durationStr,
				tagsStr,
				startedStr,
				endedStr,
				gapStr,
				status,
			}
		} else {
			row = table.Row{
				entry.ShortID,
				entry.Keyword,
				durationStr,
				tagsStr,
				startedStr,
				status,
			}
		}

		t.AppendRow(row)
	}

	if listJSON {
		// Output as clean JSON (no extra text)
		jsonData, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal entries to JSON: %w", err)
		}
		fmt.Println(string(jsonData))
	} else if IsVerbose() {
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
