package view

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mattn/go-isatty"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/rsyncstatus"
	"github.com/spf13/cobra"
)

func SqlToText(rows *sql.Rows) {
	t := table.NewWriter()
	setTableProperties(t)
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 4, WidthMax: 18}, {Number: 7, WidthMax: 60}})
	t.AppendHeader(table.Row{"ID", "Created at", "Backup type", "Profile", "Execution time", "Exit code", "Command executed"})

	if !appendRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	printer.Pager(t.Render(), "Latest backups")
}

func SqlToTextSummary(rows *sql.Rows) {
	t := table.NewWriter()
	setTableProperties(t)
	t.SetColumnConfigs([]table.ColumnConfig{{Number: 1, WidthMax: 16}, {Number: 2, WidthMax: 18}, {Number: 3, WidthMax: 20}, {Number: 4, WidthMax: 20}, {Number: 5, WidthMax: 28}})
	t.AppendHeader(table.Row{"Profile", "Backup type", "Last successful backup", "Latest attempt", "Result"})

	if !appendSummaryRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	fmt.Fprintln(os.Stdout, t.Render())
}

func appendRows(rows *sql.Rows, t table.Writer) (hasData bool) {
	for rows.Next() {
		var id, exitCode int
		var createdAt, backupType, execTime, command, profile string
		err := rows.Scan(&id, &createdAt, &backupType, &execTime, &command, &profile, &exitCode)
		cobra.CheckErr(err)
		execTimeTruncated := executionTime(execTime)
		cmd := wrappedCommand(command)
		t.AppendRow([]interface{}{id, createdAt, backupType, profile, execTimeTruncated, exitCode, cmd})
		t.AppendSeparator()
		hasData = true
	}
	return
}

func appendSummaryRows(rows *sql.Rows, t table.Writer) (hasData bool) {
	for rows.Next() {
		var profile, backupType, latestAttempt string
		var exitCode int
		var lastSuccess sql.NullString
		err := rows.Scan(&profile, &backupType, &latestAttempt, &exitCode, &lastSuccess)
		cobra.CheckErr(err)
		success := "Never recorded"
		if lastSuccess.Valid {
			success = printer.TimestampWithAge(lastSuccess.String)
		}
		result := "succeeded"
		if exitCode == -1 {
			result = "interrupted"
		} else if exitCode != 0 {
			result = fmt.Sprintf("failed (exit %d)", exitCode)
			if !strings.HasPrefix(backupType, "companion/") {
				if explanation := rsyncstatus.Explanation(exitCode); explanation != "" {
					result += ": " + explanation
				}
			}
		}
		t.AppendRow(table.Row{profile, backupType, success, printer.TimestampWithAge(latestAttempt), result})
		t.AppendSeparator()
		hasData = true
	}
	cobra.CheckErr(rows.Err())
	return
}

func executionTime(executionTime string) string {
	return printer.TruncateExecTimeToNearest(executionTime, 2)
}

func wrappedCommand(cmd string) string {
	sb := &strings.Builder{}
	sb.WriteString(cmd)
	printer.WrapLongLinesWithBackslashes(sb, 60)
	return sb.String()
}

func setTableProperties(t table.Writer) {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		t.SetStyle(table.StyleColoredYellowWhiteOnBlack)
	}
}
