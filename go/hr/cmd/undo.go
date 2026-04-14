package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
	"github.com/sglavoie/dev-helpers/go/hr/internal/storage"
	"github.com/spf13/cobra"
)

var undoYes bool

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Remove last logged entry",
	Long:  "Display the last logged entry and optionally remove it.",
	RunE:  runUndo,
}

func init() {
	undoCmd.Flags().BoolVarP(&undoYes, "yes", "y", false, "skip confirmation prompt")
}

func runUndo(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.LoadOrInit(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dataPath, err := config.DataPath()
	if err != nil {
		return err
	}

	entries, err := storage.ReadLast(dataPath, 1)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No entries to remove.")
		return nil
	}

	last := entries[0]
	ex, _ := findExercise(cfg.Exercises, last.Exercise)
	fmt.Printf("Last entry: %s — %s (%s)",
		last.Timestamp.Local().Format(cfg.DateFormat),
		last.Exercise,
		formatEntryDetails(last.Data, ex),
	)
	if last.Notes != "" {
		fmt.Printf(" (%s)", last.Notes)
	}
	fmt.Println()

	if !undoYes {
		fmt.Print("Remove this entry? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if _, err := storage.RemoveLast(dataPath); err != nil {
		return fmt.Errorf("removing entry: %w", err)
	}
	fmt.Println("Entry removed.")
	return nil
}
