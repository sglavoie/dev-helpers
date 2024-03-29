package cmd

import (
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/aliases"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/spf13/cobra"
)

// aliasRootCmd represents the alias command
var aliasRootCmd = &cobra.Command{
	Use:     "alias",
	Aliases: []string{"aka"},
	Short:   "Manage aliases for shelved commands",
	Long:    "Manage aliases for shelved commands.",
}

// aliasAddCmd represents the add command for aliases
var aliasAddCmd = &cobra.Command{
	Use:     "add commandID alias",
	Aliases: []string{"a"},
	Short:   "Alias a shelved command",
	Long: `Give an alias to a shelved command.

An alias is mapped to a single command ID and must be unique.
Multiple aliases can be mapped to the same command ID.`,
	Example: "add 1 myAlias",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return aliases.PreRunAdd(args)
	},
	Run: func(cmd *cobra.Command, args []string) {
		aliases.Add(args, config.Cfg)
	},
}

var aliasClearCmd = &cobra.Command{
	Use:     "clear",
	Aliases: []string{"c"},
	Short:   "Clear all aliases",
	Long:    "Clear all aliases. This leaves the shelved commands intact.",
	Example: "clear -f",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		aliases.ClearAliases(cmd, config.Cfg)
	},
}

var aliasFindCmd = &cobra.Command{
	Use:     "find [alias]",
	Aliases: []string{"f"},
	Short:   "Find an alias by name",
	Long:    "Find an alias by name, displaying the associated command.",
	Example: "find myAlias",
	Run: func(cmd *cobra.Command, args []string) {
		flagsPassed := clihelpers.CountSetFlags(cmd)
		if flagsPassed == 0 {
			aliases.FindAlias(args, config.Cfg)
			return
		}

		if aliases.HandleAllFlagReturns(cmd, flagsPassed, args, config.Cfg) {
			return
		}
	},
}

// aliasRemoveCmd represents the remove command for aliases
var aliasRemoveCmd = &cobra.Command{
	Aliases: []string{"r"},
	Use:     "remove [aliases...]",
	Short:   "Remove aliases by name or by command ID",
	Long:    "Remove aliases by name or by associated command IDs using the --id flag.",
	Example: "remove myAlias\nremove myAlias1 myAlias2\nremove --id 1 2 3",
	Run: func(cmd *cobra.Command, args []string) {
		aliases.Remove(cmd, args, config.Cfg)
	},
}

func init() {
	rootCmd.AddCommand(aliasRootCmd)

	// Sub-commands
	aliasRootCmd.AddCommand(aliasAddCmd)
	aliasRootCmd.AddCommand(aliasClearCmd)
	aliasRootCmd.AddCommand(aliasFindCmd)
	aliasRootCmd.AddCommand(aliasRemoveCmd)

	// Local flags
	aliasClearCmd.Flags().BoolP("force", "f", false, "Remove all aliases without confirmation")
	aliasFindCmd.Flags().BoolP("all", "a", false, "Show all aliases, ignoring search terms")
	aliasRemoveCmd.Flags().StringSliceP("id", "i", []string{}, "Remove alias(es) matching by command ID(s)")
}
