package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// Manager handles configuration file operations
type Manager struct {
	configPath string
	loaded     bool
	loadedPath string
	previous   []byte
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
	path, err := m.storagePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Create new config if file doesn't exist
		m.loaded, m.loadedPath, m.previous = true, path, nil
		return models.NewConfig(), nil
	}
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
	m.loaded, m.loadedPath, m.previous = true, path, data

	return &config, nil
}

// Save saves the configuration to file
func (m *Manager) Save(config *models.Config) error {
	path, err := m.storagePath()
	if err != nil {
		return err
	}
	if m.loaded && path != m.loadedPath {
		return fmt.Errorf("config path changed while editing; reload and retry")
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	unlock, err := lockSave(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if m.loaded && ((err == nil) != (m.previous != nil) || !bytes.Equal(current, m.previous)) {
		return fmt.Errorf("GoTime data changed in another process; reload and retry your edit")
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".gotime-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	m.loaded, m.loadedPath, m.previous = true, path, data

	return nil
}

// Follow an existing config symlink so atomic replacement preserves the link.
func (m *Manager) storagePath() (string, error) {
	path, err := filepath.Abs(m.configPath)
	if err != nil {
		return "", err
	}
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("config symlink target is missing: %s", path)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
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
