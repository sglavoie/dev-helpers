package cmd

import (
	"errors"
	"fmt"

	"github.com/mitchellh/go-homedir"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cfgFile is a global variable that will hold the path to the configuration file
var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ss",
	Short: "ShellShelf shelves your shell commands",
	Long: `Keep your favorite commands on a shelf.

Manage your shell commands with ShellShelf.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//Run: func(cmd *cobra.Command, args []string) { },
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate the autocompletion script for the specified shell",
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.shellshelf.json)")

	// Local flags, which will only run when this command is called directly
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	completion := completionCmd()
	completion.Hidden = true
	rootCmd.AddCommand(completion)
}

func initConfig() {
	var cp string
	if cfgFile != "" {
		cp = cfgFile // TODO: Check if file exists...
	} else {
		home := getHomeDir()
		cn := ".shellshelf"
		ct := "json"
		cp = home + "/" + cn + "." + ct
		viper.SetConfigName(cn)
		viper.SetConfigType(ct)
		viper.SetConfigPermissions(0600)
		viper.AddConfigPath(home)
	}

	readConfig(cp)
}

func getHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
	return home
}

func readConfig(cp string) {
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			err = viper.WriteConfigAs(cp)
			if err != nil {
				clihelpers.FatalExit(err.Error())
			}

			fmt.Printf("Configuration file created at: %s\n", cp)
		} else {
			clihelpers.FatalExit(err.Error())
		}
	}
	viper.SetConfigFile(cp)
}
