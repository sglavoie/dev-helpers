package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.DefaultRounds != 1 {
		t.Errorf("expected DefaultRounds=1, got %d", cfg.DefaultRounds)
	}
	if len(cfg.Exercises) == 0 {
		t.Fatal("expected non-empty exercises list")
	}
	names := make(map[string]bool)
	for _, e := range cfg.Exercises {
		names[e.Name] = true
	}
	for _, expected := range []string{"Push-ups", "Squats", "Pull-ups", "Dips", "Lunges", "Plank (seconds)", "Burpees", "Sit-ups"} {
		if !names[expected] {
			t.Errorf("expected exercise %q in default config", expected)
		}
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.yaml")

	original := config.DefaultConfig()
	if err := config.Save(path, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.DefaultRounds != original.DefaultRounds {
		t.Errorf("DefaultRounds mismatch: got %d, want %d", loaded.DefaultRounds, original.DefaultRounds)
	}
	if len(loaded.Exercises) != len(original.Exercises) {
		t.Errorf("Exercises count mismatch: got %d, want %d", len(loaded.Exercises), len(original.Exercises))
	}
	if loaded.DateFormat != original.DateFormat {
		t.Errorf("DateFormat mismatch: got %q, want %q", loaded.DateFormat, original.DateFormat)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/hr.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadOrInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.yaml")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist yet")
	}

	cfg, err := config.LoadOrInit(path)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("LoadOrInit should have created the file")
	}

	if len(cfg.Exercises) == 0 {
		t.Error("expected non-empty exercises in newly created config")
	}
}

func TestLoadOrInit_LoadsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hr.yaml")

	custom := config.Config{
		DefaultRounds: 3,
		DateFormat:    "2006-01-02",
		Exercises:     []config.Exercise{{Name: "Custom", DefaultReps: 99}},
	}
	if err := config.Save(path, custom); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	cfg, err := config.LoadOrInit(path)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if cfg.DefaultRounds != 3 {
		t.Errorf("expected DefaultRounds=3, got %d", cfg.DefaultRounds)
	}
	if len(cfg.Exercises) != 1 || cfg.Exercises[0].Name != "Custom" {
		t.Errorf("expected custom exercises, got %v", cfg.Exercises)
	}
}
