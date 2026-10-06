package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/scripts/ccl/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Work days:   %s\n", strings.Join(cfg.WorkDays, ", "))
		fmt.Printf("Reset time:  %s\n", cfg.ResetTime)
		fmt.Printf("Config file: %s\n", config.ConfigPath())
	},
}
