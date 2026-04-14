package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type FieldType string

const (
	FieldTypeInt    FieldType = "int"
	FieldTypeFloat  FieldType = "float"
	FieldTypeString FieldType = "string"
)

type Field struct {
	Name         string    `yaml:"name"`
	Type         FieldType `yaml:"type"`
	Default      any       `yaml:"default"`
	Primary      bool      `yaml:"primary,omitempty"`
	MultipliedBy string    `yaml:"multiplied_by,omitempty"`
}

type Exercise struct {
	Name     string  `yaml:"name"`
	Category string  `yaml:"category"`
	Fields   []Field `yaml:"fields"`
}

type Config struct {
	Exercises  []Exercise `yaml:"exercises"`
	DateFormat string     `yaml:"date_format"`
}

// PrimaryField returns the primary field for this exercise, or nil if none.
func (e Exercise) PrimaryField() *Field {
	for i := range e.Fields {
		if e.Fields[i].Primary {
			return &e.Fields[i]
		}
	}
	return nil
}

// FieldByName returns the field with the given name, or nil if not found.
func (e Exercise) FieldByName(name string) *Field {
	for i := range e.Fields {
		if e.Fields[i].Name == name {
			return &e.Fields[i]
		}
	}
	return nil
}

// Categories returns the list of categories in order of first appearance.
func (cfg Config) Categories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, e := range cfg.Exercises {
		if !seen[e.Category] {
			seen[e.Category] = true
			cats = append(cats, e.Category)
		}
	}
	return cats
}

// ExercisesByCategory returns exercises grouped by category.
func (cfg Config) ExercisesByCategory() map[string][]Exercise {
	m := make(map[string][]Exercise)
	for _, e := range cfg.Exercises {
		m[e.Category] = append(m[e.Category], e)
	}
	return m
}

// Validate checks the config for correctness.
func (cfg Config) Validate() error {
	validTypes := map[FieldType]bool{
		FieldTypeInt:    true,
		FieldTypeFloat:  true,
		FieldTypeString: true,
	}
	for _, ex := range cfg.Exercises {
		if len(ex.Fields) == 0 {
			return fmt.Errorf("exercise %q: must have at least one field", ex.Name)
		}
		primaryCount := 0
		seen := make(map[string]bool)
		for _, f := range ex.Fields {
			if seen[f.Name] {
				return fmt.Errorf("exercise %q: duplicate field name %q", ex.Name, f.Name)
			}
			seen[f.Name] = true
			if !validTypes[f.Type] {
				return fmt.Errorf("exercise %q: field %q has invalid type %q", ex.Name, f.Name, f.Type)
			}
			if f.Primary {
				primaryCount++
			}
		}
		if primaryCount != 1 {
			return fmt.Errorf("exercise %q: must have exactly one primary field, got %d", ex.Name, primaryCount)
		}
	}
	return nil
}

// bodyweightExercise builds a standard bodyweight exercise with reps+rounds fields.
func bodyweightExercise(name string, defaultReps int) Exercise {
	return Exercise{
		Name:     name,
		Category: "Bodyweight",
		Fields: []Field{
			{Name: "reps", Type: FieldTypeInt, Default: defaultReps, Primary: true, MultipliedBy: "rounds"},
			{Name: "rounds", Type: FieldTypeInt, Default: 1},
		},
	}
}

