package db

import (
	"database/sql"
	"github.com/spf13/cobra"
	"os"
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
		command TEXT NOT NULL
	);
	`
	_, err := db.Exec(sqlStmt)
	cobra.CheckErr(err)
}

func Open() *sql.DB {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	var file = home + "/.goback.db"
	db, err := sql.Open("sqlite3", file)
	cobra.CheckErr(err)
	checkTableExists(db)
	return db
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
