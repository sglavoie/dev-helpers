// Package session provides session persistence functionality for the qf Interactive Log Filter Composer.
//
// This module implements a comprehensive session persistence system with features including:
// - JSON format storage for human-readable session files
// - Atomic writes using temp file + rename strategy for corruption prevention
// - Auto-save functionality with configurable intervals (default 30 seconds)
// - Backup management with configurable retention (default 3 versions)
// - Directory management and file operations safety
// - Error handling and recovery from corrupted session files
//
// The PersistenceManager handles all session persistence operations and can be configured
// through the application's configuration system. It supports concurrent access and
// provides reliable session management for multi-file analysis workflows.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// PersistenceConfig holds configuration settings for session persistence
type PersistenceConfig struct {
	// SessionsDir is the directory where session files are stored
	SessionsDir string `json:"sessions_dir"`

	// AutoSaveInterval controls how often sessions are automatically saved
	AutoSaveInterval time.Duration `json:"auto_save_interval"`

	// BackupCount controls how many backup versions to retain
	BackupCount int `json:"backup_count"`

	// EnableAutoSave controls whether auto-save functionality is enabled
	EnableAutoSave bool `json:"enable_auto_save"`
}

// DefaultPersistenceConfig returns default persistence configuration
func DefaultPersistenceConfig() PersistenceConfig {
	sessionsDir, _ := GetSessionsDir() // Use existing function
	return PersistenceConfig{
		SessionsDir:      sessionsDir,
		AutoSaveInterval: 30 * time.Second,
		BackupCount:      3,
		EnableAutoSave:   true,
	}
}

// PersistenceManager manages session persistence operations including auto-save,
// backup management, and atomic write operations
type PersistenceManager struct {
	config   PersistenceConfig
	sessions map[string]*Session // Active sessions cache
	mutex    sync.RWMutex        // Protects sessions map and config
	autoSave *AutoSaveManager    // Auto-save functionality
	ctx      context.Context     // Context for cancellation
	cancel   context.CancelFunc  // Cancel function for graceful shutdown
}

// AutoSaveManager handles automatic session saving
type AutoSaveManager struct {
	ticker        *time.Ticker
	enabled       bool
	lastSaveTimes map[string]time.Time // Track last save time per session
	mutex         sync.RWMutex         // Protects lastSaveTimes
}

// NewPersistenceManager creates a new persistence manager with the given configuration
func NewPersistenceManager(config PersistenceConfig) (*PersistenceManager, error) {
	// Validate configuration
	if err := validatePersistenceConfig(config); err != nil {
		return nil, fmt.Errorf("invalid persistence configuration: %w", err)
	}

	// Ensure sessions directory exists
	if err := os.MkdirAll(config.SessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory %s: %w", config.SessionsDir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	pm := &PersistenceManager{
		config:   config,
		sessions: make(map[string]*Session),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Initialize auto-save if enabled
	if config.EnableAutoSave {
		pm.autoSave = &AutoSaveManager{
			ticker:        time.NewTicker(config.AutoSaveInterval),
			enabled:       true,
			lastSaveTimes: make(map[string]time.Time),
		}

		// Start auto-save goroutine
		go pm.runAutoSave()
	}

	return pm, nil
}

// validatePersistenceConfig validates persistence configuration
func validatePersistenceConfig(config PersistenceConfig) error {
	if config.SessionsDir == "" {
		return fmt.Errorf("sessions_dir cannot be empty")
	}

	if config.AutoSaveInterval < time.Second {
		return fmt.Errorf("auto_save_interval must be at least 1 second, got %v", config.AutoSaveInterval)
	}

	if config.BackupCount < 0 {
		return fmt.Errorf("backup_count must be non-negative, got %d", config.BackupCount)
	}

	return nil
}

// SaveSession saves a session to disk using atomic write operations
// This method creates backups before overwriting and ensures data integrity
func (pm *PersistenceManager) SaveSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Validate session before saving
	if err := ValidateSession(session); err != nil {
		return fmt.Errorf("session validation failed: %w", err)
	}

	// Get session file path
	sessionPath := pm.getSessionFilePath(session.Name)

	// Create backup if file exists
	if err := pm.createBackup(sessionPath); err != nil {
		return fmt.Errorf("backup creation failed: %w", err)
	}

	// Perform atomic write
	if err := pm.atomicWrite(sessionPath, session); err != nil {
		return fmt.Errorf("atomic write failed: %w", err)
	}

	// Update session cache
	pm.sessions[session.Name] = session

	// Update auto-save tracking
	if pm.autoSave != nil {
		pm.autoSave.mutex.Lock()
		pm.autoSave.lastSaveTimes[session.Name] = time.Now()
		pm.autoSave.mutex.Unlock()
	}

	return nil
}

