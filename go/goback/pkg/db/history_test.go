package db

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestCompanionBackupType(t *testing.T) {
	if got, want := CompanionBackupType("apple-photos"), "companion/apple-photos"; got != want {
		t.Fatalf("CompanionBackupType = %q, want %q", got, want)
	}
}

func TestRecordBackupWritesRsyncAndCompanionRows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	createdAt := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)
	RecordBackup(HistoryEntry{
		CreatedAt:     createdAt,
		BackupType:    "daily",
		ExecutionTime: "1m2s",
		Command:       "rsync --archive /src /dest",
		Profile:       "macbook",
	})
	RecordBackup(HistoryEntry{
		CreatedAt:     createdAt.Add(time.Minute),
		BackupType:    CompanionBackupType("apple-photos"),
		ExecutionTime: "3s",
		Command:       "photos-backup daily",
		Profile:       "macbook",
		ExitCode:      3,
	})

	type row struct {
		createdAt     string
		backupType    string
		executionTime string
		command       string
		profile       string
		exitCode      int
	}

	var rows []row
	QueryRows("SELECT created_at, backup_type, execution_time, command, profile, exit_code FROM backups ORDER BY id", func(r *sql.Rows) {
		for r.Next() {
			var got row
			if err := r.Scan(&got.createdAt, &got.backupType, &got.executionTime, &got.command, &got.profile, &got.exitCode); err != nil {
				t.Fatal(err)
			}
			rows = append(rows, got)
		}
	})

	want := []row{
		{"2026-08-11 09:30:00", "daily", "1m2s", "rsync --archive /src /dest", "macbook", 0},
		{"2026-08-11 09:31:00", "companion/apple-photos", "3s", "photos-backup daily", "macbook", 3},
	}
	if len(rows) != len(want) {
		t.Fatalf("recorded %d rows, want %d", len(rows), len(want))
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

func TestRecordBackupOnLegacyDatabase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacyDB, err := sql.Open("sqlite3", home+"/.goback.db")
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
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}

	RecordBackup(HistoryEntry{
		CreatedAt:  time.Now(),
		BackupType: CompanionBackupType("apple-photos"),
		Profile:    "macbook",
	})

	var backupType string
	QueryRows("SELECT backup_type FROM backups", func(r *sql.Rows) {
		if !r.Next() {
			t.Fatal("no row recorded")
		}
		if err := r.Scan(&backupType); err != nil {
			t.Fatal(err)
		}
	})
	if backupType != "companion/apple-photos" {
		t.Fatalf("backup_type = %q, want companion/apple-photos", backupType)
	}
}
