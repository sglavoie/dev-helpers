package cmd

import (
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

var (
	stashPopBackdate string
)

// stashPopCmd represents the stash pop command
var stashPopCmd = &cobra.Command{
	Use:   "pop [keyword | ID]...",
	Short: "Resume stashed entries",
	Long: `Resume all stashed entries or specific entries by keyword or ID.
When no arguments are provided, resumes all stashed entries.
When arguments are provided, resumes only the matching stashed entries.

The --backdate flag allows you to resume timers with a time offset, useful when you
forgot to resume tracking but know when you actually began working again.

Examples:
  gt stash pop                             # Resume all stashed entries
  gt stash pop coding                      # Resume stashed "coding" entries
  gt stash pop 5                           # Resume stashed entry with ID 5
  gt stash pop coding 3 meeting            # Resume multiple specific entries
  gt stash pop --backdate 5m               # Resume all stashed, started 5 minutes ago
  gt stash pop coding --backdate 1h30m     # Resume "coding", started 1h30m ago
  gt stash pop 5 --backdate 10             # Resume entry ID 5, started 10 minutes ago

Backdate formats: 5, 5m, 30s, 1h, 1h30, 1h30m, 2h30m30s (no unit defaults to minutes)`,
	Args: cobra.ArbitraryArgs,
	RunE: runStashPop,
}

func init() {
	stashCmd.AddCommand(stashPopCmd)

	stashPopCmd.Flags().StringVarP(&stashPopBackdate, "backdate", "b", "", "resume timers with a time offset (e.g., 5m, 1h30m, 10)")
}

func runStashPop(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there are any stashed entries
	if !cfg.HasActiveStash() {
		fmt.Println("No stash to pop.")
		return nil
	}

	if len(args) == 0 {
		// Pop all stashed entries
		return runStashPopAll(cfg, configManager)
	} else {
		// Pop specific entries
		return runStashPopSpecific(cfg, configManager, args)
	}
}

func runStashPopAll(cfg *models.Config, configManager *config.Manager) error {
	// Get all stashed entries
	stashedEntries := cfg.GetStashedEntriesPtr()

	if len(stashedEntries) == 0 {
		fmt.Println("No stash to pop.")
		return nil
	}

	// Parse backdate offset if provided
	var startTime time.Time
	if stashPopBackdate != "" {
		offset, err := ParseDuration(stashPopBackdate)
		if err != nil {
			return fmt.Errorf("invalid backdate format: %w", err)
		}
		startTime = time.Now().Add(-offset)
	} else {
		startTime = time.Now()
	}

	var resumedEntries []string

	// Resume all stashed entries
	for _, entry := range stashedEntries {
		// Check if there's already an active entry for this keyword
		if cfg.HasActiveEntryForKeyword(entry.Keyword) {
			fmt.Printf("Warning: Skipping '%s' - an active entry for this keyword already exists\n", entry.Keyword)
			continue
		}

		// Keep the original stashed entry as a completed entry (don't modify it)
		entry.Stashed = false
		// entry.Active remains false since it's a completed entry
		// entry.StartTime, entry.EndTime, and entry.Duration are preserved

		// Create a new entry with the same keyword and tags but custom start time
		shortID := getNextShortID(cfg)
		newEntry := models.NewEntryWithStartTime(entry.Keyword, entry.Tags, shortID, startTime)
		cfg.AddEntry(newEntry)

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		resumedEntries = append(resumedEntries, fmt.Sprintf("  • %s%s", entry.Keyword, tags))
	}

	// Remove the stash
	if stash := cfg.GetActiveStash(); stash != nil {
		cfg.RemoveStash(stash.ID)
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	if len(resumedEntries) > 0 {
		fmt.Printf("Resumed %d entries:\n", len(resumedEntries))
		for _, entryDesc := range resumedEntries {
			fmt.Println(entryDesc)
		}
	} else {
		fmt.Println("No entries were resumed (all keywords already have active entries).")
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

func runStashPopSpecific(cfg *models.Config, configManager *config.Manager, args []string) error {
	// Validate all arguments first (fail-fast approach)
	var targetEntries []*models.Entry

	for _, arg := range args {
		parsedArg, err := ParseKeywordOrID(arg, cfg)
		if err != nil {
			return fmt.Errorf("invalid argument '%s': %w", arg, err)
		}

		var found bool
		if parsedArg.Type == ArgumentTypeID {
			// Pop by ID
			entry := parsedArg.Entry
			if !entry.Stashed {
				return fmt.Errorf("entry with ID %d is not stashed", parsedArg.ID)
			}
			targetEntries = append(targetEntries, entry)
			found = true
		} else {
			// Pop by keyword - find stashed entries for this keyword
			keyword := parsedArg.Keyword
			stashedEntries := cfg.GetStashedEntriesPtr()

			for _, entry := range stashedEntries {
				if entry.Keyword == keyword {
					targetEntries = append(targetEntries, entry)
					found = true
				}
			}
		}

		if !found {
			return fmt.Errorf("no stashed entries found for '%s'", arg)
		}
	}

	// Parse backdate offset if provided
	var startTime time.Time
	if stashPopBackdate != "" {
		offset, err := ParseDuration(stashPopBackdate)
		if err != nil {
			return fmt.Errorf("invalid backdate format: %w", err)
		}
		startTime = time.Now().Add(-offset)
	} else {
		startTime = time.Now()
	}

	// Now resume all validated entries
	var resumedEntries []string
	var skippedEntries []string

	for _, entry := range targetEntries {
		// Check if there's already an active entry for this keyword
		if cfg.HasActiveEntryForKeyword(entry.Keyword) {
			tags := ""
			if len(entry.Tags) > 0 {
				tags = fmt.Sprintf(" %v", entry.Tags)
			}
			skippedEntries = append(skippedEntries, fmt.Sprintf("  • %s%s (active entry already exists)", entry.Keyword, tags))
			continue
		}

		// Keep the original stashed entry as a completed entry (don't modify it)
		entry.Stashed = false
		// entry.Active remains false since it's a completed entry
		// entry.StartTime, entry.EndTime, and entry.Duration are preserved

		// Create a new entry with the same keyword and tags but custom start time
		shortID := getNextShortID(cfg)
		newEntry := models.NewEntryWithStartTime(entry.Keyword, entry.Tags, shortID, startTime)
		cfg.AddEntry(newEntry)

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		resumedEntries = append(resumedEntries, fmt.Sprintf("  • %s%s", entry.Keyword, tags))
	}

	// Check if stash is now empty and remove it if so
	stashedRemaining := cfg.GetStashedEntries()
	if len(stashedRemaining) == 0 {
		if stash := cfg.GetActiveStash(); stash != nil {
			cfg.RemoveStash(stash.ID)
		}
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	if len(resumedEntries) > 0 {
		fmt.Printf("Resumed %d entries:\n", len(resumedEntries))
		for _, entryDesc := range resumedEntries {
			fmt.Println(entryDesc)
		}
	}

	if len(skippedEntries) > 0 {
		fmt.Printf("\nSkipped %d entries:\n", len(skippedEntries))
		for _, entryDesc := range skippedEntries {
			fmt.Println(entryDesc)
		}
	}

	if len(resumedEntries) == 0 && len(skippedEntries) == 0 {
		fmt.Println("No entries were resumed.")
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}