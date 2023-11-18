package cmd

import (
	"errors"
	"fmt"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/cobra"
)

func getUpdatedCommandFromFlags(cmd *cobra.Command, command models.Command) models.Command {
	setFlags := clihelpers.GetSetFlags(cmd)
	for _, flag := range setFlags {
		switch flag {
		case "command":
			getString, err := cmd.Flags().GetString("command")
			if err != nil {
				return models.Command{}
			}
			command.Command = getString
		case "description":
			getString, err := cmd.Flags().GetString("description")
			if err != nil {
				return models.Command{}
			}
			command.Description = getString
		case "name":
			getString, err := cmd.Flags().GetString("name")
			if err != nil {
				return models.Command{}
			}
			command.Name = getString
		case "tags":
			getStringSlice, err := cmd.Flags().GetStringSlice("tags")
			if err != nil {
				return models.Command{}
			}
			command.Tags = getStringSlice
		}
	}

	command.Command = commands.Encode(command.Command)
	return command
}

func preRunLogicEdit(cmd *cobra.Command) error {
	editor, err := cmd.Flags().GetBool("editor")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	flagsPassed := clihelpers.CountSetFlags(cmd)

	if !editor && flagsPassed == 0 {
		return errors.New("you must specify either --editor (with or without " +
			"other flags to limit editing to those fields) or at least one other flag")
	}

	return nil
}

func runLogicEdit(cmd *cobra.Command, args []string) {
	cmdID := args[0]
	cmds, err := commands.Load()
	if err != nil {
		fmt.Println(err)
		cmds = map[string]models.Command{}
	}

	command, ok := cmds[cmdID]
	if !ok {
		clihelpers.FatalExit("Command ID not found: %v", cmdID)
	}

	command.Command, err = commands.Decode(command.Command)
	if err != nil {
		clihelpers.FatalExit("Error decoding command: %v", err)
	}

	editor, err := cmd.Flags().GetBool("editor")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	flagsPassed := clihelpers.CountSetFlags(cmd)

	if editor {
		if flagsPassed > 1 {
			clihelpers.FatalExit("When used, `--editor` should be the only flag passed for this command")
		}

		updatedCmd, err := commands.EditAllFields(command)
		if err != nil {
			clihelpers.FatalExit("Error editing command: %v", err)
		}

		commands.RunCheckOnDecodedCommand(updatedCmd)
		updatedCmd.Command = commands.Encode(updatedCmd.Command)
		cmds[cmdID] = updatedCmd

		err = commands.Save(cmds)
		if err != nil {
			clihelpers.FatalExit("Error saving commands: %v", err)
		}
		return
	}

	updatedCmd := getUpdatedCommandFromFlags(cmd, command)
	commands.RunCheckOnDecodedCommand(updatedCmd)
	cmds[cmdID] = command
	err = commands.Save(cmds)
	if err != nil {
		clihelpers.FatalExit("Error saving commands: %v", err)
	}
}

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:     "edit ID [flags]",
	Aliases: []string{"e"},
	Short:   "Edit a shelved command",
	Long:    "Edit a shelved command by ID for the provided flags/fields, or open an editor to edit all fields.",
	Args:    cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preRunLogicEdit(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		runLogicEdit(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(editCmd)

	// Local flags
	editCmd.Flags().StringP("command", "c", "", "Edit the command directly")
	editCmd.Flags().StringP("description", "d", "", "Edit the description of the command")
	editCmd.Flags().BoolP("editor", "e", false, "Open editor to edit fields, all if none specified")
	editCmd.Flags().StringP("name", "n", "", "Edit the name of the command")
	editCmd.Flags().StringSliceP("tags", "t", []string{}, "Edit the tags for the command, comma-separated")
}
