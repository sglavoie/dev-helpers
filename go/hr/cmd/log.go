package cmd

import (
	"fmt"
	"sort"
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

	fmt.Println(renderEntryTable(entries, cfg.DateFormat, cfg))
	return nil
}

// renderEntryTable renders a go-pretty table string for the given entries,
// formatting each timestamp with dateFormat and looking up exercise definitions
// in cfg for human-friendly details.
func renderEntryTable(entries []storage.Entry, dateFormat string, cfg config.Config) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Timestamp", "Exercise", "Details", "Notes"})
	for _, e := range entries {
		ex, _ := findExercise(cfg.Exercises, e.Exercise)
		t.AppendRow(table.Row{
			e.Timestamp.Local().Format(dateFormat),
			e.Exercise,
			formatEntryDetails(e.Data, ex),
			e.Notes,
		})
	}
	return t.Render()
}

// formatEntryDetails renders a human-friendly string for an entry's data.
// If exercise has no fields (unknown), it falls back to sorted key: value pairs.
func formatEntryDetails(data map[string]any, exercise config.Exercise) string {
	if len(exercise.Fields) == 0 {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %v", k, formatValue(data[k])))
		}
		return strings.Join(parts, ", ")
	}

	primary := exercise.PrimaryField()
	if primary != nil && primary.MultipliedBy != "" {
		primaryVal := getFloat64FromMap(data, primary.Name)
		multiplierVal := getFloat64FromMap(data, primary.MultipliedBy)
		return fmt.Sprintf("%s %s x %s", formatFloat(primaryVal), primary.Name, formatFloat(multiplierVal))
	}

	// Otherwise: field order from config
	parts := make([]string, 0, len(exercise.Fields))
	for _, f := range exercise.Fields {
		val, ok := data[f.Name]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", f.Name, formatValue(val)))
	}
	return strings.Join(parts, ", ")
}

// getFloat64FromMap returns the float64 value for key in data, or 0 if missing/wrong type.
func getFloat64FromMap(data map[string]any, key string) float64 {
	if v, ok := data[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

// formatFloat displays integers without decimals, floats with necessary precision.
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// formatValue formats any data value for display.
func formatValue(v any) string {
	if f, ok := v.(float64); ok {
		return formatFloat(f)
	}
	return fmt.Sprintf("%v", v)
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
