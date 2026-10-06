package reset

import (
	"database/sql"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/cobra"
)

func Reset(k int, t models.BackupTypes) {
	db.WithDb(func(sqldb *sql.DB) {
		var result sql.Result
		if _, ok := t.(models.NoBackupType); ok {
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
	return deleteHistory(sqldb, k, "")
}

func queryBackupType(sqldb *sql.DB, k int, t models.BackupTypes) sql.Result {
	return deleteHistory(sqldb, k, t.String())
}

func deleteHistory(sqldb *sql.DB, k int, backupType string) sql.Result {
	// Apply the same scope to deletion and retention. ID breaks timestamp ties.
	scope := "(? = '' OR profile = ?) AND (? = '' OR backup_type = ?)"
	profile := config.ProfileFlag
	rows, err := sqldb.Exec("DELETE FROM backups WHERE "+scope+
		" AND id NOT IN (SELECT id FROM backups WHERE "+scope+" ORDER BY created_at DESC, id DESC LIMIT ?)",
		profile, profile, backupType, backupType, profile, profile, backupType, backupType, k)
	cobra.CheckErr(err)
	return rows
}