// LoadSession loads a session from disk with validation and error recovery
func (pm *PersistenceManager) LoadSession(sessionName string) (*Session, error) {
	if sessionName == "" {
		return nil, fmt.Errorf("session name cannot be empty")
	}

	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Check cache first
	if session, exists := pm.sessions[sessionName]; exists {
		return session, nil // Return the cached session directly
	}

	sessionPath := pm.getSessionFilePath(sessionName)

	// Try to load the session file
	session, err := pm.loadSessionFromFile(sessionPath)
	if err != nil {
		// If primary file fails, try to recover from backup
		if backupSession, backupErr := pm.recoverFromBackup(sessionName); backupErr == nil {
			return backupSession, nil
		}
		return nil, fmt.Errorf("failed to load session %s: %w", sessionName, err)
	}

	// Cache the loaded session
	pm.sessions[sessionName] = session

	return session, nil
}

// loadSessionFromFile loads a session from the specified file path
func (pm *PersistenceManager) loadSessionFromFile(filePath string) (*Session, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("session file not found: %s", filePath)
	}

	// Read session file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Unmarshal JSON
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	// Validate session
	if err := ValidateSession(&session); err != nil {
		return nil, fmt.Errorf("loaded session is invalid: %w", err)
	}

	// Migrate session if needed
	if err := MigrateSession(&session); err != nil {
		return nil, fmt.Errorf("session migration failed: %w", err)
	}

	return &session, nil
}

// atomicWrite performs an atomic write operation using temp file + rename strategy
func (pm *PersistenceManager) atomicWrite(filePath string, session *Session) error {
	// Create temporary file path
	tempPath := filePath + ".tmp"

	// Marshal session to JSON with proper formatting
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	// Write to temporary file
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomic rename to final location
	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temporary file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temporary file to final location: %w", err)
	}

	return nil
}

// BackupSession creates a timestamped backup of the specified session
func (pm *PersistenceManager) BackupSession(sessionName string) error {
	if sessionName == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	sessionPath := pm.getSessionFilePath(sessionName)

	// Check if session file exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return fmt.Errorf("session file not found: %s", sessionPath)
	}

	return pm.createBackup(sessionPath)
}

// createBackup creates a backup of the specified file if it exists
func (pm *PersistenceManager) createBackup(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	// Generate timestamped backup filename
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", filePath, timestamp)

	// Copy file to backup location
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read original file for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Clean up old backups
	go pm.cleanupOldBackups(filepath.Base(filePath))

	return nil
}

// cleanupOldBackups removes old backup files beyond the retention limit
func (pm *PersistenceManager) cleanupOldBackups(sessionFileName string) {
	if pm.config.BackupCount <= 0 {
		return
	}

	// Find all backup files for this session
	pattern := sessionFileName + ".backup.*"
	matches, err := filepath.Glob(filepath.Join(pm.config.SessionsDir, pattern))
	if err != nil {
		return // Ignore glob errors
	}

	// Sort backups by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		statI, errI := os.Stat(matches[i])
		statJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return statI.ModTime().After(statJ.ModTime())
	})

	// Remove old backups beyond retention limit
	for i := pm.config.BackupCount; i < len(matches); i++ {
		os.Remove(matches[i])
	}
}

// recoverFromBackup attempts to recover a session from its most recent backup
func (pm *PersistenceManager) recoverFromBackup(sessionName string) (*Session, error) {
	sessionFileName := sessionName + ".qf-session"
	pattern := sessionFileName + ".backup.*"
	matches, err := filepath.Glob(filepath.Join(pm.config.SessionsDir, pattern))
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("no backup files found for session %s", sessionName)
	}

	// Sort backups by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		statI, errI := os.Stat(matches[i])
		statJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return statI.ModTime().After(statJ.ModTime())
	})

	// Try to load from the most recent backup
	for _, backupPath := range matches {
		session, err := pm.loadSessionFromFile(backupPath)
		if err == nil {
			return session, nil
		}
	}

	return nil, fmt.Errorf("failed to recover session from any backup")
}

// ListSessions returns a list of available session names
func (pm *PersistenceManager) ListSessions() ([]string, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Scan sessions directory for .qf-session files
	pattern := "*.qf-session"
	matches, err := filepath.Glob(filepath.Join(pm.config.SessionsDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to scan sessions directory: %w", err)
	}

	sessions := make([]string, 0, len(matches))
	for _, match := range matches {
		// Extract session name from filename
		fileName := filepath.Base(match)
		sessionName := strings.TrimSuffix(fileName, ".qf-session")
		sessions = append(sessions, sessionName)
	}

	sort.Strings(sessions)
	return sessions, nil
}

// DeleteSession removes a session and all its backups
func (pm *PersistenceManager) DeleteSession(sessionName string) error {
	if sessionName == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	sessionPath := pm.getSessionFilePath(sessionName)

	// Remove main session file
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session file: %w", err)
	}

	// Remove all backup files
	sessionFileName := sessionName + ".qf-session"
	pattern := sessionFileName + ".backup.*"
	matches, err := filepath.Glob(filepath.Join(pm.config.SessionsDir, pattern))
	if err == nil {
		for _, match := range matches {
			os.Remove(match)
		}
	}

	// Remove from cache
	delete(pm.sessions, sessionName)

	// Remove from auto-save tracking
	if pm.autoSave != nil {
		pm.autoSave.mutex.Lock()
		delete(pm.autoSave.lastSaveTimes, sessionName)
		pm.autoSave.mutex.Unlock()
	}

	return nil
}

