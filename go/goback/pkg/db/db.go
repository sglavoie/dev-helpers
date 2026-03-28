package db

import (
	"database/sql"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func CreateDatabaseFileIfNotExists() {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	var file = home + "/.goback.db"
	if _, err := os.Stat(file); os.IsNotExist(err) {
		_, err := os.Create(file)
		cobra.CheckErr(err)
	}
}

func CreateTableIfNotExists(db *sql.DB) {
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
	cobra.CheckErr(err)
}

// MigrateProfileColumn adds the profile column to existing databases that lack it.
func MigrateProfileColumn(db *sql.DB) {
	_, err := db.Exec("ALTER TABLE backups ADD COLUMN profile TEXT NOT NULL DEFAULT ''")
	if err != nil {
		// "duplicate column name" means the column already exists — safe to ignore
		if strings.Contains(err.Error(), "duplicate column") {
			return
		}
		cobra.CheckErr(err)
	}
}

// MigrateExitCodeColumn adds the exit_code column to existing databases that lack it.
func MigrateExitCodeColumn(db *sql.DB) {
	_, err := db.Exec("ALTER TABLE backups ADD COLUMN exit_code INTEGER NOT NULL DEFAULT 0")
	if err != nil {
		if strings.Contains(err.Error(), "duplicate column") {
			return
		}
		cobra.CheckErr(err)
	}
}

func WithDb(callback func(*sql.DB)) {
	sqldb := open()
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

func checkTableExists(db *sql.DB) {
	sqlStmt := `SELECT name FROM sqlite_master WHERE type='table' AND name='backups';`
	row := db.QueryRow(sqlStmt)
	var name string
	err := row.Scan(&name)
	if err != nil {
		CreateTableIfNotExists(db)
	}
}

func open() *sql.DB {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	var file = home + "/.goback.db"
	db, err := sql.Open("sqlite3", file)
	cobra.CheckErr(err)
	checkTableExists(db)
	return db
}
