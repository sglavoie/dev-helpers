package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

var (
	stopAll bool
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop [keyword | ID | --all]",
	Short: "Stop tracking time",
	Long: `Stop time tracking for the specified entry.
When no arguments are provided, displays an interactive table of active entries to select from.
You can stop by keyword (stops the most recent active entry for that keyword),
by ID number (1-1000), or stop all active entries.

Examples:
  gt stop                            # Interactive selection from active entries
  gt stop coding                     # Stop the latest active "coding" entry
  gt stop 3                          # Stop entry with short ID 3
  gt stop --all                      # Stop all active entries`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Either provide a keyword/ID argument or use --all flag, or no args for interactive
		hasArgument := len(args) > 0
		hasAll := stopAll

		if hasArgument && hasAll {
			return fmt.Errorf("cannot specify both an argument and --all flag")
		}

		return nil
	},
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "stop all active entries")
}

func runStop(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	stoppedCount := 0

	// Interactive selection if no arguments and no --all flag
	if len(args) == 0 && !stopAll {
		return runInteractiveStop(cfg, configManager)
	}

	if stopAll {
		// Stop all active entries
		for i := range cfg.Entries {
			if cfg.Entries[i].Active {
				cfg.Entries[i].Stop()
				stoppedCount++

				duration := formatDuration(cfg.Entries[i].Duration)
				fmt.Printf("Stopped: %s %v - %s\n",
					cfg.Entries[i].Keyword,
					cfg.Entries[i].Tags,
					duration)
			}
		}

		if stoppedCount == 0 {
			fmt.Println("No active entries to stop")
		} else {
			fmt.Printf("Stopped %d entries\n", stoppedCount)
		}

	} else {
		// Parse keyword or ID argument
		parsedArg, err := ParseKeywordOrID(args[0], cfg)
		if err != nil {
			return err
		}

		if parsedArg.Type == ArgumentTypeID {
			// Stop by short ID
			entry := parsedArg.Entry

			if !entry.Active {
				return fmt.Errorf("entry with ID %d is already stopped", parsedArg.ID)
			}

			entry.Stop()
			stoppedCount = 1

			duration := formatDuration(entry.Duration)
			fmt.Printf("Stopped: %s %v - %s\n",
				entry.Keyword,
				entry.Tags,
				duration)
		} else {
			// Stop by keyword
			keyword := parsedArg.Keyword
			var targetEntry *models.Entry

			// Find the most recent active entry for this keyword
			for i := range cfg.Entries {
				entry := &cfg.Entries[i]
				if entry.Keyword == keyword && entry.Active {
					if targetEntry == nil || entry.StartTime.After(targetEntry.StartTime) {
						targetEntry = entry
					}
				}
			}

			if targetEntry == nil {
				return fmt.Errorf("no active entry found for keyword '%s'", keyword)
			}

			targetEntry.Stop()
			stoppedCount = 1

			duration := formatDuration(targetEntry.Duration)
			fmt.Printf("Stopped: %s %v - %s\n",
				targetEntry.Keyword,
				targetEntry.Tags,
				duration)
		}
	}

	// Save configuration
	if stoppedCount > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if IsVerbose() {
			fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
		}
	}

	return nil
}

func runInteractiveStop(cfg *models.Config, configManager *config.Manager) error {
	// Get all active entries
	activeEntries := cfg.GetActiveEntries()
	if len(activeEntries) == 0 {
		fmt.Println("No active entries to stop.")
		return nil
	}

	// Create selector items from active entries
	var items []tui.SelectorItem
	for _, entry := range activeEntries {
		duration := formatDuration(entry.GetCurrentDuration())
		displayText := fmt.Sprintf("ID:%d | %s %v | %s | %s",
			entry.ShortID,
			entry.Keyword,
			entry.Tags,
			entry.StartTime.Format("3:04PM"),
			duration,
		)

		items = append(items, tui.SelectorItem{
			ID:          entry.ID,
			DisplayText: displayText,
			Data:        &entry,
			Columns: []string{
				fmt.Sprintf("%d", entry.ShortID),
				entry.Keyword,
				fmt.Sprintf("%v", entry.Tags),
				entry.StartTime.Format("3:04PM"),
				duration,
			},
		})
	}

	// Show selector
	selected, err := tui.RunSelector("Select active entry to stop:", items)
	if err != nil {
		return err
	}

	// Get the selected entry and find it in the config
	selectedID := selected.Data.(*models.Entry).ID
	var targetEntry *models.Entry

	for i := range cfg.Entries {
		if cfg.Entries[i].ID == selectedID {
			targetEntry = &cfg.Entries[i]
			break
		}
	}

	if targetEntry == nil {
		return fmt.Errorf("selected entry not found")
	}

	// Stop the entry
	targetEntry.Stop()

	duration := formatDuration(targetEntry.Duration)
	fmt.Printf("Stopped: %s %v - %s\n",
		targetEntry.Keyword,
		targetEntry.Tags,
		duration)

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}
