package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

var (
	undoList bool
)

// undoCmd represents the undo command
var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last destructive operation or list available operations",
	Long: `Undo the most recent destructive operation such as delete, bulk edit, or stash clear.
This command can reverse the last operation that modified or removed entries.
Use the --list flag to see available operations that can be undone.

Examples:
  gt undo                            # Undo the last destructive operation
  gt undo --list                     # List all available undo operations
  
Note: Only the most recent operation can be undone. Undo history is maintained for the last 10 operations.`,
	Args:    cobra.NoArgs,
	RunE:    runUndo,
	Aliases: []string{"u"},
}

func init() {
	rootCmd.AddCommand(undoCmd)

	undoCmd.Flags().BoolVarP(&undoList, "list", "l", false, "list available undo operations")
}

func runUndo(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Handle list flag
	if undoList {
		return runUndoList(cfg)
	}

	// Check if there's anything to undo
	if !cfg.HasUndoHistory() {
		fmt.Println("No operations to undo.")
		return nil
	}

	// Get the last operation
	lastRecord := cfg.GetLastUndoRecord()
	if lastRecord == nil {
		fmt.Println("No operations to undo.")
		return nil
	}

	// Perform the undo based on operation type
	switch lastRecord.Operation {
	case models.UndoOperationDelete:
		return undoDelete(cfg, configManager, lastRecord)
	case models.UndoOperationBulkEdit:
		return undoBulkEdit(cfg, configManager, lastRecord)
	case models.UndoOperationClear:
		return undoClear(cfg, configManager, lastRecord)
	default:
		return fmt.Errorf("unsupported undo operation: %s", lastRecord.Operation)
	}
}

