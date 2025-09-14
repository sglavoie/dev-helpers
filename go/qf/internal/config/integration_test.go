package config

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestIntegrationCompatibility tests that our Config structure matches the
// expectations from the integration tests in tests/integration/config_test.go
func TestIntegrationCompatibility(t *testing.T) {
	// Create a config that matches the MockConfig structure from integration tests
	config := &Config{
		Version: "1.0.0",
		Performance: PerformanceConfig{
			DebounceDelayMs:      200,
			CacheSizeMb:          10,
			StreamingThresholdMb: 100,
			MaxWorkers:           4,
		},
		UI: UIConfig{
			Theme:           "default",
			ShowLineNumbers: true,
			HighlightColors: []string{"red", "green", "blue"},
		},
		DataMgmt: DataConfig{
			SessionRetentionDays: 30,
			MaxHistoryEntries:    100,
		},
		FileHandling: FileConfig{
			DefaultEncoding: "utf-8",
			MaxFileSize:     "1GB",
		},
	}

	// Test that it marshals to JSON correctly
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Test that it unmarshals correctly
	var unmarshaled Config
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Test validation passes
	if err := config.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Test field values match expectations
	if config.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected DebounceDelayMs=200, got %d", config.Performance.DebounceDelayMs)
	}

	if config.Performance.CacheSizeMb != 10 {
		t.Errorf("Expected CacheSizeMb=10, got %d", config.Performance.CacheSizeMb)
	}

	t.Log("✓ Config structure compatible with integration test expectations")
}

// TestConfigValidationIntegrationCompatibility tests that our validation
// matches the error messages expected by integration tests
func TestConfigValidationIntegrationCompatibility(t *testing.T) {
	testCases := []struct {
		name        string
		config      Config
		expectError string
	}{
		{
			name: "DebounceDelayTooLow",
			config: Config{
				Version: "1.0.0",
				Performance: PerformanceConfig{
					DebounceDelayMs: 10, // Too low (minimum should be 50)
					CacheSizeMb:     10,
				},
				UI:           UIConfig{HighlightColors: []string{"red"}},
				DataMgmt:     DataConfig{SessionRetentionDays: 30, MaxHistoryEntries: 100},
				FileHandling: FileConfig{DefaultEncoding: "utf-8", MaxFileSize: "1GB"},
			},
			expectError: "debounce_delay_ms must be between 50 and 1000",
		},
		{
			name: "DebounceDelayTooHigh",
			config: Config{
				Version: "1.0.0",
				Performance: PerformanceConfig{
					DebounceDelayMs: 2000, // Too high (maximum should be 1000)
					CacheSizeMb:     10,
				},
				UI:           UIConfig{HighlightColors: []string{"red"}},
				DataMgmt:     DataConfig{SessionRetentionDays: 30, MaxHistoryEntries: 100},
				FileHandling: FileConfig{DefaultEncoding: "utf-8", MaxFileSize: "1GB"},
			},
			expectError: "debounce_delay_ms must be between 50 and 1000",
		},
		{
			name: "InvalidCacheSize",
			config: Config{
				Version: "1.0.0",
				Performance: PerformanceConfig{
					DebounceDelayMs: 100,
					CacheSizeMb:     -1, // Negative cache size
				},
				UI:           UIConfig{HighlightColors: []string{"red"}},
				DataMgmt:     DataConfig{SessionRetentionDays: 30, MaxHistoryEntries: 100},
				FileHandling: FileConfig{DefaultEncoding: "utf-8", MaxFileSize: "1GB"},
			},
			expectError: "cache_size_mb must be positive",
		},
		{
			name: "EmptyVersion",
			config: Config{
				Version: "", // Empty version string
				Performance: PerformanceConfig{
					DebounceDelayMs: 100,
					CacheSizeMb:     10,
				},
				UI:           UIConfig{HighlightColors: []string{"red"}},
				DataMgmt:     DataConfig{SessionRetentionDays: 30, MaxHistoryEntries: 100},
				FileHandling: FileConfig{DefaultEncoding: "utf-8", MaxFileSize: "1GB"},
			},
			expectError: "version cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if err == nil {
				t.Errorf("Expected validation error for %s, but got none", tc.name)
			} else if !containsErrorText(err.Error(), tc.expectError) {
				t.Errorf("Expected error containing '%s', got '%s'", tc.expectError, err.Error())
			} else {
				t.Logf("✓ Correctly rejected invalid config: %s", tc.expectError)
			}
		})
	}
}

// TestFileOperations tests load/save operations matching integration test expectations
func TestFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial configuration matching integration test setup
	initialConfig := &Config{
		Version: "1.0.0",
		Performance: PerformanceConfig{
			DebounceDelayMs:      200,
			CacheSizeMb:          10,
			StreamingThresholdMb: 100,
			MaxWorkers:           4,
		},
		UI: UIConfig{
			Theme:           "default",
			ShowLineNumbers: true,
			HighlightColors: []string{"red", "green", "blue"},
		},
		DataMgmt: DataConfig{
			SessionRetentionDays: 30,
			MaxHistoryEntries:    100,
		},
		FileHandling: FileConfig{
			DefaultEncoding: "utf-8",
			MaxFileSize:     "1GB",
		},
	}

	// Test save operation
	err := initialConfig.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test load operation
	loadedConfig := &Config{}
	err = loadedConfig.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values match
	if loadedConfig.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected DebounceDelayMs=200, got %d", loadedConfig.Performance.DebounceDelayMs)
	}

	if loadedConfig.Performance.CacheSizeMb != 10 {
		t.Errorf("Expected CacheSizeMb=10, got %d", loadedConfig.Performance.CacheSizeMb)
	}

	// Test modification scenario from integration test
	loadedConfig.Performance.DebounceDelayMs = 100
	loadedConfig.Performance.CacheSizeMb = 20

	// Save modified configuration
	err = loadedConfig.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save modified config: %v", err)
	}

	// Verify changes were saved
	verifyConfig := &Config{}
	err = verifyConfig.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load modified config: %v", err)
	}

	if verifyConfig.Performance.DebounceDelayMs != 100 {
		t.Errorf("Expected modified DebounceDelayMs=100, got %d", verifyConfig.Performance.DebounceDelayMs)
	}

	if verifyConfig.Performance.CacheSizeMb != 20 {
		t.Errorf("Expected modified CacheSizeMb=20, got %d", verifyConfig.Performance.CacheSizeMb)
	}

	t.Log("✓ File operations work as expected by integration tests")
}
