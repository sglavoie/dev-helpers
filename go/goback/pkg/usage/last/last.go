package last

import (
	"database/sql"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/view"
)

func Last(e int) {
	rows := queryAllLatestBackupTypes(e)
	db.WithRows(rows, func(rows *sql.Rows) {
		view.SqlToText(rows)
	})
}

func Summary() {
	rows := querySummaryBackupTypes()
	db.WithRows(rows, func(rows *sql.Rows) {
		view.SqlToTextSummary(rows)
	})
}

func SummaryWithLineBreak() {
	fmt.Println()
	Summary()
}

func queryAllLatestBackupTypes(e int) *sql.Rows {
	return db.WithQuery(`
WITH ranked_backups AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY backup_type ORDER BY created_at DESC) as row_num
    FROM backups
)
SELECT id, created_at, backup_type, execution_time, command FROM ranked_backups
WHERE row_num <= ?
ORDER BY created_at DESC;
	`, e)
}

func querySummaryBackupTypes() *sql.Rows {
	return queryAllLatestBackupTypes(1)
}
