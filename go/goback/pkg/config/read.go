package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

func readConfigFile() {
	if _, err := os.Stat(viper.ConfigFileUsed()); err != nil {
		if os.IsNotExist(err) {
			createConfigFileWithConfirmation()
		} else {
			cobra.CheckErr(err)
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		askToRecreateInvalidConfigFile()
	}
}
