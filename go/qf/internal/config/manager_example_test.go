package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ExampleComponent demonstrates how to implement ComponentRegistry interface
type ExampleComponent struct {
	id          string
	config      *Config
	updateCount int
}

func (ec *ExampleComponent) HandleConfigUpdate(newConfig, oldConfig *Config, updatedSections []string) tea.Cmd {
	ec.updateCount++
	ec.config = newConfig

	fmt.Printf("Component %s received config update (sections: %v)\n", ec.id, updatedSections)

	// In a real Bubble Tea application, you would return a command to update the UI
	return nil
}

func (ec *ExampleComponent) GetComponentID() string {
	return ec.id
}

// ExampleConfigManager demonstrates basic usage of the ConfigManager
func ExampleConfigManager_basicUsage() {
	// Create a temporary configuration file for this example
	tempDir, err := os.MkdirTemp("", "config-manager-example")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	// Create the ConfigManager with custom options
	manager, err := NewConfigManager(configPath,
		WithDebounceDelay(200*time.Millisecond),
		WithLogger(log.New(os.Stdout, "[Example] ", log.LstdFlags)),
	)
	if err != nil {
		log.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer manager.Stop()

	// Register some components
	component1 := &ExampleComponent{id: "ui-component"}
	component2 := &ExampleComponent{id: "filter-engine"}

	if err := manager.RegisterComponent(component1); err != nil {
		log.Printf("Failed to register component: %v", err)
	}

	if err := manager.RegisterComponent(component2); err != nil {
		log.Printf("Failed to register component: %v", err)
	}

	// Start the manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Get current configuration
	currentConfig := manager.GetCurrentConfig()
	fmt.Printf("Current debounce delay: %dms\n", currentConfig.Performance.DebounceDelayMs)
	fmt.Printf("Current theme: %s\n", currentConfig.UI.Theme)

	// Simulate configuration change by manually reloading
	if err := manager.ReloadConfigManually(); err != nil {
		log.Printf("Failed to reload config: %v", err)
	}

	// Show statistics
	stats := manager.GetStats()
	fmt.Printf("Manager statistics: %d components registered\n", stats.ComponentCount)

	// Output:
	// Current debounce delay: 150ms
	// Current theme: default
	// Manager statistics: 2 components registered
}

// ExampleConfigManager_validation demonstrates configuration validation
func ExampleConfigManager_validation() {
	tempDir, err := os.MkdirTemp("", "config-validation-example")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	manager, err := NewConfigManager(configPath)
	if err != nil {
		log.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer manager.Stop()

	// Create a valid configuration
	validConfig := NewDefaultConfig()
	validConfig.Performance.DebounceDelayMs = 100

	// Validate valid configuration
	if err := manager.ValidateConfig(validConfig); err != nil {
		fmt.Printf("Valid config failed validation: %v\n", err)
	} else {
		fmt.Println("Valid configuration passed validation")
	}

	// Create an invalid configuration
	invalidConfig := NewDefaultConfig()
	invalidConfig.Performance.DebounceDelayMs = 0 // Invalid value

	// Validate invalid configuration
	if err := manager.ValidateConfig(invalidConfig); err != nil {
		fmt.Printf("Invalid config correctly rejected: %v\n", err)
	} else {
		fmt.Println("Invalid configuration unexpectedly passed validation")
	}

	// Output:
	// Valid configuration passed validation
	// Invalid config correctly rejected: performance validation failed: debounce_delay_ms must be between 50 and 1000, got 0
}

// ExampleConfigManager_hotReload demonstrates configuration hot-reload functionality
func ExampleConfigManager_hotReload() {
	tempDir, err := os.MkdirTemp("", "config-hot-reload-example")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	// Create the ConfigManager with polling for reliability in tests
	manager, err := NewConfigManager(configPath,
		WithPollingFallback(),
		WithDebounceDelay(100*time.Millisecond), // Faster for example
	)
	if err != nil {
		log.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer manager.Stop()

	// Register a component to receive updates
	component := &ExampleComponent{id: "example-component"}
	if err := manager.RegisterComponent(component); err != nil {
		log.Printf("Failed to register component: %v", err)
	}

	// Start the manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Get initial configuration
	initialConfig := manager.GetCurrentConfig()
	fmt.Printf("Initial debounce delay: %dms\n", initialConfig.Performance.DebounceDelayMs)
	fmt.Printf("Initial update count: %d\n", component.updateCount)

	// Simulate configuration file change
	updatedConfig := NewDefaultConfig()
	updatedConfig.Performance.DebounceDelayMs = 250
	updatedConfig.UI.Theme = "dark"

	// Save the updated configuration (simulating external file modification)
	if err := updatedConfig.SaveToFile(configPath); err != nil {
		log.Printf("Failed to save updated config: %v", err)
		return
	}

	// Trigger manual reload (in real usage, this would happen automatically)
	if err := manager.ReloadConfigManually(); err != nil {
		log.Printf("Failed to reload config: %v", err)
		return
	}

	// Verify configuration was updated
	newConfig := manager.GetCurrentConfig()
	fmt.Printf("Updated debounce delay: %dms\n", newConfig.Performance.DebounceDelayMs)
	fmt.Printf("Updated theme: %s\n", newConfig.UI.Theme)
	fmt.Printf("Component update count: %d\n", component.updateCount)

	// Output:
	// Initial debounce delay: 150ms
	// Initial update count: 0
	// Updated debounce delay: 250ms
	// Updated theme: dark
	// Component update count: 1
}

// ExampleConfigManager_rollback demonstrates configuration rollback functionality
func ExampleConfigManager_rollback() {
	tempDir, err := os.MkdirTemp("", "config-rollback-example")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	manager, err := NewConfigManager(configPath)
	if err != nil {
		log.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer manager.Stop()

	// Get initial configuration
	initialConfig := manager.GetCurrentConfig()
	fmt.Printf("Initial debounce delay: %dms\n", initialConfig.Performance.DebounceDelayMs)

	// Start the manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start ConfigManager: %v", err)
	}

	// Update configuration
	updatedConfig := NewDefaultConfig()
	updatedConfig.Performance.DebounceDelayMs = 300

	if err := updatedConfig.SaveToFile(configPath); err != nil {
		log.Printf("Failed to save updated config: %v", err)
		return
	}

	if err := manager.ReloadConfigManually(); err != nil {
		log.Printf("Failed to reload config: %v", err)
		return
	}

	// Verify update
	currentConfig := manager.GetCurrentConfig()
	fmt.Printf("After update debounce delay: %dms\n", currentConfig.Performance.DebounceDelayMs)

	// Rollback to previous configuration
	if err := manager.RollbackToPrevious(); err != nil {
		log.Printf("Failed to rollback: %v", err)
		return
	}

	// Verify rollback
	rolledBackConfig := manager.GetCurrentConfig()
	fmt.Printf("After rollback debounce delay: %dms\n", rolledBackConfig.Performance.DebounceDelayMs)

	// Output:
	// Initial debounce delay: 150ms
	// After update debounce delay: 300ms
	// After rollback debounce delay: 150ms
}

// ExampleConfigManager_errorHandling demonstrates error handling capabilities
func ExampleConfigManager_errorHandling() {
	tempDir, err := os.MkdirTemp("", "config-error-example")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	// Error callback to capture errors
	var capturedError error
	errorCallback := func(err error) {
		capturedError = err
		fmt.Printf("Error captured: %v\n", err)
	}

	manager, err := NewConfigManager(configPath,
		WithErrorCallback(errorCallback),
	)
	if err != nil {
		log.Fatalf("Failed to create ConfigManager: %v", err)
	}
	defer manager.Stop()

	// Simulate an error by trying to validate an invalid configuration
	invalidConfig := NewDefaultConfig()
	invalidConfig.Performance.DebounceDelayMs = -1

	if err := manager.ValidateConfig(invalidConfig); err != nil {
		fmt.Printf("Validation correctly failed: %v\n", err)
	}

	// Check if any error was captured (in this case, validation doesn't use the callback)
	if capturedError != nil {
		fmt.Printf("Last captured error: %v\n", capturedError)
	} else {
		fmt.Println("No errors captured by callback")
	}

	// Check manager's last error
	if lastError := manager.GetLastError(); lastError != nil {
		fmt.Printf("Manager last error: %v\n", lastError)
	} else {
		fmt.Println("Manager has no last error")
	}

	// Output:
	// Validation correctly failed: performance validation failed: debounce_delay_ms must be between 50 and 1000, got -1
	// No errors captured by callback
	// Manager has no last error
}

// ExampleConfigManager_lifecycle demonstrates complete lifecycle management
func ExampleConfigManager_lifecycle() {
	tempDir, err := os.MkdirTemp("", "config-lifecycle-example")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	manager, err := NewConfigManager(configPath)
	if err != nil {
		log.Fatalf("Failed to create ConfigManager: %v", err)
	}

	fmt.Printf("Manager created, started: %v\n", manager.IsStarted())

	// Start the manager
	if err := manager.Start(); err != nil {
		log.Printf("Failed to start: %v", err)
		return
	}
	fmt.Printf("Manager started: %v\n", manager.IsStarted())

	// Register components
	component := &ExampleComponent{id: "lifecycle-component"}
	if err := manager.RegisterComponent(component); err != nil {
		log.Printf("Failed to register component: %v", err)
	}

	fmt.Printf("Component count: %d\n", manager.GetComponentCount())

	// Unregister component
	if err := manager.UnregisterComponent("lifecycle-component"); err != nil {
		log.Printf("Failed to unregister component: %v", err)
	}

	fmt.Printf("Component count after unregister: %d\n", manager.GetComponentCount())

	// Stop the manager
	if err := manager.Stop(); err != nil {
		log.Printf("Failed to stop: %v", err)
		return
	}
	fmt.Printf("Manager stopped: %v\n", manager.IsStarted())

	// Output:
	// Manager created, started: false
	// Manager started: true
	// Component count: 1
	// Component count after unregister: 0
	// Manager stopped: false
}
