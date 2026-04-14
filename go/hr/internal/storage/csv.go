package storage

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"
)

var ErrNoEntries = errors.New("no entries found")

var csvHeaders = []string{"timestamp", "exercise", "reps", "rounds", "notes"}

// Entry represents a single exercise log record.
type Entry struct {
	Timestamp time.Time
	Exercise  string
	Reps      int
	Rounds    int
	Notes     string
}

// Append adds one entry to the CSV file, creating it with headers if necessary.
func Append(path string, entry Entry) error {
	needsHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needsHeader = true
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if needsHeader {
		if err := w.Write(csvHeaders); err != nil {
			return fmt.Errorf("writing csv header: %w", err)
		}
	}
	row := []string{
		entry.Timestamp.UTC().Format(time.RFC3339),
		entry.Exercise,
		strconv.Itoa(entry.Reps),
		strconv.Itoa(entry.Rounds),
		entry.Notes,
	}
	if err := w.Write(row); err != nil {
		return fmt.Errorf("writing csv row: %w", err)
	}
	w.Flush()
	return w.Error()
}

// ReadAll reads all entries from the CSV file.
func ReadAll(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening csv file: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading csv: %w", err)
	}

	// Skip header row
	if len(records) < 1 {
		return nil, nil
	}
	records = records[1:]

	entries := make([]Entry, 0, len(records))
	for i, rec := range records {
		if len(rec) < 5 {
			return nil, fmt.Errorf("row %d: expected 5 fields, got %d", i+2, len(rec))
		}
		ts, err := time.Parse(time.RFC3339, rec[0])
		if err != nil {
			return nil, fmt.Errorf("row %d: parsing timestamp: %w", i+2, err)
		}
		reps, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("row %d: parsing reps: %w", i+2, err)
		}
		rounds, err := strconv.Atoi(rec[3])
		if err != nil {
			return nil, fmt.Errorf("row %d: parsing rounds: %w", i+2, err)
		}
		entries = append(entries, Entry{
			Timestamp: ts,
			Exercise:  rec[1],
			Reps:      reps,
			Rounds:    rounds,
			Notes:     rec[4],
		})
	}
	return entries, nil
}

// ReadLast returns the last n entries from the CSV file.
func ReadLast(path string, n int) ([]Entry, error) {
	all, err := ReadAll(path)
	if err != nil {
		return nil, err
	}
	if n >= len(all) {
		return all, nil
	}
	return all[len(all)-n:], nil
}

// RemoveLast removes the last entry from the CSV file and returns it.
func RemoveLast(path string) (Entry, error) {
	all, err := ReadAll(path)
	if err != nil {
		return Entry{}, err
	}
	if len(all) == 0 {
		return Entry{}, ErrNoEntries
	}

	last := all[len(all)-1]
	remaining := all[:len(all)-1]

	if err := rewrite(path, remaining); err != nil {
		return Entry{}, fmt.Errorf("rewriting csv: %w", err)
	}
	return last, nil
}

// ActiveDays returns the unique calendar dates (in local time) for which
// entries exist, sorted ascending.
func ActiveDays(path string) ([]time.Time, error) {
	entries, err := ReadAll(path)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var days []time.Time
	for _, e := range entries {
		local := e.Timestamp.Local()
		key := local.Format("2006-01-02")
		if !seen[key] {
			seen[key] = true
			days = append(days, time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()))
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days, nil
}

func rewrite(path string, entries []Entry) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(csvHeaders); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	for _, e := range entries {
		row := []string{
			e.Timestamp.UTC().Format(time.RFC3339),
			e.Exercise,
			strconv.Itoa(e.Reps),
			strconv.Itoa(e.Rounds),
			e.Notes,
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}
