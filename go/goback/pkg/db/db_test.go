package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpenMigratesLegacyBackupsTable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacyDB, err := sql.Open("sqlite3", filepath.Join(home, ".goback.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacyDB.Exec(`
CREATE TABLE backups (
	id INTEGER PRIMARY KEY,
	created_at TEXT NOT NULL,
	backup_type TEXT NOT NULL,
	execution_time TEXT NOT NULL,
	command TEXT NOT NULL
);
INSERT INTO backups (created_at, backup_type, execution_time, command)
VALUES ('2026-07-03 09:00:00', 'daily', '1s', 'rsync --archive');
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}

	WithDb(func(sqldb *sql.DB) {
		var profile string
		var exitCode int
		err := sqldb.QueryRow("SELECT profile, exit_code FROM backups WHERE id = 1").Scan(&profile, &exitCode)
		if err != nil {
			t.Fatal(err)
		}
		if profile != "" {
			t.Fatalf("profile = %q, want empty default", profile)
		}
		if exitCode != 0 {
			t.Fatalf("exitCode = %d, want 0", exitCode)
		}
	})
}
