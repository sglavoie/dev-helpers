package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  "Manage the hr configuration file (init, show, edit, path).",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config if it doesn't exist",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.ConfigPath()
		if err != nil {
			return err
		}
		_, err = config.LoadOrInit(cfgPath)
		if err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}
		fmt.Printf("Config ready: %s\n", cfgPath)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print config file contents",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.ConfigPath()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("config file not found; run 'hr config init' to create it")
			}
			return fmt.Errorf("reading config: %w", err)
		}
		fmt.Print(string(data))
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.ConfigPath()
		if err != nil {
			return err
		}
		// Ensure file exists before opening
		if _, statErr := config.LoadOrInit(cfgPath); statErr != nil {
			return fmt.Errorf("initializing config: %w", statErr)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		c := exec.Command(editor, cfgPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config and data file paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.ConfigPath()
		if err != nil {
			return err
		}
		dataPath, err := config.DataPath()
		if err != nil {
			return err
		}
		fmt.Printf("Config: %s\n", cfgPath)
		fmt.Printf("Data:   %s\n", dataPath)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configPathCmd)
}
