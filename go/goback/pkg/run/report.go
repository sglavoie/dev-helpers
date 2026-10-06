package run

import (
	"fmt"
	"io"

	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mattn/go-isatty"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/rsyncstatus"
)

// Report collects every command of the current run so the outcome of the main
// backups and of their companions can be shown together once everything has
// finished.
type Report struct {
	Mains      []MainResult
	Companions []CompanionResult
}

// CompletedBackup reports whether every requested main transfer and companion
// succeeded and every main transfer was real. Declines and dry runs do not eject.
func (r *Report) CompletedBackup() bool {
	if len(r.Mains) == 0 {
		return false
	}
	for _, main := range r.Mains {
		if main.Status != MainSucceeded || main.DryRun {
			return false
		}
	}
	for _, companion := range r.Companions {
		if !companion.Succeeded() {
			return false
		}
	}
	return true
}

// Print renders the results of the current run as one table. It prints
// nothing when the run produced no result at all.
func (r *Report) Print(w io.Writer) {
	if len(r.Mains) == 0 && len(r.Companions) == 0 {
		return
	}

	t := table.NewWriter()
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 1, WidthMax: 24}, {Number: 2, WidthMax: 20}, {Number: 3, WidthMax: 12}, {Number: 4, WidthMax: 48}})
	t.SetOutputMirror(w)
	if terminal, ok := w.(interface{ Fd() uintptr }); ok && isatty.IsTerminal(terminal.Fd()) {
		t.SetStyle(table.StyleColoredYellowWhiteOnBlack)
	}
	t.AppendHeader(table.Row{"Step", "Result", "Duration", "Details"})

	for _, main := range r.Mains {
		status := main.Status.String()
		if main.DryRun {
			status = "dry run: " + status
		}
		t.AppendRow([]any{main.BackupType, status, duration(main.Duration), mainDetails(main)})
	}
	for _, companion := range r.Companions {
		name := "companion/" + companion.Companion.ID
		status := companion.Status()
		if companion.DryRun {
			status = "dry run: " + status
		}
		t.AppendRow([]any{name, status, duration(companion.Duration), companionDetails(companion)})
	}

	fmt.Fprintln(w)
	t.Render()
	for _, main := range r.Mains {
		if main.DiagnosticLog != "" {
			fmt.Fprintf(w, "%s diagnostic log: %s\n", main.BackupType, main.DiagnosticLog)
		}
	}
}

func mainDetails(main MainResult) string {
	switch {
	case main.SkipReason != "":
		return main.SkipReason
	case main.Status == MainFailed:
		detail := fmt.Sprintf("exit code %d", main.ExitCode)
		if explanation := rsyncstatus.Explanation(main.ExitCode); explanation != "" {
			detail += ": " + explanation
		}
		return detail
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
