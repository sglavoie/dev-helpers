package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
)

type SummaryRow struct {
	Profile       string
	BackupType    string
	LatestAttempt string
	ExitCode      int
	LastSuccess   *string
}

// ReadSummary does not create or migrate the database. Legacy rows retain the
// same empty-profile and successful-exit defaults used by normal migrations.
func ReadSummary() ([]SummaryRow, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	conn, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	columns, err := conn.Query("PRAGMA table_info(backups)")
	if err != nil {
		return nil, err
	}
	profile, exitCode := "''", "0"
	for columns.Next() {
		var id, notNull, primary int
		var name, kind string
		var defaultValue any
		if err := columns.Scan(&id, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
			columns.Close()
			return nil, err
		}
		if name == "profile" {
			profile = "profile"
		}
		if name == "exit_code" {
			exitCode = "exit_code"
		}
	}
	err = columns.Err()
	columns.Close()
	if err != nil {
		return nil, err
	}
	// Both substitutions are fixed SQL expressions, never user input.
	query := fmt.Sprintf(`WITH history AS (
 SELECT id, created_at, backup_type, %s AS profile, %s AS exit_code FROM backups
), ranked AS (
 SELECT profile, backup_type, created_at, exit_code,
 ROW_NUMBER() OVER (PARTITION BY profile, backup_type ORDER BY created_at DESC, id DESC) AS rank,
 MAX(CASE WHEN exit_code = 0 THEN created_at END) OVER (PARTITION BY profile, backup_type) AS success
 FROM history
) SELECT profile, backup_type, created_at, exit_code, success FROM ranked WHERE rank = 1 ORDER BY profile, backup_type`, profile, exitCode)
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SummaryRow
	for rows.Next() {
		var row SummaryRow
		var success sql.NullString
		if err := rows.Scan(&row.Profile, &row.BackupType, &row.LatestAttempt, &row.ExitCode, &success); err != nil {
			return nil, err
		}
		if success.Valid {
			row.LastSuccess = &success.String
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
