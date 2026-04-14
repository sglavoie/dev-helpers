package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if len(cfg.Exercises) == 0 {
		t.Fatal("expected non-empty exercises list")
	}

	names := make(map[string]bool)
	for _, e := range cfg.Exercises {
		names[e.Name] = true
	}
	for _, expected := range []string{"Push-ups", "Squats", "Pull-ups", "Dips", "Lunges", "Plank", "Burpees", "Sit-ups", "Rowing", "Running"} {
		if !names[expected] {
			t.Errorf("expected exercise %q in default config", expected)
		}
	}

	// Old name should be gone
	if names["Plank (seconds)"] {
		t.Error("old exercise name 'Plank (seconds)' should not exist in default config")
	}
}

func TestDefaultConfig_ValidatesClean(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid, got: %v", err)
	}
}

func TestDefaultConfig_ExerciseFields(t *testing.T) {
	cfg := config.DefaultConfig()
	names := make(map[string]config.Exercise)
	for _, e := range cfg.Exercises {
		names[e.Name] = e
	}

	pushups := names["Push-ups"]
	if pushups.Category != "Bodyweight" {
		t.Errorf("Push-ups: expected category Bodyweight, got %q", pushups.Category)
	}
	primary := pushups.PrimaryField()
	if primary == nil || primary.Name != "reps" {
		t.Errorf("Push-ups: expected primary field 'reps', got %v", primary)
	}
	if primary.MultipliedBy != "rounds" {
		t.Errorf("Push-ups: expected MultipliedBy 'rounds', got %q", primary.MultipliedBy)
	}

	plank := names["Plank"]
	if plank.Category != "Bodyweight" {
		t.Errorf("Plank: expected category Bodyweight, got %q", plank.Category)
	}
	plankPrimary := plank.PrimaryField()
	if plankPrimary == nil || plankPrimary.Name != "seconds" {
		t.Errorf("Plank: expected primary field 'seconds', got %v", plankPrimary)
	}

	rowing := names["Rowing"]
	if rowing.Category != "Cardio" {
		t.Errorf("Rowing: expected category Cardio, got %q", rowing.Category)
	}

	running := names["Running"]
	if running.Category != "Cardio" {
		t.Errorf("Running: expected category Cardio, got %q", running.Category)
	}
}

func TestExercise_FieldByName(t *testing.T) {
	cfg := config.DefaultConfig()
	var pushups config.Exercise
	for _, e := range cfg.Exercises {
		if e.Name == "Push-ups" {
			pushups = e
			break
		}
	}

	f := pushups.FieldByName("reps")
	if f == nil {
		t.Fatal("expected to find field 'reps'")
	}
	if f.Name != "reps" {
		t.Errorf("expected name 'reps', got %q", f.Name)
	}

	missing := pushups.FieldByName("nonexistent")
	if missing != nil {
		t.Error("expected nil for nonexistent field")
	}
}

func TestConfig_Categories(t *testing.T) {
	cfg := config.DefaultConfig()
	cats := cfg.Categories()

	if len(cats) < 2 {
		t.Fatalf("expected at least 2 categories, got %d", len(cats))
	}
	// First category should be Bodyweight (first appearance order)
	if cats[0] != "Bodyweight" {
		t.Errorf("expected first category to be Bodyweight, got %q", cats[0])
	}
	// Cardio should appear after Bodyweight
	found := false
	for _, c := range cats {
		if c == "Cardio" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Cardio' in categories")
	}
}

func TestConfig_ExercisesByCategory(t *testing.T) {
	cfg := config.DefaultConfig()
	bycat := cfg.ExercisesByCategory()

	bw, ok := bycat["Bodyweight"]
	if !ok {
		t.Fatal("expected 'Bodyweight' category")
	}
	if len(bw) != 8 {
		t.Errorf("expected 8 bodyweight exercises, got %d", len(bw))
	}

	cardio, ok := bycat["Cardio"]
	if !ok {
		t.Fatal("expected 'Cardio' category")
	}
	if len(cardio) != 2 {
		t.Errorf("expected 2 cardio exercises, got %d", len(cardio))
	}
}

func TestValidate_NoFields(t *testing.T) {
	cfg := config.Config{
		Exercises: []config.Exercise{
			{Name: "Bad", Category: "Test", Fields: nil},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for exercise with no fields")
	}
}

func TestValidate_NoPrimary(t *testing.T) {
	cfg := config.Config{
		Exercises: []config.Exercise{
			{Name: "Bad", Category: "Test", Fields: []config.Field{
				{Name: "reps", Type: config.FieldTypeInt, Default: 10},
			}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for exercise with no primary field")
	}
}

func TestValidate_MultiplePrimary(t *testing.T) {
	cfg := config.Config{
		Exercises: []config.Exercise{
			{Name: "Bad", Category: "Test", Fields: []config.Field{
				{Name: "reps", Type: config.FieldTypeInt, Default: 10, Primary: true},
				{Name: "rounds", Type: config.FieldTypeInt, Default: 1, Primary: true},
			}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for exercise with multiple primary fields")
	}
}

func TestValidate_InvalidType(t *testing.T) {
	cfg := config.Config{
		Exercises: []config.Exercise{
			{Name: "Bad", Category: "Test", Fields: []config.Field{
				{Name: "reps", Type: "badtype", Default: 10, Primary: true},
			}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for exercise with invalid field type")
	}
}

func TestValidate_DuplicateFieldName(t *testing.T) {
	cfg := config.Config{
		Exercises: []config.Exercise{
			{Name: "Bad", Category: "Test", Fields: []config.Field{
				{Name: "reps", Type: config.FieldTypeInt, Default: 10, Primary: true},
				{Name: "reps", Type: config.FieldTypeInt, Default: 5},
			}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for duplicate field name")
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
		DateFormat: "2006-01-02",
		Exercises: []config.Exercise{
			{
				Name:     "Custom",
				Category: "Test",
				Fields:   []config.Field{{Name: "reps", Type: config.FieldTypeInt, Default: 99, Primary: true}},
			},
		},
	}
	if err := config.Save(path, custom); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	cfg, err := config.LoadOrInit(path)
	if err != nil {
		t.Fatalf("LoadOrInit failed: %v", err)
	}

	if len(cfg.Exercises) != 1 || cfg.Exercises[0].Name != "Custom" {
		t.Errorf("expected custom exercises, got %v", cfg.Exercises)
	}
}
