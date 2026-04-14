package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultRounds int        `yaml:"default_rounds"`
	Exercises     []Exercise `yaml:"exercises"`
	DateFormat    string     `yaml:"date_format"`
}

type Exercise struct {
	Name        string `yaml:"name"`
	DefaultReps int    `yaml:"default_reps"`
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

// DefaultConfig returns a Config populated with default exercises.
func DefaultConfig() Config {
	return Config{
		DefaultRounds: 1,
		DateFormat:    "2006-01-02 15:04",
		Exercises: []Exercise{
			{Name: "Push-ups", DefaultReps: 20},
			{Name: "Squats", DefaultReps: 25},
			{Name: "Pull-ups", DefaultReps: 10},
			{Name: "Dips", DefaultReps: 15},
			{Name: "Lunges", DefaultReps: 20},
			{Name: "Plank (seconds)", DefaultReps: 60},
			{Name: "Burpees", DefaultReps: 10},
			{Name: "Sit-ups", DefaultReps: 30},
		},
	}
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

// LoadOrInit loads the config if it exists, otherwise creates a default one.
func LoadOrInit(path string) (Config, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg = DefaultConfig()
	if err := Save(path, cfg); err != nil {
		return Config{}, fmt.Errorf("saving default config: %w", err)
	}
	return cfg, nil
}
