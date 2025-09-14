// Package config provides configuration management with hot-reload capabilities
// for the qf Interactive Log Filter Composer.
//
// The ConfigManager handles configuration file watching, validation, component
// notification, and automatic reload functionality. It provides both fsnotify-based
// and polling-based file watching with fallback support.
package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// ComponentRegistry represents a component that can receive configuration updates
type ComponentRegistry interface {
	// HandleConfigUpdate processes configuration updates for the component
	HandleConfigUpdate(newConfig, oldConfig *Config, updatedSections []string) tea.Cmd
	// GetComponentID returns a unique identifier for this component
	GetComponentID() string
}

// ConfigManager handles configuration hot-reload, validation, and component notification
type ConfigManager struct {
	// Configuration state
	currentConfig  *Config
	configPath     string
	previousConfig *Config
	lastModTime    time.Time

	// File watching
	watcher       *fsnotify.Watcher
	pollingTicker *time.Ticker
	usePolling    bool
	debounceTimer *time.Timer
	debounceDelay time.Duration

	// Component management
	components     map[string]ComponentRegistry
	componentMutex sync.RWMutex

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	started    bool
	startMutex sync.Mutex

	// Configuration state
	configMutex sync.RWMutex

	// Error handling and logging
	logger        *log.Logger
	errorCallback func(error)
	lastError     error
}

// ConfigManagerOption allows configuration of ConfigManager behavior
type ConfigManagerOption func(*ConfigManager)

// WithLogger sets a custom logger for the ConfigManager
func WithLogger(logger *log.Logger) ConfigManagerOption {
	return func(cm *ConfigManager) {
		cm.logger = logger
	}
}

// WithErrorCallback sets a callback function for handling errors
func WithErrorCallback(callback func(error)) ConfigManagerOption {
	return func(cm *ConfigManager) {
		cm.errorCallback = callback
	}
}

// WithDebounceDelay sets the debounce delay for config file changes
func WithDebounceDelay(delay time.Duration) ConfigManagerOption {
	return func(cm *ConfigManager) {
		if delay > 0 {
			cm.debounceDelay = delay
		}
	}
}

// WithPollingFallback forces the use of polling instead of fsnotify
func WithPollingFallback() ConfigManagerOption {
	return func(cm *ConfigManager) {
		cm.usePolling = true
	}
}

// NewConfigManager creates a new configuration manager instance
func NewConfigManager(configPath string, options ...ConfigManagerOption) (*ConfigManager, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())

	cm := &ConfigManager{
		configPath:    configPath,
		components:    make(map[string]ComponentRegistry),
		ctx:           ctx,
		cancel:        cancel,
		debounceDelay: 300 * time.Millisecond, // Default debounce delay
		logger:        log.New(os.Stderr, "[ConfigManager] ", log.LstdFlags|log.Lshortfile),
	}

	// Apply options
	for _, option := range options {
		option(cm)
	}

	// Load initial configuration
	if err := cm.loadInitialConfig(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load initial configuration: %w", err)
	}

	return cm, nil
}

