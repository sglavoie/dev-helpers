package session

import (
	"fmt"
	"os"
	"time"
)

// Example demonstrates basic session usage
func ExampleSession_basic() {
	// Create a new session
	session := NewSession("my-analysis-session")

	// Add some file tabs
	session.AddFileTab("/var/log/server1.log", []string{
		"2023-01-01 ERROR: Database connection failed",
		"2023-01-01 INFO: Retrying connection",
		"2023-01-01 WARNING: High memory usage",
	})

	session.AddFileTab("/var/log/server2.log", []string{
		"2023-01-01 DEBUG: Processing request",
		"2023-01-01 ERROR: Authentication failed",
		"2023-01-01 INFO: Request completed",
	})

	// Add some filters
	filterSet := FilterSet{
		Name: "Error Analysis",
		Include: []FilterPattern{
			{
				ID:         "error-filter",
				Expression: "ERROR",
				Type:       FilterInclude,
				Color:      "red",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
		Exclude: []FilterPattern{
			{
				ID:         "debug-filter",
				Expression: "DEBUG",
				Type:       FilterExclude,
				Color:      "",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
	}
	session.UpdateFilterSet(filterSet)

	// Switch to second tab
	session.SetActiveTab(1)

	// Get session information
	info := session.GetSessionInfo()
	fmt.Printf("Session: %s\n", info.Name)
	fmt.Printf("Files: %d\n", info.TabCount)
	fmt.Printf("Filters: %d\n", info.FilterCount)

	// Save session
	err := session.SaveSession()
	if err != nil {
		fmt.Printf("Save error: %v\n", err)
		return
	}

	fmt.Println("Session saved successfully")

	// Clean up for example
	sessionPath := GetSessionPath(session.Name)
	os.Remove(sessionPath)
	os.Remove(sessionPath + ".backup")

	// Output:
	// Session: my-analysis-session
	// Files: 2
	// Filters: 2
	// Session saved successfully
}

// Example demonstrates session persistence
func ExampleLoadSession() {
	// Create and save a session
	originalSession := NewSession("persistence-example")
	originalSession.AddFileTab("/path/to/important.log", []string{"Important data"})

	filterSet := FilterSet{
		Include: []FilterPattern{
			{
				ID:         "critical-filter",
				Expression: "CRITICAL",
				Type:       FilterInclude,
				Color:      "red",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
	}
	originalSession.UpdateFilterSet(filterSet)

	// Save the session
	err := originalSession.SaveSession()
	if err != nil {
		fmt.Printf("Save error: %v\n", err)
		return
	}

	// Load the session
	loadedSession, err := LoadSession("persistence-example")
	if err != nil {
		fmt.Printf("Load error: %v\n", err)
		return
	}

	fmt.Printf("Loaded session: %s\n", loadedSession.Name)
	fmt.Printf("Files: %d\n", len(loadedSession.OpenFiles))
	fmt.Printf("Filters: %d\n", len(loadedSession.FilterSet.Include))

	if len(loadedSession.OpenFiles) > 0 {
		fmt.Printf("First file: %s\n", loadedSession.OpenFiles[0].FilePath)
	}

	// Clean up
	sessionPath := GetSessionPath(loadedSession.Name)
	os.Remove(sessionPath)
	os.Remove(sessionPath + ".backup")

	// Output:
	// Loaded session: persistence-example
	// Files: 1
	// Filters: 1
	// First file: /path/to/important.log
}

// Example demonstrates session cloning
func ExampleSession_Clone() {
	// Create original session
	original := NewSession("original")
	original.AddFileTab("/logs/app.log", []string{"Log data"})

	// Clone the session
	clone := original.Clone()

	fmt.Printf("Original: %s\n", original.Name)
	fmt.Printf("Clone: %s\n", clone.Name)
	fmt.Printf("Same ID: %t\n", original.ID == clone.ID)
	fmt.Printf("Both have files: %t\n", len(original.OpenFiles) == len(clone.OpenFiles))

	// Output:
	// Original: original
	// Clone: original-copy
	// Same ID: false
	// Both have files: true
}
