package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/find"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/fzfinder"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/osutils"
	"github.com/spf13/cobra"
)

func interactiveFind(cfg *models.Config) {
	selected, err := fzfinder.SelectCommand(cfg.Commands)
	if err != nil {
		return
	}

	decoded, err := commands.Decode(selected.Command)
	if err != nil {
		clihelpers.FatalExit("Error decoding command: %v", err)
	}

	actions := []string{"Run", "Copy", "Edit", "Print"}
	action, err := fzfinder.SelectAction(actions)
	if err != nil {
		return
	}

	switch action {
	case "Run":
		if cfg.Settings.ConfirmBeforeRun {
			fmt.Printf("About to run the following command:\n%v\n\n", decoded)
			proceeding, err := clihelpers.WarnBeforeProceeding()
			if err != nil {
				clihelpers.FatalExit("Error getting confirmation: %v", err)
			}
			if !proceeding {
				clihelpers.FatalExit("Operation aborted")
			}
		}
		osutils.ExecShellCommand(decoded)
	case "Copy":
		if err := osutils.CopyToClipboard(decoded); err != nil {
			clihelpers.FatalExit("Error copying to clipboard: %v", err)
		}
		fmt.Println("Command copied to clipboard")
	case "Edit":
		runLogicEdit(editCmd, []string{selected.Id}, cfg)
	case "Print":
		fmt.Println(decoded)
	}
}

func runLogicFind(cmd *cobra.Command, args []string, cfg *models.Config) {
	flagsPassed := clihelpers.CountSetFlags(cmd)

	// Interactive mode: no args and no flags
	if len(args) == 0 && flagsPassed == 0 {
		interactiveFind(cfg)
		return
	}

	var err error
	cfg.Commands, err = commands.LoadDecoded(cfg.Commands)
	if err != nil {
		return
	}

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

If no flags are specified, the search will be performed on all fields.
If called with no arguments and no flags, an interactive fuzzy finder is shown.`,
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
