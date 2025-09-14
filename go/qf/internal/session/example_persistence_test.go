package session

import (
	"fmt"
	"log"
	"os"
	"time"
)

// ExamplePersistenceManager_comprehensive demonstrates comprehensive usage of the session persistence system
func ExamplePersistenceManager_comprehensive() {
	// Create a temporary directory for this example
	tempDir, _ := os.MkdirTemp("", "qf-persistence-example-*")
	defer os.RemoveAll(tempDir)

	// Configure persistence with custom settings
	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: 5 * time.Second, // Quick auto-save for demo
		BackupCount:      2,               // Keep 2 backups
		EnableAutoSave:   true,
	}

	// Create persistence manager
	pm, err := NewPersistenceManager(config)
	if err != nil {
		log.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create a session for multi-file log analysis
	session := NewSession("production-analysis")

	// Add multiple log files to the session
	logFiles := []string{
		"/var/log/app/server1.log",
		"/var/log/app/server2.log",
		"/var/log/database/db.log",
	}

	for _, filePath := range logFiles {
		content := []string{
			"2023-09-14 10:00:00 INFO Application started",
			"2023-09-14 10:01:00 ERROR Database connection failed",
			"2023-09-14 10:02:00 INFO Retrying database connection",
		}
		session.AddFileTab(filePath, content)
	}

	// Set up filters for error analysis
	errorFilter := FilterPattern{
		ID:         "error-filter",
		Expression: "ERROR|FATAL",
		Type:       FilterInclude,
		Color:      "red",
		Created:    time.Now(),
		IsValid:    true,
	}

	debugExclude := FilterPattern{
		ID:         "debug-exclude",
		Expression: "DEBUG",
		Type:       FilterExclude,
		Color:      "",
		Created:    time.Now(),
		IsValid:    true,
	}

	filterSet := FilterSet{
		Name:    "error-analysis",
		Include: []FilterPattern{errorFilter},
		Exclude: []FilterPattern{debugExclude},
	}

	session.UpdateFilterSet(filterSet)

	// Save the session
	err = pm.SaveSession(session)
	if err != nil {
		log.Fatalf("Failed to save session: %v", err)
	}

	fmt.Println("Session saved successfully")

	// Demonstrate session listing
	sessions, err := pm.ListSessions()
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}

	fmt.Printf("Available sessions: %v\n", sessions)

	// Load session back
	loadedSession, err := pm.LoadSession("production-analysis")
	if err != nil {
		log.Fatalf("Failed to load session: %v", err)
	}

	fmt.Printf("Loaded session with %d files and %d filters\n",
		len(loadedSession.OpenFiles),
		len(loadedSession.FilterSet.Include)+len(loadedSession.FilterSet.Exclude))

	// Get session metadata without loading full session
	sessionInfo, err := pm.GetSessionInfo("production-analysis")
	if err != nil {
		log.Fatalf("Failed to get session info: %v", err)
	}

	fmt.Printf("Session info - Tabs: %d, Filters: %d\n",
		sessionInfo.TabCount,
		sessionInfo.FilterCount)

	// Create a backup explicitly
	err = pm.BackupSession("production-analysis")
	if err != nil {
		log.Fatalf("Failed to backup session: %v", err)
	}

	fmt.Println("Session backed up successfully")

	// Output:
	// Session saved successfully
	// Available sessions: [production-analysis]
	// Loaded session with 3 files and 2 filters
	// Session info - Tabs: 3, Filters: 2
	// Session backed up successfully
}

