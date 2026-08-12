package reset

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

func seedHistory(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	createdAt := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)
	entries := []db.HistoryEntry{
		{CreatedAt: createdAt, BackupType: "daily", Profile: "macbook"},
		{CreatedAt: createdAt.Add(time.Minute), BackupType: models.Mirror{}.String(), Profile: db.MirrorProfile},
		{CreatedAt: createdAt.Add(2 * time.Minute), BackupType: models.Mirror{}.String(), Profile: db.MirrorProfile},
	}
	for _, entry := range entries {
		db.RecordBackup(entry)
	}
}

func remainingTypes(t *testing.T) []string {
	t.Helper()

	var types []string
	db.QueryRows("SELECT backup_type FROM backups ORDER BY id", func(r *sql.Rows) {
		for r.Next() {
			var backupType string
			if err := r.Scan(&backupType); err != nil {
				t.Fatal(err)
			}
			types = append(types, backupType)
		}
	})
	return types
}

// Resetting mirror usage must never touch the snapshot history it sits beside.
func TestResetMirrorDeletesOnlyMirrorRows(t *testing.T) {
	seedHistory(t)

	Reset(0, models.Mirror{})

	got := remainingTypes(t)
	if len(got) != 1 || got[0] != "daily" {
		t.Fatalf("remaining rows = %v, want the daily row only", got)
	}
}

func TestResetMirrorKeepsTheRequestedNumber(t *testing.T) {
	seedHistory(t)

	Reset(1, models.Mirror{})

	got := remainingTypes(t)
	if len(got) != 2 || got[0] != "daily" || got[1] != "mirror" {
		t.Fatalf("remaining rows = %v, want the daily row and the newest mirror row", got)
	}
}

// A reset with no selector still clears everything, mirror rows included.
func TestResetWithoutSelectorDeletesMirrorToo(t *testing.T) {
	seedHistory(t)

	Reset(0, models.NoBackupType{})

	if got := remainingTypes(t); len(got) != 0 {
		t.Fatalf("remaining rows = %v, want none", got)
	}
}
