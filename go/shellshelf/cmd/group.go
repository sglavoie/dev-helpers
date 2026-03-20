package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/fzfinder"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/groups"
	"github.com/spf13/cobra"
)

var groupCmd = &cobra.Command{
	Use:     "group",
	Aliases: []string{"g"},
	Short:   "Manage command groups",
	Long:    "Create, list, show, run, and remove named sequences of commands.",
}

var groupAddCmd = &cobra.Command{
	Use:   "add <name> <id1> [id2...]",
	Short: "Create a command group",
	Long: `Create a named group from one or more command IDs.
By default, execution stops on the first error. Use --continue to keep going.`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		ids := args[1:]

		cont, _ := cmd.Flags().GetBool("continue")
		stopOnError := !cont

		if err := groups.Add(name, ids, stopOnError); err != nil {
			clihelpers.FatalExit("Error creating group: %v", err)
		}
		fmt.Printf("Group '%s' created with %d command(s)\n", name, len(ids))
	},
}

var groupListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all command groups",
	Run: func(cmd *cobra.Command, args []string) {
		allGroups := groups.List()
		if len(allGroups) == 0 {
			fmt.Println("No groups defined")
			return
		}

		for _, g := range allGroups {
			stopLabel := "stop-on-error"
			if !g.StopOnError {
				stopLabel = "continue-on-error"
			}
			fmt.Printf("%s  (%d commands, %s)\n", g.Name, len(g.CommandIDs), stopLabel)
		}
	},
}

var groupShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show commands in a group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		group, err := groups.Get(args[0])
		if err != nil {
			clihelpers.FatalExit("%v", err)
		}

		stopLabel := "stop-on-error"
		if !group.StopOnError {
			stopLabel = "continue-on-error"
		}
		fmt.Printf("Group: %s  (%s)\n", group.Name, stopLabel)
		clihelpers.PrintLineSeparator()

		cfg := config.Cfg
		for i, id := range group.CommandIDs {
			c, err := commands.GetById(cfg.Commands, id)
			if err != nil {
				fmt.Printf("  %d. [%s] (not found)\n", i+1, id)
				continue
			}
			decoded, err := commands.Decode(c.Command)
			if err != nil {
				decoded = "(decode error)"
			}
			fmt.Printf("  %d. [%s] %s\n     %s\n", i+1, id, c.Name, decoded)
		}
	},
}

var groupRunCmd = &cobra.Command{
	Use:     "run [name]",
	Aliases: []string{"r"},
	Short:   "Run all commands in a group",
	Run: func(cmd *cobra.Command, args []string) {
		var groupName string

		if len(args) == 0 {
			// Interactive: pick from fuzzy finder
			allGroups := groups.List()
			if len(allGroups) == 0 {
				fmt.Println("No groups defined")
				return
			}
			names := make([]string, len(allGroups))
			for i, g := range allGroups {
				names[i] = fmt.Sprintf("%s  (%d commands)", g.Name, len(g.CommandIDs))
			}
			selected, err := fzfinder.SelectAction(names)
			if err != nil {
				return
			}
			// Extract group name (before the first "  (")
			groupName = strings.SplitN(selected, "  (", 2)[0]
		} else {
			groupName = args[0]
		}

		group, err := groups.Get(groupName)
		if err != nil {
			clihelpers.FatalExit("%v", err)
		}

		if err := groups.Run(group); err != nil {
			clihelpers.FatalExit("Group execution failed: %v", err)
		}
	},
}

var groupRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a command group",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("Remove group '%s'?\n", name)
			confirmed, err := clihelpers.ReadUserConfirmation()
			if err != nil || !confirmed {
				return
			}
		}

		if err := groups.Remove(name); err != nil {
			clihelpers.FatalExit("%v", err)
		}
		fmt.Printf("Group '%s' removed\n", name)
	},
}

func init() {
	rootCmd.AddCommand(groupCmd)

	groupCmd.AddCommand(groupAddCmd)
	groupCmd.AddCommand(groupListCmd)
	groupCmd.AddCommand(groupShowCmd)
	groupCmd.AddCommand(groupRunCmd)
	groupCmd.AddCommand(groupRemoveCmd)

	groupAddCmd.Flags().Bool("continue", false, "Continue execution on error instead of stopping")
	groupRemoveCmd.Flags().BoolP("force", "f", false, "Remove without confirmation")
}
