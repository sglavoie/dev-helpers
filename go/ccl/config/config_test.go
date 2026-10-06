package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	overrideConfigPath(t, filepath.Join(dir, "config.json"))

	cfg := &Config{
		WorkDays:  []string{"mon", "wed", "fri"},
		ResetTime: "Mon 9:00 AM",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ResetTime != cfg.ResetTime {
		t.Errorf("ResetTime: got %q, want %q", got.ResetTime, cfg.ResetTime)
	}
	if len(got.WorkDays) != len(cfg.WorkDays) {
		t.Errorf("WorkDays length: got %d, want %d", len(got.WorkDays), len(cfg.WorkDays))
	}
	for i, d := range cfg.WorkDays {
		if got.WorkDays[i] != d {
			t.Errorf("WorkDays[%d]: got %q, want %q", i, got.WorkDays[i], d)
		}
	}
}

func TestLoadCreatesDefaultOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	overrideConfigPath(t, filepath.Join(dir, "subdir", "config.json"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	def := DefaultConfig()
	if cfg.ResetTime != def.ResetTime {
		t.Errorf("ResetTime: got %q, want %q", cfg.ResetTime, def.ResetTime)
	}
	if len(cfg.WorkDays) != len(def.WorkDays) {
		t.Errorf("WorkDays length: got %d, want %d", len(cfg.WorkDays), len(def.WorkDays))
	}

	// Verify the file was actually created
	path := ConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created, but it does not exist")
	}
}

func TestLoadParsesJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	overrideConfigPath(t, path)

	raw := `{"work_days":["tue","thu"],"reset_time":"Fri 10:00 AM"}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ResetTime != "Fri 10:00 AM" {
		t.Errorf("ResetTime: got %q", cfg.ResetTime)
	}
	if len(cfg.WorkDays) != 2 || cfg.WorkDays[0] != "tue" || cfg.WorkDays[1] != "thu" {
		t.Errorf("WorkDays: got %v", cfg.WorkDays)
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	overrideConfigPath(t, path)

	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	overrideConfigPath(t, path)

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Error("expected error for empty file, got nil")
	}
}

func TestLoadMissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	overrideConfigPath(t, path)

	// Only reset_time provided, no work_days.
	raw := `{"reset_time":"Thu 2:59 PM"}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ResetTime != "Thu 2:59 PM" {
		t.Errorf("ResetTime: got %q, want %q", cfg.ResetTime, "Thu 2:59 PM")
	}
	if len(cfg.WorkDays) != 0 {
		t.Errorf("WorkDays: expected empty, got %v", cfg.WorkDays)
	}
}

func TestSaveWritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	overrideConfigPath(t, path)

	cfg := DefaultConfig()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out Config
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
}

// ValidateDays tests

func TestValidateDays_Valid(t *testing.T) {
	cases := [][]string{
		{"mon"},
		{"mon", "tue", "wed", "thu", "fri"},
		{"mon", "tue", "wed", "thu", "fri", "sat"},
		{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
		{"sat", "sun"},
	}
	for _, days := range cases {
		if err := ValidateDays(days); err != nil {
			t.Errorf("ValidateDays(%v): unexpected error: %v", days, err)
		}
	}
}

func TestValidateDays_Empty(t *testing.T) {
	if err := ValidateDays([]string{}); err == nil {
		t.Error("expected error for empty days, got nil")
	}
}

func TestValidateDays_InvalidName(t *testing.T) {
	cases := [][]string{
		{"monday"},
		{"Mon"},
		{"MON"},
		{"xyz"},
		{"mon", "invalid"},
	}
	for _, days := range cases {
		if err := ValidateDays(days); err == nil {
			t.Errorf("ValidateDays(%v): expected error, got nil", days)
		}
	}
}

func TestValidateDays_Duplicates(t *testing.T) {
	cases := [][]string{
		{"mon", "mon"},
		{"tue", "wed", "tue"},
	}
	for _, days := range cases {
		if err := ValidateDays(days); err == nil {
			t.Errorf("ValidateDays(%v): expected duplicate error, got nil", days)
		}
	}
}

// ValidateResetTime tests

func TestValidateResetTime_Valid(t *testing.T) {
	cases := []string{
		"Thu 2:59 PM",
		"Mon 11:00 AM",
		"Fri 9:00 AM",
		"Sun 12:00 PM",
	}
	for _, s := range cases {
		if err := ValidateResetTime(s); err != nil {
			t.Errorf("ValidateResetTime(%q): unexpected error: %v", s, err)
		}
	}
}

func TestValidateResetTime_Invalid(t *testing.T) {
	cases := []string{
		"",
		"Thursday 2:59 PM",
		"Thu 2:59",
		"2:59 PM",
		"bad input",
		"Thu 25:00 PM",
	}
	for _, s := range cases {
		if err := ValidateResetTime(s); err == nil {
			t.Errorf("ValidateResetTime(%q): expected error, got nil", s)
		}
	}
}

// ParseResetTime tests

func TestParseResetTime(t *testing.T) {
	cases := []struct {
		input   string
		weekday string
		hour    int
		minute  int
	}{
		{"Thu 2:59 PM", "Thursday", 14, 59},
		{"Mon 11:00 AM", "Monday", 11, 0},
		{"Fri 9:00 AM", "Friday", 9, 0},
		{"Sun 12:00 PM", "Sunday", 12, 0},
	}
	for _, tc := range cases {
		wd, h, m := ParseResetTime(tc.input)
		if wd.String() != tc.weekday {
			t.Errorf("ParseResetTime(%q) weekday: got %q, want %q", tc.input, wd, tc.weekday)
		}
		if h != tc.hour {
			t.Errorf("ParseResetTime(%q) hour: got %d, want %d", tc.input, h, tc.hour)
		}
		if m != tc.minute {
			t.Errorf("ParseResetTime(%q) minute: got %d, want %d", tc.input, m, tc.minute)
		}
	}
}

func overrideConfigPath(t *testing.T, path string) {
	t.Helper()
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = "" })
}
