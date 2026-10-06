package reset

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
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

func TestResetScopesRetentionAndDeletionToProfile(t *testing.T) {
	for _, kind := range []models.BackupTypes{models.NoBackupType{}, models.Daily{}} {
		t.Run(kind.String(), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			previous := config.ProfileFlag
			config.ProfileFlag = "alpha"
			t.Cleanup(func() { config.ProfileFlag = previous })
			now := time.Now()
			for _, profile := range []string{"alpha", "alpha", "beta", db.MirrorProfile} {
				db.RecordBackup(db.HistoryEntry{CreatedAt: now, BackupType: "daily", Profile: profile})
			}
			Reset(1, kind)
			var ids []int
			db.QueryRows("SELECT id FROM backups ORDER BY id", func(rows *sql.Rows) {
				for rows.Next() {
					var id int
					if err := rows.Scan(&id); err != nil {
						t.Fatal(err)
					}
					ids = append(ids, id)
				}
			})
			if !reflect.DeepEqual(ids, []int{2, 3, 4}) {
				t.Fatalf("remaining IDs = %v", ids)
			}
			Reset(0, kind)
			if got := remainingTypes(t); len(got) != 2 {
				t.Fatalf("other profiles were removed: %v", got)
			}
		})
	}
}