// undoDelete restores deleted entries
func undoDelete(cfg *models.Config, configManager *config.Manager, record *models.UndoRecord) error {
	// Extract the deleted entries from the undo data
	entriesData, ok := record.Data["entries"]
	if !ok {
		return fmt.Errorf("invalid undo data: missing entries")
	}

	// Convert the data back to Entry objects
	var deletedEntries []models.Entry
	entriesBytes, err := json.Marshal(entriesData)
	if err != nil {
		return fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &deletedEntries); err != nil {
		return fmt.Errorf("failed to restore deleted entries: %w", err)
	}

	// Restore the entries
	restoredCount := 0
	var restoredDescriptions []string

	for _, entry := range deletedEntries {
		// Add the entry back
		cfg.Entries = append(cfg.Entries, entry)
		restoredCount++

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		duration := formatDuration(entry.Duration)
		status := "stopped"
		if entry.Active {
			status = "active"
		} else if entry.Stashed {
			status = "stashed"
		}
		restoredDescriptions = append(restoredDescriptions,
			fmt.Sprintf("  • %s%s (ID: %d) - %s - %s", entry.Keyword, tags, entry.ShortID, duration, status))
	}

	// Update short IDs
	cfg.UpdateShortIDs()

	// Remove the undo record
	cfg.RemoveLastUndoRecord()

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Undone: %s\n", record.Description)
	fmt.Printf("Restored %d entries:\n", restoredCount)
	for _, desc := range restoredDescriptions {
		fmt.Println(desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// undoBulkEdit restores entries to their previous state before bulk editing
func undoBulkEdit(cfg *models.Config, configManager *config.Manager, record *models.UndoRecord) error {
	// Extract the original entries from the undo data
	originalData, ok := record.Data["original_entries"]
	if !ok {
		return fmt.Errorf("invalid undo data: missing original entries")
	}

	// Convert the data back to Entry objects
	var originalEntries []models.Entry
	entriesBytes, err := json.Marshal(originalData)
	if err != nil {
		return fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &originalEntries); err != nil {
		return fmt.Errorf("failed to restore original entries: %w", err)
	}

	// Restore the original state of the entries
	restoredCount := 0
	var restoredDescriptions []string

	for _, originalEntry := range originalEntries {
		// Find and replace the current entry with the original
		for i := range cfg.Entries {
			if cfg.Entries[i].ID == originalEntry.ID {
				cfg.Entries[i] = originalEntry
				restoredCount++

				// Prepare display info
				tags := ""
				if len(originalEntry.Tags) > 0 {
					tags = fmt.Sprintf(" %v", originalEntry.Tags)
				}
				duration := formatDuration(originalEntry.Duration)
				status := "stopped"
				if originalEntry.Active {
					status = "active"
				} else if originalEntry.Stashed {
					status = "stashed"
				}
				restoredDescriptions = append(restoredDescriptions,
					fmt.Sprintf("  • %s%s (ID: %d) - %s - %s", originalEntry.Keyword, tags, originalEntry.ShortID, duration, status))
				break
			}
		}
	}

	// Remove the undo record
	cfg.RemoveLastUndoRecord()

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Undone: %s\n", record.Description)
	fmt.Printf("Restored %d entries to their original state:\n", restoredCount)
	for _, desc := range restoredDescriptions {
		fmt.Println(desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// undoClear restores entries that were cleared/deleted in bulk
func undoClear(cfg *models.Config, configManager *config.Manager, record *models.UndoRecord) error {
	// Extract the cleared entries from the undo data
	entriesData, ok := record.Data["entries"]
	if !ok {
		return fmt.Errorf("invalid undo data: missing entries")
	}

	// Also restore any stashes if they existed
	stashesData, hasStashes := record.Data["stashes"]

	// Convert the entries data back to Entry objects
	var clearedEntries []models.Entry
	entriesBytes, err := json.Marshal(entriesData)
	if err != nil {
		return fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &clearedEntries); err != nil {
		return fmt.Errorf("failed to restore cleared entries: %w", err)
	}

	// Restore stashes if they existed
	if hasStashes {
		var clearedStashes []models.Stash
		stashesBytes, err := json.Marshal(stashesData)
		if err != nil {
			return fmt.Errorf("failed to process stash undo data: %w", err)
		}

		if err := json.Unmarshal(stashesBytes, &clearedStashes); err != nil {
			return fmt.Errorf("failed to restore stashes: %w", err)
		}

		// Restore the stashes
		cfg.Stashes = append(cfg.Stashes, clearedStashes...)
	}

	// Restore the entries
	restoredCount := 0
	var restoredDescriptions []string

	for _, entry := range clearedEntries {
		// Add the entry back
		cfg.Entries = append(cfg.Entries, entry)
		restoredCount++

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		duration := formatDuration(entry.Duration)
		status := "stopped"
		if entry.Active {
			status = "active"
		} else if entry.Stashed {
			status = "stashed"
		}
		restoredDescriptions = append(restoredDescriptions,
			fmt.Sprintf("  • %s%s (ID: %d) - %s - %s", entry.Keyword, tags, entry.ShortID, duration, status))
	}

	// Update short IDs
	cfg.UpdateShortIDs()

	// Remove the undo record
	cfg.RemoveLastUndoRecord()

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Undone: %s\n", record.Description)
	fmt.Printf("Restored %d entries:\n", restoredCount)
	for _, desc := range restoredDescriptions {
		fmt.Println(desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// runUndoList displays available undo operations as JSON
func runUndoList(cfg *models.Config) error {
	if !cfg.HasUndoHistory() {
		fmt.Println("No operations available to undo.")
		return nil
	}

	// Marshal the undo history to JSON
	jsonData, err := json.Marshal(cfg.UndoHistory)
	if err != nil {
		return fmt.Errorf("failed to marshal undo history to JSON: %w", err)
	}

	// Try to use jq for pretty-printing
	jqCmd := exec.Command("jq", ".")
	jqCmd.Stdin = strings.NewReader(string(jsonData))
	jqCmd.Stdout = os.Stdout
	jqCmd.Stderr = os.Stderr

	if err := jqCmd.Run(); err != nil {
		// If jq fails, fall back to Go's JSON pretty-printing
		var prettyJSON interface{}
		if err := json.Unmarshal(jsonData, &prettyJSON); err != nil {
			return fmt.Errorf("failed to unmarshal JSON for pretty-printing: %w", err)
		}

		prettyData, err := json.MarshalIndent(prettyJSON, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to pretty-print JSON: %w", err)
		}

		fmt.Println(string(prettyData))
	}

	return nil
}
