package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/filters"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configPath string
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gt",
	Short: "A fast and simple time tracking CLI tool",
	Long: `GoTime (gt) is a personal time tracking CLI tool designed to replace 
commercial solutions like Clockify. It supports multiple concurrent stopwatches,
keyword-based organization, tag filtering, and powerful reporting capabilities.

Examples:
  gt start coding golang cli       # Start tracking "coding" with tags
  gt stop coding                   # Stop the latest "coding" entry
  gt list --active                 # Show all running timers
  gt report --today --keyword coding  # Today's coding report`,
	RunE: runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// Add 'h' as an alias for help
	helpCmd := &cobra.Command{
		Use:   "h",
		Short: "Help about any command",
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.Help()
		},
	}
	helpCmd.Hidden = true
	rootCmd.AddCommand(helpCmd)

	// Add '.' as a hidden alias for 'stop --all'
	dotCmd := &cobra.Command{
		Use:   ".",
		Short: "Stop all active entries (alias for 'stop --all')",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set the stopAll flag to true and call runStop
			stopAll = true
			return runStop(cmd, args)
		},
	}
	dotCmd.Hidden = true
	rootCmd.AddCommand(dotCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (default: ~/.gotime.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if configPath != "" {
		viper.Set("config", configPath)
	}
}

// GetConfigPath returns the configuration file path
func GetConfigPath() string {
	return viper.GetString("config")
}

// IsVerbose returns whether verbose output is enabled
func IsVerbose() bool {
	return viper.GetBool("verbose")
}

// runRoot displays a summary status when gt is called without arguments
func runRoot(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show recent entries (last 5)
	recentEntries := getRecentEntries(cfg.Entries, 5)
	if len(recentEntries) > 0 {
		fmt.Println("📝 RECENT ENTRIES")
		displayRecentEntries(recentEntries)
		fmt.Println()
	}

	// Show today's report
	if err := displayTodaysReport(cfg.Entries); err != nil {
		return fmt.Errorf("failed to generate today's report: %w", err)
	}

	// Show active timers
	activeEntries := cfg.GetActiveEntries()
	if len(activeEntries) > 0 {
		fmt.Println()
		fmt.Println("🟢 ACTIVE TIMERS")
		displayActiveTimers(activeEntries)
	} else {
		fmt.Println()
		fmt.Println("⏸️ No active timers running")
	}

	return nil
}

// displayActiveTimers shows active timers in a table with enhanced visual feedback
func displayActiveTimers(entries []models.Entry) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "Started"})

	for _, entry := range entries {
		currentDuration := entry.GetCurrentDuration()

		// Enhanced duration display with warnings
		durationStr := formatDurationWithWarning(currentDuration, true)

		// Enhanced keyword display
		keywordStr := formatKeywordWithStyle(entry.Keyword, true)

		// Enhanced tag display with colors
		tagsStr := formatTagsWithColors(entry.Tags)

		t.AppendRow(table.Row{
			entry.ShortID,
			keywordStr,
			durationStr,
			tagsStr,
			entry.StartTime.Format("3:04 PM"),
		})
	}

	fmt.Println(t.Render())
}

// displayRecentEntries shows recent entries with enhanced visual feedback
func displayRecentEntries(entries []models.Entry) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "When"})

	for _, entry := range entries {
		// Enhanced tag display with colors
		tagsStr := formatTagsWithColors(entry.Tags)

		// Enhanced status display
		var whenStr string
		if entry.Active {
			activeStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")). // Green
				Bold(true)
			whenStr = activeStyle.Render("▶ Running")
		} else if entry.Stashed {
			stashedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")). // Orange
				Bold(true)
			whenStr = stashedStyle.Render("⏸️ Stashed")
		} else {
			// Show relative time for completed entries
			if entry.EndTime != nil {
				whenStr = getRelativeTime(*entry.EndTime)
			} else {
				whenStr = getRelativeTime(entry.StartTime)
			}
		}

		duration := entry.Duration
		if entry.Active {
			duration = entry.GetCurrentDuration()
		}

		// Enhanced duration and keyword display
		durationStr := formatDurationWithWarning(duration, entry.Active)
		keywordStr := formatKeywordWithStyle(entry.Keyword, entry.Active)

		t.AppendRow(table.Row{
			entry.ShortID,
			keywordStr,
			durationStr,
			tagsStr,
			whenStr,
		})
	}

	fmt.Println(t.Render())
}

