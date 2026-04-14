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
	Exercise     string
	Category     string
	PrimaryLabel string
	PrimaryTotal float64
	EntryCount   int
}

// aggregateStats returns per-exercise summaries for the given entries,
// preserving order of first appearance and computing primary field totals.
func aggregateStats(entries []storage.Entry, cfg config.Config) []ExerciseSummary {
	seen := make(map[string]int)
	var summaries []ExerciseSummary
	for _, e := range entries {
		ex, err := findExercise(cfg.Exercises, e.Exercise)
		found := err == nil
		if idx, ok := seen[e.Exercise]; ok {
			summaries[idx].EntryCount++
			if found {
				summaries[idx].PrimaryTotal += primaryValue(e.Data, ex)
			}
		} else {
			s := ExerciseSummary{
				Exercise:   e.Exercise,
				EntryCount: 1,
			}
			if found {
				s.Category = ex.Category
				if p := ex.PrimaryField(); p != nil {
					s.PrimaryLabel = p.Name
					s.PrimaryTotal = primaryValue(e.Data, ex)
				}
			}
			seen[e.Exercise] = len(summaries)
			summaries = append(summaries, s)
		}
	}
	return summaries
}

// primaryValue returns the primary field value for an entry, multiplied if configured.
func primaryValue(data map[string]any, ex config.Exercise) float64 {
	p := ex.PrimaryField()
	if p == nil {
		return 0
	}
	val := getFloat64FromMap(data, p.Name)
	if p.MultipliedBy != "" {
		val *= getFloat64FromMap(data, p.MultipliedBy)
	}
	return val
}

// renderStatsTable renders category-grouped tables for the given summaries.
func renderStatsTable(summaries []ExerciseSummary) string {
	type categoryGroup struct {
		name      string
		summaries []ExerciseSummary
	}

	var groups []categoryGroup
	seenCat := make(map[string]int)
	var unknowns []ExerciseSummary

	for _, s := range summaries {
		if s.Category == "" {
			unknowns = append(unknowns, s)
			continue
		}
		if idx, ok := seenCat[s.Category]; ok {
			groups[idx].summaries = append(groups[idx].summaries, s)
		} else {
			seenCat[s.Category] = len(groups)
			groups = append(groups, categoryGroup{name: s.Category, summaries: []ExerciseSummary{s}})
		}
	}

	var sb strings.Builder

	for i, g := range groups {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Check if all exercises in category share the same primary label
		allSame := true
		label := g.summaries[0].PrimaryLabel
		for _, s := range g.summaries[1:] {
			if s.PrimaryLabel != label {
				allSame = false
				break
			}
		}

		t := table.NewWriter()
		t.SetStyle(table.StyleLight)
		t.SetTitle(g.name)

		if allSame && label != "" {
			t.AppendHeader(table.Row{"Exercise", capitalize(label), "Entries"})
			total := 0.0
			totalEntries := 0
			for _, s := range g.summaries {
				t.AppendRow(table.Row{s.Exercise, formatFloat(s.PrimaryTotal), s.EntryCount})
				total += s.PrimaryTotal
				totalEntries += s.EntryCount
			}
			t.AppendSeparator()
			t.AppendRow(table.Row{"Total", formatFloat(total), totalEntries})
		} else {
			t.AppendHeader(table.Row{"Exercise", "Total", "Entries"})
			totalEntries := 0
			for _, s := range g.summaries {
				totalStr := ""
				if s.PrimaryLabel != "" {
					totalStr = fmt.Sprintf("%s %s", formatFloat(s.PrimaryTotal), s.PrimaryLabel)
				}
				t.AppendRow(table.Row{s.Exercise, totalStr, s.EntryCount})
				totalEntries += s.EntryCount
			}
			t.AppendSeparator()
			t.AppendRow(table.Row{"Entries", "", totalEntries})
		}

		sb.WriteString(t.Render())
		sb.WriteString("\n")
	}

	if len(unknowns) > 0 {
		if len(groups) > 0 {
			sb.WriteString("\n")
		}
		t := table.NewWriter()
		t.SetStyle(table.StyleLight)
		t.SetTitle("Unknown")
		t.AppendHeader(table.Row{"Exercise", "Entries"})
		total := 0
		for _, s := range unknowns {
			t.AppendRow(table.Row{s.Exercise, s.EntryCount})
			total += s.EntryCount
		}
		t.AppendSeparator()
		t.AppendRow(table.Row{"Total", total})
		sb.WriteString(t.Render())
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// capitalize uppercases the first letter of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func runStats(cmd *cobra.Command, args []string) error {
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

	summaries := aggregateStats(filtered, cfg)
	fmt.Printf("%s:\n", periodLabel)
	fmt.Println(renderStatsTable(summaries))
	return nil
}
