package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MockComponent implements ComponentRegistry for testing
type MockComponent struct {
	id                  string
	updateCallCount     int
	lastNewConfig       *Config
	lastOldConfig       *Config
	lastUpdatedSections []string
}

func (mc *MockComponent) HandleConfigUpdate(newConfig, oldConfig *Config, updatedSections []string) tea.Cmd {
	mc.updateCallCount++
	mc.lastNewConfig = newConfig
	mc.lastOldConfig = oldConfig
	mc.lastUpdatedSections = updatedSections
	return nil
}

func (mc *MockComponent) GetComponentID() string {
	return mc.id
}

// Test helper functions
func createTempConfigFile(t *testing.T, config *Config) string {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	return configPath
}

func TestNewConfigManager(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		options     []ConfigManagerOption
		expectError bool
	}{
		{
			name:       "valid config path",
			configPath: "/tmp/test-config.json",
			options:    nil,
		},
		{
			name:        "empty config path",
			configPath:  "",
			expectError: true,
		},
		{
			name:       "with custom logger",
			configPath: "/tmp/test-config.json",
			options: []ConfigManagerOption{
				WithLogger(log.New(os.Stdout, "[TEST] ", log.LstdFlags)),
			},
		},
		{
			name:       "with error callback",
			configPath: "/tmp/test-config.json",
			options: []ConfigManagerOption{
				WithErrorCallback(func(err error) {
					// Mock error handler
				}),
			},
		},
		{
			name:       "with debounce delay",
			configPath: "/tmp/test-config.json",
			options: []ConfigManagerOption{
				WithDebounceDelay(500 * time.Millisecond),
			},
		},
		{
			name:       "with polling fallback",
			configPath: "/tmp/test-config.json",
			options: []ConfigManagerOption{
				WithPollingFallback(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a valid config file for non-error cases
			if !tt.expectError {
				defaultConfig := NewDefaultConfig()
				tt.configPath = createTempConfigFile(t, defaultConfig)
			}

			cm, err := NewConfigManager(tt.configPath, tt.options...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if cm == nil {
				t.Error("Expected ConfigManager instance but got nil")
				return
			}

			// Verify configuration was loaded
			config := cm.GetCurrentConfig()
			if config == nil {
				t.Error("Expected configuration to be loaded")
			}

			// Clean up
			cm.Stop()
		})
	}
}

func TestConfigManager_StartStop(t *testing.T) {
	defaultConfig := NewDefaultConfig()
	configPath := createTempConfigFile(t, defaultConfig)

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}

	// Test initial state
	if cm.IsStarted() {
		t.Error("ConfigManager should not be started initially")
	}

	// Test start
	if err := cm.Start(); err != nil {
		t.Errorf("Failed to start ConfigManager: %v", err)
	}

	if !cm.IsStarted() {
		t.Error("ConfigManager should be started after calling Start()")
	}

	// Test double start
	if err := cm.Start(); err == nil {
		t.Error("Expected error when starting already started ConfigManager")
	}

	// Test stop
	if err := cm.Stop(); err != nil {
		t.Errorf("Failed to stop ConfigManager: %v", err)
	}

	if cm.IsStarted() {
		t.Error("ConfigManager should not be started after calling Stop()")
	}

	// Test double stop (should not error)
	if err := cm.Stop(); err != nil {
		t.Errorf("Unexpected error on double stop: %v", err)
	}
}

func TestConfigManager_ComponentRegistration(t *testing.T) {
	defaultConfig := NewDefaultConfig()
	configPath := createTempConfigFile(t, defaultConfig)

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Test initial state
	if count := cm.GetComponentCount(); count != 0 {
		t.Errorf("Expected 0 components initially, got %d", count)
	}

	// Test component registration
	component1 := &MockComponent{id: "test-component-1"}
	if err := cm.RegisterComponent(component1); err != nil {
		t.Errorf("Failed to register component: %v", err)
	}

	if count := cm.GetComponentCount(); count != 1 {
		t.Errorf("Expected 1 component after registration, got %d", count)
	}

	// Test duplicate registration
	if err := cm.RegisterComponent(component1); err == nil {
		t.Error("Expected error when registering duplicate component")
	}

	// Test nil component registration
	if err := cm.RegisterComponent(nil); err == nil {
		t.Error("Expected error when registering nil component")
	}

	// Test component with empty ID
	emptyIDComponent := &MockComponent{id: ""}
	if err := cm.RegisterComponent(emptyIDComponent); err == nil {
		t.Error("Expected error when registering component with empty ID")
	}

	// Test multiple component registration
	component2 := &MockComponent{id: "test-component-2"}
	if err := cm.RegisterComponent(component2); err != nil {
		t.Errorf("Failed to register second component: %v", err)
	}

	if count := cm.GetComponentCount(); count != 2 {
		t.Errorf("Expected 2 components after second registration, got %d", count)
	}

	// Test component unregistration
	if err := cm.UnregisterComponent("test-component-1"); err != nil {
		t.Errorf("Failed to unregister component: %v", err)
	}

	if count := cm.GetComponentCount(); count != 1 {
		t.Errorf("Expected 1 component after unregistration, got %d", count)
	}

	// Test unregistering non-existent component
	if err := cm.UnregisterComponent("non-existent"); err == nil {
		t.Error("Expected error when unregistering non-existent component")
	}
}

