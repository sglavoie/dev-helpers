package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/fzfinder"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/groups"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/cobra"
)

func warnIfInGroups(ids []string) {
	for _, id := range ids {
		groupNames := groups.GroupsContainingCommand(id)
		if len(groupNames) > 0 {
			fmt.Printf("Warning: command %s is used in group(s): %s\n", id, strings.Join(groupNames, ", "))
		}
	}
}

func removeCommands(ids []string, cfg *models.Config) {
	for _, id := range ids {
		delete(cfg.Commands, id)
	}

	// Also remove aliases that reference the removed commands
	for cmdId, alias := range cfg.Aliases {
		for _, id := range ids {
			if cmdId == id {
				fmt.Printf("Removing alias '%v'\n", alias)
				delete(cfg.Aliases, cmdId)
			}
		}
	}

	config.SaveCommands(cfg)
	config.SaveAliases(cfg)
}

func runLogicRemove(cmd *cobra.Command, args []string, cfg *models.Config) {
	// Interactive mode: no args
	if len(args) == 0 {
		selected, err := fzfinder.SelectCommands(cfg.Commands)
		if err != nil {
			return
		}
		ids := make([]string, len(selected))
		for i, c := range selected {
			ids[i] = c.Id
		}

		warnIfInGroups(ids)
		fmt.Println("Are you sure you want to remove the following command(s)?")
		for _, c := range selected {
			desc := c.Description
			if desc != "" {
				desc = "- " + desc
			}
			fmt.Printf("[%v] %v %v\n", c.Id, c.Name, desc)
		}
		confirmed, err := clihelpers.ReadUserConfirmation()
		if err != nil || !confirmed {
			return
		}

		removeCommands(ids, cfg)
		return
	}

	cmds := cfg.Commands
	err := commands.AreAllCommandIDsValid(cmds, args)
	if err != nil {
		clihelpers.FatalExit("Error checking command IDs: %v", err)
	}

	warnIfInGroups(args)

	f, err := clihelpers.GetFlagBool(cmd, "force")
	if err != nil {
		clihelpers.FatalExit("Error getting flag: %v", err)
	}

	if !f {
		_, err := confirmRemovalCommand(args, cmds)
		if err != nil {
			clihelpers.FatalExit("Error confirming removal: %v", err)
		}
	}

	removeCommands(args, cfg)
}

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:     "remove [ID [ID]...]",
	Aliases: []string{"rm"},
	Short:   "Remove commands from the shelf",
	Long: `Remove one or more command(s) from the shelf by ID(s).
If called with no arguments, an interactive multi-select fuzzy finder is shown.`,
	Run: func(cmd *cobra.Command, args []string) {
		runLogicRemove(cmd, args, config.Cfg)
	},
}

func confirmRemovalCommand(args []string, cmds models.Commands) (bool, error) {
	fmt.Println("Are you sure you want to remove the following command(s)?")
	for _, id := range args {
		desc := cmds[id].Description
		if desc != "" {
			desc = "- " + desc
		}
		fmt.Printf("[%v] %v %v\n", id, cmds[id].Name, desc)
	}
	return clihelpers.ReadUserConfirmation()
}

func init() {
	rootCmd.AddCommand(removeCmd)

	// Local flags
	removeCmd.Flags().BoolP("force", "f", false, "Remove commands without confirmation")
}
