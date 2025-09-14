package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTempSessionsDir creates a temporary directory for testing
func createTempSessionsDir(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "qf-sessions-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createTestSession creates a test session with some data
func createTestSession(name string) *Session {
	session := NewSession(name)

	// Add some test data
	session.AddFileTab("/path/to/test1.log", []string{"test content 1"})
	session.AddFileTab("/path/to/test2.log", []string{"test content 2"})

	// Add test filters
	filterSet := FilterSet{
		Name: name + "-filters",
		Include: []FilterPattern{
			{
				ID:         "test-include",
				Expression: "INFO",
				Type:       FilterInclude,
				Color:      "blue",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
		Exclude: []FilterPattern{
			{
				ID:         "test-exclude",
				Expression: "DEBUG",
				Type:       FilterExclude,
				Color:      "red",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
	}
	session.UpdateFilterSet(filterSet)

	return session
}

func TestNewPersistenceManager(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: 5 * time.Second,
		BackupCount:      3,
		EnableAutoSave:   true,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Verify configuration
	if pm.config.SessionsDir != tempDir {
		t.Errorf("Expected sessions dir %s, got %s", tempDir, pm.config.SessionsDir)
	}

	if pm.config.AutoSaveInterval != 5*time.Second {
		t.Errorf("Expected auto-save interval 5s, got %v", pm.config.AutoSaveInterval)
	}

	if pm.config.BackupCount != 3 {
		t.Errorf("Expected backup count 3, got %d", pm.config.BackupCount)
	}

	if !pm.config.EnableAutoSave {
		t.Error("Expected auto-save to be enabled")
	}

	// Verify auto-save manager is created
	if pm.autoSave == nil {
		t.Error("Auto-save manager should be created")
	}

	// Verify sessions directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("Sessions directory should be created")
	}
}

func TestValidatePersistenceConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      PersistenceConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: PersistenceConfig{
				SessionsDir:      "/valid/path",
				AutoSaveInterval: time.Minute,
				BackupCount:      3,
				EnableAutoSave:   true,
			},
			expectError: false,
		},
		{
			name: "empty sessions dir",
			config: PersistenceConfig{
				SessionsDir:      "",
				AutoSaveInterval: time.Minute,
				BackupCount:      3,
				EnableAutoSave:   true,
			},
			expectError: true,
		},
		{
			name: "invalid auto-save interval",
			config: PersistenceConfig{
				SessionsDir:      "/valid/path",
				AutoSaveInterval: 500 * time.Millisecond,
				BackupCount:      3,
				EnableAutoSave:   true,
			},
			expectError: true,
		},
		{
			name: "negative backup count",
			config: PersistenceConfig{
				SessionsDir:      "/valid/path",
				AutoSaveInterval: time.Minute,
				BackupCount:      -1,
				EnableAutoSave:   true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePersistenceConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour, // Long interval for testing
		BackupCount:      3,
		EnableAutoSave:   false, // Disable for controlled testing
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create test session
	originalSession := createTestSession("test-session")

	// Save session
	err = pm.SaveSession(originalSession)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify file exists
	sessionPath := pm.getSessionFilePath("test-session")
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		t.Error("Session file should exist after save")
	}

	// Load session
	loadedSession, err := pm.LoadSession("test-session")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	// Verify loaded session data
	if loadedSession.Name != originalSession.Name {
		t.Errorf("Expected name %s, got %s", originalSession.Name, loadedSession.Name)
	}

	if len(loadedSession.OpenFiles) != len(originalSession.OpenFiles) {
		t.Errorf("Expected %d files, got %d", len(originalSession.OpenFiles), len(loadedSession.OpenFiles))
	}

	if len(loadedSession.FilterSet.Include) != len(originalSession.FilterSet.Include) {
		t.Errorf("Expected %d include filters, got %d",
			len(originalSession.FilterSet.Include), len(loadedSession.FilterSet.Include))
	}

	if len(loadedSession.FilterSet.Exclude) != len(originalSession.FilterSet.Exclude) {
		t.Errorf("Expected %d exclude filters, got %d",
			len(originalSession.FilterSet.Exclude), len(loadedSession.FilterSet.Exclude))
	}
}

func TestAtomicWrite(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	session := createTestSession("atomic-test")
	sessionPath := pm.getSessionFilePath("atomic-test")

	// Test atomic write
	err = pm.atomicWrite(sessionPath, session)
	if err != nil {
		t.Fatalf("Atomic write failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		t.Error("Session file should exist after atomic write")
	}

	// Verify temporary file is cleaned up
	tempPath := sessionPath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temporary file should be cleaned up after atomic write")
	}

	// Verify file contents are valid JSON
	loadedSession, err := pm.loadSessionFromFile(sessionPath)
	if err != nil {
		t.Fatalf("Failed to load session after atomic write: %v", err)
	}

	if loadedSession.Name != session.Name {
		t.Errorf("Expected name %s, got %s", session.Name, loadedSession.Name)
	}
}

func TestBackupManagement(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      2, // Keep only 2 backups for testing
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	session := createTestSession("backup-test")
	sessionPath := pm.getSessionFilePath("backup-test")

	// Create initial session file
	err = pm.atomicWrite(sessionPath, session)
	if err != nil {
		t.Fatalf("Failed to create initial session: %v", err)
	}

	// Create multiple backups
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		err = pm.createBackup(sessionPath)
		if err != nil {
			t.Fatalf("Failed to create backup %d: %v", i, err)
		}
	}

	// Allow time for cleanup goroutine to run
	time.Sleep(100 * time.Millisecond)

	// Count backup files
	pattern := "backup-test.qf-session.backup.*"
	matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
	if err != nil {
		t.Fatalf("Failed to glob backup files: %v", err)
	}

	// Should have at most BackupCount (2) backup files
	if len(matches) > config.BackupCount {
		t.Errorf("Expected at most %d backup files, found %d", config.BackupCount, len(matches))
	}
}

func TestRecoverFromBackup(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	session := createTestSession("recovery-test")
	sessionPath := pm.getSessionFilePath("recovery-test")

	// Save session first time (no backup created)
	err = pm.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Save session second time (creates backup of the first save)
	err = pm.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session second time: %v", err)
	}

	// Corrupt the main session file
	err = os.WriteFile(sessionPath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to corrupt session file: %v", err)
	}

	// Try to recover
	recoveredSession, err := pm.recoverFromBackup("recovery-test")
	if err != nil {
		t.Fatalf("Failed to recover from backup: %v", err)
	}

	if recoveredSession.Name != session.Name {
		t.Errorf("Expected recovered name %s, got %s", session.Name, recoveredSession.Name)
	}
}

func TestListSessions(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create multiple test sessions
	sessions := []string{"session1", "session2", "session3"}
	for _, name := range sessions {
		session := createTestSession(name)
		err = pm.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save session %s: %v", name, err)
		}
	}

	// List sessions
	listedSessions, err := pm.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	// Verify all sessions are listed
	if len(listedSessions) != len(sessions) {
		t.Errorf("Expected %d sessions, got %d", len(sessions), len(listedSessions))
	}

	for _, expectedName := range sessions {
		found := false
		for _, listedName := range listedSessions {
			if listedName == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Session %s not found in list", expectedName)
		}
	}
}

