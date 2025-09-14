// Package config provides configuration management for the qf Interactive Log Filter Composer.
//
// This package handles loading, saving, validation, and hot-reload functionality for
// application configuration. It supports JSON marshaling and comprehensive validation
// rules as required by the integration tests.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config represents the complete application configuration structure.
// All fields use json tags for marshaling and include validation rules.
type Config struct {
	// Version is the configuration schema version
	Version string `json:"version" validate:"required"`

	// Performance controls application performance settings
	Performance PerformanceConfig `json:"performance" validate:"required"`

	// UI controls user interface settings
	UI UIConfig `json:"ui" validate:"required"`

	// DataMgmt controls data management and retention policies
	DataMgmt DataConfig `json:"data_management" validate:"required"`

	// FileHandling controls file operations and encoding
	FileHandling FileConfig `json:"file_handling" validate:"required"`
}

// PerformanceConfig controls application performance and resource usage settings.
type PerformanceConfig struct {
	// DebounceDelayMs controls the delay between filter updates (50-1000ms)
	DebounceDelayMs int `json:"debounce_delay_ms" validate:"min=50,max=1000"`

	// CacheSizeMb controls the regex pattern cache size in MB (must be positive)
	CacheSizeMb int `json:"cache_size_mb" validate:"min=1"`

	// StreamingThresholdMb controls when to switch to streaming mode for large files
	StreamingThresholdMb int `json:"streaming_threshold_mb" validate:"min=1"`

	// MaxWorkers controls the maximum number of worker goroutines (1-16)
	MaxWorkers int `json:"max_workers" validate:"min=1,max=16"`
}

// UIConfig controls user interface appearance and behavior settings.
type UIConfig struct {
	// Theme controls the application color theme
	Theme string `json:"theme" validate:"oneof=default dark light"`

	// ShowLineNumbers controls whether line numbers are displayed
	ShowLineNumbers bool `json:"show_line_numbers"`

	// HighlightColors defines the colors used for pattern highlighting
	HighlightColors []string `json:"highlight_colors" validate:"min=1"`

	// KeyBindings allows customization of keyboard shortcuts
	KeyBindings map[string]string `json:"key_bindings,omitempty"`
}

// DataConfig controls data management, retention, and limits.
type DataConfig struct {
	// SessionRetentionDays controls how long sessions are kept (minimum 1 day)
	SessionRetentionDays int `json:"session_retention_days" validate:"min=1"`

	// MaxHistoryEntries controls maximum pattern history entries (minimum 10)
	MaxHistoryEntries int `json:"max_history_entries" validate:"min=10"`

	// MaxOpenFiles controls maximum number of simultaneously open file tabs
	MaxOpenFiles int `json:"max_open_files,omitempty" validate:"min=1"`
}

// FileConfig controls file handling, encoding, and backup settings.
type FileConfig struct {
	// DefaultEncoding specifies the default text encoding for files
	DefaultEncoding string `json:"default_encoding" validate:"oneof=utf-8 utf-16 ascii"`

	// MaxFileSize specifies the maximum file size to handle (e.g., "1GB", "500MB")
	MaxFileSize string `json:"max_file_size" validate:"required"`

	// BackupCount controls the number of backup files to retain
	BackupCount int `json:"backup_count,omitempty" validate:"min=0"`
}

// configMutex protects concurrent access to configuration operations
var configMutex sync.RWMutex

// NewDefaultConfig creates a new Config instance with default values that match
// the project specifications and integration test expectations.
func NewDefaultConfig() *Config {
	return &Config{
		Version: "1.0.0",
		Performance: PerformanceConfig{
			DebounceDelayMs:      150, // Default debounce delay
			CacheSizeMb:          10,  // Default cache size
			StreamingThresholdMb: 100, // Default streaming threshold
			MaxWorkers:           4,   // Default worker count
		},
		UI: UIConfig{
			Theme:           "default",
			ShowLineNumbers: true,
			HighlightColors: []string{"red", "green", "blue", "yellow", "magenta", "cyan"},
			KeyBindings:     make(map[string]string),
		},
		DataMgmt: DataConfig{
			SessionRetentionDays: 30,  // Default retention period
			MaxHistoryEntries:    100, // Default history size
			MaxOpenFiles:         10,  // Default tab limit
		},
		FileHandling: FileConfig{
			DefaultEncoding: "utf-8",
			MaxFileSize:     "1GB",
			BackupCount:     3,
		},
	}
}

