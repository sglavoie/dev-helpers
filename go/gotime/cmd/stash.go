package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

// stashCmd represents the stash command
var stashCmd = &cobra.Command{
	Use:   "stash [show|apply|clear|pop]",
	Short: "Stash all running entries or manage stashed entries",
	Long: `Stash all currently running entries to temporarily pause them.
When no arguments are provided, stashes all active entries.
Use subcommands to manage stashed entries.

Subcommands:
  show    - View and manage stashed entries
  apply   - Unstash entries as stopped entries
  clear   - Delete all stashed entries
  pop     - Resume stashed entries

Examples:
  gt stash                           # Stash all running entries
  gt stash show                      # View and manage stashed entries
  gt stash apply                     # Unstash entries as stopped entries
  gt stash clear                     # Delete all stashed entries
  gt stash pop                       # Resume all stashed entries
  gt stash pop coding                # Resume specific stashed entries`,
	Args:    cobra.ArbitraryArgs,
	RunE:    runStash,
	Aliases: []string{"s"},
}

func init() {
	rootCmd.AddCommand(stashCmd)
}

func runStash(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Default stash behavior: stash all running entries
	return runStashDefault(cfg, configManager)
}

func runStashDefault(cfg *models.Config, configManager *config.Manager) error {
	// Get all active entries
	activeEntries := cfg.GetActiveEntries()

	if len(activeEntries) == 0 {
		fmt.Println("No entries to stash.")
		fmt.Println("Use 'gt stash show' to view existing stashes.")
		return nil
	}

	// Check if a stash already exists
	if cfg.HasActiveStash() {
		return fmt.Errorf("stash already exists, use 'stop' command first or 'stop --all' to stop all running entries\nNote: this program supports a single stash only")
	}

	// Collect entry IDs and stop all active entries
	var entryIDs []string
	var stashedEntries []string

	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if entry.Active {
			// Stop the entry and calculate duration
			entry.Stop()
			// Mark as stashed
			entry.Stashed = true
			// Add to stash
			entryIDs = append(entryIDs, entry.ID)
			// Prepare display info
			tags := ""
			if len(entry.Tags) > 0 {
				tags = fmt.Sprintf(" %v", entry.Tags)
			}
			duration := formatDuration(entry.Duration)
			stashedEntries = append(stashedEntries, fmt.Sprintf("  • %s%s - %s", entry.Keyword, tags, duration))
		}
	}

	// Create the stash
	cfg.CreateStash(entryIDs)

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Stashed %d entries:\n", len(entryIDs))
	for _, entryDesc := range stashedEntries {
		fmt.Println(entryDesc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