// ExamplePersistenceManager_autoSave demonstrates the auto-save functionality
func ExamplePersistenceManager_autoSave() {
	tempDir, _ := os.MkdirTemp("", "qf-autosave-example-*")
	defer os.RemoveAll(tempDir)

	// Configure with short auto-save interval for demonstration
	config := PersistenceConfig{
		SessionsDir:      tempDir,
		AutoSaveInterval: 1 * time.Second,
		BackupCount:      3,
		EnableAutoSave:   true,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		log.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create and modify session
	session := NewSession("auto-save-demo")
	session.AddFileTab("/tmp/test.log", []string{"initial content"})

	// Save session initially
	err = pm.SaveSession(session)
	if err != nil {
		log.Fatalf("Failed to save session: %v", err)
	}

	fmt.Println("Initial session saved")

	// Modify session (this would trigger auto-save after interval)
	session.AddFileTab("/tmp/test2.log", []string{"more content"})

	// In a real application, auto-save would handle this automatically
	// For demo purposes, we manually save to show the functionality
	err = pm.SaveSession(session)
	if err != nil {
		log.Fatalf("Failed to save updated session: %v", err)
	}

	fmt.Printf("Session auto-saved with %d files\n", len(session.OpenFiles))

	// Output:
	// Initial session saved
	// Session auto-saved with 2 files
}

// ExamplePersistenceManager_errorRecovery demonstrates error recovery from backup
func ExamplePersistenceManager_errorRecovery() {
	tempDir, _ := os.MkdirTemp("", "qf-recovery-example-*")
	defer os.RemoveAll(tempDir)

	config := DefaultPersistenceConfig()
	config.SessionsDir = tempDir

	pm, err := NewPersistenceManager(config)
	if err != nil {
		log.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create session
	session := NewSession("recovery-demo")
	session.AddFileTab("/tmp/important.log", []string{"critical data"})

	// Save session twice to create a backup
	pm.SaveSession(session)
	pm.SaveSession(session)

	// Simulate file corruption
	sessionPath := pm.getSessionFilePath("recovery-demo")
	os.WriteFile(sessionPath, []byte("corrupted data"), 0644)

	// Try to load - this will attempt recovery automatically
	recoveredSession, err := pm.LoadSession("recovery-demo")
	if err != nil {
		log.Fatalf("Failed to recover session: %v", err)
	}
	fmt.Println("Session recovered successfully")

	fmt.Printf("Recovered session has %d files\n", len(recoveredSession.OpenFiles))

	// Output:
	// Session recovered successfully
	// Recovered session has 1 files
}

// ExamplePersistenceManager_configUpdate demonstrates configuration hot-reload
func ExamplePersistenceManager_configUpdate() {
	tempDir1, _ := os.MkdirTemp("", "qf-config1-*")
	tempDir2, _ := os.MkdirTemp("", "qf-config2-*")
	defer func() {
		os.RemoveAll(tempDir1)
		os.RemoveAll(tempDir2)
	}()

	// Initial configuration
	config := PersistenceConfig{
		SessionsDir:      tempDir1,
		AutoSaveInterval: 30 * time.Second,
		BackupCount:      3,
		EnableAutoSave:   true,
	}

	pm, err := NewPersistenceManager(config)
	if err != nil {
		log.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	fmt.Println("Initial configuration set")

	// Update configuration with new directory and settings
	newConfig := PersistenceConfig{
		SessionsDir:      tempDir2,
		AutoSaveInterval: 10 * time.Second,
		BackupCount:      5,
		EnableAutoSave:   false, // Disable auto-save
	}

	err = pm.UpdateConfig(newConfig)
	if err != nil {
		log.Fatalf("Failed to update config: %v", err)
	}

	fmt.Println("Configuration updated successfully")
	fmt.Printf("Auto-save enabled: %v\n", newConfig.EnableAutoSave)
	fmt.Printf("Backup count: %d\n", newConfig.BackupCount)

	// Output:
	// Initial configuration set
	// Configuration updated successfully
	// Auto-save enabled: false
	// Backup count: 5
}

// ExamplePersistenceManager_sessionManagement demonstrates complete session lifecycle
func ExamplePersistenceManager_sessionManagement() {
	tempDir, _ := os.MkdirTemp("", "qf-lifecycle-*")
	defer os.RemoveAll(tempDir)

	config := DefaultPersistenceConfig()
	config.SessionsDir = tempDir
	config.EnableAutoSave = false // Disable for controlled demo

	pm, err := NewPersistenceManager(config)
	if err != nil {
		log.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer pm.Shutdown()

	// Create multiple sessions
	sessions := []string{"analysis-1", "analysis-2", "debug-session"}
	for i, name := range sessions {
		session := NewSession(name)
		session.AddFileTab(fmt.Sprintf("/tmp/file%d.log", i+1),
			[]string{fmt.Sprintf("content for %s", name)})

		err = pm.SaveSession(session)
		if err != nil {
			log.Fatalf("Failed to save session %s: %v", name, err)
		}
	}

	// List all sessions
	allSessions, _ := pm.ListSessions()
	fmt.Printf("Created %d sessions: %v\n", len(allSessions), allSessions)

	// Check if specific session exists in cache
	cached := pm.IsSessionCached("analysis-1")
	fmt.Printf("Session 'analysis-1' is cached: %v\n", cached)

	// Get info for a session
	info, _ := pm.GetSessionInfo("analysis-2")
	fmt.Printf("Session 'analysis-2': %d tabs, %d filters\n",
		info.TabCount, info.FilterCount)

	// Delete a session
	err = pm.DeleteSession("debug-session")
	if err != nil {
		log.Fatalf("Failed to delete session: %v", err)
	}

	// List sessions after deletion
	remainingSessions, _ := pm.ListSessions()
	fmt.Printf("Remaining sessions after deletion: %v\n", remainingSessions)

	// Clear cache
	pm.ClearCache()
	cached = pm.IsSessionCached("analysis-1")
	fmt.Printf("Session 'analysis-1' cached after clear: %v\n", cached)

	// Output:
	// Created 3 sessions: [analysis-1 analysis-2 debug-session]
	// Session 'analysis-1' is cached: true
	// Session 'analysis-2': 1 tabs, 0 filters
	// Remaining sessions after deletion: [analysis-1 analysis-2]
	// Session 'analysis-1' cached after clear: false
}
