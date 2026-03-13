package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/spf13/cobra"
)

var validConfigKeys = []string{"confirmBeforeRun", "editor"}

var configRootCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"},
	Short:   "Manage settings",
	Long:    "View and modify ShellShelf settings.",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all settings with current values",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Cfg
		fmt.Printf("confirmBeforeRun: %v\n", cfg.Settings.ConfirmBeforeRun)
		fmt.Printf("editor: %s\n", cfg.Settings.Editor)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a specific setting's value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		cfg := config.Cfg

		switch key {
		case "confirmBeforeRun":
			fmt.Println(cfg.Settings.ConfirmBeforeRun)
		case "editor":
			fmt.Println(cfg.Settings.Editor)
		default:
			unknownKeyError(key)
		}
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a specific setting",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]
		cfg := config.Cfg

		switch key {
		case "confirmBeforeRun":
			lower := strings.ToLower(value)
			if lower != "true" && lower != "false" {
				clihelpers.FatalExit("Invalid value %q for %s. Must be true or false.", value, key)
			}
			cfg.Settings.ConfirmBeforeRun = lower == "true"
		case "editor":
			cfg.Settings.Editor = value
		default:
			unknownKeyError(key)
		}

		config.SaveSettings(cfg)
		fmt.Printf("Set %s to %s\n", key, value)
	},
}

func unknownKeyError(key string) {
	clihelpers.FatalExit("Unknown key %q. Valid keys: %s", key, strings.Join(validConfigKeys, ", "))
}

func init() {
	rootCmd.AddCommand(configRootCmd)

	configRootCmd.AddCommand(configListCmd)
	configRootCmd.AddCommand(configGetCmd)
	configRootCmd.AddCommand(configSetCmd)
}
