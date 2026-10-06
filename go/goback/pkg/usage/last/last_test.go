package last

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

func TestSummaryJSONEmptyAndFilteredHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.ProfileFlag = ""
	t.Cleanup(func() { config.ProfileFlag = "" })
	var output bytes.Buffer
	if err := SummaryJSON(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "[]\n" {
		t.Fatalf("empty summary = %q", output.String())
	}
	success := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)
	for _, entry := range []db.HistoryEntry{
		{CreatedAt: success, BackupType: "daily", Profile: "macbook"},
		{CreatedAt: success.Add(time.Hour), BackupType: "daily", Profile: "macbook", ExitCode: 23},
		{CreatedAt: success, BackupType: "weekly", Profile: "macbook", ExitCode: -1},
		{CreatedAt: success, BackupType: "mirror", Profile: db.MirrorProfile},
	} {
		db.RecordBackup(entry)
	}
	config.ProfileFlag = "macbook"
	output.Reset()
	if err := SummaryJSON(&output); err != nil {
		t.Fatal(err)
	}
	var rows []SummaryRow
	if err := json.Unmarshal(output.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Profile != "macbook" || rows[0].BackupType != "daily" || rows[0].ExitCode != 23 || rows[0].LastSuccess == nil || *rows[0].LastSuccess != "2026-08-11 09:30:00" || rows[0].LatestAttempt != "2026-08-11 10:30:00" {
		t.Fatalf("daily = %+v", rows[0])
	}
	if rows[1].LastSuccess != nil || rows[1].ExitCode != -1 {
		t.Fatalf("weekly = %+v", rows[1])
	}
	if !bytes.Contains(output.Bytes(), []byte(`"last_success": null`)) {
		t.Fatal("missing successes must be JSON null")
	}
}

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

func TestSummaryKeepsEachProfileAndLastSuccessAfterFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.ProfileFlag = ""
	t.Cleanup(func() { config.ProfileFlag = "" })
	earlier := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)
	latest := earlier.Add(time.Hour)
	for _, entry := range []db.HistoryEntry{
		{CreatedAt: earlier, BackupType: "daily", Profile: "macbook"},
		{CreatedAt: latest, BackupType: "daily", Profile: "macbook", ExitCode: 23},
		{CreatedAt: earlier, BackupType: "daily", Profile: "media"},
		// Same-second attempts are ordered by ID; the interruption is latest.
		{CreatedAt: latest, BackupType: "daily", Profile: "media"},
		{CreatedAt: latest, BackupType: "daily", Profile: "media", ExitCode: -1},
		{CreatedAt: latest, BackupType: "weekly", Profile: "macbook", ExitCode: 1},
		{CreatedAt: latest, BackupType: "mirror", Profile: db.MirrorProfile},
		{CreatedAt: latest, BackupType: "companion/photos", Profile: "macbook", ExitCode: 3},
	} {
		db.RecordBackup(entry)
	}
	type summary struct {
		profile, kind, attempt string
		code                   int
		success                sql.NullString
	}
	read := func() map[string]summary {
		got := map[string]summary{}
		querySummaryBackupTypes(func(rows *sql.Rows) {
			for rows.Next() {
				var row summary
				if err := rows.Scan(&row.profile, &row.kind, &row.attempt, &row.code, &row.success); err != nil {
					t.Fatal(err)
				}
				got[row.profile+"/"+row.kind] = row
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
		})
		return got
	}
	got := read()
	if len(got) != 5 {
		t.Fatalf("summary = %+v", got)
	}
	if row := got["macbook/daily"]; row.code != 23 || row.success.String != "2026-08-11 09:30:00" || row.attempt != "2026-08-11 10:30:00" {
		t.Fatalf("macbook daily = %+v", row)
	}
	if row := got["media/daily"]; row.code != -1 || row.success.String != "2026-08-11 10:30:00" {
		t.Fatalf("media daily = %+v", row)
	}
	if row := got["macbook/weekly"]; row.success.Valid {
		t.Fatalf("failed-only backup has success: %+v", row)
	}
	if types := latestTypes(t, 1); len(types) != 5 {
		t.Fatalf("latest attempts hid a profile: %v", types)
	}
	config.ProfileFlag = "macbook"
	got = read()
	if len(got) != 3 {
		t.Fatalf("filtered summary = %+v", got)
	}
	for _, row := range got {
		if row.profile != "macbook" {
			t.Fatalf("unexpected profile: %+v", row)
		}
	}
}
