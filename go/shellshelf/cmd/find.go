package cmd

import (
	"fmt"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/find"
	"github.com/spf13/cobra"
)

func runLogicFind(cmd *cobra.Command, args []string, cfg *models.Config) {
	var err error
	cfg.Commands, err = commands.LoadDecoded(cfg.Commands)
	if err != nil {
		return
	}

	flagsPassed := clihelpers.CountSetFlags(cmd)

	var matches []string
	if flagsPassed == 0 {
		find.HandleFindInAllCommands(cfg.Commands, args)
		return
	}

	if find.HandleAllFlagReturns(cmd, flagsPassed, cfg.Commands, args) {
		return
	}

	if find.HandleAllExceptFlagReturns(cmd, flagsPassed, cfg.Commands, args) {
		return
	}

	matches = find.InFlagsPassed(cmd, cfg.Commands)
	if len(matches) == 0 {
		fmt.Println("No matches found")
		return
	}

	editor, err := cmd.Flags().GetBool("editor")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if editor {
		err := find.ShowMatchesInEditor(cfg.Commands, matches)
		if err != nil {
			fmt.Println("Error ShowMatchesInEditor:", err)
		}
		return
	}

	find.PrintMatches(cfg.Commands, matches)
}

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:     "find {text... | [flags]... [text...]}",
	Aliases: []string{"f"},
	Short:   "Find a command on the shelf",
	Long: `Find a command on the shelf by searching for text anywhere in the
command name, description, tags, etc.

If no flags are specified, the search will be performed on all fields.`,
	Run: func(cmd *cobra.Command, args []string) {
		runLogicFind(cmd, args, config.Cfg)
	},
}

func init() {
	rootCmd.AddCommand(findCmd)

	// Local flags
	findCmd.Flags().BoolP("all", "a", false, "Show all commands, ignoring search terms")
	findCmd.Flags().BoolP("editor", "e", false, "Show the results in an editor")
	findCmd.Flags().BoolP("exclude", "x", false, "Show all commands except those matching any of the search terms")
	findCmd.Flags().StringSliceP("command", "c", []string{}, "Restrict search to the command contents")
	findCmd.Flags().StringSliceP("description", "d", []string{}, "Restrict search to the command description")
	findCmd.Flags().StringSliceP("name", "n", []string{}, "Restrict search to the command name")
	findCmd.Flags().StringSliceP("tags", "t", []string{}, "Restrict search to the command tags")
}
