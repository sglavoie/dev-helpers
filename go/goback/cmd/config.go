package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
)

func init() {
	configCmd.AddCommand(editCmd)
	configCmd.AddCommand(printCmd)
	configCmd.AddCommand(resetCmd)
	RootCmd.AddCommand(configCmd)
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Work with the configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			return
		}
	},
}

// editCmd edits the configuration file with the default editor
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the configuration file with the default editor",
	Run: func(cmd *cobra.Command, args []string) {
		config.Edit()
	},
}

// printCmd prints the configuration file
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Print the configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("raw").Value.String() == "true" {
			config.PrintRaw()
			return
		}
		config.Print()
	},
}

// resetCmd resets the configuration file
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the configuration file to its default values",
	Run: func(cmd *cobra.Command, args []string) {
		config.Reset()
	},
}
