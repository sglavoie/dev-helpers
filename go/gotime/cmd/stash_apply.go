package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/spf13/cobra"
)

// stashApplyCmd represents the stash apply command
var stashApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Unstash entries as stopped entries",
	Long: `Convert all stashed entries to stopped entries without resuming them.
This is useful when you want to permanently convert stashed entries to 
completed entries without restarting the timers.

Examples:
  gt stash apply                   # Convert all stashed entries to stopped entries`,
	RunE: runStashApply,
}

func init() {
	stashCmd.AddCommand(stashApplyCmd)
}

func runStashApply(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there are any stashed entries
	stashedEntries := cfg.GetStashedEntriesPtr()
	if len(stashedEntries) == 0 {
		fmt.Println("No stash to apply.")
		return nil
	}

	var appliedEntries []string
	appliedCount := 0

	// Apply all stashed entries (convert to stopped entries)
	for _, entry := range stashedEntries {
		// Unstash the entry by setting Stashed = false
		// Keep Active = false so it remains a stopped entry
		entry.Stashed = false
		appliedCount++

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		duration := formatDuration(entry.Duration)
		appliedEntries = append(appliedEntries, fmt.Sprintf("  • %s%s - %s", entry.Keyword, tags, duration))
	}

	// Remove the stash since all entries are now unstashed
	if stash := cfg.GetActiveStash(); stash != nil {
		cfg.RemoveStash(stash.ID)
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Applied %d stashed entries (converted to stopped entries):\n", appliedCount)
	for _, entryDesc := range appliedEntries {
		fmt.Println(entryDesc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}