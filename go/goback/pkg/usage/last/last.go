package last

import (
	"database/sql"

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
