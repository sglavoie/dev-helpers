package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/find"
	"github.com/spf13/cobra"
)

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find text",
	Short: "Find a command on the shelf",
	Long: `Find a command on the shelf by searching for text anywhere in the
command name, description, tags, etc.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		decoded, err := commands.LoadDecoded()
		if err != nil {
			return
		}

		matches := find.InCommandFields(decoded, args)
		if len(matches) == 0 {
			fmt.Println("No matches found")
			return
		}

		fmt.Println("Commands:")
		fmt.Println("--------------------------------------------------------------------------------")
		for _, id := range matches {
			decodedCmd := decoded[id]
			if decodedCmd.Description == "" {
				fmt.Printf("[%v] %v\n", id, decodedCmd.Name)
			} else {
				fmt.Printf("[%v] %v - %v\n", id, decodedCmd.Name, decodedCmd.Description)
			}
			if len(decodedCmd.Tags) > 0 {
				fmt.Printf("Tags: %v\n", decodedCmd.Tags)
			}
			fmt.Printf("%v\n", decodedCmd.Command)
			fmt.Println("--------------------------------------------------------------------------------")
		}
	},
}

func init() {
	rootCmd.AddCommand(findCmd)
}
