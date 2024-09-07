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
	t.SetAllowedRowLength(120)
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredYellowWhiteOnBlack)
	t.AppendHeader(table.Row{"ID", "Created at", "Backup type", "Execution time", "Command executed"})

	rowCount := appendRows(rows, t)
	if rowCount == 0 {
		fmt.Println("No backups data found")
		return
	}
	printer.Pager(t.Render(), "Latest backups")
}

func appendRows(rows *sql.Rows, t table.Writer) int {
	rowCount := 0
	for rows.Next() {
		var id int
		var createdAt, backupType, executionTime, command string
		err := rows.Scan(&id, &createdAt, &backupType, &executionTime, &command)
		cobra.CheckErr(err)
		execTime := getExecutionTime(executionTime)
		cmd := getWrappedCommand(command)
		t.AppendRow([]interface{}{id, createdAt, backupType, execTime, cmd})
		t.AppendSeparator()
		rowCount++
	}

	return rowCount
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
