package view

import (
	"database/sql"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/cobra"
)

func View(e int, t string) {
	db.WithDb(func(sqldb *sql.DB) {
		var rows *sql.Rows
		if t == "" {
			rows = queryAllBackupTypes(sqldb, e)
		} else {
			rows = queryBackupType(sqldb, e, t)
		}
		defer func(rows *sql.Rows) {
			err := rows.Close()
			cobra.CheckErr(err)
		}(rows)

		SqlToText(rows)
	})
}

func queryAllBackupTypes(sqldb *sql.DB, e int) *sql.Rows {
	rows, err := sqldb.Query("SELECT * FROM backups ORDER BY created_at DESC LIMIT ?", e)
	cobra.CheckErr(err)
	return rows
}

func queryBackupType(sqldb *sql.DB, e int, t string) *sql.Rows {
	rows, err := sqldb.Query("SELECT * FROM backups WHERE backup_type = ? ORDER BY created_at DESC LIMIT ?", t, e)
	cobra.CheckErr(err)
	return rows
}
