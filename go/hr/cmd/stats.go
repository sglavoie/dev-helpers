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
	statsWeek     bool
	statsMonth    bool
	statsExercise string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show exercise statistics",
	Long:  "Aggregate exercise statistics for today, this week, or this month.",
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().BoolVarP(&statsWeek, "week", "w", false, "show this week's stats")
	statsCmd.Flags().BoolVarP(&statsMonth, "month", "m", false, "show this month's stats")
	statsCmd.Flags().StringVarP(&statsExercise, "exercise", "e", "", "filter by exercise name")
}

// ExerciseSummary holds aggregated totals for a single exercise.
type ExerciseSummary struct {
	Exercise    string
	TotalReps   int
	TotalRounds int
}

// aggregateStats returns per-exercise totals for the given entries, preserving
// order of first appearance.
func aggregateStats(entries []storage.Entry) []ExerciseSummary {
	seen := make(map[string]int)
	var summaries []ExerciseSummary
	for _, e := range entries {
		if idx, ok := seen[e.Exercise]; ok {
			summaries[idx].TotalReps += e.Reps * e.Rounds
			summaries[idx].TotalRounds += e.Rounds
		} else {
			seen[e.Exercise] = len(summaries)
			summaries = append(summaries, ExerciseSummary{
				Exercise:    e.Exercise,
				TotalReps:   e.Reps * e.Rounds,
				TotalRounds: e.Rounds,
			})
		}
	}
	return summaries
}

// renderStatsTable renders a go-pretty table string for the given summaries.
func renderStatsTable(summaries []ExerciseSummary) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Exercise", "Total Reps", "Total Rounds"})
	totalReps := 0
	for _, s := range summaries {
		t.AppendRow(table.Row{s.Exercise, s.TotalReps, s.TotalRounds})
		totalReps += s.TotalReps
	}
	t.AppendSeparator()
	t.AppendRow(table.Row{"Total", totalReps, fmt.Sprintf("%d exercise(s)", len(summaries))})
	return t.Render()
}

func runStats(cmd *cobra.Command, args []string) error {
	dataPath, err := config.DataPath()
	if err != nil {
		return err
	}
	entries, err := storage.ReadAll(dataPath)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}

	now := time.Now()
	var periodLabel string
	var start time.Time

	switch {
	case statsMonth:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodLabel = fmt.Sprintf("This month (%s %d)", now.Month(), now.Year())
	case statsWeek:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		periodLabel = fmt.Sprintf("This week (since %s)", start.Format("2006-01-02"))
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		periodLabel = fmt.Sprintf("Today (%s)", now.Format("2006-01-02"))
	}

	// Filter by time period
	var filtered []storage.Entry
	for _, e := range entries {
		local := e.Timestamp.Local()
		if !local.Before(start) {
			filtered = append(filtered, e)
		}
	}

	// Filter by exercise
	if statsExercise != "" {
		var byEx []storage.Entry
		for _, e := range filtered {
			if strings.EqualFold(e.Exercise, statsExercise) {
				byEx = append(byEx, e)
			}
		}
		filtered = byEx
	}

	if len(filtered) == 0 {
		fmt.Println("No entries found.")
		return nil
	}

	summaries := aggregateStats(filtered)
	fmt.Printf("%s:\n", periodLabel)
	fmt.Println(renderStatsTable(summaries))
	return nil
}
