package cmd

import (
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
	"github.com/sglavoie/dev-helpers/go/hr/internal/storage"
	"github.com/spf13/cobra"
)

var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show today's dashboard",
	Long:  "Display today's entries, exercise summary, and current streak.",
	RunE:  runToday,
}

func runToday(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	_, err = config.LoadOrInit(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dataPath, err := config.DataPath()
	if err != nil {
		return err
	}
	entries, err := storage.ReadAll(dataPath)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var todayEntries []storage.Entry
	for _, e := range entries {
		local := e.Timestamp.Local()
		entryDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
		if entryDay.Equal(todayStart) {
			todayEntries = append(todayEntries, e)
		}
	}

	if len(todayEntries) == 0 {
		fmt.Println("No entries today.")
		return nil
	}

	fmt.Printf("Today \u2014 %s\n\n", now.Format("2006-01-02"))
	fmt.Println(renderEntryTable(todayEntries, "15:04"))

	summaries := aggregateStats(todayEntries)
	fmt.Println("\nSummary")
	fmt.Println(renderStatsTable(summaries))

	days, err := storage.ActiveDays(dataPath)
	if err != nil {
		return fmt.Errorf("reading active days: %w", err)
	}
	count, _, _ := currentStreak(days)
	fmt.Printf("\nCurrent streak: %d %s\n", count, dayWord(count))
	return nil
}