// loadInitialConfig loads the configuration file for the first time
func (cm *ConfigManager) loadInitialConfig() error {
	config, err := LoadFromFile(cm.configPath)
	if err != nil {
		// If file doesn't exist, create default config
		if os.IsNotExist(err) {
			cm.logger.Printf("Configuration file not found, creating default: %s", cm.configPath)
			defaultConfig := NewDefaultConfig()
			if saveErr := defaultConfig.SaveToFile(cm.configPath); saveErr != nil {
				return fmt.Errorf("failed to create default config file: %w", saveErr)
			}
			config = defaultConfig
		} else {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
	}

	cm.configMutex.Lock()
	cm.currentConfig = config
	cm.previousConfig = nil
	cm.configMutex.Unlock()

	// Get initial modification time
	if stat, err := os.Stat(cm.configPath); err == nil {
		cm.lastModTime = stat.ModTime()
	}

	cm.logger.Printf("Loaded initial configuration from: %s", cm.configPath)
	return nil
}

// Start begins the configuration watching process
func (cm *ConfigManager) Start() error {
	cm.startMutex.Lock()
	defer cm.startMutex.Unlock()

	if cm.started {
		return fmt.Errorf("config manager already started")
	}

	cm.started = true

	// Initialize file watching
	if !cm.usePolling {
		if err := cm.initFSNotify(); err != nil {
			cm.logger.Printf("FSNotify initialization failed, falling back to polling: %v", err)
			cm.usePolling = true
		}
	}

	if cm.usePolling {
		cm.initPolling()
	}

	// Start the watching goroutine
	go cm.watchLoop()

	cm.logger.Printf("Configuration manager started (polling: %v)", cm.usePolling)
	return nil
}

// Stop stops the configuration manager and cleans up resources
func (cm *ConfigManager) Stop() error {
	cm.startMutex.Lock()
	defer cm.startMutex.Unlock()

	if !cm.started {
		return nil
	}

	cm.started = false

	// Cancel context to signal shutdown
	cm.cancel()

	// Clean up resources
	if cm.watcher != nil {
		cm.watcher.Close()
	}

	if cm.pollingTicker != nil {
		cm.pollingTicker.Stop()
	}

	if cm.debounceTimer != nil {
		cm.debounceTimer.Stop()
	}

	cm.logger.Printf("Configuration manager stopped")
	return nil
}

// initFSNotify initializes fsnotify-based file watching
func (cm *ConfigManager) initFSNotify() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	// Add the config file to the watcher
	if err := watcher.Add(cm.configPath); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add config file to watcher: %w", err)
	}

	cm.watcher = watcher
	return nil
}

// initPolling initializes polling-based file watching
func (cm *ConfigManager) initPolling() {
	cm.pollingTicker = time.NewTicker(500 * time.Millisecond) // Check every 500ms
}

// watchLoop is the main loop for handling file changes
func (cm *ConfigManager) watchLoop() {
	defer func() {
		if r := recover(); r != nil {
			cm.logger.Printf("Watch loop recovered from panic: %v", r)
		}
	}()

	for {
		select {
		case <-cm.ctx.Done():
			return

		case event, ok := <-cm.getWatcherEvents():
			if !ok {
				return
			}
			if cm.shouldProcessEvent(event) {
				cm.scheduleReload("fsnotify")
			}

		case err, ok := <-cm.getWatcherErrors():
			if !ok {
				return
			}
			cm.handleError(fmt.Errorf("fsnotify error: %w", err))

		case <-cm.getPollingTicker():
			if cm.usePolling {
				cm.checkForPollingChanges()
			}
		}
	}
}

// getWatcherEvents returns the fsnotify events channel or a dummy channel if using polling
func (cm *ConfigManager) getWatcherEvents() <-chan fsnotify.Event {
	if cm.watcher != nil {
		return cm.watcher.Events
	}
	// Return a channel that will never send to prevent blocking
	dummy := make(chan fsnotify.Event)
	close(dummy)
	return dummy
}

// getWatcherErrors returns the fsnotify errors channel or a dummy channel if using polling
func (cm *ConfigManager) getWatcherErrors() <-chan error {
	if cm.watcher != nil {
		return cm.watcher.Errors
	}
	// Return a channel that will never send to prevent blocking
	dummy := make(chan error)
	close(dummy)
	return dummy
}

// getPollingTicker returns the polling ticker channel or a dummy channel if using fsnotify
func (cm *ConfigManager) getPollingTicker() <-chan time.Time {
	if cm.pollingTicker != nil {
		return cm.pollingTicker.C
	}
	// Return a channel that will never send to prevent blocking
	dummy := make(chan time.Time)
	close(dummy)
	return dummy
}

// shouldProcessEvent determines if a file system event should trigger a reload
func (cm *ConfigManager) shouldProcessEvent(event fsnotify.Event) bool {
	// Only process write events for our config file
	return event.Name == cm.configPath && (event.Op&fsnotify.Write == fsnotify.Write)
}

// checkForPollingChanges checks for file modifications using polling
func (cm *ConfigManager) checkForPollingChanges() {
	stat, err := os.Stat(cm.configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			cm.handleError(fmt.Errorf("failed to stat config file during polling: %w", err))
		}
		return
	}

	modTime := stat.ModTime()
	if modTime.After(cm.lastModTime) {
		cm.lastModTime = modTime
		cm.scheduleReload("polling")
	}
}

