package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/spf13/cobra"
)

// undoCmd represents the undo command
var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last destructive operation or list available operations",
	Long: `Undo destructive operations such as delete, bulk edit, or stash clear.
Displays a table showing all undoable operations with their affected entries.
You can select multiple operations to restore at once.
  
Note: You can restore operations in any order. Undo history is maintained for the last 10 operations.`,
	Args:    cobra.NoArgs,
	RunE:    runUndo,
	Aliases: []string{"u"},
}

func init() {
	rootCmd.AddCommand(undoCmd)

	// No flags needed for the simplified undo command
}

func runUndo(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there's anything to undo
	if !cfg.HasUndoHistory() {
		fmt.Println("No operations to undo.")
		return nil
	}

	// Run interactive table view for undo selection
	return runInteractiveUndo(cfg, configManager)
}
