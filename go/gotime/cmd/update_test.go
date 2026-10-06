package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/viper"
)

func updateFixture(t *testing.T) (*config.Manager, models.Entry) {
	t.Helper()
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.FixedZone("local", -6*3600))
	end := start.Add(time.Hour)
	entry := models.Entry{ID: uuid.NewString(), Keyword: "coding", Tags: []string{"work"}, StartTime: start, EndTime: &end, Duration: 3600}
	manager := config.NewManager(filepath.Join(t.TempDir(), "data.json"))
	if err := manager.Save(&models.Config{Entries: []models.Entry{entry}}); err != nil {
		t.Fatal(err)
	}
	previous := GetConfigPath()
	viper.Set("config", manager.GetConfigPath())
	t.Cleanup(func() { viper.Set("config", previous) })
	return manager, entry
}

func executeUpdate(args ...string) error {
	cmd := newUpdateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestUpdatePreservesIdentityAcrossRenumberingAndCanUndo(t *testing.T) {
	manager, original := updateFixture(t)
	cfg, _ := manager.Load()
	other := models.NewEntry("meeting", nil, 1)
	cfg.AddEntry(other)
	if err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
	start := "2026-01-02T10:15:23.123-06:00"
	end := "2026-01-02T11:15:23.123-06:00"
	if err := executeUpdate(original.ID, "--start", start, "--end", end, "--keyword", "review", "--tags", ""); err != nil {
		t.Fatal(err)
	}
	cfg, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Entries) != 2 {
		t.Fatalf("entry count changed: %d", len(cfg.Entries))
	}
	parsed, err := ParseKeywordOrID(original.ID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	entry := parsed.Entry
	if entry.Keyword != "review" || entry.Active || entry.Duration != 3600 || len(entry.Tags) != 0 || entry.StartTime.Format(time.RFC3339Nano) != start || entry.EndTime.Format(time.RFC3339Nano) != end {
		t.Fatalf("incorrect edit: %+v", entry)
	}
	if cfg.Entries[1].ID != other.ID || !cfg.Entries[1].Active {
		t.Fatal("unrelated timer changed")
	}
	if len(cfg.UndoHistory) != 1 {
		t.Fatal("expected one undo record for the complete edit")
	}
	if err := undoBulkEdit(cfg, manager, &cfg.UndoHistory[0]); err != nil {
		t.Fatal(err)
	}
	restored, _ := ParseKeywordOrID(original.ID, cfg)
	if restored.Entry.Keyword != original.Keyword || !restored.Entry.StartTime.Equal(original.StartTime) {
		t.Fatal("undo did not restore original")
	}
}

func TestInvalidUpdateLeavesFileUntouched(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--end", "2026-01-01T08:00:00-06:00"}, {"--start", "2026-01-01 09:00:00"},
		{"--duration", "-1"}, {"--duration", "9223372036854775807"},
		{"--duration", "60", "--end", "active"}, {"--keyword", ""},
		{"--start", "2999-01-01T00:00:00Z"},
	} {
		t.Run(fmtArgs(args), func(t *testing.T) {
			manager, original := updateFixture(t)
			before, _ := os.ReadFile(manager.GetConfigPath())
			if err := executeUpdate(append([]string{original.ID}, args...)...); err == nil {
				t.Fatal("expected validation error")
			}
			after, _ := os.ReadFile(manager.GetConfigPath())
			if !bytes.Equal(before, after) {
				t.Fatal("invalid edit changed the saved file")
			}
		})
	}
}

func fmtArgs(args []string) string {
	if len(args) == 0 {
		return "no changes"
	}
	return args[0] + "=" + args[1]
}

func TestUpdateRunningAndCompletedStates(t *testing.T) {
	manager, original := updateFixture(t)
	if err := executeUpdate(original.ID, "--end", "active"); err != nil {
		t.Fatal(err)
	}
	if err := executeUpdate(original.ID, "--keyword", "review"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := manager.Load()
	entry := cfg.Entries[0]
	if !entry.Active || entry.EndTime != nil || !entry.StartTime.Equal(original.StartTime) {
		t.Fatalf("metadata edit reset running timer: %+v", entry)
	}
	if err := executeUpdate(original.ID, "--end", "2026-01-01T09:30:00-06:00"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = manager.Load()
	if cfg.Entries[0].Active || cfg.Entries[0].Duration != 1800 {
		t.Fatal("explicit end did not stop the entry")
	}
	if err := executeUpdate(original.ID, "--duration", "900"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = manager.Load()
	if cfg.Entries[0].Duration != 900 || cfg.Entries[0].EndTime.Sub(original.StartTime) != 15*time.Minute {
		t.Fatal("duration and end time disagree")
	}
}

func TestUpdateRejectsConflictingActiveKeyword(t *testing.T) {
	manager, original := updateFixture(t)
	cfg, _ := manager.Load()
	cfg.AddEntry(models.NewEntry(original.Keyword, nil, 1))
	if err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(manager.GetConfigPath())
	if err := executeUpdate(original.ID, "--end", "active"); err == nil {
		t.Fatal("expected active keyword conflict")
	}
	after, _ := os.ReadFile(manager.GetConfigPath())
	if !bytes.Equal(before, after) {
		t.Fatal("conflicting edit changed data")
	}
}

func TestMissingUUIDDoesNotFallBackToKeyword(t *testing.T) {
	id := uuid.NewString()
	cfg := &models.Config{Entries: []models.Entry{{ID: uuid.NewString(), Keyword: id}}}
	if _, err := ParseKeywordOrID(id, cfg); err == nil {
		t.Fatal("missing UUID was treated as a keyword")
	}
}