func TestDeleteSession(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	session := createTestSession("delete-test")

	// Save session (creates backup)
	err = pm.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Create additional backup
	err = pm.BackupSession("delete-test")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	sessionPath := pm.getSessionFilePath("delete-test")

	// Verify files exist
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		t.Error("Session file should exist before delete")
	}

	// Delete session
	err = pm.DeleteSession("delete-test")
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify main file is deleted
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("Session file should not exist after delete")
	}

	// Verify backups are deleted
	pattern := "delete-test.qf-session.backup.*"
	matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
	if err != nil {
		t.Fatalf("Failed to check for backup files: %v", err)
	}

	if len(matches) > 0 {
		t.Errorf("Expected no backup files after delete, found %d", len(matches))
	}

	// Verify session is removed from cache
	if pm.IsSessionCached("delete-test") {
		t.Error("Session should not be cached after delete")
	}
}

func TestUpdateConfig(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	initialConfig := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: 30 * time.Second,
		BackupCount:      3,
		EnableAutoSave:   true,
	}

	pm, err := NewPersistenceManager(initialConfig)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create new temp directory for updated config
	newTempDir, newCleanup := createTempSessionsDir(t)
	defer newCleanup()

	newConfig := PersistenceConfig{
		SessionsDir:      newTempDir,
		AutoSaveInterval: time.Minute,
		BackupCount:      5,
		EnableAutoSave:   false,
	}

	// Update configuration
	err = pm.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update configuration: %v", err)
	}

	// Verify configuration is updated
	if pm.config.SessionsDir != newTempDir {
		t.Errorf("Expected sessions dir %s, got %s", newTempDir, pm.config.SessionsDir)
	}

	if pm.config.AutoSaveInterval != time.Minute {
		t.Errorf("Expected auto-save interval 1m, got %v", pm.config.AutoSaveInterval)
	}

	if pm.config.BackupCount != 5 {
		t.Errorf("Expected backup count 5, got %d", pm.config.BackupCount)
	}

	if pm.config.EnableAutoSave {
		t.Error("Expected auto-save to be disabled")
	}

	// Verify new sessions directory exists
	if _, err := os.Stat(newTempDir); os.IsNotExist(err) {
		t.Error("New sessions directory should be created")
	}
}