// Validate performs comprehensive validation of the configuration structure.
// It checks all validation rules as expected by the integration tests.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate version
	if c.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Validate performance settings
	if err := c.validatePerformance(); err != nil {
		return fmt.Errorf("performance validation failed: %w", err)
	}

	// Validate UI settings
	if err := c.validateUI(); err != nil {
		return fmt.Errorf("ui validation failed: %w", err)
	}

	// Validate data management settings
	if err := c.validateDataMgmt(); err != nil {
		return fmt.Errorf("data_management validation failed: %w", err)
	}

	// Validate file handling settings
	if err := c.validateFileHandling(); err != nil {
		return fmt.Errorf("file_handling validation failed: %w", err)
	}

	return nil
}

// validatePerformance validates performance configuration settings.
func (c *Config) validatePerformance() error {
	perf := c.Performance

	// Validate debounce delay - must be between 50 and 1000ms
	if perf.DebounceDelayMs < 50 || perf.DebounceDelayMs > 1000 {
		return fmt.Errorf("debounce_delay_ms must be between 50 and 1000, got %d", perf.DebounceDelayMs)
	}

	// Validate cache size - must be positive
	if perf.CacheSizeMb <= 0 {
		return fmt.Errorf("cache_size_mb must be positive, got %d", perf.CacheSizeMb)
	}

	// Validate streaming threshold - must be positive
	if perf.StreamingThresholdMb <= 0 {
		return fmt.Errorf("streaming_threshold_mb must be positive, got %d", perf.StreamingThresholdMb)
	}

	// Validate max workers - must be between 1 and 16
	if perf.MaxWorkers < 1 || perf.MaxWorkers > 16 {
		return fmt.Errorf("max_workers must be between 1 and 16, got %d", perf.MaxWorkers)
	}

	return nil
}

// validateUI validates user interface configuration settings.
func (c *Config) validateUI() error {
	ui := c.UI

	// Validate theme
	validThemes := []string{"default", "dark", "light"}
	if !contains(validThemes, ui.Theme) {
		return fmt.Errorf("theme must be one of %v, got %q", validThemes, ui.Theme)
	}

	// Validate highlight colors - must have at least one color
	if len(ui.HighlightColors) == 0 {
		return fmt.Errorf("highlight_colors must contain at least one color")
	}

	return nil
}

// validateDataMgmt validates data management configuration settings.
func (c *Config) validateDataMgmt() error {
	data := c.DataMgmt

	// Validate session retention - must be at least 1 day
	if data.SessionRetentionDays < 1 {
		return fmt.Errorf("session_retention_days must be at least 1, got %d", data.SessionRetentionDays)
	}

	// Validate max history entries - must be at least 10
	if data.MaxHistoryEntries < 10 {
		return fmt.Errorf("max_history_entries must be at least 10, got %d", data.MaxHistoryEntries)
	}

	// Validate max open files if specified
	if data.MaxOpenFiles != 0 && data.MaxOpenFiles < 1 {
		return fmt.Errorf("max_open_files must be positive if specified, got %d", data.MaxOpenFiles)
	}

	return nil
}

// validateFileHandling validates file handling configuration settings.
func (c *Config) validateFileHandling() error {
	file := c.FileHandling

	// Validate default encoding
	validEncodings := []string{"utf-8", "utf-16", "ascii"}
	if !contains(validEncodings, file.DefaultEncoding) {
		return fmt.Errorf("default_encoding must be one of %v, got %q", validEncodings, file.DefaultEncoding)
	}

	// Validate max file size format
	if file.MaxFileSize == "" {
		return fmt.Errorf("max_file_size cannot be empty")
	}

	if err := validateFileSizeFormat(file.MaxFileSize); err != nil {
		return fmt.Errorf("max_file_size format invalid: %w", err)
	}

	// Validate backup count - must be non-negative
	if file.BackupCount < 0 {
		return fmt.Errorf("backup_count must be non-negative, got %d", file.BackupCount)
	}

	return nil
}

