package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// Manager handles configuration file operations
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager
func NewManager(customPath string) *Manager {
	var configPath string

	if customPath != "" {
		configPath = customPath
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home directory is not accessible
			configPath = ".gotime.json"
		} else {
			configPath = filepath.Join(homeDir, ".gotime.json")
		}
	}

	return &Manager{
		configPath: configPath,
	}
}

// Load loads the configuration from file
func (m *Manager) Load() (*models.Config, error) {
	// Check if file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create new config if file doesn't exist
		return models.NewConfig(), nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Ensure config has valid values
	if config.NextShortID < 1 {
		config.NextShortID = 1
	}

	// Update short IDs to ensure consistency
	config.UpdateShortIDs()

	return &config, nil
}

// Save saves the configuration to file
func (m *Manager) Save(config *models.Config) error {
	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the current config file path
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// Exists checks if the config file exists
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.configPath)
	return !os.IsNotExist(err)
}

// LoadOrCreate loads existing config or creates a new one
func (m *Manager) LoadOrCreate() (*models.Config, error) {
	config, err := m.Load()
	if err != nil {
		return nil, err
	}

	// Save the config to ensure file exists
	if !m.Exists() {
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to create initial config file: %w", err)
		}
	}

	return config, nil
}
