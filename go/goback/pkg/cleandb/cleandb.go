package cleandb

import (
	"database/sql"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/cobra"
	"strconv"
)

func Remove(id string) {
	idInt, err := strconv.Atoi(id)
	cobra.CheckErr(err)

	rows := db.WithQuery("SELECT id FROM backups WHERE id = ?", idInt)
	db.WithRows(rows, func(rows *sql.Rows) {
		if !rows.Next() {
			cobra.CheckErr("No backup found with id " + id)
		}
		return
	})

	db.WithDb(func(sqldb *sql.DB) {
		_, errDb := sqldb.Exec("DELETE FROM backups WHERE id = ?", idInt)
		cobra.CheckErr(errDb)
	})
}
