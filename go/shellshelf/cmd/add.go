package cmd

import (
	"errors"
	"fmt"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/cobra"
)

func preRunLogicAdd(cmd *cobra.Command) error {
	editor, err := cmd.Flags().GetBool("editor")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	command, err := cmd.Flags().GetString("command")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	if (editor && command != "") || (!editor && command == "") {
		return errors.New("you must specify either --editor or --command, but not both")
	}

	return nil
}

func runLogicAdd(cmd *cobra.Command, cfg *models.Config) {
	command, cmdIDs, err := buildCommand(cmd, cfg)
	if err != nil {
		clihelpers.FatalExit("Error building command: %v", err)
	}

	if cmdIDs != nil {
		cfg.Commands = commands.Remove(cfg.Commands, cmdIDs)
	}
	cfg.Commands = commands.Add(cfg.Commands, command)

	config.SaveCommands(cfg)
}

func buildCommand(cmd *cobra.Command, cfg *models.Config) (models.Command, []string, error) {
	command := models.Command{}

	name, err := clihelpers.GetFlagString(cmd, "name")
	if err != nil {
		return command, []string{}, err
	}
	command.Name = name

	cmdTaken, cmdIDs := commands.IsCommandNameAlreadyTaken(cfg.Commands, command.Name)
	if cmdTaken {
		fmt.Println("This command name already exists: it would be overwritten by this command.")
		proceeding, err := clihelpers.WarnBeforeProceeding()
		if err != nil {
			return command, []string{}, err
		}
		if !proceeding {
			return command, []string{}, errors.New("operation aborted")
		}
	}

	if description, err := clihelpers.GetFlagString(cmd, "description"); err == nil && description != "" {
		command.Description = description
	}

	command, err = readCommand(cmd, command)
	if err != nil {
		fmt.Println("Error reading command:", err)
		return command, []string{}, err
	}

	if tags, err := clihelpers.GetFlagStringSlice(cmd, "tags"); err == nil && len(tags) > 0 {
		command.Tags = tags
	}

	return command, cmdIDs, nil
}

func readCommand(cmd *cobra.Command, command models.Command) (models.Command, error) {
	v, err := clihelpers.GetFlagString(cmd, "command")
	if err != nil {
		return command, err
	}

	// Get command from flag
	if v != "" {
		command.Command = v
		return command, nil
	}

	// Get command from editor
	editor, err := cmd.Flags().GetBool("editor")
	if err != nil {
		return command, err
	}
	if !editor {
		return command, errors.New("no editor specified")
	}

	v, err = commands.GetCommandWithEditor()
	if err != nil {
		return command, err
	}
	if v == "" {
		return command, errors.New("no command specified")
	}
	command.Command = v
	return command, nil
}

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"a"},
	Short:   "Add a command to the shelf",
	Long:    "Add a command to the shelf. You can specify the command directly or open an editor to enter it.",
	Example: "add -n 'my command' -c 'echo hello world'\nadd -n 'my command' -e -t tag1,tag2",
	Args:    cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preRunLogicAdd(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		runLogicAdd(cmd, &config.Cfg)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Local flags
	addCmd.Flags().StringP("command", "c", "", "Specify the command directly")
	addCmd.Flags().StringP("description", "d", "", "Description of the command")
	addCmd.Flags().BoolP("editor", "e", false, "Open editor to enter command")
	addCmd.Flags().StringP("name", "n", "", "Name of the command")
	addCmd.Flags().StringSliceP("tags", "t", []string{}, "Tags for the command, comma-separated")

	// Required flags
	err := addCmd.MarkFlagRequired("name")
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
}