// scheduleReload schedules a configuration reload with debouncing
func (cm *ConfigManager) scheduleReload(source string) {
	// Reset debounce timer
	if cm.debounceTimer != nil {
		cm.debounceTimer.Stop()
	}

	cm.debounceTimer = time.AfterFunc(cm.debounceDelay, func() {
		if err := cm.reloadConfiguration(source); err != nil {
			cm.handleError(fmt.Errorf("configuration reload failed: %w", err))
		}
	})
}

// reloadConfiguration reloads the configuration from disk with validation
func (cm *ConfigManager) reloadConfiguration(source string) error {
	cm.logger.Printf("Reloading configuration from: %s (source: %s)", cm.configPath, source)

	// Load new configuration
	newConfig, err := LoadFromFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to load new configuration: %w", err)
	}

	// Validate new configuration
	if err := newConfig.Validate(); err != nil {
		cm.logger.Printf("Invalid configuration detected, rejecting changes: %v", err)
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Store previous config for rollback and comparison
	cm.configMutex.Lock()
	oldConfig := cm.currentConfig
	cm.previousConfig = oldConfig
	cm.currentConfig = newConfig
	cm.configMutex.Unlock()

	// Determine what sections changed
	updatedSections := cm.getUpdatedSections(oldConfig, newConfig)

	// Notify all registered components
	if err := cm.notifyComponents(newConfig, oldConfig, updatedSections, source); err != nil {
		cm.logger.Printf("Component notification failed: %v", err)
		// Continue anyway - this shouldn't prevent the config update
	}

	cm.logger.Printf("Configuration successfully reloaded (updated sections: %v)", updatedSections)
	return nil
}

// getUpdatedSections compares configurations and returns which sections changed
func (cm *ConfigManager) getUpdatedSections(oldConfig, newConfig *Config) []string {
	if oldConfig == nil {
		return []string{"performance", "ui", "data_management", "file_handling"}
	}

	var updatedSections []string

	// Compare each section using reflection
	if !reflect.DeepEqual(oldConfig.Performance, newConfig.Performance) {
		updatedSections = append(updatedSections, "performance")
	}
	if !reflect.DeepEqual(oldConfig.UI, newConfig.UI) {
		updatedSections = append(updatedSections, "ui")
	}
	if !reflect.DeepEqual(oldConfig.DataMgmt, newConfig.DataMgmt) {
		updatedSections = append(updatedSections, "data_management")
	}
	if !reflect.DeepEqual(oldConfig.FileHandling, newConfig.FileHandling) {
		updatedSections = append(updatedSections, "file_handling")
	}
	if oldConfig.Version != newConfig.Version {
		updatedSections = append(updatedSections, "version")
	}

	return updatedSections
}

