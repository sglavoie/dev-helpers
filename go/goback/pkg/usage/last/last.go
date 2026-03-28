package last

import (
	"database/sql"
	"fmt"

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

func SummaryWithLineBreak() {
	fmt.Println()
	Summary()
}

func queryAllLatestBackupTypes(e int, callback func(*sql.Rows)) {
	profile := config.ProfileFlag
	if profile != "" {
		db.QueryRows(`
WITH ranked_backups AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY backup_type ORDER BY created_at DESC) as row_num
    FROM backups
    WHERE profile = ?
)
SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM ranked_backups
WHERE row_num <= ?
ORDER BY created_at DESC;
		`, callback, profile, e)
		return
	}
	db.QueryRows(`
WITH ranked_backups AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY backup_type ORDER BY created_at DESC) as row_num
    FROM backups
)
SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM ranked_backups
WHERE row_num <= ?
ORDER BY created_at DESC;
	`, callback, e)
}

func querySummaryBackupTypes(callback func(*sql.Rows)) {
	queryAllLatestBackupTypes(1, callback)
}
