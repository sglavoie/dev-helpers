package storage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/hr/internal/storage"
)

func makeEntry(exercise string, reps, rounds int, notes string) storage.Entry {
	return storage.Entry{
		Timestamp: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		Exercise:  exercise,
		Reps:      reps,
		Rounds:    rounds,
		Notes:     notes,
	}
}

func TestAppend_CreatesFileWithHeaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	entry := makeEntry("Push-ups", 20, 3, "")
	if err := storage.Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	entries, err := storage.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Exercise != "Push-ups" {
		t.Errorf("expected exercise Push-ups, got %q", entries[0].Exercise)
	}
}

func TestAppend_MultipleRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	entries := []storage.Entry{
		makeEntry("Push-ups", 20, 3, ""),
		makeEntry("Squats", 25, 2, "deep"),
		makeEntry("Pull-ups", 10, 1, ""),
	}
	for _, e := range entries {
		if err := storage.Append(path, e); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	all, err := storage.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
	if all[1].Exercise != "Squats" || all[1].Notes != "deep" {
		t.Errorf("unexpected entry[1]: %+v", all[1])
	}
}

func TestReadAll_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	// Non-existent file returns nil, nil
	entries, err := storage.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for missing file, got %v", entries)
	}
}

func TestReadLast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	for _, name := range []string{"A", "B", "C", "D", "E"} {
		if err := storage.Append(path, makeEntry(name, 10, 1, "")); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	last2, err := storage.ReadLast(path, 2)
	if err != nil {
		t.Fatalf("ReadLast failed: %v", err)
	}
	if len(last2) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(last2))
	}
	if last2[0].Exercise != "D" || last2[1].Exercise != "E" {
		t.Errorf("unexpected last 2: %v, %v", last2[0].Exercise, last2[1].Exercise)
	}

	// n > total entries returns all
	all, err := storage.ReadLast(path, 100)
	if err != nil {
		t.Fatalf("ReadLast failed: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 entries, got %d", len(all))
	}
}

func TestRemoveLast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	for _, name := range []string{"A", "B", "C"} {
		if err := storage.Append(path, makeEntry(name, 10, 1, "")); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	removed, err := storage.RemoveLast(path)
	if err != nil {
		t.Fatalf("RemoveLast failed: %v", err)
	}
	if removed.Exercise != "C" {
		t.Errorf("expected removed exercise C, got %q", removed.Exercise)
	}

	remaining, err := storage.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining entries, got %d", len(remaining))
	}
}

func TestRemoveLast_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	_, err := storage.RemoveLast(path)
	if err == nil {
		t.Fatal("expected error for empty/missing file, got nil")
	}
}

func TestRemoveLast_SingleEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	if err := storage.Append(path, makeEntry("Solo", 5, 1, "")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	removed, err := storage.RemoveLast(path)
	if err != nil {
		t.Fatalf("RemoveLast failed: %v", err)
	}
	if removed.Exercise != "Solo" {
		t.Errorf("expected removed exercise Solo, got %q", removed.Exercise)
	}

	remaining, err := storage.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining entries, got %d", len(remaining))
	}
}

func TestActiveDays_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.csv")

	days, err := storage.ActiveDays(path)
	if err != nil {
		t.Fatalf("ActiveDays failed: %v", err)
	}
	if len(days) != 0 {
		t.Errorf("expected 0 days for missing file, got %d", len(days))
	}
}

func TestActiveDays_SameDayDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	base := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	for _, h := range []int{8, 12, 17} {
		e := storage.Entry{
			Timestamp: base.Add(time.Duration(h) * time.Hour),
			Exercise:  "Push-ups",
			Reps:      10,
			Rounds:    1,
		}
		if err := storage.Append(path, e); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	days, err := storage.ActiveDays(path)
	if err != nil {
		t.Fatalf("ActiveDays failed: %v", err)
	}
	if len(days) != 1 {
		t.Errorf("expected 1 unique day, got %d", len(days))
	}
}

func TestActiveDays_MultipleDaysSorted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	dates := []time.Time{
		time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC), // dupe of Apr 12
	}
	for _, ts := range dates {
		e := storage.Entry{Timestamp: ts, Exercise: "Squats", Reps: 10, Rounds: 1}
		if err := storage.Append(path, e); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	days, err := storage.ActiveDays(path)
	if err != nil {
		t.Fatalf("ActiveDays failed: %v", err)
	}
	if len(days) != 3 {
		t.Fatalf("expected 3 unique days, got %d", len(days))
	}
	for i := 1; i < len(days); i++ {
		if !days[i].After(days[i-1]) {
			t.Errorf("days not sorted ascending: %v >= %v", days[i-1], days[i])
		}
	}
}

func TestActiveDays_UTCMidnightCrossing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	// UTC 23:00 on Apr 13 = Apr 14 in UTC+1 (Europe/Paris)
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("Europe/Paris timezone not available")
	}
	ts := time.Date(2026, 4, 13, 23, 0, 0, 0, time.UTC)
	e := storage.Entry{Timestamp: ts, Exercise: "Pull-ups", Reps: 5, Rounds: 1}
	if err := storage.Append(path, e); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Override local time by converting manually — ActiveDays uses .Local(),
	// so we verify the raw behavior using in-Paris conversion.
	localTs := ts.In(paris)
	expectedDate := time.Date(localTs.Year(), localTs.Month(), localTs.Day(), 0, 0, 0, 0, paris)

	// The test just validates the function returns exactly 1 day and
	// the returned day matches the local interpretation of the UTC time.
	days, err := storage.ActiveDays(path)
	if err != nil {
		t.Fatalf("ActiveDays failed: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	// Verify the day is the local calendar date of the stored UTC timestamp.
	localDay := ts.Local()
	wantKey := time.Date(localDay.Year(), localDay.Month(), localDay.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
	gotKey := days[0].Format("2006-01-02")
	if gotKey != wantKey {
		t.Errorf("expected date %s (local), got %s; paris would give %s",
			wantKey, gotKey, expectedDate.Format("2006-01-02"))
	}
}

func TestEntry_TimestampRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.csv")

	ts := time.Date(2026, 4, 14, 10, 30, 0, 0, time.UTC)
	entry := storage.Entry{
		Timestamp: ts,
		Exercise:  "Push-ups",
		Reps:      20,
		Rounds:    3,
		Notes:     "test note",
	}
	if err := storage.Append(path, entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	entries, err := storage.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !entries[0].Timestamp.Equal(ts) {
		t.Errorf("timestamp mismatch: got %v, want %v", entries[0].Timestamp, ts)
	}
	if entries[0].Notes != "test note" {
		t.Errorf("notes mismatch: got %q, want %q", entries[0].Notes, "test note")
	}
}
