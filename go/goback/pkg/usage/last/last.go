package last

import (
	"database/sql"
	"encoding/json"
	"io"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/view"
)

func Last(e int) {
	queryAllLatestBackupTypes(e, func(rows *sql.Rows) {
		view.SqlToText(rows)
	})
}

func Summary() {
	querySummaryBackupTypes(func(rows *sql.Rows) {
		view.SqlToTextSummary(rows)
	})
}

// SummaryRow uses stored timestamps verbatim: old history has no time zone.
type SummaryRow struct {
	Profile       string  `json:"profile"`
	BackupType    string  `json:"backup_type"`
	LatestAttempt string  `json:"latest_attempt"`
	ExitCode      int     `json:"exit_code"`
	LastSuccess   *string `json:"last_success"`
}

func SummaryJSON(w io.Writer) error {
	result := []SummaryRow{}
	var readErr error
	querySummaryBackupTypes(func(rows *sql.Rows) {
		for rows.Next() {
			var row SummaryRow
			var success sql.NullString
			if err := rows.Scan(&row.Profile, &row.BackupType, &row.LatestAttempt, &row.ExitCode, &success); err != nil {
				readErr = err
				return
			}
			if success.Valid {
				row.LastSuccess = &success.String
			}
			result = append(result, row)
		}
		readErr = rows.Err()
	})
	if readErr != nil {
		return readErr
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func queryAllLatestBackupTypes(e int, callback func(*sql.Rows)) {
	profile := config.ProfileFlag
	if profile != "" {
		db.QueryRows(`
WITH ranked_backups AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY profile, backup_type ORDER BY created_at DESC, id DESC) as row_num
    FROM backups
    WHERE profile = ?
)
SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM ranked_backups
WHERE row_num <= ?
ORDER BY created_at DESC, id DESC;
		`, callback, profile, e)
		return
	}
	db.QueryRows(`
WITH ranked_backups AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY profile, backup_type ORDER BY created_at DESC, id DESC) as row_num
    FROM backups
)
SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM ranked_backups
WHERE row_num <= ?
ORDER BY created_at DESC, id DESC;
	`, callback, e)
}

func querySummaryBackupTypes(callback func(*sql.Rows)) {
	db.QueryRows(`
WITH ranked_backups AS (
    SELECT profile, backup_type, created_at, exit_code,
           ROW_NUMBER() OVER (
               PARTITION BY profile, backup_type ORDER BY created_at DESC, id DESC
           ) AS row_num,
           MAX(CASE WHEN exit_code = 0 THEN created_at END) OVER (
               PARTITION BY profile, backup_type
           ) AS last_success
    FROM backups
    WHERE (? = '' OR profile = ?)
)
SELECT profile, backup_type, created_at, exit_code, last_success
FROM ranked_backups
WHERE row_num = 1
ORDER BY profile, backup_type;
    `, callback, config.ProfileFlag, config.ProfileFlag)
}
