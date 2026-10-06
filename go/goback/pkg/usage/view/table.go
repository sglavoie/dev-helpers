package view

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/cobra"
)

func SqlToText(rows *sql.Rows) {
	t := table.NewWriter()
	setTableProperties(t)
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
	t.AppendHeader(table.Row{"Profile", "Backup type", "Last successful backup", "Latest attempt", "Result"})

	if !appendSummaryRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	t.Render()
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
			success = lastSuccess.String
		}
		result := "succeeded"
		if exitCode == -1 {
			result = "interrupted"
		} else if exitCode != 0 {
			result = fmt.Sprintf("failed (exit %d)", exitCode)
		}
		t.AppendRow(table.Row{profile, backupType, success, latestAttempt, result})
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
	t.SetAllowedRowLength(120)
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredYellowWhiteOnBlack)
}