// DefaultConfig returns a Config populated with default exercises.
func DefaultConfig() Config {
	return Config{
		DateFormat: "2006-01-02 15:04",
		Exercises: []Exercise{
			bodyweightExercise("Push-ups", 20),
			bodyweightExercise("Squats", 25),
			bodyweightExercise("Pull-ups", 10),
			bodyweightExercise("Dips", 15),
			bodyweightExercise("Lunges", 20),
			{
				Name:     "Plank",
				Category: "Bodyweight",
				Fields: []Field{
					{Name: "seconds", Type: FieldTypeInt, Default: 60, Primary: true, MultipliedBy: "rounds"},
					{Name: "rounds", Type: FieldTypeInt, Default: 1},
				},
			},
			bodyweightExercise("Burpees", 10),
			bodyweightExercise("Sit-ups", 30),
			{
				Name:     "Rowing",
				Category: "Cardio",
				Fields: []Field{
					{Name: "duration_min", Type: FieldTypeFloat, Default: 20.0, Primary: true},
					{Name: "resistance", Type: FieldTypeInt, Default: 5},
				},
			},
			{
				Name:     "Running",
				Category: "Cardio",
				Fields: []Field{
					{Name: "duration_min", Type: FieldTypeFloat, Default: 30.0, Primary: true},
					{Name: "distance_km", Type: FieldTypeFloat, Default: 5.0},
				},
			},
		},
	}
}

// ConfigDir returns the config directory (~/.config/hr/), creating it if missing.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "hr")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating config dir: %w", err)
	}
	return dir, nil
}

// ConfigPath returns the path to the YAML config file.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hr.yaml"), nil
}

// DataPath returns the path to the CSV data file.
func DataPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hr.csv"), nil
}

// Load reads and unmarshals the YAML config file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// Save marshals and writes cfg to the YAML config file.
func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// DefaultConfigYAML returns the default config as a documented YAML string.
func DefaultConfigYAML() string {
	return `# hr — Health Records configuration
#
# Each exercise has:
#   name:     Display name (used in TUI selector and CSV logs)
#   category: Grouping label (e.g. Bodyweight, Cardio) — shown in stats and TUI
#   fields:   List of data fields prompted when logging the exercise
#
# Field properties:
#   name:          Identifier stored in CSV data (e.g. reps, duration_min)
#   type:          One of: int, float, string
#   default:       Default value shown in prompts (press Enter to accept)
#   primary:       true/false — exactly one field per exercise must be primary;
#                  this is the value aggregated in stats (e.g. total reps, total minutes)
#   multiplied_by: (optional) name of another field whose value multiplies the primary
#                  during stats aggregation (e.g. reps multiplied_by rounds gives total reps)
#
# Tips:
#   - Add your own exercises by following the patterns below
#   - Categories are free-form strings — invent your own (e.g. Flexibility, Swimming)
#   - The --field / -f flag on the CLI can set any field: hr -e rowing -f resistance=7
#   - String fields are useful for metadata: shoes, weather, location, etc.

exercises:

    # ── Bodyweight ──────────────────────────────────────────────

    - name: Push-ups
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 20, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Squats
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 25, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Pull-ups
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 10, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Dips
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 15, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Lunges
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 20, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Plank
      category: Bodyweight
      fields:
        - {name: seconds, type: int, default: 60, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Burpees
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 10, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    - name: Sit-ups
      category: Bodyweight
      fields:
        - {name: reps, type: int, default: 30, primary: true, multiplied_by: rounds}
        - {name: rounds, type: int, default: 1}

    # ── Cardio ──────────────────────────────────────────────────

    - name: Rowing
      category: Cardio
      fields:
        - {name: duration_min, type: float, default: 20.0, primary: true}
        - {name: resistance, type: int, default: 5}

    - name: Running
      category: Cardio
      fields:
        - {name: duration_min, type: float, default: 30.0, primary: true}
        - {name: distance_km, type: float, default: 5.0}

date_format: "2006-01-02 15:04"
`
}

// LoadOrInit loads the config if it exists, otherwise creates a default one.
// Returns an error if the loaded config fails validation.
func LoadOrInit(path string) (Config, error) {
	cfg, err := Load(path)
	if err == nil {
		if err := cfg.Validate(); err != nil {
			return Config{}, fmt.Errorf("invalid config: %w", err)
		}
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if err := os.WriteFile(path, []byte(DefaultConfigYAML()), 0644); err != nil {
		return Config{}, fmt.Errorf("saving default config: %w", err)
	}
	cfg, err = Load(path)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}
