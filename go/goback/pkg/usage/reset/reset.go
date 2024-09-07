package reset

import (
	"database/sql"
	"fmt"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/cobra"
)

func Reset(k int, t string) {
	db.WithDb(func(sqldb *sql.DB) {
		var result sql.Result
		if t == "" {
			result = queryAllBackupTypes(sqldb, k)
		} else {
			result = queryBackupType(sqldb, k, t)
		}

		n, err := result.RowsAffected()
		cobra.CheckErr(err)
		var entriesToKeep string
		if k == 0 {
			entriesToKeep = ""
		} else {
			entriesToKeep = fmt.Sprint("(keeping at least ", k, ")")
		}

		if n == 0 {
			fmt.Println("No entries to delete", entriesToKeep)
			return
		}
		if n != 1 {
			fmt.Printf("Deleted %d entries\n", n)
			return
		}
		fmt.Println("Deleted 1 entry")
	})
}

func queryAllBackupTypes(sqldb *sql.DB, k int) sql.Result {
	rows, err := sqldb.Exec("DELETE FROM backups WHERE id NOT IN (SELECT id FROM backups ORDER BY created_at DESC LIMIT ?)", k)
	cobra.CheckErr(err)
	return rows
}

func queryBackupType(sqldb *sql.DB, k int, t string) sql.Result {
	rows, err := sqldb.Exec("DELETE FROM backups WHERE backup_type = ? AND id NOT IN (SELECT id FROM backups WHERE backup_type = ? ORDER BY created_at DESC LIMIT ?)", t, t, k)
	cobra.CheckErr(err)
	return rows
}
