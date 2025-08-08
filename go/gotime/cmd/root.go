package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	rootCmd.AddCommand(helpCmd)

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

	// Display header
	fmt.Println("GoTime Status Summary")
	fmt.Println("=====================")
	fmt.Println()

	// Show active timers
	activeEntries := cfg.GetActiveEntries()
	if len(activeEntries) > 0 {
		fmt.Println("🟢 ACTIVE TIMERS")
		displayActiveTimers(activeEntries)
		fmt.Println()
	} else {
		fmt.Println("⏸️  No active timers running")
		fmt.Println()
	}

	// Show recent entries (last 5)
	recentEntries := getRecentEntries(cfg.Entries, 5)
	if len(recentEntries) > 0 {
		fmt.Println("📝 RECENT ENTRIES")
		displayRecentEntries(recentEntries)
		fmt.Println()
	}

	// Show today's summary
	displayTodaySummary(cfg.Entries)

	return nil
}

// displayActiveTimers shows active timers in a table
func displayActiveTimers(entries []models.Entry) {
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

// displayRecentEntries shows recent entries
func displayRecentEntries(entries []models.Entry) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Keyword", "Duration", "Tags", "When"})

	for _, entry := range entries {
		tagsStr := strings.Join(entry.Tags, ", ")
		if tagsStr == "" {
			tagsStr = "-"
		}

		var whenStr string
		if entry.Active {
			whenStr = "Running"
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

		t.AppendRow(table.Row{
			entry.ShortID,
			entry.Keyword,
			formatDuration(duration),
			tagsStr,
			whenStr,
		})
	}

	fmt.Println(t.Render())
}

// displayTodaySummary shows today's time tracking summary
func displayTodaySummary(entries []models.Entry) {
	today := time.Now()
	todayDuration := 0
	activeDuration := 0
	todayEntries := 0

	for _, entry := range entries {
		if isSameDay(entry.StartTime, today) {
			todayEntries++
			if entry.Active {
				activeDuration += entry.GetCurrentDuration()
			} else {
				todayDuration += entry.Duration
			}
		}
	}

	totalToday := todayDuration + activeDuration

	fmt.Println("📊 TODAY'S SUMMARY")
	fmt.Printf("   Entries: %d\n", todayEntries)
	fmt.Printf("   Completed: %s\n", formatDuration(todayDuration))
	if activeDuration > 0 {
		fmt.Printf("   Active: %s\n", formatDuration(activeDuration))
		fmt.Printf("   Total: %s\n", formatDuration(totalToday))
	} else {
		fmt.Printf("   Total: %s\n", formatDuration(todayDuration))
	}
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

// isSameDay checks if two times are on the same day
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
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
