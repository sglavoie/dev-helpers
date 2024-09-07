package view

import (
	"database/sql"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
)

func SqlToText(rows *sql.Rows) {
	t := table.NewWriter()
	setTableProperties(t)
	t.AppendHeader(table.Row{"ID", "Created at", "Backup type", "Execution time", "Command executed"})

	if !appendRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	printer.Pager(t.Render(), "Latest backups")
}

func SqlToTextSummary(rows *sql.Rows) {
	t := table.NewWriter()
	setTableProperties(t)
	t.AppendHeader(table.Row{"ID", "Created at", "Relative time", "Backup type"})

	if !appendSummaryRows(rows, t) {
		fmt.Println("No backups data found")
		return
	}
	t.Render()
}

func appendRows(rows *sql.Rows, t table.Writer) (hasData bool) {
	for rows.Next() {
		var id int
		var createdAt, backupType, executionTime, command string
		err := rows.Scan(&id, &createdAt, &backupType, &executionTime, &command)
		cobra.CheckErr(err)
		execTime := getExecutionTime(executionTime)
		cmd := getWrappedCommand(command)
		t.AppendRow([]interface{}{id, createdAt, backupType, execTime, cmd})
		t.AppendSeparator()
		hasData = true
	}
	return
}

func appendSummaryRows(rows *sql.Rows, t table.Writer) (hasData bool) {
	for rows.Next() {
		var id int
		var createdAt, backupType, executionTime, command string
		err := rows.Scan(&id, &createdAt, &backupType, &executionTime, &command)
		cobra.CheckErr(err)

		relTime := printer.GetRelativeTime(createdAt)
		t.AppendRow([]interface{}{id, createdAt, relTime, backupType})
		t.AppendSeparator()
		hasData = true
	}
	return
}

func getExecutionTime(executionTime string) string {
	return printer.TruncateExecTimeToNearest(executionTime, 2)
}

func getWrappedCommand(cmd string) string {
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
