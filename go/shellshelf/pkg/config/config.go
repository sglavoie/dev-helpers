package config

import (
	"github.com/mitchellh/go-homedir"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/viper"
)

// Cfg is the configuration for the application.
var Cfg models.Config

func Init() {
	home := getHomeDir()
	cn := ".shellshelf"
	ct := "json"
	cp := home + "/" + cn + "." + ct
	viper.SetConfigName(cn)
	viper.SetConfigType(ct)
	viper.SetConfigPermissions(0600)
	viper.AddConfigPath(home)
	viper.SetConfigFile(cp)

	setDefaultValues()
	Cfg = loadConfig()
}

func SaveAliases(cfg *models.Config) {
	viper.Set("aliases", cfg.Aliases)
	err := viper.WriteConfig()
	if err != nil {
		clihelpers.FatalExit("Error saving aliases: %v", err)
	}
}

func SaveCommands(cfg *models.Config) {
	viper.Set("commands", cfg.Commands)
	err := viper.WriteConfig()
	if err != nil {
		clihelpers.FatalExit("Error saving commands: %v", err)
	}
}

func getHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
	return home
}

// loadConfig loads the configuration from the file.
func loadConfig() models.Config {
	var config models.Config

	err := viper.ReadInConfig()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}

	if err = viper.Unmarshal(&config); err != nil {
		clihelpers.FatalExit(err.Error())
	}

	return config
}

// setDefaultValues sets the default values for the configuration.
func setDefaultValues() {
	viper.SetDefault("aliases", map[string]string{})
	viper.SetDefault("commands", map[string]models.Command{})
	viper.SetDefault("settings.confirmBeforeRun", true)
}