// notifyComponents sends configuration updates to all registered components
func (cm *ConfigManager) notifyComponents(newConfig, oldConfig *Config, updatedSections []string, source string) error {
	cm.componentMutex.RLock()
	defer cm.componentMutex.RUnlock()

	if len(cm.components) == 0 {
		return nil
	}

	var errors []string

	for componentID, component := range cm.components {
		if cmd := component.HandleConfigUpdate(newConfig, oldConfig, updatedSections); cmd != nil {
			// In a real Bubble Tea application, you would send this command to the program
			cm.logger.Printf("Component %s generated config update command", componentID)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("component notification errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// RegisterComponent registers a component to receive configuration updates
func (cm *ConfigManager) RegisterComponent(component ComponentRegistry) error {
	if component == nil {
		return fmt.Errorf("component cannot be nil")
	}

	componentID := component.GetComponentID()
	if componentID == "" {
		return fmt.Errorf("component ID cannot be empty")
	}

	cm.componentMutex.Lock()
	defer cm.componentMutex.Unlock()

	if _, exists := cm.components[componentID]; exists {
		return fmt.Errorf("component with ID %s is already registered", componentID)
	}

	cm.components[componentID] = component
	cm.logger.Printf("Registered component: %s", componentID)

	return nil
}

// UnregisterComponent removes a component from receiving configuration updates
func (cm *ConfigManager) UnregisterComponent(componentID string) error {
	cm.componentMutex.Lock()
	defer cm.componentMutex.Unlock()

	if _, exists := cm.components[componentID]; !exists {
		return fmt.Errorf("component with ID %s is not registered", componentID)
	}

	delete(cm.components, componentID)
	cm.logger.Printf("Unregistered component: %s", componentID)

	return nil
}

// GetCurrentConfig returns the current configuration (thread-safe)
func (cm *ConfigManager) GetCurrentConfig() *Config {
	cm.configMutex.RLock()
	defer cm.configMutex.RUnlock()

	if cm.currentConfig == nil {
		return NewDefaultConfig()
	}

	// Return a clone to prevent external modifications
	clone, err := cm.currentConfig.Clone()
	if err != nil {
		cm.logger.Printf("Failed to clone config, returning default: %v", err)
		return NewDefaultConfig()
	}

	return clone
}

// GetPreviousConfig returns the previous configuration for rollback scenarios
func (cm *ConfigManager) GetPreviousConfig() *Config {
	cm.configMutex.RLock()
	defer cm.configMutex.RUnlock()

	if cm.previousConfig == nil {
		return nil
	}

	// Return a clone to prevent external modifications
	clone, err := cm.previousConfig.Clone()
	if err != nil {
		cm.logger.Printf("Failed to clone previous config: %v", err)
		return nil
	}

	return clone
}

// ValidateConfig validates a configuration without applying it
func (cm *ConfigManager) ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	return config.Validate()
}

// ReloadConfigManually triggers a manual configuration reload
func (cm *ConfigManager) ReloadConfigManually() error {
	return cm.reloadConfiguration("manual")
}

// RollbackToPrevious rolls back to the previous configuration
func (cm *ConfigManager) RollbackToPrevious() error {
	cm.configMutex.Lock()
	defer cm.configMutex.Unlock()

	if cm.previousConfig == nil {
		return fmt.Errorf("no previous configuration available for rollback")
	}

	// Swap configurations
	temp := cm.currentConfig
	cm.currentConfig = cm.previousConfig
	cm.previousConfig = temp

	// Save rolled back configuration to disk
	if err := cm.currentConfig.SaveToFile(cm.configPath); err != nil {
		// Revert the swap if save failed
		cm.currentConfig = temp
		cm.previousConfig = cm.previousConfig
		return fmt.Errorf("failed to save rolled back configuration: %w", err)
	}

	cm.logger.Printf("Configuration rolled back successfully")
	return nil
}

// GetComponentCount returns the number of registered components
func (cm *ConfigManager) GetComponentCount() int {
	cm.componentMutex.RLock()
	defer cm.componentMutex.RUnlock()

	return len(cm.components)
}

// IsStarted returns whether the configuration manager is currently started
func (cm *ConfigManager) IsStarted() bool {
	cm.startMutex.Lock()
	defer cm.startMutex.Unlock()

	return cm.started
}

// GetConfigPath returns the path to the configuration file
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

// GetLastError returns the last error encountered by the configuration manager
func (cm *ConfigManager) GetLastError() error {
	return cm.lastError
}

// handleError processes errors with logging and optional callback
func (cm *ConfigManager) handleError(err error) {
	cm.lastError = err

	cm.logger.Printf("Configuration manager error: %v", err)

	if cm.errorCallback != nil {
		cm.errorCallback(err)
	}
}

// GetStats returns statistics about the configuration manager
type ConfigManagerStats struct {
	ConfigPath     string    `json:"config_path"`
	Started        bool      `json:"started"`
	UsePolling     bool      `json:"use_polling"`
	ComponentCount int       `json:"component_count"`
	LastModTime    time.Time `json:"last_mod_time"`
	DebounceDelay  string    `json:"debounce_delay"`
	LastError      string    `json:"last_error,omitempty"`
}

// GetStats returns current statistics about the configuration manager
func (cm *ConfigManager) GetStats() ConfigManagerStats {
	cm.componentMutex.RLock()
	componentCount := len(cm.components)
	cm.componentMutex.RUnlock()

	cm.startMutex.Lock()
	started := cm.started
	cm.startMutex.Unlock()

	stats := ConfigManagerStats{
		ConfigPath:     cm.configPath,
		Started:        started,
		UsePolling:     cm.usePolling,
		ComponentCount: componentCount,
		LastModTime:    cm.lastModTime,
		DebounceDelay:  cm.debounceDelay.String(),
	}

	if cm.lastError != nil {
		stats.LastError = cm.lastError.Error()
	}

	return stats
}
