package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/sglavoie/scripts/ccl/config"
	"github.com/sglavoie/scripts/ccl/cycle"
	"github.com/spf13/cobra"
)

var dayNameMap = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

func parseWorkDays(days []string) []time.Weekday {
	out := make([]time.Weekday, 0, len(days))
	for _, d := range days {
		if wd, ok := dayNameMap[d]; ok {
			out = append(out, wd)
		}
	}
	return out
}

var shortFlag bool

var rootCmd = &cobra.Command{
	Use:   "ccl",
	Short: "Claude Code Limits CLI — track expected usage across the week",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			fmt.Fprintln(os.Stderr, "Try: ccl set days mon,tue,wed,thu,fri,sat")
			os.Exit(1)
		}

		workDays := parseWorkDays(cfg.WorkDays)
		if len(workDays) == 0 {
			fmt.Fprintln(os.Stderr, "No valid work days configured. Try: ccl set days mon,tue,wed,thu,fri")
			os.Exit(1)
		}

		resetWeekday, resetClock, err := cycle.ParseResetTime(cfg.ResetTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid reset time in config: %v\n", err)
			fmt.Fprintln(os.Stderr, "Try: ccl set reset \"Thu 2:59 PM\"")
			os.Exit(1)
		}

		pct := cycle.ExpectedUsage(time.Now(), workDays, resetWeekday, resetClock)
		if shortFlag {
			fmt.Printf("%.1f", pct)
		} else {
			fmt.Printf("Expected usage: %.1f%%\n", pct)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&shortFlag, "short", false, "Output compact XX.X format for statusline integration")
	rootCmd.AddCommand(setCmd)
	setCmd.AddCommand(setDaysCmd, setResetCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(statusCmd)
}
