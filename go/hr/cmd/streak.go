package cmd

import (
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
	"github.com/sglavoie/dev-helpers/go/hr/internal/storage"
	"github.com/spf13/cobra"
)

var streakCmd = &cobra.Command{
	Use:   "streak",
	Short: "Show exercise streak statistics",
	Long:  "Display current streak, longest streak, and total active days.",
	RunE:  runStreak,
}

func runStreak(cmd *cobra.Command, args []string) error {
	dataPath, err := config.DataPath()
	if err != nil {
		return err
	}
	days, err := storage.ActiveDays(dataPath)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}
	if len(days) == 0 {
		fmt.Println("No entries yet.")
		return nil
	}

	current, currentStart, currentEnd := currentStreak(days)
	longest, longestStart, longestEnd := longestStreak(days)

	fmt.Printf("Current streak:    %s\n", formatStreakLine(current, currentStart, currentEnd))
	fmt.Printf("Longest streak:    %s\n", formatStreakLine(longest, longestStart, longestEnd))
	fmt.Printf("Total active days: %d\n", len(days))
	return nil
}

// currentStreak counts consecutive days ending today (or yesterday if no entry
// today). Returns the streak count and the start/end dates of that streak.
func currentStreak(days []time.Time) (count int, start, end time.Time) {
	today := truncateDay(time.Now().Local())
	// Walk backwards from today
	cursor := today
	for i := len(days) - 1; i >= 0; i-- {
		d := truncateDay(days[i])
		if d.Equal(cursor) {
			count++
			end = d
			cursor = cursor.AddDate(0, 0, -1)
		} else if d.Before(cursor) {
			break
		}
	}
	if count == 0 {
		return 0, time.Time{}, time.Time{}
	}
	start = cursor.AddDate(0, 0, 1)
	return count, start, end
}

// longestStreak finds the longest run of consecutive days in the slice.
// days must be sorted ascending.
func longestStreak(days []time.Time) (count int, start, end time.Time) {
	if len(days) == 0 {
		return 0, time.Time{}, time.Time{}
	}
	best := 1
	bestStart := truncateDay(days[0])
	bestEnd := bestStart

	run := 1
	runStart := truncateDay(days[0])

	for i := 1; i < len(days); i++ {
		prev := truncateDay(days[i-1])
		curr := truncateDay(days[i])
		if curr.Equal(prev.AddDate(0, 0, 1)) {
			run++
			if run > best {
				best = run
				bestStart = runStart
				bestEnd = curr
			}
		} else {
			run = 1
			runStart = curr
		}
	}
	return best, bestStart, bestEnd
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func dayWord(n int) string {
	if n == 1 {
		return "day"
	}
	return "days"
}

func formatStreakLine(count int, start, end time.Time) string {
	if count == 0 {
		return fmt.Sprintf("%d days", count)
	}
	return fmt.Sprintf("%d %s (%s \u2014 %s)",
		count, dayWord(count),
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
}
