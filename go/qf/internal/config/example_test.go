package config_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/qf/internal/config"
)

// Example demonstrates how to use the Config package
func ExampleConfig() {
	// Create a default configuration
	cfg := config.NewDefaultConfig()
	fmt.Printf("Default debounce delay: %dms\n", cfg.Performance.DebounceDelayMs)
	fmt.Printf("Default theme: %s\n", cfg.UI.Theme)

	// Modify some settings
	cfg.Performance.DebounceDelayMs = 100
	cfg.UI.Theme = "dark"

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	fmt.Println("Configuration is valid!")
	// Output:
	// Default debounce delay: 150ms
	// Default theme: default
	// Configuration is valid!
}

// ExampleLoadFromFile demonstrates loading configuration from a file
func ExampleLoadFromFile() {
	// Load configuration from default location or get default if not exists
	cfg, err := config.LoadFromFile()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Printf("Loaded config version: %s\n", cfg.Version)
	fmt.Printf("Cache size: %dMB\n", cfg.Performance.CacheSizeMb)
	// Output will vary based on whether config file exists
}

func TestConfigPathGeneration(t *testing.T) {
	path := config.GetConfigPath()

	// Should end with qf/config.json
	expected := filepath.Join("qf", "config.json")
	if !strings.HasSuffix(path, expected) {
		t.Errorf("Expected path to end with %s, got %s", expected, path)
	}

	t.Logf("Config path: %s", path)
}

func TestConfigMergeExample(t *testing.T) {
	// Start with default config
	base := config.NewDefaultConfig()
	originalDelay := base.Performance.DebounceDelayMs

	// Create partial config with updates
	partial := &config.Config{
		Version: "2.0.0",
		Performance: config.PerformanceConfig{
			DebounceDelayMs: 75, // Custom delay
			CacheSizeMb:     25, // Increased cache
		},
		UI: config.UIConfig{
			Theme:           "light",
			HighlightColors: []string{"purple", "orange"},
		},
		DataMgmt:     config.DataConfig{}, // Empty - won't override
		FileHandling: config.FileConfig{}, // Empty - won't override
	}

	// Merge partial into base
	if err := base.Merge(partial); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Verify merged values
	if base.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", base.Version)
	}

	if base.Performance.DebounceDelayMs != 75 {
		t.Errorf("Expected debounce delay 75, got %d", base.Performance.DebounceDelayMs)
	}

	if base.UI.Theme != "light" {
		t.Errorf("Expected theme light, got %s", base.UI.Theme)
	}

	// Verify unchanged values remain
	if base.DataMgmt.SessionRetentionDays != 30 {
		t.Errorf("Expected session retention unchanged at 30, got %d", base.DataMgmt.SessionRetentionDays)
	}

	t.Logf("Successfully merged config: %dms -> %dms", originalDelay, base.Performance.DebounceDelayMs)
}