// validateFileSizeFormat validates file size format (e.g., "1GB", "500MB", "1024KB").
func validateFileSizeFormat(size string) error {
	// Regex to match size format: number followed by unit (B, KB, MB, GB, TB)
	sizeRegex := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$`)
	matches := sizeRegex.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(size)))

	if len(matches) != 3 {
		return fmt.Errorf("invalid format, expected format like '1GB', '500MB', got %q", size)
	}

	// Validate the number part
	if _, err := strconv.ParseFloat(matches[1], 64); err != nil {
		return fmt.Errorf("invalid number in size specification: %q", matches[1])
	}

	return nil
}

// Load reads and parses the configuration from the specified file path.
// It performs validation after loading to ensure the configuration is valid.
func (c *Config) Load(filePath string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("configuration file not found: %s", filePath)
		}
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	if err := c.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// Save writes the configuration to the specified file path with proper formatting.
// It performs atomic write operations to prevent corruption during concurrent access.
func (c *Config) Save(filePath string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Validate before saving
	if err := c.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid configuration: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create configuration directory: %w", err)
	}

	// Marshal configuration to JSON with proper formatting
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to temporary file first for atomic operation
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary configuration file: %w", err)
	}

	// Atomically replace the original file
	if err := os.Rename(tempFile, filePath); err != nil {
		// Clean up temporary file on failure
		os.Remove(tempFile)
		return fmt.Errorf("failed to replace configuration file: %w", err)
	}

	return nil
}

// Merge updates the current configuration with values from another config.
// It performs deep merge operations and validates the result.
func (c *Config) Merge(other *Config) error {
	if other == nil {
		return fmt.Errorf("cannot merge with nil config")
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	// Merge version if provided
	if other.Version != "" {
		c.Version = other.Version
	}

	// Merge performance settings
	c.mergePerformance(&other.Performance)

	// Merge UI settings
	c.mergeUI(&other.UI)

	// Merge data management settings
	c.mergeDataMgmt(&other.DataMgmt)

	// Merge file handling settings
	c.mergeFileHandling(&other.FileHandling)

	// Validate merged configuration
	if err := c.Validate(); err != nil {
		return fmt.Errorf("merged configuration validation failed: %w", err)
	}

	return nil
}

// mergePerformance merges performance configuration settings.
func (c *Config) mergePerformance(other *PerformanceConfig) {
	if other.DebounceDelayMs > 0 {
		c.Performance.DebounceDelayMs = other.DebounceDelayMs
	}
	if other.CacheSizeMb > 0 {
		c.Performance.CacheSizeMb = other.CacheSizeMb
	}
	if other.StreamingThresholdMb > 0 {
		c.Performance.StreamingThresholdMb = other.StreamingThresholdMb
	}
	if other.MaxWorkers > 0 {
		c.Performance.MaxWorkers = other.MaxWorkers
	}
}

// mergeUI merges user interface configuration settings.
func (c *Config) mergeUI(other *UIConfig) {
	if other.Theme != "" {
		c.UI.Theme = other.Theme
	}
	// ShowLineNumbers is a boolean, so we always take the other value
	c.UI.ShowLineNumbers = other.ShowLineNumbers

	if len(other.HighlightColors) > 0 {
		c.UI.HighlightColors = make([]string, len(other.HighlightColors))
		copy(c.UI.HighlightColors, other.HighlightColors)
	}

	if other.KeyBindings != nil && len(other.KeyBindings) > 0 {
		if c.UI.KeyBindings == nil {
			c.UI.KeyBindings = make(map[string]string)
		}
		for k, v := range other.KeyBindings {
			c.UI.KeyBindings[k] = v
		}
	}
}

// mergeDataMgmt merges data management configuration settings.
func (c *Config) mergeDataMgmt(other *DataConfig) {
	if other.SessionRetentionDays > 0 {
		c.DataMgmt.SessionRetentionDays = other.SessionRetentionDays
	}
	if other.MaxHistoryEntries > 0 {
		c.DataMgmt.MaxHistoryEntries = other.MaxHistoryEntries
	}
	if other.MaxOpenFiles > 0 {
		c.DataMgmt.MaxOpenFiles = other.MaxOpenFiles
	}
}

// mergeFileHandling merges file handling configuration settings.
func (c *Config) mergeFileHandling(other *FileConfig) {
	if other.DefaultEncoding != "" {
		c.FileHandling.DefaultEncoding = other.DefaultEncoding
	}
	if other.MaxFileSize != "" {
		c.FileHandling.MaxFileSize = other.MaxFileSize
	}
	if other.BackupCount >= 0 {
		c.FileHandling.BackupCount = other.BackupCount
	}
}

// GetConfigPath returns the platform-specific path to the configuration file.
// It follows XDG Base Directory Specification on Linux and uses appropriate
// directories on other platforms.
func GetConfigPath() string {
	var configDir string

	// Try to get XDG_CONFIG_HOME first (Linux/Unix standard)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		configDir = xdgConfig
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		// Fall back to platform-specific defaults
		switch {
		case strings.Contains(strings.ToLower(os.Getenv("OS")), "windows"):
			// Windows: %APPDATA%
			if appData := os.Getenv("APPDATA"); appData != "" {
				configDir = appData
			} else {
				configDir = filepath.Join(homeDir, "AppData", "Roaming")
			}
		default:
			// macOS and Linux: ~/.config
			configDir = filepath.Join(homeDir, ".config")
		}
	} else {
		// Final fallback to current directory
		configDir = "."
	}

	return filepath.Join(configDir, "qf", "config.json")
}

// LoadFromFile is a convenience function that loads configuration from the default
// or specified file path. If no path is provided, it uses GetConfigPath().
func LoadFromFile(filePath ...string) (*Config, error) {
	path := GetConfigPath()
	if len(filePath) > 0 && filePath[0] != "" {
		path = filePath[0]
	}

	config := NewDefaultConfig()

	// If config file doesn't exist, return default config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, nil
	}

	err := config.Load(path)
	return config, err
}

// SaveToFile is a convenience function that saves configuration to the default
// or specified file path. If no path is provided, it uses GetConfigPath().
func (c *Config) SaveToFile(filePath ...string) error {
	path := GetConfigPath()
	if len(filePath) > 0 && filePath[0] != "" {
		path = filePath[0]
	}

	return c.Save(path)
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() (*Config, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config for cloning: %w", err)
	}

	clone := &Config{}
	if err := json.Unmarshal(data, clone); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config clone: %w", err)
	}

	return clone, nil
}

// GetPerformanceSettings returns a copy of the performance settings.
func (c *Config) GetPerformanceSettings() PerformanceConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return c.Performance
}

// GetUISettings returns a copy of the UI settings.
func (c *Config) GetUISettings() UIConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return c.UI
}

// GetDataMgmtSettings returns a copy of the data management settings.
func (c *Config) GetDataMgmtSettings() DataConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return c.DataMgmt
}

// GetFileHandlingSettings returns a copy of the file handling settings.
func (c *Config) GetFileHandlingSettings() FileConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return c.FileHandling
}

// contains is a utility function to check if a string slice contains a specific value.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ConfigWatcher provides hot-reload functionality for configuration changes.
type ConfigWatcher struct {
	configPath string
	callback   func(*Config)
	lastMod    time.Time
	mutex      sync.RWMutex
}

// NewConfigWatcher creates a new configuration file watcher.
func NewConfigWatcher(configPath string, callback func(*Config)) *ConfigWatcher {
	return &ConfigWatcher{
		configPath: configPath,
		callback:   callback,
	}
}

// Watch monitors the configuration file for changes and calls the callback
// when changes are detected. This function blocks and should be run in a goroutine.
func (w *ConfigWatcher) Watch() error {
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.checkForChanges(); err != nil {
				// Log error but continue watching
				continue
			}
		}
	}
}

// checkForChanges checks if the configuration file has been modified.
func (w *ConfigWatcher) checkForChanges() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	stat, err := os.Stat(w.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, ignore
			return nil
		}
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	modTime := stat.ModTime()
	if modTime.After(w.lastMod) {
		w.lastMod = modTime

		// Load new configuration
		config, err := LoadFromFile(w.configPath)
		if err != nil {
			return fmt.Errorf("failed to reload config: %w", err)
		}

		// Call callback with new configuration
		if w.callback != nil {
			w.callback(config)
		}
	}

	return nil
}

// Stop stops the configuration watcher (placeholder for future implementation).
func (w *ConfigWatcher) Stop() {
	// Implementation will depend on the final architecture
	// For now, this is a placeholder
}
