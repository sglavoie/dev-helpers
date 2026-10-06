package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/scripts/ccl/config"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Update configuration values",
}

var setDaysCmd = &cobra.Command{
	Use:   "days <mon,tue,...>",
	Short: "Set work days (comma-separated)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		parts := strings.Split(args[0], ",")
		days := make([]string, 0, len(parts))
		for _, p := range parts {
			days = append(days, strings.ToLower(strings.TrimSpace(p)))
		}

		if err := config.ValidateDays(days); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		cfg.WorkDays = days
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Work days updated: %s\n", strings.Join(days, ", "))
	},
}

var setResetCmd = &cobra.Command{
	Use:   "reset <\"Thu 2:59 PM\">",
	Short: "Set cycle reset time",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.ValidateResetTime(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		cfg.ResetTime = args[0]
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Reset time updated: %s\n", args[0])
	},
}
