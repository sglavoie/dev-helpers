package cmd

import (
	"github.com/spf13/cobra"
)

// keywordsCmd represents the keywords command
var keywordsCmd = &cobra.Command{
	Use:   "keywords",
	Short: "Manage keywords across entries",
	Long: `Manage keywords across time tracking entries with various operations.

Subcommands:
  list    - List all unique keywords in use

Examples:
  gt keywords list                   # List all unique keywords
  gt keywords list --json            # List keywords as JSON (for programmatic use)
  gt keywords list --count           # List keywords with usage counts
  gt keywords list --usage           # List keywords with detailed usage info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(keywordsCmd)
}
