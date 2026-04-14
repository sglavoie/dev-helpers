package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
	"github.com/sglavoie/dev-helpers/go/hr/internal/storage"
	"github.com/spf13/cobra"
)

var (
	logLast     int
	logToday    bool
	logExercise string
	logAll      bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View exercise log",
	Long:  "Display recent exercise log entries with optional filtering.",
	RunE:  runLog,
}

func init() {
	logCmd.Flags().IntVarP(&logLast, "last", "n", 10, "number of entries to show")
	logCmd.Flags().BoolVarP(&logToday, "today", "t", false, "show only today's entries")
	logCmd.Flags().StringVarP(&logExercise, "exercise", "e", "", "filter by exercise name")
	logCmd.Flags().BoolVarP(&logAll, "all", "a", false, "show all entries")
}

func runLog(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.LoadOrInit(cfgPath)
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

	entries = filterEntries(entries, logToday, logExercise)

	if !logAll {
		n := logLast
		if n < len(entries) {
			entries = entries[len(entries)-n:]
		}
	}

	if len(entries) == 0 {
		fmt.Println("No entries found.")
		return nil
	}

	fmt.Println(renderEntryTable(entries, cfg.DateFormat))
	return nil
}

// renderEntryTable renders a go-pretty table string for the given entries,
// formatting each timestamp with dateFormat.
func renderEntryTable(entries []storage.Entry, dateFormat string) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Timestamp", "Exercise", "Reps", "Rounds", "Notes"})
	for _, e := range entries {
		t.AppendRow(table.Row{
			e.Timestamp.Local().Format(dateFormat),
			e.Exercise,
			e.Reps,
			e.Rounds,
			e.Notes,
		})
	}
	return t.Render()
}

func filterEntries(entries []storage.Entry, todayOnly bool, exercise string) []storage.Entry {
	if !todayOnly && exercise == "" {
		return entries
	}
	now := time.Now()
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	filtered := entries[:0:0]
	for _, e := range entries {
		if todayOnly {
			local := e.Timestamp.Local()
			entryDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
			if !entryDate.Equal(todayDate) {
				continue
			}
		}
		if exercise != "" && !strings.EqualFold(e.Exercise, exercise) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}
