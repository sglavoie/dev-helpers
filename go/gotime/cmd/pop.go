package cmd

import (
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

// popCmd represents the pop command
var popCmd = &cobra.Command{
	Use:   "pop [keyword | ID]...",
	Short: "Resume stashed entries",
	Long: `Resume all stashed entries or specific entries by keyword or ID.
When no arguments are provided, resumes all stashed entries.
When arguments are provided, resumes only the matching stashed entries.

Examples:
  gt pop                             # Resume all stashed entries
  gt pop coding                      # Resume stashed "coding" entries
  gt pop 5                           # Resume stashed entry with ID 5
  gt pop coding 3 meeting            # Resume multiple specific entries`,
	Args: cobra.ArbitraryArgs,
	RunE: runPop,
	Aliases: []string{"p"},
}

func init() {
	rootCmd.AddCommand(popCmd)
}

func runPop(cmd *cobra.Command, args []string) error {
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
		return runPopAll(cfg, configManager)
	} else {
		// Pop specific entries
		return runPopSpecific(cfg, configManager, args)
	}
}

func runPopAll(cfg *models.Config, configManager *config.Manager) error {
	// Get all stashed entries
	stashedEntries := cfg.GetStashedEntriesPtr()
	
	if len(stashedEntries) == 0 {
		fmt.Println("No stash to pop.")
		return nil
	}

	var resumedEntries []string
	
	// Resume all stashed entries
	for _, entry := range stashedEntries {
		// Check if there's already an active entry for this keyword
		if cfg.HasActiveEntryForKeyword(entry.Keyword) {
			fmt.Printf("Warning: Skipping '%s' - an active entry for this keyword already exists\n", entry.Keyword)
			continue
		}
		
		// Resume the entry
		entry.Stashed = false
		entry.Active = true
		entry.StartTime = time.Now()
		entry.EndTime = nil
		
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

func runPopSpecific(cfg *models.Config, configManager *config.Manager, args []string) error {
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
		
		// Resume the entry
		entry.Stashed = false
		entry.Active = true
		entry.StartTime = time.Now()
		entry.EndTime = nil
		
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