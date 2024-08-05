package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"goback/pkg/internal/inputs"
	"os"
)

func Reset() {
	resets := inputs.AskYesNoQuestion("Are you sure you want to reset the config file to its default values?")
	if !resets {
		os.Exit(0)
	}

	if _, errRead := os.Stat(viper.ConfigFileUsed()); errRead == nil {
		errRemove := os.Remove(viper.ConfigFileUsed())
		if errRemove != nil {
			cobra.CheckErr(errRemove)
		}
	}
	createConfigFileWithoutConfirmation()
}