// displayTodaysReport shows today's detailed time tracking report
func displayTodaysReport(entries []models.Entry) error {
	// Create filter for today
	filter := filters.NewFilter()
	filter.TimeRange = filters.TimeRangeToday

	// Apply filter to get today's entries
	todayEntries := filter.Apply(entries)

	if len(todayEntries) == 0 {
		fmt.Println("📊 TODAY'S REPORT")
		fmt.Println("No entries for today")
		return nil
	}

	// Separate active and completed entries
	var completedEntries []models.Entry
	var activeEntries []models.Entry

	for _, entry := range todayEntries {
		if entry.Active {
			activeEntries = append(activeEntries, entry)
		} else {
			completedEntries = append(completedEntries, entry)
		}
	}

	// Generate completed entries summary
	if len(completedEntries) > 0 {
		fmt.Println("✅ COMPLETED ENTRIES")
		GenerateKeywordSummary(completedEntries, false)
	}

	return nil
}

// Helper functions

// getRecentEntries returns the most recent entries, sorted by start time
func getRecentEntries(entries []models.Entry, limit int) []models.Entry {
	if len(entries) == 0 {
		return nil
	}

	// Create a copy and sort by start time (most recent first)
	recentEntries := make([]models.Entry, len(entries))
	copy(recentEntries, entries)

	// Simple bubble sort by start time (descending)
	for i := 0; i < len(recentEntries)-1; i++ {
		for j := 0; j < len(recentEntries)-i-1; j++ {
			if recentEntries[j].StartTime.Before(recentEntries[j+1].StartTime) {
				recentEntries[j], recentEntries[j+1] = recentEntries[j+1], recentEntries[j]
			}
		}
	}

	// Return up to limit entries
	if len(recentEntries) > limit {
		return recentEntries[:limit]
	}
	return recentEntries
}

// formatDuration formats seconds into a readable duration string
func formatDuration(seconds int) string {
	duration := time.Duration(seconds) * time.Second
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	return fmt.Sprintf("%dh %02dm %02ds", hours, minutes, secs)
}

// getRelativeTime returns a human-readable relative time string
func getRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

// ParseDateRange parses a date range string and sets it on a filter
func ParseDateRange(filter *filters.Filter, rangeStr string) error {
	parts := strings.Split(rangeStr, ",")
	if len(parts) != 2 {
		return fmt.Errorf("date range must be in format YYYY-MM-DD,YYYY-MM-DD")
	}

	startDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(parts[0]), time.Local)
	if err != nil {
		return fmt.Errorf("invalid start date: %w", err)
	}

	endDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(parts[1]), time.Local)
	if err != nil {
		return fmt.Errorf("invalid end date: %w", err)
	}

	// Set end date to end of day
	endDate = endDate.Add(24*time.Hour - time.Second)

	filter.SetDateRange(startDate, endDate)
	return nil
}

// ParseFromToDateRange parses separate from and to date strings and sets them on a filter
func ParseFromToDateRange(filter *filters.Filter, fromStr, toStr string) error {
	var startDate, endDate time.Time
	var err error

	// Parse from date
	if fromStr != "" {
		startDate, err = time.ParseInLocation("2006-01-02", strings.TrimSpace(fromStr), time.Local)
		if err != nil {
			return fmt.Errorf("invalid from date: %w", err)
		}
	} else {
		// If no from date, use a very old date (effectively no start limit)
		startDate = time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
	}

	// Parse to date
	if toStr != "" {
		endDate, err = time.ParseInLocation("2006-01-02", strings.TrimSpace(toStr), time.Local)
		if err != nil {
			return fmt.Errorf("invalid to date: %w", err)
		}
		// Set end date to end of day
		endDate = endDate.Add(24*time.Hour - time.Second)
	} else {
		// If no to date, use current time + some margin (effectively no end limit)
		endDate = time.Now().Add(24 * time.Hour)
	}

	// Validate that from is before to
	if fromStr != "" && toStr != "" && startDate.After(endDate) {
		return fmt.Errorf("from date must be before to date")
	}

	filter.SetDateRange(startDate, endDate)
	return nil
}
