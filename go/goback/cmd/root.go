package cmd

import (
	"goback/pkg/config"
	"os"

	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "goback",
	Short: "A no-nonsense backup tool using rsync",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Parent().Name() == "config" && cmd.Name() == "print" {
			config.InitConfig(false)
			return
		}

		config.InitConfig(true)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&config.CfgFile, "config", "", "config file (default is $HOME/.goback.json)")
	printCmd.Flags().Bool("raw", false, "print the raw configuration without unmarshalling it")
}
