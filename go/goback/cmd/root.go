package cmd

import (
	"fmt"
	"log"

	"github.com/carlmjohnson/versioninfo"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "goback",
	Version: fmt.Sprintf("%s (built on %s)", versioninfo.Short(), lastCommitDate()),
	Short:   "A no-nonsense backup tool using rsync",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Parent().Name() != "config" {
			if err := config.MustInitConfig(true, true); err != nil {
				return err
			}
			return config.ResolveProfiles()
		}
		return config.MustInitConfig(false, false)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		log.Fatal("Could not execute: ", err)
	}
}

func init() {
	RootCmd.CompletionOptions.HiddenDefaultCmd = true
	RootCmd.PersistentFlags().StringVar(&config.CfgFile, "config", "", "config file (default is $HOME/.goback.json)")
	RootCmd.PersistentFlags().StringVarP(&config.ProfileFlag, "profile", "p", "", "profile to use (e.g. macbook, media)")
	RootCmd.PersistentFlags().BoolVar(&config.AllProfiles, "all", false, "run all profiles regardless of hostname")
	RootCmd.PersistentFlags().Bool("verbose", false, "enable verbose rsync output (overrides per-profile setting)")
	RootCmd.PersistentFlags().Bool("quiet", false, "suppress non-essential rsync output (--progress and --stats)")
	printCmd.Flags().Bool("raw", false, "print the raw configuration without unmarshalling it")

	if err := viper.BindPFlag("cliVerbose", RootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("cliQuiet", RootCmd.PersistentFlags().Lookup("quiet")); err != nil {
		panic(err)
	}
	RootCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")
}

func lastCommitDate() string {
	return versioninfo.LastCommit.Format("2006-01-02")
}
