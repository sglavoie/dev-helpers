package view

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

// seedHistory writes one daily row of a profile and one global mirror row into
// a throwaway database, which is the mix every usage query has to handle.
func seedHistory(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	createdAt := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)
	db.RecordBackup(db.HistoryEntry{
		CreatedAt:     createdAt,
		BackupType:    "daily",
		ExecutionTime: "1m2s",
		Command:       "rsync --archive /src /dest",
		Profile:       "macbook",
	})
	db.RecordBackup(db.HistoryEntry{
		CreatedAt:     createdAt.Add(time.Minute),
		BackupType:    models.Mirror{}.String(),
		ExecutionTime: "1m30s",
		Command:       "rsync --archive /Volumes/SanDisk/Media/ /Volumes/Elements/Media",
		Profile:       db.MirrorProfile,
		ExitCode:      23,
	})
}

type queriedRow struct {
	backupType string
	profile    string
	exitCode   int
}

func collect(t *testing.T) (func(*sql.Rows), *[]queriedRow) {
	t.Helper()

	var rows []queriedRow
	return func(r *sql.Rows) {
		for r.Next() {
			var id, exitCode int
			var createdAt, backupType, executionTime, command, profile string
			if err := r.Scan(&id, &createdAt, &backupType, &executionTime, &command, &profile, &exitCode); err != nil {
				t.Fatal(err)
			}
			rows = append(rows, queriedRow{backupType: backupType, profile: profile, exitCode: exitCode})
		}
	}, &rows
}

// The general view selects every backup type, so a mirror shows up next to the
// snapshots without any change to the query.
func TestQueryAllBackupTypesIncludesMirror(t *testing.T) {
	seedHistory(t)

	callback, rows := collect(t)
	queryAllBackupTypes(10, callback)

	want := []queriedRow{
		{backupType: "mirror", profile: "global", exitCode: 23},
		{backupType: "daily", profile: "macbook"},
	}
	if len(*rows) != len(want) {
		t.Fatalf("selected %d rows, want %d: %+v", len(*rows), len(want), *rows)
	}
	for i := range want {
		if (*rows)[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, (*rows)[i], want[i])
		}
	}
}

func TestQueryBackupTypeSelectsOnlyMirror(t *testing.T) {
	seedHistory(t)

	callback, rows := collect(t)
	queryBackupType(10, models.Mirror{}, callback)

	if len(*rows) != 1 {
		t.Fatalf("selected %d rows, want exactly the mirror row: %+v", len(*rows), *rows)
	}
	if want := (queriedRow{backupType: "mirror", profile: "global", exitCode: 23}); (*rows)[0] != want {
		t.Fatalf("row = %+v, want %+v", (*rows)[0], want)
	}
}

// Profile filtering is unchanged: a mirror belongs to no profile, so asking for
// a profile's history keeps returning that profile's rows only.
func TestProfileFilteringLeavesMirrorOutOfAProfile(t *testing.T) {
	seedHistory(t)

	callback, rows := collect(t)
	queryAllBackupTypesWithProfile(10, "macbook", callback)

	if len(*rows) != 1 {
		t.Fatalf("selected %d rows, want the profile's row only: %+v", len(*rows), *rows)
	}
	if (*rows)[0].backupType != "daily" {
		t.Fatalf("row = %+v, want the daily row", (*rows)[0])
	}

	mirrorCallback, mirrorRows := collect(t)
	queryBackupTypeWithProfile(10, models.Mirror{}, db.MirrorProfile, mirrorCallback)
	if len(*mirrorRows) != 1 {
		t.Fatalf("selected %d rows for the global profile, want the mirror row: %+v", len(*mirrorRows), *mirrorRows)
	}
}
