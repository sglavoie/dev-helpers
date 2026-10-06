package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var validDays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// configPathOverride is used in tests to redirect the config path.
var configPathOverride string

type Config struct {
	WorkDays  []string `json:"work_days"`
	ResetTime string   `json:"reset_time"`
}

func DefaultConfig() *Config {
	return &Config{
		WorkDays:  []string{"mon", "tue", "wed", "thu", "fri", "sat"},
		ResetTime: "Thu 2:59 PM",
	}
}

func ConfigPath() string {
	if configPathOverride != "" {
		return configPathOverride
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/claude-usage/config.json"
	}
	return filepath.Join(home, ".config", "claude-usage", "config.json")
}

func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			if saveErr := Save(cfg); saveErr != nil {
				return nil, fmt.Errorf("creating default config: %w", saveErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func ValidateDays(days []string) error {
	if len(days) == 0 {
		return errors.New("work days cannot be empty")
	}
	seen := make(map[string]bool)
	for _, d := range days {
		if !slices.Contains(validDays, d) {
			return fmt.Errorf("invalid day %q: must be one of %s", d, strings.Join(validDays, ", "))
		}
		if seen[d] {
			return fmt.Errorf("duplicate day: %q", d)
		}
		seen[d] = true
	}
	return nil
}

func ValidateResetTime(s string) error {
	_, err := time.Parse("Mon 3:04 PM", s)
	if err != nil {
		return fmt.Errorf("invalid reset time %q: expected format like \"Thu 2:59 PM\"", s)
	}
	return nil
}

var weekdayAbbr = map[string]time.Weekday{
	"Sun": time.Sunday, "Mon": time.Monday, "Tue": time.Tuesday,
	"Wed": time.Wednesday, "Thu": time.Thursday, "Fri": time.Friday,
	"Sat": time.Saturday,
}

func ParseResetTime(s string) (time.Weekday, int, int) {
	t, err := time.Parse("Mon 3:04 PM", s)
	if err != nil {
		panic(fmt.Sprintf("ParseResetTime called with invalid string %q: %v", s, err))
	}
	// t.Weekday() is unreliable without a full date (zero date is Saturday).
	// Extract the weekday abbreviation from the string directly.
	abbr := strings.SplitN(s, " ", 2)[0]
	wd, ok := weekdayAbbr[abbr]
	if !ok {
		panic(fmt.Sprintf("ParseResetTime: unrecognized weekday abbreviation %q", abbr))
	}
	return wd, t.Hour(), t.Minute()
}
