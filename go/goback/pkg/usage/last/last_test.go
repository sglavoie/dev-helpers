package last

import (
	"database/sql"
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

func latestTypes(t *testing.T, e int) []string {
	t.Helper()

	var types []string
	queryAllLatestBackupTypes(e, func(r *sql.Rows) {
		for r.Next() {
			var id, exitCode int
			var createdAt, backupType, executionTime, command, profile string
			if err := r.Scan(&id, &createdAt, &backupType, &executionTime, &command, &profile, &exitCode); err != nil {
				t.Fatal(err)
			}
			types = append(types, backupType)
		}
	})
	return types
}

// `usage last` partitions by backup type, so a mirror is one more partition
// rather than a query change.
func TestLastIncludesMirrorAsItsOwnBackupType(t *testing.T) {
	seedHistory(t)

	if got := latestTypes(t, 1); len(got) != 2 || got[0] != "mirror" || got[1] != "daily" {
		t.Fatalf("latest rows = %v, want the newest mirror row and the daily row", got)
	}
}

// Profile filtering is unchanged, and a mirror carries the global profile, so
// it is not part of a profile's own history.
func TestLastWithAProfileExcludesMirror(t *testing.T) {
	seedHistory(t)

	config.ProfileFlag = "macbook"
	t.Cleanup(func() { config.ProfileFlag = "" })

	if got := latestTypes(t, 3); len(got) != 1 || got[0] != "daily" {
		t.Fatalf("latest rows = %v, want the profile's daily row only", got)
	}
}
