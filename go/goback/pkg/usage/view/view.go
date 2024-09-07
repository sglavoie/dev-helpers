package view

import (
	"database/sql"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
)

func View(e int, t string) {
	var rows *sql.Rows
	if t == "" {
		rows = queryAllBackupTypes(e)
	} else {
		rows = queryBackupType(e, t)
	}
	db.WithRows(rows, func(rows *sql.Rows) {
		SqlToText(rows)
	})
}

func queryAllBackupTypes(e int) *sql.Rows {
	return db.WithQuery("SELECT * FROM backups ORDER BY created_at DESC LIMIT ?", e)
}

func queryBackupType(e int, t string) *sql.Rows {
	return db.WithQuery("SELECT * FROM backups WHERE backup_type = ? ORDER BY created_at DESC LIMIT ?", t, e)
}