func TestGetSessionInfo(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	session := createTestSession("info-test")

	// Save session
	err = pm.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Get session info
	info, err := pm.GetSessionInfo("info-test")
	if err != nil {
		t.Fatalf("Failed to get session info: %v", err)
	}

	// Verify info
	if info.Name != "info-test" {
		t.Errorf("Expected name info-test, got %s", info.Name)
	}

	if info.TabCount != 2 {
		t.Errorf("Expected 2 tabs, got %d", info.TabCount)
	}

	if info.FilterCount != 2 {
		t.Errorf("Expected 2 filters, got %d", info.FilterCount)
	}

	if info.FilePath != pm.getSessionFilePath("info-test") {
		t.Errorf("Expected file path %s, got %s", pm.getSessionFilePath("info-test"), info.FilePath)
	}
}

func TestSessionCaching(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	session := createTestSession("cache-test")

	// Session should not be cached initially
	if pm.IsSessionCached("cache-test") {
		t.Error("Session should not be cached initially")
	}

	// Save session (adds to cache)
	err = pm.SaveSession(session)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Session should now be cached
	if !pm.IsSessionCached("cache-test") {
		t.Error("Session should be cached after save")
	}

	// Clear cache
	pm.ClearCache()

	// Session should not be cached after clear
	if pm.IsSessionCached("cache-test") {
		t.Error("Session should not be cached after clear")
	}

	// Load session (adds to cache)
	_, err = pm.LoadSession("cache-test")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	// Session should be cached again
	if !pm.IsSessionCached("cache-test") {
		t.Error("Session should be cached after load")
	}
}

func TestInvalidInputs(t *testing.T) {
	tempDir, cleanup := createTempSessionsDir(t)
	defer cleanup()

	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: time.Hour,
		BackupCount:      3,
		EnableAutoSave:   false,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Test SaveSession with nil session
	err = pm.SaveSession(nil)
	if err == nil {
		t.Error("SaveSession should fail with nil session")
	}

	// Test LoadSession with empty name
	_, err = pm.LoadSession("")
	if err == nil {
		t.Error("LoadSession should fail with empty name")
	}

	// Test LoadSession with non-existent session
	_, err = pm.LoadSession("non-existent")
	if err == nil {
		t.Error("LoadSession should fail with non-existent session")
	}

	// Test DeleteSession with empty name
	err = pm.DeleteSession("")
	if err == nil {
		t.Error("DeleteSession should fail with empty name")
	}

	// Test GetSessionInfo with empty name
	_, err = pm.GetSessionInfo("")
	if err == nil {
		t.Error("GetSessionInfo should fail with empty name")
	}

	// Test GetSessionInfo with non-existent session
	_, err = pm.GetSessionInfo("non-existent")
	if err == nil {
		t.Error("GetSessionInfo should fail with non-existent session")
	}
}