// UpdateConfig updates the persistence configuration and restarts auto-save if needed
func (pm *PersistenceManager) UpdateConfig(config PersistenceConfig) error {
	if err := validatePersistenceConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Update configuration
	oldConfig := pm.config
	pm.config = config

	// Handle sessions directory change
	if oldConfig.SessionsDir != config.SessionsDir {
		if err := os.MkdirAll(config.SessionsDir, 0755); err != nil {
			pm.config = oldConfig // Restore old config on failure
			return fmt.Errorf("failed to create new sessions directory: %w", err)
		}
	}

	// Handle auto-save configuration changes
	if pm.autoSave != nil {
		pm.autoSave.ticker.Stop()
		pm.autoSave = nil
	}

	if config.EnableAutoSave {
		pm.autoSave = &AutoSaveManager{
			ticker:        time.NewTicker(config.AutoSaveInterval),
			enabled:       true,
			lastSaveTimes: make(map[string]time.Time),
		}
		go pm.runAutoSave()
	}

	return nil
}

// runAutoSave runs the auto-save functionality in a separate goroutine
func (pm *PersistenceManager) runAutoSave() {
	if pm.autoSave == nil {
		return
	}

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-pm.autoSave.ticker.C:
			pm.performAutoSave()
		}
	}
}

// performAutoSave performs automatic saving of all modified sessions
func (pm *PersistenceManager) performAutoSave() {
	if pm.autoSave == nil || !pm.autoSave.enabled {
		return
	}

	pm.mutex.RLock()
	sessionsCopy := make(map[string]*Session)
	for name, session := range pm.sessions {
		sessionsCopy[name] = session
	}
	pm.mutex.RUnlock()

	pm.autoSave.mutex.RLock()
	lastSaveTimes := make(map[string]time.Time)
	for name, lastSave := range pm.autoSave.lastSaveTimes {
		lastSaveTimes[name] = lastSave
	}
	pm.autoSave.mutex.RUnlock()

	// Save sessions that have been modified since last save
	for name, session := range sessionsCopy {
		lastSave, exists := lastSaveTimes[name]
		if !exists || session.LastModified.After(lastSave) {
			// Save session without holding locks to prevent deadlock
			go func(s *Session) {
				if err := pm.SaveSession(s); err != nil {
					// In a real application, this would be logged
					// For now, we silently continue
				}
			}(session)
		}
	}
}

// Shutdown gracefully shuts down the persistence manager
func (pm *PersistenceManager) Shutdown() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Cancel context to stop auto-save
	if pm.cancel != nil {
		pm.cancel()
	}

	// Stop auto-save ticker
	if pm.autoSave != nil && pm.autoSave.ticker != nil {
		pm.autoSave.ticker.Stop()
	}

	// Perform final save of all sessions
	for name, session := range pm.sessions {
		if err := pm.atomicWrite(pm.getSessionFilePath(name), session); err != nil {
			// Log error but continue with other sessions
			continue
		}
	}
}

// GetSessionInfo returns metadata about a session without loading the full session
func (pm *PersistenceManager) GetSessionInfo(sessionName string) (*SessionInfo, error) {
	if sessionName == "" {
		return nil, fmt.Errorf("session name cannot be empty")
	}

	sessionPath := pm.getSessionFilePath(sessionName)

	// Check if file exists
	stat, err := os.Stat(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", sessionName)
		}
		return nil, fmt.Errorf("failed to stat session file: %w", err)
	}

	// Try to load just enough data to get session info
	session, err := pm.loadSessionFromFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load session for info: %w", err)
	}

	return &SessionInfo{
		ID:           session.ID,
		Name:         session.Name,
		Created:      session.Created,
		LastModified: stat.ModTime(), // Use file modification time
		FilePath:     sessionPath,
		TabCount:     len(session.OpenFiles),
		FilterCount:  len(session.FilterSet.Include) + len(session.FilterSet.Exclude),
	}, nil
}

// getSessionFilePath returns the full file path for a session
func (pm *PersistenceManager) getSessionFilePath(sessionName string) string {
	return filepath.Join(pm.config.SessionsDir, sessionName+".qf-session")
}

// IsSessionCached returns true if the session is currently in the cache
func (pm *PersistenceManager) IsSessionCached(sessionName string) bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	_, exists := pm.sessions[sessionName]
	return exists
}

// ClearCache removes all sessions from the cache (they remain on disk)
func (pm *PersistenceManager) ClearCache() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.sessions = make(map[string]*Session)
}
