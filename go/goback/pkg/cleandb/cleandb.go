package cleandb

import (
	"database/sql"
	"strconv"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/cobra"
)

func Remove(id string) {
	idInt, err := strconv.Atoi(id)
	cobra.CheckErr(err)

	db.QueryRows("SELECT id FROM backups WHERE id = ?", func(rows *sql.Rows) {
		if !rows.Next() {
			cobra.CheckErr("No backup found with id " + id)
		}
	}, idInt)

	db.WithDb(func(sqldb *sql.DB) {
		_, errDb := sqldb.Exec("DELETE FROM backups WHERE id = ?", idInt)
		cobra.CheckErr(errDb)
	})
}
