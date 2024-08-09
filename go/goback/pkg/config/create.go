package config

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"goback/pkg/inputs"
	"os"
)

func askToRecreateInvalidConfigFile() {
	creates := inputs.AskYesNoQuestion("config file invalid. Do you want to recreate it?")
	if creates {
		createDefaultConfigFile()
	}
	os.Exit(0)
}

func createConfigFileWithoutConfirmation() {
	createDefaultConfigFile()
	os.Exit(0)
}

func createConfigFileWithConfirmation() {
	creates := inputs.AskYesNoQuestion("config file not found. Do you want to create one?")
	if creates {
		createDefaultConfigFile()
	}
	os.Exit(0)
}

func createDefaultConfigFile() {
	_, errFile := os.Create(viper.ConfigFileUsed())
	if errFile != nil {
		cobra.CheckErr(errFile)
	}

	setDefaultValues()

	err := viper.WriteConfig()
	if err != nil {
		fmt.Println("Error writing config file:", err)
		cobra.CheckErr(err)
	}
	fmt.Println("config file created.")
}
