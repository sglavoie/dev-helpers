package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var CfgFile string

type rsyncFlags struct {
	archive        bool
	delete         bool
	deleteExcluded bool
	dryRun         bool
	force          bool
	hardLinks      bool
	ignoreErrors   bool
	pruneEmptyDirs bool
	verbose        bool
}

type config struct {
	rsyncFlags rsyncFlags
}

type cliConfig struct {
	configExtension string
}

var cfg config
var cliCfg cliConfig

// InitConfig reads in config file.
func InitConfig(recreateInvalid bool) {
	setCliCfg()
	setViperCfg()
	readConfigFile()

	if recreateInvalid {
		recreateInvalidConfigFile()
	}

	cfg.Unmarshal()
	err := cfg.Validate()
	if err != nil {
		cobra.CheckErr(err)
	}
}

func (c *config) Unmarshal() {
	err := viper.Unmarshal(&c)
	if err != nil {
		cobra.CheckErr(err)
	}
}

func (c *config) Validate() error {
	// TODO
	return nil
}

func recreateInvalidConfigFile() {
	if err := viper.ReadInConfig(); err != nil {
		askToRecreateInvalidConfigFile()
	}
}

func setCliCfg() {
	cliCfg.configExtension = ".json"
}

func setViperCfg() {
	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".goback" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("json")
		viper.SetConfigName(".goback")
		viper.SetConfigFile(home + "/.goback.json")
	}
}
