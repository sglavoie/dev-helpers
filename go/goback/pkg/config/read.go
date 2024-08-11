package config

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func mustReadFile() {
	if _, err := os.Stat(viper.ConfigFileUsed()); err != nil {
		if os.IsNotExist(err) {
			mustCreateFileWithConfirmation()
		} else {
			cobra.CheckErr(err)
		}
	}
}
