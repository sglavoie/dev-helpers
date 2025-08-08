package cmd

import (
	"github.com/spf13/cobra"
)

// tagsCmd represents the tags command
var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags across entries",
	Long: `Manage tags across time tracking entries with various operations.

Subcommands:
  list    - List all unique tags in use
  rename  - Rename a tag across all entries
  remove  - Remove a tag from entries

Examples:
  gt tags list                      # List all unique tags
  gt tags list --count              # List tags with usage counts
  gt tags list --usage              # List tags with detailed usage info
  gt tags rename old-tag new-tag    # Rename 'old-tag' to 'new-tag' in all entries
  gt tags remove deprecated-tag     # Remove 'deprecated-tag' from all entries
  gt tags remove work --id 5        # Remove 'work' tag only from entry ID 5
  gt tags remove meeting coding     # Remove 'meeting' tag only from entries with 'coding' keyword`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(tagsCmd)
}
