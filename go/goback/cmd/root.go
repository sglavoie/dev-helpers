package cmd

import (
	"fmt"
	"log"

	"github.com/carlmjohnson/versioninfo"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Args:    cobra.NoArgs,
	Use:     "goback",
	Version: fmt.Sprintf("%s (built on %s)", versioninfo.Short(), lastCommitDate()),
	Short:   "A no-nonsense backup tool using rsync",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd == profilesCmd || cmd == statusCmd {
			return config.LoadReadOnly()
		}
		// Checking must not create or offer to replace an invalid file.
		if cmd == checkCmd {
			return nil
		}
		// Completion and history do not depend on backup configuration. History
		// filters can also name profiles removed from the configuration.
		if cmd.Name() == "completion" || cmd.Name() == "__complete" || cmd.Name() == "__completeNoDesc" || cmd == usageCmd || cmd.Parent() == usageCmd {
			return nil
		}
		if cmd.Parent().Name() == "config" {
			return config.MustInitConfig(false, false)
		}
		if err := config.MustInitConfig(true, true); err != nil {
			return err
		}
		if err := config.ValidateCompanionPlacement(); err != nil {
			return err
		}
		if cmd.Parent() == runCmd || cmd.Parent() == previewCmd || cmd == mirrorCmd || cmd == cleanBackupCmd {
			if _, err := config.Check(false); err != nil {
				return err
			}
		}
		if !needsProfileResolution(cmd) {
			return nil
		}
		return config.ResolveProfiles()
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
	RootCmd.PersistentFlags().BoolVar(&printer.NoPager, "no-pager", false, "Print output directly instead of opening the pager")
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
	if err := RootCmd.RegisterFlagCompletionFunc("profile", completeProfiles); err != nil {
		panic(err)
	}
}

func lastCommitDate() string {
	return versioninfo.LastCommit.Format("2006-01-02")
}
