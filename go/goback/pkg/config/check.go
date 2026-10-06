package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// These types describe file settings only. CLI flags are deliberately excluded:
// a separate Viper reader prevents overrides and coercions from hiding mistakes.
type fileSettings struct {
	ConfirmExec  bool
	EjectOnExit  bool
	ShowProgress bool
	Editor       string
	Profiles     map[string]profileSettings
	Mirror       *MirrorSettings
}

type profileSettings struct {
	Source          string
	Destination     string
	Hostname        string
	BackupMedia     bool
	Rsync           map[string]rsyncSettings
	DailyCompanions []map[string]any
}

type rsyncSettings struct {
	Archive          *bool
	Delete           bool
	DeleteExcluded   bool
	Force            bool
	HardLinks        bool
	IgnoreErrors     bool
	PruneEmptyDirs   bool
	DryRun           bool
	Verbose          bool
	IncludedPatterns []string
	ExcludedPatterns []string
}

// Check reads the selected file without prompting, writing, resolving a hostname,
// or touching backup paths. complete additionally requires every declared backup
// to have its necessary settings; execution checks those for the requested type.
func Check(complete bool) (string, error) {
	path := CfgFile
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, ".goback.json")
	}
	reader := viper.New()
	reader.SetConfigFile(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return path, fmt.Errorf("configuration %s: %w", path, err)
	}
	if err := reader.ReadConfig(bytes.NewReader(data)); err != nil {
		return path, fmt.Errorf("configuration %s: %w", path, err)
	}
	raw := reader.AllSettings()
	// Viper flattens settings and drops empty objects and null values. Preserve
	// those in JSON so empty misspelled keys cannot disappear before validation.
	if strings.EqualFold(filepath.Ext(path), ".json") {
		raw = nil
		if err := json.Unmarshal(data, &raw); err != nil {
			return path, fmt.Errorf("configuration %s: %w", path, err)
		}
	}
	if err := checkSettings(raw, complete); err != nil {
		return path, fmt.Errorf("configuration %s: %w", path, err)
	}
	return path, nil
}

func checkSettings(raw map[string]any, complete bool) error {
	if err := normalizeSettings(raw, ""); err != nil {
		return err
	}
	if _, misplaced := raw["dailycompanions"]; misplaced {
		return fmt.Errorf("move top-level dailyCompanions under profiles.<name>.dailyCompanions")
	}
	var settings fileSettings
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &settings, ErrorUnused: true})
	if err != nil {
		return err
	}
	if err := decoder.Decode(raw); err != nil {
		return err
	}
	var problems []error
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Errorf(format, args...))
	}
	names := make([]string, 0, len(settings.Profiles))
	for name := range settings.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		profile := settings.Profiles[name]
		prefix := "profiles." + name
		entries := make([]any, len(profile.DailyCompanions))
		for i, entry := range profile.DailyCompanions {
			entries[i] = entry
		}
		if _, err := parseCompanions(entries, prefix+".dailyCompanions"); err != nil {
			problems = append(problems, err)
		}
		kinds := make([]string, 0, len(profile.Rsync))
		for kind := range profile.Rsync {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			if kind != "daily" && kind != "weekly" && kind != "monthly" {
				add("%s.rsync.%s: unknown backup type; expected daily, weekly, or monthly", prefix, kind)
			} else if complete && profile.Rsync[kind].Archive == nil {
				add("%s.rsync.%s.archive is required", prefix, kind)
			}
		}
		if complete {
			if profile.Destination == "" {
				add("%s.destination is required", prefix)
			}
			if len(profile.Rsync) == 0 {
				add("%s.rsync must configure at least one of daily, weekly, or monthly", prefix)
			}
			if _, daily := profile.Rsync["daily"]; daily && profile.Source == "" {
				add("%s.source is required for daily backups", prefix)
			}
		}
	}
	if complete {
		if len(settings.Profiles) == 0 && settings.Mirror == nil {
			add("configure at least one backup profile or a mirror")
		}
		if settings.Mirror != nil {
			for _, endpoint := range []struct{ key, value string }{
				{"source", settings.Mirror.Source}, {"destination", settings.Mirror.Destination},
			} {
				value := strings.TrimSpace(endpoint.value)
				if value == "" {
					add("mirror.%s is required", endpoint.key)
				} else if !filepath.IsAbs(value) {
					add("mirror.%s must be an absolute path", endpoint.key)
				}
			}
		}
	}
	return errors.Join(problems...)
}

// Match Viper's case-insensitive keys while rejecting null settings instead of
// silently interpreting them as false, empty, or absent.
func normalizeSettings(raw map[string]any, prefix string) error {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := raw[key]
		lower := strings.ToLower(key)
		if _, exists := raw[lower]; exists && lower != key {
			return fmt.Errorf("%s%s duplicates the case-insensitive key %s", prefix, key, lower)
		}
		if err := normalizeValue(value, prefix+key); err != nil {
			return err
		}
		delete(raw, key)
		raw[lower] = value
	}
	return nil
}

func normalizeValue(value any, path string) error {
	switch value := value.(type) {
	case nil:
		return fmt.Errorf("%s must not be null", path)
	case map[string]any:
		return normalizeSettings(value, path+".")
	case []any:
		for i, entry := range value {
			if err := normalizeValue(entry, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
