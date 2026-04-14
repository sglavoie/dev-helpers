package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "hr",
	Short:         "Health Records — track bodyweight exercises",
	Long:          "hr is a CLI for logging bodyweight exercise reps and rounds to a local CSV file.",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(undoCmd)
	rootCmd.AddCommand(streakCmd)
	rootCmd.AddCommand(todayCmd)
	rootCmd.Flags().StringVarP(&addExercise, "exercise", "e", "", "exercise name (skip TUI selector)")
	rootCmd.Flags().IntVarP(&addReps, "reps", "r", 0, "number of reps (0 = use config default)")
	rootCmd.Flags().IntVarP(&addRounds, "rounds", "R", 0, "number of rounds (0 = use config default)")
	rootCmd.Flags().StringVarP(&addNotes, "notes", "n", "", "optional notes")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
