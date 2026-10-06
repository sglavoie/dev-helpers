package db

import (
	"database/sql"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func CreateTableIfNotExists(db *sql.DB) error {
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS backups (
		id INTEGER PRIMARY KEY,
		created_at TEXT NOT NULL,
		backup_type TEXT NOT NULL,
		execution_time TEXT NOT NULL,
		command TEXT NOT NULL,
		profile TEXT NOT NULL DEFAULT '',
		exit_code INTEGER NOT NULL DEFAULT 0
	);
	`
	_, err := db.Exec(sqlStmt)
	return err
}

// MigrateProfileColumn adds the profile column to existing databases that lack it.
func MigrateProfileColumn(db *sql.DB) error {
	_, err := db.Exec("ALTER TABLE backups ADD COLUMN profile TEXT NOT NULL DEFAULT ''")
	if err != nil {
		// "duplicate column name" means the column already exists — safe to ignore
		if strings.Contains(err.Error(), "duplicate column") {
			return nil
		}
		return err
	}
	return nil
}

// MigrateExitCodeColumn adds the exit_code column to existing databases that lack it.
func MigrateExitCodeColumn(db *sql.DB) error {
	_, err := db.Exec("ALTER TABLE backups ADD COLUMN exit_code INTEGER NOT NULL DEFAULT 0")
	if err != nil {
		if strings.Contains(err.Error(), "duplicate column") {
			return nil
		}
		return err
	}
	return nil
}

func WithDb(callback func(*sql.DB)) {
	sqldb, err := open()
	cobra.CheckErr(err)
	defer func(sqldb *sql.DB) {
		err := sqldb.Close()
		cobra.CheckErr(err)
	}(sqldb)

	callback(sqldb)
}

func QueryRows(query string, callback func(*sql.Rows), args ...any) {
	WithDb(func(sqldb *sql.DB) {
		rows, err := sqldb.Query(query, args...)
		cobra.CheckErr(err)
		defer func() {
			err := rows.Close()
			cobra.CheckErr(err)
		}()
		callback(rows)
	})
}

func open() (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sqldb, err := sql.Open("sqlite3", home+"/.goback.db")
	if err != nil {
		return nil, err
	}
	for _, initialize := range []func(*sql.DB) error{CreateTableIfNotExists, MigrateProfileColumn, MigrateExitCodeColumn} {
		if err := initialize(sqldb); err != nil {
			_ = sqldb.Close()
			return nil, err
		}
	}
	return sqldb, nil
}
