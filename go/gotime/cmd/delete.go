package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [keyword | ID]",
	Short: "Delete time tracking entries",
	Long: `Delete time tracking entries by keyword or short ID.
When no arguments are provided, displays an interactive table to select entries for deletion.
In interactive mode, use Space to toggle selection and Enter to confirm deletion.
When deleting by keyword, all entries for that keyword will be deleted.
When deleting by ID, only the specific entry will be deleted.

Examples:
  gt delete                          # Interactive multi-selection
  gt delete coding                   # Delete all "coding" entries
  gt delete 5                        # Delete entry with short ID 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Entries) == 0 {
		fmt.Println("No entries to delete.")
		return nil
	}

	deletedCount := 0

	// Interactive selection if no arguments provided
	if len(args) == 0 {
		return runInteractiveDelete(cfg, configManager)
	}

	// Parse keyword or ID argument
	parsedArg, err := ParseKeywordOrID(args[0], cfg)
	if err != nil {
		return err
	}

	if parsedArg.Type == ArgumentTypeID {
		// Delete by short ID
		entry := parsedArg.Entry

		// Capture display info before removal (RemoveEntry updates short IDs)
		keyword := entry.Keyword
		tags := entry.Tags
		shortID := entry.ShortID

		if cfg.RemoveEntry(entry.ID) {
			deletedCount = 1
			fmt.Printf("Deleted entry: %s %v (ID: %d)\n",
				keyword, tags, shortID)
		}

	} else {
		// Delete by keyword
		keyword := parsedArg.Keyword

		// Collect entries to delete
		var toDelete []string
		for _, entry := range cfg.Entries {
			if entry.Keyword == keyword {
				toDelete = append(toDelete, entry.ID)
			}
		}

		if len(toDelete) == 0 {
			return fmt.Errorf("no entries found for keyword '%s'", keyword)
		}

		// Confirm deletion of multiple entries
		if len(toDelete) > 1 {
			message := fmt.Sprintf("This will delete %d entries for keyword '%s'.\nAre you sure you want to proceed?", len(toDelete), keyword)
			confirmed, err := tui.RunConfirm(message)
			if err != nil {
				return fmt.Errorf("confirmation failed: %w", err)
			}
			if !confirmed {
				fmt.Println("Deletion cancelled.")
				return nil
			}
		}

		// Delete entries
		for _, id := range toDelete {
			if cfg.RemoveEntry(id) {
				deletedCount++
			}
		}

		fmt.Printf("Deleted %d entries for keyword '%s'\n", deletedCount, keyword)
	}

	// Save configuration
	if deletedCount > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if IsVerbose() {
			fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
		}
	}

	return nil
}

func runInteractiveDelete(cfg *models.Config, configManager *config.Manager) error {
	// Create selector items from all entries
	var items []tui.SelectorItem
	for _, entry := range cfg.Entries {
		status := "completed"
		duration := formatDuration(entry.Duration)
		if entry.Active {
			status = "running"
			duration = formatDuration(entry.GetCurrentDuration())
		}

		displayText := fmt.Sprintf("ID:%d | %s %v | %s | %s",
			entry.ShortID,
			entry.Keyword,
			entry.Tags,
			status,
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
				status,
				duration,
			},
		})
	}

	// Show multi-selector
	selectedItems, err := tui.RunMultiSelector("Select entries to delete:", items)
	if err != nil {
		return err
	}

	if len(selectedItems) == 0 {
		fmt.Println("No entries selected for deletion.")
		return nil
	}

	// Build confirmation message
	var confirmMessage strings.Builder
	confirmMessage.WriteString("Are you sure you want to delete the following entries?\n\n")
	
	for i, item := range selectedItems {
		entry := item.Data.(*models.Entry)
		duration := formatDuration(entry.Duration)
		if entry.Active {
			duration = formatDuration(entry.GetCurrentDuration())
		}
		
		confirmMessage.WriteString(fmt.Sprintf("%d. %s %v (ID: %d) - %s\n", 
			i+1, entry.Keyword, entry.Tags, entry.ShortID, duration))
	}

	confirmed, err := tui.RunConfirm(confirmMessage.String())
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}
	if !confirmed {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	// Delete all selected entries
	deletedCount := 0
	var deletedEntries []string

	for _, item := range selectedItems {
		entry := item.Data.(*models.Entry)
		
		// Capture display info before removal (RemoveEntry updates short IDs)
		keyword := entry.Keyword
		tags := entry.Tags
		shortID := entry.ShortID
		
		if cfg.RemoveEntry(entry.ID) {
			deletedCount++
			deletedEntries = append(deletedEntries, 
				fmt.Sprintf("%s %v (ID: %d)", keyword, tags, shortID))
		}
	}

	// Display results
	fmt.Printf("Deleted %d entries:\n", deletedCount)
	for _, entryDesc := range deletedEntries {
		fmt.Printf("  • %s\n", entryDesc)
	}

	// Save configuration
	if deletedCount > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if IsVerbose() {
			fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
		}
	}

	return nil
}
