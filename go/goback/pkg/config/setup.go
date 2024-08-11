package config

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type rsyncSupportedFlags struct {
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

type rsyncFlagsDaily struct {
	flags rsyncSupportedFlags
}

type rsyncFlagsWeekly struct {
	flags rsyncSupportedFlags
}

type rsync struct {
	daily  rsyncFlagsDaily
	weekly rsyncFlagsWeekly
}

type Config struct {
	confirmExec      bool
	excludedPatterns []string
	rsync            rsync
	srcDest          map[string]string
}

func (c *Config) Unmarshal() {
	err := viper.Unmarshal(&c)
	if err != nil {
		cobra.CheckErr(err)
	}
}

type CliConfig struct {
	ConfigExtension string
}

var CfgFile string

var cfg Config
var cliCfg CliConfig

// MustInitConfig reads in config file.
func MustInitConfig(recreateInvalid bool, readConfig bool) {
	setCliCfg()
	setViperCfg()
	mustReadFile()

	if recreateInvalid {
		recreateInvalidFile()
	} else if readConfig {
		err := viper.ReadInConfig()
		if err != nil {
			cobra.CheckErr(err)
		}
	}

	cfg.Unmarshal()
}

func recreateInvalidFile() {
	if err := viper.ReadInConfig(); err != nil {
		askToRecreateInvalidFile()
	}
}

func setCliCfg() {
	cliCfg.ConfigExtension = ".json"
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