func TestConfigManager_ConfigurationValidation(t *testing.T) {
	defaultConfig := NewDefaultConfig()
	configPath := createTempConfigFile(t, defaultConfig)

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Test valid configuration validation
	validConfig := NewDefaultConfig()
	if err := cm.ValidateConfig(validConfig); err != nil {
		t.Errorf("Valid configuration failed validation: %v", err)
	}

	// Test invalid configuration validation
	invalidConfig := NewDefaultConfig()
	invalidConfig.Performance.DebounceDelayMs = 0 // Invalid value

	if err := cm.ValidateConfig(invalidConfig); err == nil {
		t.Error("Expected error when validating invalid configuration")
	}

	// Test nil configuration validation
	if err := cm.ValidateConfig(nil); err == nil {
		t.Error("Expected error when validating nil configuration")
	}
}

func TestConfigManager_ConfigurationReload(t *testing.T) {
	// Create initial config
	initialConfig := NewDefaultConfig()
	initialConfig.Performance.DebounceDelayMs = 100
	configPath := createTempConfigFile(t, initialConfig)

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Register a mock component
	component := &MockComponent{id: "test-component"}
	if err := cm.RegisterComponent(component); err != nil {
		t.Fatalf("Failed to register component: %v", err)
	}

	// Start the manager
	if err := cm.Start(); err != nil {
		t.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Verify initial configuration
	currentConfig := cm.GetCurrentConfig()
	if currentConfig.Performance.DebounceDelayMs != 100 {
		t.Errorf("Expected initial debounce delay 100, got %d", currentConfig.Performance.DebounceDelayMs)
	}

	// Update configuration file
	updatedConfig := NewDefaultConfig()
	updatedConfig.Performance.DebounceDelayMs = 200
	updatedConfig.UI.Theme = "dark"

	data, err := json.MarshalIndent(updatedConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal updated config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// Trigger manual reload (simulating file change detection)
	if err := cm.ReloadConfigManually(); err != nil {
		t.Errorf("Failed to manually reload configuration: %v", err)
	}

	// Verify configuration was updated
	newConfig := cm.GetCurrentConfig()
	if newConfig.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected updated debounce delay 200, got %d", newConfig.Performance.DebounceDelayMs)
	}

	if newConfig.UI.Theme != "dark" {
		t.Errorf("Expected updated theme 'dark', got %q", newConfig.UI.Theme)
	}

	// Verify component was notified
	if component.updateCallCount == 0 {
		t.Error("Expected component to be notified of configuration update")
	}

	if len(component.lastUpdatedSections) == 0 {
		t.Error("Expected updated sections to be provided to component")
	}

	// Verify expected sections were updated
	expectedSections := []string{"performance", "ui"}
	for _, expectedSection := range expectedSections {
		found := false
		for _, actualSection := range component.lastUpdatedSections {
			if actualSection == expectedSection {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected section %q to be in updated sections %v", expectedSection, component.lastUpdatedSections)
		}
	}
}

func TestConfigManager_InvalidConfigurationReload(t *testing.T) {
	// Create initial valid config
	initialConfig := NewDefaultConfig()
	configPath := createTempConfigFile(t, initialConfig)

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Register a mock component
	component := &MockComponent{id: "test-component"}
	if err := cm.RegisterComponent(component); err != nil {
		t.Fatalf("Failed to register component: %v", err)
	}

	if err := cm.Start(); err != nil {
		t.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Store initial update count
	initialUpdateCount := component.updateCallCount

	// Write invalid configuration
	invalidConfigJSON := `{
		"version": "1.0.0",
		"performance": {
			"debounce_delay_ms": 0,
			"cache_size_mb": -1
		}
	}`

	if err := os.WriteFile(configPath, []byte(invalidConfigJSON), 0644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// Attempt manual reload
	err = cm.ReloadConfigManually()
	if err == nil {
		t.Error("Expected error when reloading invalid configuration")
	}

	// Verify component was not notified (configuration should remain unchanged)
	if component.updateCallCount != initialUpdateCount {
		t.Error("Component should not be notified when invalid configuration is rejected")
	}

	// Verify configuration remains unchanged
	currentConfig := cm.GetCurrentConfig()
	if err := currentConfig.Validate(); err != nil {
		t.Error("Current configuration should still be valid after rejected invalid reload")
	}
}

func TestConfigManager_Rollback(t *testing.T) {
	// Create initial config
	initialConfig := NewDefaultConfig()
	initialConfig.Performance.DebounceDelayMs = 100
	configPath := createTempConfigFile(t, initialConfig)

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Test rollback with no previous config
	if err := cm.RollbackToPrevious(); err == nil {
		t.Error("Expected error when rolling back with no previous configuration")
	}

	if err := cm.Start(); err != nil {
		t.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Update configuration
	updatedConfig := NewDefaultConfig()
	updatedConfig.Performance.DebounceDelayMs = 200

	data, err := json.MarshalIndent(updatedConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal updated config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	if err := cm.ReloadConfigManually(); err != nil {
		t.Fatalf("Failed to reload configuration: %v", err)
	}

	// Verify new configuration is active
	currentConfig := cm.GetCurrentConfig()
	if currentConfig.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected debounce delay 200, got %d", currentConfig.Performance.DebounceDelayMs)
	}

	// Test rollback
	if err := cm.RollbackToPrevious(); err != nil {
		t.Errorf("Failed to rollback configuration: %v", err)
	}

	// Verify previous configuration is now active
	rolledBackConfig := cm.GetCurrentConfig()
	if rolledBackConfig.Performance.DebounceDelayMs != 100 {
		t.Errorf("Expected rolled back debounce delay 100, got %d", rolledBackConfig.Performance.DebounceDelayMs)
	}

	// Verify previous config is available for another rollback
	previousConfig := cm.GetPreviousConfig()
	if previousConfig == nil {
		t.Error("Expected previous configuration to be available after rollback")
	}

	if previousConfig.Performance.DebounceDelayMs != 200 {
		t.Errorf("Expected previous config debounce delay 200, got %d", previousConfig.Performance.DebounceDelayMs)
	}
}

func TestConfigManager_GetStats(t *testing.T) {
	defaultConfig := NewDefaultConfig()
	configPath := createTempConfigFile(t, defaultConfig)

	cm, err := NewConfigManager(configPath,
		WithDebounceDelay(500*time.Millisecond),
		WithPollingFallback(),
	)
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Register components
	component1 := &MockComponent{id: "component-1"}
	component2 := &MockComponent{id: "component-2"}
	cm.RegisterComponent(component1)
	cm.RegisterComponent(component2)

	if err := cm.Start(); err != nil {
		t.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Get stats
	stats := cm.GetStats()

	// Verify stats
	if stats.ConfigPath != configPath {
		t.Errorf("Expected config path %q, got %q", configPath, stats.ConfigPath)
	}

	if !stats.Started {
		t.Error("Expected Started to be true")
	}

	if !stats.UsePolling {
		t.Error("Expected UsePolling to be true")
	}

	if stats.ComponentCount != 2 {
		t.Errorf("Expected ComponentCount 2, got %d", stats.ComponentCount)
	}

	if stats.DebounceDelay != "500ms" {
		t.Errorf("Expected DebounceDelay '500ms', got %q", stats.DebounceDelay)
	}

	if stats.LastError != "" {
		t.Errorf("Expected no last error, got %q", stats.LastError)
	}
}

func TestConfigManager_GetUpdatedSections(t *testing.T) {
	cm := &ConfigManager{}

	// Test with nil old config (initial load)
	newConfig := NewDefaultConfig()
	sections := cm.getUpdatedSections(nil, newConfig)

	expectedAllSections := []string{"performance", "ui", "data_management", "file_handling"}
	if len(sections) != len(expectedAllSections) {
		t.Errorf("Expected %d sections for initial load, got %d", len(expectedAllSections), len(sections))
	}

	// Test with identical configs
	oldConfig := NewDefaultConfig()
	identicalConfig := NewDefaultConfig()
	sections = cm.getUpdatedSections(oldConfig, identicalConfig)

	if len(sections) != 0 {
		t.Errorf("Expected 0 sections for identical configs, got %d: %v", len(sections), sections)
	}

	// Test with performance changes
	performanceChanged := NewDefaultConfig()
	performanceChanged.Performance.DebounceDelayMs = 999
	sections = cm.getUpdatedSections(oldConfig, performanceChanged)

	if len(sections) != 1 || sections[0] != "performance" {
		t.Errorf("Expected only 'performance' section, got %v", sections)
	}

	// Test with UI changes
	uiChanged := NewDefaultConfig()
	uiChanged.UI.Theme = "dark"
	sections = cm.getUpdatedSections(oldConfig, uiChanged)

	if len(sections) != 1 || sections[0] != "ui" {
		t.Errorf("Expected only 'ui' section, got %v", sections)
	}

	// Test with multiple changes
	multipleChanged := NewDefaultConfig()
	multipleChanged.Performance.CacheSizeMb = 999
	multipleChanged.DataMgmt.MaxHistoryEntries = 999
	multipleChanged.Version = "2.0.0"
	sections = cm.getUpdatedSections(oldConfig, multipleChanged)

	expectedSections := []string{"performance", "data_management", "version"}
	if len(sections) != len(expectedSections) {
		t.Errorf("Expected %d sections, got %d", len(expectedSections), len(sections))
	}

	// Verify all expected sections are present
	for _, expected := range expectedSections {
		found := false
		for _, actual := range sections {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected section %q not found in %v", expected, sections)
		}
	}
}

func TestConfigManager_ErrorHandling(t *testing.T) {
	// Create a read-only directory to simulate permission issues
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0444); err != nil {
		t.Fatalf("Failed to create read-only directory: %v", err)
	}

	// Try to create ConfigManager with path that cannot be written to
	invalidPath := filepath.Join(readOnlyDir, "config.json")

	// Note: LoadFromFile will return default config when file doesn't exist,
	// so let's create a corrupt file instead
	corruptConfig := `{"version": "1.0.0", "performance": {"invalid_json": true`
	if err := os.WriteFile(invalidPath, []byte(corruptConfig), 0644); err != nil {
		// If we can't write to the readonly dir (which is expected),
		// let's test with a corrupt file in a writable location instead
		writablePath := filepath.Join(tempDir, "corrupt.json")
		if err := os.WriteFile(writablePath, []byte(corruptConfig), 0644); err != nil {
			t.Fatalf("Failed to write corrupt config file: %v", err)
		}
		invalidPath = writablePath
	}

	cm, err := NewConfigManager(invalidPath)
	if err == nil {
		t.Error("Expected error when creating ConfigManager with corrupt config file")
		if cm != nil {
			cm.Stop()
		}
		return
	}

	// Verify the error message is appropriate
	if !strings.Contains(err.Error(), "failed to load initial configuration") {
		t.Errorf("Expected error about initial configuration loading, got: %v", err)
	}

	// Test error callback
	var capturedError error
	errorCallback := func(err error) {
		capturedError = err
	}

	// Create valid config for testing error callback
	defaultConfig := NewDefaultConfig()
	configPath := createTempConfigFile(t, defaultConfig)

	cm, err = NewConfigManager(configPath, WithErrorCallback(errorCallback))
	if err != nil {
		t.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	// Simulate an error by calling handleError directly
	testError := fmt.Errorf("test error")
	cm.handleError(testError)

	// Verify error was captured
	if capturedError == nil {
		t.Error("Expected error to be captured by callback")
	}

	if capturedError != testError {
		t.Errorf("Expected captured error to be %v, got %v", testError, capturedError)
	}

	// Verify last error is stored
	if cm.GetLastError() != testError {
		t.Errorf("Expected last error to be %v, got %v", testError, cm.GetLastError())
	}
}

// Benchmark tests
func BenchmarkConfigManager_GetCurrentConfig(b *testing.B) {
	// Create temp config file for benchmark
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	defaultConfig := NewDefaultConfig()
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		b.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		b.Fatalf("Failed to write config file: %v", err)
	}

	cm, err := NewConfigManager(configPath)
	if err != nil {
		b.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cm.GetCurrentConfig()
	}
}

func BenchmarkConfigManager_ValidateConfig(b *testing.B) {
	// Create temp config file for benchmark
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	defaultConfig := NewDefaultConfig()
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		b.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		b.Fatalf("Failed to write config file: %v", err)
	}

	cm, err := NewConfigManager(configPath)
	if err != nil {
		b.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer cm.Stop()

	testConfig := NewDefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cm.ValidateConfig(testConfig)
	}
}
