package config

import (
	"fmt"
	"os"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/inputs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func askToRecreateInvalidFile() {
	creates := inputs.AskYesNoQuestion("config file invalid. Do you want to recreate it?")
	if creates {
		mustCreateDefaultFile()
	}
	os.Exit(0)
}

func createFileWithoutConfirmation() {
	mustCreateDefaultFile()
	os.Exit(0)
}

func mustCreateFileWithConfirmation() {
	creates := inputs.AskYesNoQuestion("config file not found. Do you want to create one?")
	if creates {
		mustCreateDefaultFile()
	}
	os.Exit(0)
}

func mustCreateDefaultFile() {
	_, errFile := os.Create(viper.ConfigFileUsed())
	cobra.CheckErr(errFile)

	setDefaultValues()

	err := viper.WriteConfig()
	cobra.CheckErr(fmt.Sprintf("error writing config file: %s", err))
	fmt.Println("config file created.")
}
