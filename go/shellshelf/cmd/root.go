package cmd

import (
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ss",
	Short: "ShellShelf shelves your shell commands",
	Long: `Keep your favorite commands on a shelf.

Manage your shell commands with ShellShelf.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//Run: func(cmd *cobra.Command, args []string) { },
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate the autocompletion script for the specified shell",
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
}

func init() {
	cobra.OnInitialize(config.Init)
	completion := completionCmd()
	completion.Hidden = true
	rootCmd.AddCommand(completion)
}
