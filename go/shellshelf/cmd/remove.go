package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove commands from the shelf",
	Long:  "Remove one or more command(s) from the shelf by ID(s).",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmds, err := commands.Load()
		if err != nil {
			fmt.Println(err)
			cmds = map[string]models.Command{}
		}

		err = commands.AreAllCommandIDsValid(cmds, args)
		if err != nil {
			fmt.Println("Error checking command IDs:", err)
			return
		}

		f, err := clihelpers.GetFlagBool(cmd, "force")
		if err != nil {
			fmt.Println("Error getting flag:", err)
			return
		}

		if !f {
			confirmRemoval(args, cmds)
		}

		// Only remove commands if all IDs are valid and user confirmed
		for _, id := range args {
			delete(cmds, id)
		}

		err = commands.Save(cmds)
		if err != nil {
			fmt.Println("Error saving commands:", err)
			return
		}
	},
}

func confirmRemoval(args []string, cmds map[string]models.Command) {
	fmt.Println("Are you sure you want to remove the following command(s)?")
	for _, id := range args {
		desc := cmds[id].Description
		if desc != "" {
			desc = "- " + desc
		}
		fmt.Printf("[%v] %v %v\n", id, cmds[id].Name, desc)
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("y/N: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return
	}

	input = strings.TrimSpace(input)

	if input != "y" && input != "Y" {
		fmt.Println("Aborting")
		return
	}
}

func init() {
	rootCmd.AddCommand(removeCmd)

	// Local flags
	removeCmd.Flags().BoolP("force", "f", false, "Remove commands without confirmation")
}
