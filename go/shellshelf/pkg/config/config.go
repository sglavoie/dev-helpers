package config

import (
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/viper"
)

var (
	// Cfg is the configuration for the application.
	Cfg *models.Config

	// cfgMutex is a mutex to protect the configuration.
	cfgMutex sync.RWMutex
)

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

// loadConfig loads the configuration from the file.
func loadConfig() *models.Config {
	var config *models.Config

	err := viper.ReadInConfig()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}

	cfgMutex.Lock()
	defer cfgMutex.Unlock()

	if err = viper.Unmarshal(&config); err != nil {
		clihelpers.FatalExit(err.Error())
	}

	return config
}

// LoadConfigAsValue loads the configuration from the file as a value.
func LoadConfigAsValue() (models.Config, error) {
	var config models.Config

	err := viper.ReadInConfig()
	if err != nil {
		return models.Config{}, err
	}

	cfgMutex.Lock()
	defer cfgMutex.Unlock()

	if err = viper.Unmarshal(&config); err != nil {
		return models.Config{}, err
	}

	return config, nil
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

func SaveEncodedCommands(cfg models.Config) error {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()

	cfg.Commands = commands.EncodeAll(cfg.Commands)

	viper.Set("commands", &cfg.Commands)
	err := viper.WriteConfig()
	if err != nil {
		return err
	}
	return nil
}

func getHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
	return home
}

// setDefaultValues sets the default values for the configuration.
func setDefaultValues() {
	viper.SetDefault("aliases", map[string]string{})
	viper.SetDefault("commands", map[string]models.Command{})
	viper.SetDefault("settings.confirmBeforeRun", true)
}
