package last

import (
	"database/sql"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/view"
	"github.com/spf13/cobra"
)

func Last(e int) {
	db.WithDb(func(sqldb *sql.DB) {
		var rows *sql.Rows
		rows = queryAllBackupTypes(sqldb, e)
		view.SqlToText(rows)
		defer func(rows *sql.Rows) {
			err := rows.Close()
			cobra.CheckErr(err)
		}(rows)
	})
}

func queryAllBackupTypes(sqldb *sql.DB, e int) *sql.Rows {
	query := `
WITH ranked_backups AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY backup_type ORDER BY created_at DESC) as row_num
    FROM backups
)
SELECT id, created_at, backup_type, execution_time, command FROM ranked_backups
WHERE row_num <= ?
ORDER BY created_at DESC;
	`
	rows, err := sqldb.Query(query, e)
	cobra.CheckErr(err)
	return rows
}
