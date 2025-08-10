package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

var (
	continueLast bool
)

// continueCmd represents the continue command
var continueCmd = &cobra.Command{
	Use:   "continue [keyword | ID | --last]",
	Short: "Continue tracking time from a previous entry",
	Long: `Continue time tracking by creating a new entry based on a previous one.
When no arguments are provided, displays an interactive table of unique keywords from the last month.
You can continue the last stopped entry, continue by keyword (most recent for that keyword),
or continue by ID number.

Examples:
  gt continue                       # Interactive selection from recent keywords
  gt continue --last                # Continue the most recent stopped entry
  gt continue coding                # Continue the most recent "coding" entry
  gt continue 5                     # Continue entry with short ID 5`,
	Args: func(cmd *cobra.Command, args []string) error {
		hasArgument := len(args) > 0
		hasLast := continueLast

		if hasArgument && hasLast {
			return fmt.Errorf("cannot specify both an argument and --last flag")
		}

		return nil
	},
	RunE:    runContinue,
	Aliases: []string{"cont", "c"},
}

func init() {
	rootCmd.AddCommand(continueCmd)

	continueCmd.Flags().BoolVar(&continueLast, "last", false, "continue the last stopped entry")
}

func runContinue(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var sourceEntry *models.Entry

	// Interactive selection if no arguments and no flags provided
	if len(args) == 0 && !continueLast {
		return runInteractiveContinue(cfg, configManager)
	}

	if continueLast {
		// Find the most recent stopped entry (excluding stashed entries)
		for i := range cfg.Entries {
			entry := &cfg.Entries[i]
			if !entry.Active && !entry.Stashed {
				if sourceEntry == nil || entry.StartTime.After(sourceEntry.StartTime) {
					sourceEntry = entry
				}
			}
		}

		if sourceEntry == nil {
			return fmt.Errorf("no previous entries found to continue")
		}

		// Check if there's already an active entry for this keyword
		if cfg.HasActiveEntryForKeyword(sourceEntry.Keyword) {
			return fmt.Errorf("an active stopwatch for keyword '%s' is already running", sourceEntry.Keyword)
		}

	} else {
		// Parse keyword or ID argument
		parsedArg, err := ParseKeywordOrID(args[0], cfg)
		if err != nil {
			return err
		}

		if parsedArg.Type == ArgumentTypeID {
			// Continue by ID
			sourceEntry = parsedArg.Entry

			if sourceEntry.Active {
				return fmt.Errorf("entry with ID %d is already running", parsedArg.ID)
			}

			// Check if there's already an active entry for this keyword
			if cfg.HasActiveEntryForKeyword(sourceEntry.Keyword) {
				return fmt.Errorf("an active stopwatch for keyword '%s' is already running", sourceEntry.Keyword)
			}

		} else {
			// Continue by keyword
			keyword := parsedArg.Keyword
			sourceEntry = cfg.GetLastNonStashedEntryByKeyword(keyword)

			if sourceEntry == nil {
				return fmt.Errorf("no previous entries found for keyword '%s'", keyword)
			}

			if sourceEntry.Active {
				return fmt.Errorf("entry for keyword '%s' is already running", keyword)
			}

			// Check if there's already an active entry for this keyword
			// (this check is redundant with the above since we're looking for the last entry by keyword,
			// but it's more explicit and consistent with other methods)
			if cfg.HasActiveEntryForKeyword(keyword) {
				return fmt.Errorf("an active stopwatch for keyword '%s' is already running", keyword)
			}
		}
	}

	// Create new entry based on source entry
	shortID := getNextShortID(cfg)
	newEntry := models.NewEntry(sourceEntry.Keyword, sourceEntry.Tags, shortID)

	// Add to configuration
	cfg.AddEntry(newEntry)

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Format output
	timeStr := newEntry.StartTime.Format("3:04:05 PM")
	if len(newEntry.Tags) > 0 {
		fmt.Printf("Continued: %s %v at %s\n", newEntry.Keyword, newEntry.Tags, timeStr)
	} else {
		fmt.Printf("Continued: %s at %s\n", newEntry.Keyword, timeStr)
	}

	if IsVerbose() {
		fmt.Printf("Based on entry ID: %s\n", sourceEntry.ID)
		fmt.Printf("New entry ID: %s (Short ID: %d)\n", newEntry.ID, newEntry.ShortID)
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

func runInteractiveContinue(cfg *models.Config, configManager *config.Manager) error {
	if len(cfg.Entries) == 0 {
		fmt.Println("No previous entries found to continue.")
		return nil
	}

	// Get unique keywords from the last month
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	keywordEntries := make(map[string]*models.Entry)

	// Find the most recent entry for each keyword within the last month
	// but exclude keywords that already have active timers and exclude stashed entries
	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if entry.StartTime.After(oneMonthAgo) && !entry.Active && !entry.Stashed {
			// Skip this keyword if there's already an active timer for it
			if cfg.HasActiveEntryForKeyword(entry.Keyword) {
				continue
			}

			if existing, exists := keywordEntries[entry.Keyword]; !exists || entry.StartTime.After(existing.StartTime) {
				keywordEntries[entry.Keyword] = entry
			}
		}
	}

	if len(keywordEntries) == 0 {
		fmt.Println("No entries from the last month available to continue.")
		fmt.Println("(Keywords with active timers are not shown)")
		return nil
	}

	// Create selector items
	var items []tui.SelectorItem
	for keyword, entry := range keywordEntries {
		duration := formatDuration(entry.Duration)
		displayText := fmt.Sprintf("%s %v | %s | %s",
			keyword,
			entry.Tags,
			entry.StartTime.Format("Jan 02 3:04PM"),
			duration,
		)

		items = append(items, tui.SelectorItem{
			ID:          entry.ID,
			DisplayText: displayText,
			Data:        entry,
			Columns: []string{
				keyword,
				fmt.Sprintf("%v", entry.Tags),
				entry.StartTime.Format("Jan 02 3:04PM"),
				duration,
			},
		})
	}

	// Show multi-selector for continuing multiple keywords
	selectedItems, err := tui.RunMultiSelector("Select keywords to continue (multi-selection supported):", items)
	if err != nil {
		return err
	}

	if len(selectedItems) == 0 {
		fmt.Println("No keywords selected for continuation.")
		return nil
	}

	// Build confirmation message for multiple entries
	var confirmMessage strings.Builder
	if len(selectedItems) > 1 {
		confirmMessage.WriteString("Are you sure you want to continue the following keywords?\n\n")

		for i, item := range selectedItems {
			entry := item.Data.(*models.Entry)
			confirmMessage.WriteString(fmt.Sprintf("%d. %s %v (last used: %s)\n",
				i+1, entry.Keyword, entry.Tags, entry.StartTime.Format("Jan 02 3:04PM")))
		}

		confirmed, err := tui.RunConfirm(confirmMessage.String())
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !confirmed {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// Continue all selected keywords
	continuedCount := 0
	var continuedEntries []string
	var skippedEntries []string

	for _, item := range selectedItems {
		sourceEntry := item.Data.(*models.Entry)

		// Double-check that there's no active entry for this keyword
		if cfg.HasActiveEntryForKeyword(sourceEntry.Keyword) {
			skippedEntries = append(skippedEntries,
				fmt.Sprintf("%s %v (already has active timer)", sourceEntry.Keyword, sourceEntry.Tags))
			continue
		}

		// Create new entry based on source entry
		shortID := getNextShortID(cfg)
		newEntry := models.NewEntry(sourceEntry.Keyword, sourceEntry.Tags, shortID)

		// Add to configuration
		cfg.AddEntry(newEntry)
		continuedCount++

		timeStr := newEntry.StartTime.Format("3:04:05 PM")
		if len(newEntry.Tags) > 0 {
			continuedEntries = append(continuedEntries,
				fmt.Sprintf("%s %v at %s", newEntry.Keyword, newEntry.Tags, timeStr))
		} else {
			continuedEntries = append(continuedEntries,
				fmt.Sprintf("%s at %s", newEntry.Keyword, timeStr))
		}
	}

	// Save configuration if any entries were continued
	if continuedCount > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Display results
	if continuedCount > 0 {
		if continuedCount == 1 {
			fmt.Printf("Continued: %s\n", continuedEntries[0])
		} else {
			fmt.Printf("Continued %d entries:\n", continuedCount)
			for _, entryDesc := range continuedEntries {
				fmt.Printf("  • %s\n", entryDesc)
			}
		}
	}

	if len(skippedEntries) > 0 {
		fmt.Printf("\nSkipped %d entries:\n", len(skippedEntries))
		for _, entryDesc := range skippedEntries {
			fmt.Printf("  • %s\n", entryDesc)
		}
	}

	if continuedCount == 0 && len(skippedEntries) == 0 {
		fmt.Println("No entries were continued.")
	}

	if IsVerbose() && continuedCount > 0 {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}
