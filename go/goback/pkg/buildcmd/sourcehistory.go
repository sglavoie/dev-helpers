package buildcmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
)

// dailySourceHistory describes retained history, not the age of every file in
// daily/. Reading it must neither create history nor prevent an offline copy.
func (r *builder) dailySourceHistory() string {
	if kind := r.builderType.String(); kind != "weekly" && kind != "monthly" {
		return ""
	}
	rows, err := db.ReadSummary()
	if err != nil {
		return fmt.Sprintf("Copying the existing daily backup; history unavailable: %v\n", err)
	}
	return dailyHistoryDescription(rows, config.ActiveProfileName)
}

func dailyHistoryDescription(rows []db.SummaryRow, profile string) string {
	var text strings.Builder
	text.WriteString("Copying the existing daily backup; last recorded success: ")
	for _, row := range rows {
		if row.Profile != profile || row.BackupType != "daily" {
			continue
		}
		if row.LastSuccess == nil {
			text.WriteString("never recorded.\n")
		} else {
			text.WriteString(printer.TimestampWithAge(*row.LastSuccess) + ".\n")
		}
		if row.ExitCode != 0 {
			result := fmt.Sprintf("failed (exit %d)", row.ExitCode)
			if row.ExitCode == -1 {
				result = "interrupted"
			}
			fmt.Fprintf(&text, "Warning: latest daily attempt %s at %s; daily may be partially updated.\n", result, row.LatestAttempt)
		}
		return text.String()
	}
	return text.String() + "never recorded.\n"
}
