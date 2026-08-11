package run

import (
	"fmt"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

// Report collects every command of the current run so the outcome of the main
// backups and of their companions can be shown together once everything has
// finished.
type Report struct {
	Mains      []MainResult
	Companions []CompanionResult
}

// Print renders the results of the current run as one table. It prints
// nothing when the run produced no result at all.
func (r *Report) Print(w io.Writer) {
	if len(r.Mains) == 0 && len(r.Companions) == 0 {
		return
	}

	t := table.NewWriter()
	t.SetAllowedRowLength(120)
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleColoredYellowWhiteOnBlack)
	t.AppendHeader(table.Row{"Step", "Result", "Duration", "Details"})

	for _, main := range r.Mains {
		t.AppendRow([]any{main.BackupType, main.Status.String(), duration(main.Duration), mainDetails(main)})
	}
	for _, companion := range r.Companions {
		name := "companion/" + companion.Companion.ID
		t.AppendRow([]any{name, companion.Status(), duration(companion.Duration), companionDetails(companion)})
	}

	fmt.Fprintln(w)
	t.Render()
}

func mainDetails(main MainResult) string {
	switch {
	case main.Status == MainFailed:
		return fmt.Sprintf("exit code %d", main.ExitCode)
	case main.Err != nil:
		return main.Err.Error()
	default:
		return ""
	}
}

func companionDetails(companion CompanionResult) string {
	switch {
	case !companion.Started:
		return companion.Err.Error()
	case !companion.Interrupted && companion.ExitCode != 0:
		return fmt.Sprintf("exit code %d: %s", companion.ExitCode, companion.CommandString())
	default:
		return companion.CommandString()
	}
}

func duration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return d.Round(time.Millisecond).String()
}
