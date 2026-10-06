package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/sglavoie/scripts/ccl/config"
	"github.com/sglavoie/scripts/ccl/cycle"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show verbose cycle status",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
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
			os.Exit(1)
		}

		now := time.Now()
		info := cycle.GetCycleInfo(now, workDays, resetWeekday, resetClock)

		cycleStartStr := info.CycleStart.Format("Mon Jan 2 3:04 PM")
		cycleEndStr := info.CycleEnd.Format("Mon Jan 2 3:04 PM")
		nowStr := now.Format("Mon Jan 2 3:04 PM")

		remaining := info.CycleEnd.Sub(now)
		remainingStr := formatDuration(remaining)

		nextWork := findNextWorkEnd(info.NextBoundary, info.IsWorkDay, workDays, info.CycleEnd)
		nextWorkStr := ""
		if !nextWork.IsZero() {
			nextWorkStr = nextWork.Format("Mon Jan 2 3:04 PM")
		} else {
			nextWorkStr = "none remaining this cycle"
		}
		workLabel := "Next work segment ends:"
		if info.IsWorkDay {
			workLabel = "Current work segment ends:"
		}

		fmt.Printf("%-27s %s -> %s\n", "Cycle:", cycleStartStr, cycleEndStr)
		fmt.Printf("%-27s %s\n", "Now:", nowStr)
		fmt.Printf("%-27s %.1f%%\n", "Expected:", info.Expected)
		fmt.Printf("%-27s %d/%d completed\n", "Work days:", info.WorkDaysDone, info.WorkDaysTotal)
		fmt.Printf("%-27s %s\n", workLabel, nextWorkStr)
		fmt.Printf("%-27s %s\n", "Remaining:", remainingStr)
	},
}

// findNextWorkEnd returns the end of the next (or current) work segment.
// If IsWorkDay, returns NextBoundary (end of current work segment).
// Otherwise walks forward until a work segment is found.
func findNextWorkEnd(nextBoundary time.Time, isWorkDay bool, workDays []time.Weekday, cycleEnd time.Time) time.Time {
	if isWorkDay {
		return nextBoundary
	}
	cur := nextBoundary
	for cur.Before(cycleEnd) {
		segEnd := cur.AddDate(0, 0, 1)
		tag := cur.AddDate(0, 0, 1).Weekday()
		for _, wd := range workDays {
			if wd == tag {
				return segEnd
			}
		}
		cur = segEnd
	}
	return time.Time{}
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}
