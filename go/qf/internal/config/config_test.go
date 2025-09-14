package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()

	// Test that default values match project specifications
	if config.Version == "" {
		t.Error("Version should not be empty")
	}

	if config.Performance.DebounceDelayMs != 150 {
		t.Errorf("Expected DebounceDelayMs=150, got %d", config.Performance.DebounceDelayMs)
	}

	if config.Performance.CacheSizeMb != 10 {
		t.Errorf("Expected CacheSizeMb=10, got %d", config.Performance.CacheSizeMb)
	}

	if config.Performance.StreamingThresholdMb != 100 {
		t.Errorf("Expected StreamingThresholdMb=100, got %d", config.Performance.StreamingThresholdMb)
	}

	if config.Performance.MaxWorkers != 4 {
		t.Errorf("Expected MaxWorkers=4, got %d", config.Performance.MaxWorkers)
	}

	// Test validation passes on default config
	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name         string
		modifyConfig func(*Config)
		expectError  string
	}{
		{
			name: "DebounceDelayTooLow",
			modifyConfig: func(c *Config) {
				c.Performance.DebounceDelayMs = 10
			},
			expectError: "debounce_delay_ms must be between 50 and 1000",
		},
		{
			name: "DebounceDelayTooHigh",
			modifyConfig: func(c *Config) {
				c.Performance.DebounceDelayMs = 2000
			},
			expectError: "debounce_delay_ms must be between 50 and 1000",
		},
		{
			name: "InvalidCacheSize",
			modifyConfig: func(c *Config) {
				c.Performance.CacheSizeMb = -1
			},
			expectError: "cache_size_mb must be positive",
		},
		{
			name: "EmptyVersion",
			modifyConfig: func(c *Config) {
				c.Version = ""
			},
			expectError: "version cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewDefaultConfig()
			tt.modifyConfig(config)

			err := config.Validate()
			if err == nil {
				t.Errorf("Expected validation error for %s", tt.name)
			} else if !containsErrorText(err.Error(), tt.expectError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectError, err.Error())
			}
		})
	}
}

func TestConfigLoadSave(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	// Create and save config
	config := NewDefaultConfig()
	config.Performance.DebounceDelayMs = 200
	config.UI.Theme = "dark"

	err := config.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loadedConfig := &Config{}
	err = loadedConfig.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify values
	if loadedConfig.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected DebounceDelayMs=200, got %d", loadedConfig.Performance.DebounceDelayMs)
	}

	if loadedConfig.UI.Theme != "dark" {
		t.Errorf("Expected Theme=dark, got %s", loadedConfig.UI.Theme)
	}
}

func TestConfigMerge(t *testing.T) {
	base := NewDefaultConfig()
	base.Performance.DebounceDelayMs = 100

	other := &Config{
		Version: "2.0.0",
		Performance: PerformanceConfig{
			DebounceDelayMs: 200,
			CacheSizeMb:     20,
		},
		UI: UIConfig{
			Theme:           "light",
			ShowLineNumbers: false,
			HighlightColors: []string{"red"},
		},
		DataMgmt: DataConfig{
			SessionRetentionDays: 60,
			MaxHistoryEntries:    200,
		},
		FileHandling: FileConfig{
			DefaultEncoding: "utf-16",
			MaxFileSize:     "2GB",
		},
	}

	err := base.Merge(other)
	if err != nil {
		t.Fatalf("Failed to merge configs: %v", err)
	}

	// Verify merged values
	if base.Version != "2.0.0" {
		t.Errorf("Expected Version=2.0.0, got %s", base.Version)
	}
	if base.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected DebounceDelayMs=200, got %d", base.Performance.DebounceDelayMs)
	}
	if base.UI.Theme != "light" {
		t.Errorf("Expected Theme=light, got %s", base.UI.Theme)
	}
}

func TestJSONMarshaling(t *testing.T) {
	config := NewDefaultConfig()
	config.Performance.DebounceDelayMs = 300

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Unmarshal from JSON
	var unmarshaled Config
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify values preserved
	if unmarshaled.Performance.DebounceDelayMs != 300 {
		t.Errorf("Expected DebounceDelayMs=300, got %d", unmarshaled.Performance.DebounceDelayMs)
	}

	// Verify JSON structure matches test expectations
	if unmarshaled.DataMgmt.SessionRetentionDays != 30 {
		t.Errorf("Expected SessionRetentionDays=30, got %d", unmarshaled.DataMgmt.SessionRetentionDays)
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()

	// Verify path is not empty
	if path == "" {
		t.Error("GetConfigPath should not return empty string")
	}

	// Verify path ends with expected file name
	if filepath.Base(path) != "config.json" {
		t.Errorf("Expected path to end with config.json, got %s", path)
	}

	// Verify path contains qf directory
	if !containsPath(path, "qf") {
		t.Errorf("Expected path to contain 'qf' directory, got %s", path)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Test loading non-existent file returns default config
	config, err := LoadFromFile("/nonexistent/path/config.json")
	if err != nil {
		t.Errorf("Loading non-existent file should return default config, got error: %v", err)
	}

	if config.Performance.DebounceDelayMs != 150 {
		t.Errorf("Expected default DebounceDelayMs=150, got %d", config.Performance.DebounceDelayMs)
	}

	// Test loading existing file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create test config file
	testConfig := NewDefaultConfig()
	testConfig.Performance.DebounceDelayMs = 250
	err = testConfig.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Load it
	loadedConfig, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedConfig.Performance.DebounceDelayMs != 250 {
		t.Errorf("Expected loaded DebounceDelayMs=250, got %d", loadedConfig.Performance.DebounceDelayMs)
	}
}

// Helper functions
func containsErrorText(actual, expected string) bool {
	return strings.Contains(actual, expected)
}

func containsPath(path, segment string) bool {
	return filepath.Base(filepath.Dir(path)) == segment ||
		filepath.Base(filepath.Dir(filepath.Dir(path))) == segment
}
