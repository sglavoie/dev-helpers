package cmd

import (
	"errors"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/aliases"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/osutils"
	"github.com/spf13/cobra"
)

func getCommandToRun(cmd *cobra.Command, args []string, cfg *models.Config) models.Command {
	var command models.Command
	var err error

	// Run by name
	if len(args) == 1 {
		cmdName := args[0]
		command, err = commands.GetByName(cfg.Commands, cmdName)
		if err != nil {
			clihelpers.FatalExit("Error getting command by name: %v", err)
		}
		return command
	}

	// Run by ID
	if id, _ := cmd.Flags().GetString("id"); id != "" {
		command, err = commands.GetById(cfg.Commands, id)
		if err != nil {
			clihelpers.FatalExit("Error getting command by ID: %v", err)
		}
		return command
	}

	// Run by alias
	var cmdId string
	alias, _ := cmd.Flags().GetString("alias")
	cmdId, err = aliases.Get(cfg.Aliases, alias)
	if err != nil {
		clihelpers.FatalExit("Error getting command by alias: %v", err)
	}
	command, err = commands.GetById(cfg.Commands, cmdId)
	if err != nil {
		clihelpers.FatalExit("Error getting command by name: %v", err)
	}
	return command
}

func preRunLogicRun(cmd *cobra.Command) error {
	id, err := cmd.Flags().GetString("id")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	alias, err := cmd.Flags().GetString("alias")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	if id != "" && alias != "" {
		return errors.New("you must specify either --id or --alias, but not both")
	}

	numArgs := len(cmd.Flags().Args())

	if (id != "" || alias != "") && numArgs > 0 {
		return errors.New("you cannot use a command argument if either --id or --alias is used")
	}

	if (id == "" && alias == "") && numArgs == 0 {
		return errors.New("you must specify either --id or --alias, or provide a command argument")
	}

	if numArgs > 1 {
		return errors.New("you can only run one command at a time")
	}

	return nil
}

func runLogicRun(cmd *cobra.Command, args []string, cfg *models.Config) {
	command := getCommandToRun(cmd, args, cfg)
	decoded, err := commands.Decode(command.Command)
	if err != nil {
		return
	}

	if p, _ := cmd.Flags().GetBool("print"); p {
		fmt.Printf("Would run the following command:\n%v\n", decoded)
		return
	}

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
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:     "run {[flags] arg | name}",
	Aliases: []string{"r"},
	Short:   "Execute a command from the shelf",
	Long:    `Execute a command from the shelf by name (default), ID or alias.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preRunLogicRun(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		runLogicRun(cmd, args, config.Cfg)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringP("alias", "a", "", "Specify the alias corresponding to the command to run")
	runCmd.Flags().StringP("id", "i", "", "Specify the ID of the command to run")
	runCmd.Flags().BoolP("print", "p", false, "When set, the command will only be printed")
}
