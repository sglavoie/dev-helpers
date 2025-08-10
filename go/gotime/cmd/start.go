package cmd

import (
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/logic"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

var (
	startBackdate string
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [keyword] [tag1] [tag2]...",
	Short: "Start tracking time for a keyword with optional tags",
	Long: `Start a new time tracking session for the specified keyword.
When no arguments are provided, displays an interactive interface to set keyword and tags.
You can add multiple tags to categorize the activity further.

The --backdate flag allows you to start the timer with a time offset, useful when you
forgot to start tracking but know when you actually began working.

Examples:
  gt start                           # Interactive input for keyword and tags
  gt start coding                    # Start tracking "coding"
  gt start coding golang cli         # Start "coding" with tags "golang" and "cli"
  gt start meeting team planning     # Start "meeting" with tags "team" and "planning"
  gt start coding --backdate 5m      # Start "coding", started 5 minutes ago
  gt start meeting --backdate 1h30m  # Start "meeting", started 1h30m ago
  gt start coding --backdate 10      # Start "coding", started 10 minutes ago

Backdate formats: 5, 5m, 30s, 1h, 1h30, 1h30m, 2h30m30s (no unit defaults to minutes)`,
	Args: cobra.ArbitraryArgs,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	
	startCmd.Flags().StringVar(&startBackdate, "backdate", "", "start the timer with a time offset (e.g., 5m, 1h30m, 10)")
}

func runStart(cmd *cobra.Command, args []string) error {
	var keyword string
	var tags []string

	// Interactive input if no arguments provided
	if len(args) == 0 {
		var err error
		keyword, tags, err = tui.RunStartInput()
		if err != nil {
			return err
		}
	} else {
		keyword = args[0]
		tags = args[1:]
	}

	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there's already an active entry for this keyword
	if cfg.HasActiveEntryForKeyword(keyword) {
		return fmt.Errorf("An active stopwatch for keyword '%s' is already running. Stop it first, then use 'gt continue %s' to resume.", keyword, keyword)
	}

	if logic.IsReservedKeyword(keyword) {
		return fmt.Errorf("keyword cannot be a number")
	}

	// Parse backdate offset if provided
	var startTime time.Time
	if startBackdate != "" {
		offset, err := ParseDuration(startBackdate)
		if err != nil {
			return fmt.Errorf("invalid backdate format: %w", err)
		}
		startTime = time.Now().Add(-offset)
	} else {
		startTime = time.Now()
	}

	// Create new entry
	shortID := getNextShortID(cfg)
	entry := models.NewEntryWithStartTime(keyword, tags, shortID, startTime)

	// Add to configuration
	cfg.AddEntry(entry)

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Format output
	timeStr := entry.StartTime.Format("3:04:05 PM")
	if len(tags) > 0 {
		fmt.Printf("Started: %s %v at %s\n", keyword, tags, timeStr)
	} else {
		fmt.Printf("Started: %s at %s\n", keyword, timeStr)
	}

	if IsVerbose() {
		fmt.Printf("Entry ID: %s (Short ID: %d)\n", entry.ID, entry.ShortID)
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// getNextShortID determines the next available short ID
func getNextShortID(cfg *models.Config) int {
	// Find the lowest available short ID
	used := make(map[int]bool)
	for _, entry := range cfg.Entries {
		if entry.ShortID >= 1 && entry.ShortID <= 1_000 {
			used[entry.ShortID] = true
		}
	}

	for i := 1; i <= 1_000; i++ {
		if !used[i] {
			return i
		}
	}

	// If all short IDs are used, return 1 (will be reassigned when updateShortIDs is called)
	return 1
}
