package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadSummaryKeepsLastSuccessAndTimestampTieOrder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if rows, err := ReadSummary(); err != nil || len(rows) != 0 {
		t.Fatalf("%v: %v", rows, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".goback.db")); !os.IsNotExist(err) {
		t.Fatal("created history")
	}
	now := time.Now()
	RecordBackup(HistoryEntry{CreatedAt: now, Profile: "test", BackupType: "daily"})
	RecordBackup(HistoryEntry{CreatedAt: now, Profile: "test", BackupType: "daily", ExitCode: 23})
	rows, err := ReadSummary()
	if err != nil || len(rows) != 1 || rows[0].ExitCode != 23 || rows[0].LastSuccess == nil {
		t.Fatalf("%+v: %v", rows, err)
	}
}

func TestReadSummaryDoesNotMigrateLegacyHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	conn, err := sql.Open("sqlite3", filepath.Join(home, ".goback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = conn.Exec(`CREATE TABLE backups (id INTEGER PRIMARY KEY, created_at TEXT, backup_type TEXT, execution_time TEXT, command TEXT);
 INSERT INTO backups VALUES (1, '2026-10-01 12:00:00', 'daily', '1s', 'rsync');`)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ReadSummary()
	if err != nil || len(rows) != 1 || rows[0].Profile != "" || rows[0].LastSuccess == nil {
		t.Fatalf("%+v: %v", rows, err)
	}
	if r, err := conn.Query("SELECT profile FROM backups"); err == nil {
		r.Close()
		t.Fatal("read-only summary migrated database")
	}
}

func TestSummaryAcceptsVanishedSourcesButNotCompanionExit24(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Truncate(time.Second)
	for _, kind := range []string{"daily", "weekly", "monthly", "mirror", "companion/photos"} {
		RecordBackup(HistoryEntry{CreatedAt: now.Add(-time.Hour), Profile: "test", BackupType: kind, ExitCode: 24})
		RecordBackup(HistoryEntry{CreatedAt: now, Profile: "test", BackupType: kind, ExitCode: 23})
	}
	rows, err := ReadSummary()
	if err != nil || len(rows) != 5 {
		t.Fatalf("%+v: %v", rows, err)
	}
	for _, row := range rows {
		if row.ExitCode != 23 {
			t.Fatalf("raw code lost: %+v", row)
		}
		if row.BackupType == "companion/photos" {
			if row.LastSuccess != nil {
				t.Fatal("companion exit 24 counted as success")
			}
		} else if row.LastSuccess == nil || *row.LastSuccess != now.Add(-time.Hour).Format("2006-01-02 15:04:05") {
			t.Fatalf("vanished-source success lost after failure: %+v", row)
		}
	}
}
