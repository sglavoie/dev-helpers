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
	t.AppendHeader(table.Row{"ID", "Created at", "Backup type", "Profile", "Execution time", "Command executed"})

	if !appendRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	printer.Pager(t.Render(), "Latest backups")
}

func SqlToTextSummary(rows *sql.Rows) {
	t := table.NewWriter()
	setTableProperties(t)
	t.AppendHeader(table.Row{"ID", "Created at", "Relative time", "Backup type", "Profile"})

	if !appendSummaryRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	t.Render()
}

func appendRows(rows *sql.Rows, t table.Writer) (hasData bool) {
	for rows.Next() {
		var id int
		var createdAt, backupType, execTime, command, profile string
		err := rows.Scan(&id, &createdAt, &backupType, &execTime, &command, &profile)
		cobra.CheckErr(err)
		execTimeTruncated := executionTime(execTime)
		cmd := wrappedCommand(command)
		t.AppendRow([]interface{}{id, createdAt, backupType, profile, execTimeTruncated, cmd})
		t.AppendSeparator()
		hasData = true
	}
	return
}

func appendSummaryRows(rows *sql.Rows, t table.Writer) (hasData bool) {
	for rows.Next() {
		var id int
		var createdAt, backupType, executionTime, command, profile string
		err := rows.Scan(&id, &createdAt, &backupType, &executionTime, &command, &profile)
		cobra.CheckErr(err)

		relTime := printer.RelativeTime(createdAt)
		t.AppendRow([]interface{}{id, createdAt, relTime, backupType, profile})
		t.AppendSeparator()
		hasData = true
	}
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
