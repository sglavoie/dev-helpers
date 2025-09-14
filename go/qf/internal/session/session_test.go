package session

import (
	"os"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	sessionName := "test-session"
	session := NewSession(sessionName)

	if session == nil {
		t.Fatal("NewSession returned nil")
	}

	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}

	if session.Name != sessionName {
		t.Errorf("Expected session name %s, got %s", sessionName, session.Name)
	}

	if session.ActiveTabIndex != -1 {
		t.Errorf("Expected ActiveTabIndex -1, got %d", session.ActiveTabIndex)
	}

	if len(session.OpenFiles) != 0 {
		t.Errorf("Expected no open files, got %d", len(session.OpenFiles))
	}

	if session.FilterSet.Name != sessionName {
		t.Errorf("Expected filter set name %s, got %s", sessionName, session.FilterSet.Name)
	}

	if session.Created.IsZero() {
		t.Error("Session creation time should not be zero")
	}

	if session.LastModified.IsZero() {
		t.Error("Session last modified time should not be zero")
	}
}

func TestAddFileTab(t *testing.T) {
	session := NewSession("test-session")

	// Test adding first tab
	content := []string{"line 1", "line 2", "line 3"}
	tab, err := session.AddFileTab("/path/to/file1.log", content)
	if err != nil {
		t.Fatalf("Failed to add first tab: %v", err)
	}

	if tab == nil {
		t.Fatal("AddFileTab returned nil tab")
	}

	if tab.ID == "" {
		t.Error("Tab ID should not be empty")
	}

	if tab.FilePath != "/path/to/file1.log" {
		t.Errorf("Expected file path /path/to/file1.log, got %s", tab.FilePath)
	}

	if !tab.Active {
		t.Error("First tab should be active")
	}

	if session.ActiveTabIndex != 0 {
		t.Errorf("Expected ActiveTabIndex 0, got %d", session.ActiveTabIndex)
	}

	if session.UIState.PaneLayout.TabBarVisible {
		t.Error("Tab bar should not be visible for single tab")
	}

	// Test adding second tab
	_, err = session.AddFileTab("/path/to/file2.log", []string{"other content"})
	if err != nil {
		t.Fatalf("Failed to add second tab: %v", err)
	}

	if len(session.OpenFiles) != 2 {
		t.Errorf("Expected 2 open files, got %d", len(session.OpenFiles))
	}

	if !session.UIState.PaneLayout.TabBarVisible {
		t.Error("Tab bar should be visible for multiple tabs")
	}

	// Test adding duplicate file
	_, err = session.AddFileTab("/path/to/file1.log", content)
	if err == nil {
		t.Error("Expected error when adding duplicate file")
	}
}

func TestRemoveFileTab(t *testing.T) {
	session := NewSession("test-session")

	// Add some tabs
	tab1, _ := session.AddFileTab("/path/to/file1.log", []string{"content1"})
	tab2, _ := session.AddFileTab("/path/to/file2.log", []string{"content2"})
	tab3, _ := session.AddFileTab("/path/to/file3.log", []string{"content3"})

	// Remove middle tab
	err := session.RemoveFileTab(tab2.ID)
	if err != nil {
		t.Fatalf("Failed to remove tab: %v", err)
	}

	if len(session.OpenFiles) != 2 {
		t.Errorf("Expected 2 tabs remaining, got %d", len(session.OpenFiles))
	}

	// Verify active tab is still correct
	if session.ActiveTabIndex != 0 {
		t.Errorf("Expected ActiveTabIndex 0, got %d", session.ActiveTabIndex)
	}

	// Remove non-existent tab
	err = session.RemoveFileTab("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent tab")
	}

	// Remove all remaining tabs
	session.RemoveFileTab(tab1.ID)
	session.RemoveFileTab(tab3.ID)

	if len(session.OpenFiles) != 0 {
		t.Errorf("Expected 0 tabs remaining, got %d", len(session.OpenFiles))
	}

	if session.ActiveTabIndex != -1 {
		t.Errorf("Expected ActiveTabIndex -1, got %d", session.ActiveTabIndex)
	}

	if session.UIState.PaneLayout.TabBarVisible {
		t.Error("Tab bar should not be visible when no tabs")
	}
}

func TestSetActiveTab(t *testing.T) {
	session := NewSession("test-session")

	// Add tabs
	session.AddFileTab("/path/to/file1.log", []string{"content1"})
	session.AddFileTab("/path/to/file2.log", []string{"content2"})
	session.AddFileTab("/path/to/file3.log", []string{"content3"})

	// Test setting active tab
	err := session.SetActiveTab(1)
	if err != nil {
		t.Fatalf("Failed to set active tab: %v", err)
	}

	if session.ActiveTabIndex != 1 {
		t.Errorf("Expected ActiveTabIndex 1, got %d", session.ActiveTabIndex)
	}

	if !session.OpenFiles[1].Active {
		t.Error("Tab 1 should be active")
	}

	if session.OpenFiles[0].Active || session.OpenFiles[2].Active {
		t.Error("Only tab 1 should be active")
	}

	// Test invalid index
	err = session.SetActiveTab(10)
	if err == nil {
		t.Error("Expected error for invalid tab index")
	}

	err = session.SetActiveTab(-1)
	if err == nil {
		t.Error("Expected error for negative tab index")
	}
}

func TestGetActiveTab(t *testing.T) {
	session := NewSession("test-session")

	// No tabs initially
	activeTab := session.GetActiveTab()
	if activeTab != nil {
		t.Error("Expected nil for no active tab")
	}

	// Add tab
	tab1, _ := session.AddFileTab("/path/to/file1.log", []string{"content1"})

	activeTab = session.GetActiveTab()
	if activeTab == nil {
		t.Fatal("Expected active tab, got nil")
	}

	if activeTab.ID != tab1.ID {
		t.Error("Active tab ID mismatch")
	}
}

func TestUpdateFilterSet(t *testing.T) {
	session := NewSession("test-session")

	originalModTime := session.LastModified
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	newFilterSet := FilterSet{
		Name: "updated-filters",
		Include: []FilterPattern{
			{
				ID:         "test-pattern",
				Expression: "ERROR",
				Type:       FilterInclude,
				Color:      "red",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
	}

	session.UpdateFilterSet(newFilterSet)

	if session.FilterSet.Name != "updated-filters" {
		t.Error("Filter set was not updated")
	}

	if len(session.FilterSet.Include) != 1 {
		t.Error("Include patterns were not updated")
	}

	if !session.LastModified.After(originalModTime) {
		t.Error("Last modified time was not updated")
	}
}

func TestClone(t *testing.T) {
	session := NewSession("original-session")
	session.AddFileTab("/path/to/file1.log", []string{"content"})

	filterSet := FilterSet{
		Include: []FilterPattern{
			{
				ID:         "original-pattern",
				Expression: "ERROR",
				Type:       FilterInclude,
				Created:    time.Now(),
				IsValid:    true,
			},
		},
	}
	session.UpdateFilterSet(filterSet)

	clone := session.Clone()

	// Verify clone has different ID and name
	if clone.ID == session.ID {
		t.Error("Clone should have different ID")
	}

	if clone.Name == session.Name {
		t.Error("Clone should have different name")
	}

	// Verify clone has same structure but independent data
	if len(clone.OpenFiles) != len(session.OpenFiles) {
		t.Error("Clone should have same number of files")
	}

	if clone.OpenFiles[0].ID == session.OpenFiles[0].ID {
		t.Error("Clone files should have different IDs")
	}

	if len(clone.FilterSet.Include) != len(session.FilterSet.Include) {
		t.Error("Clone should have same number of filters")
	}

	if clone.FilterSet.Include[0].ID == session.FilterSet.Include[0].ID {
		t.Error("Clone filters should have different IDs")
	}
}

func TestSessionPersistence(t *testing.T) {
	// Create temp session
	sessionName := "persistence-test"
	session := NewSession(sessionName)
	session.AddFileTab("/path/to/test.log", []string{"test content"})

	// Save session
	err := session.SaveSession()
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Load session
	loadedSession, err := LoadSession(sessionName)
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	// Verify data
	if loadedSession.Name != sessionName {
		t.Errorf("Expected session name %s, got %s", sessionName, loadedSession.Name)
	}

	if len(loadedSession.OpenFiles) != 1 {
		t.Errorf("Expected 1 file, got %d", len(loadedSession.OpenFiles))
	}

	if loadedSession.OpenFiles[0].FilePath != "/path/to/test.log" {
		t.Error("File path not preserved")
	}

	// Clean up
	sessionPath := GetSessionPath(sessionName)
	os.Remove(sessionPath)
	os.Remove(sessionPath + ".backup")
}

func TestValidateSession(t *testing.T) {
	// Valid session
	validSession := NewSession("valid-session")
	err := ValidateSession(validSession)
	if err != nil {
		t.Errorf("Valid session should pass validation: %v", err)
	}

	// Invalid session - nil
	err = ValidateSession(nil)
	if err == nil {
		t.Error("nil session should fail validation")
	}

	// Invalid session - empty ID
	invalidSession := NewSession("invalid-session")
	invalidSession.ID = ""
	err = ValidateSession(invalidSession)
	if err == nil {
		t.Error("Session with empty ID should fail validation")
	}

	// Invalid session - empty name
	invalidSession = NewSession("invalid-session")
	invalidSession.Name = ""
	err = ValidateSession(invalidSession)
	if err == nil {
		t.Error("Session with empty name should fail validation")
	}

	// Invalid session - bad active tab index
	invalidSession = NewSession("invalid-session")
	invalidSession.AddFileTab("/path/test.log", []string{"content"})
	invalidSession.ActiveTabIndex = 5 // Out of range
	err = ValidateSession(invalidSession)
	if err == nil {
		t.Error("Session with invalid active tab index should fail validation")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("generateID should not return empty string")
	}

	if id1 == id2 {
		t.Error("generateID should return unique IDs")
	}

	// Check UUID-like format (roughly)
	if len(id1) < 32 {
		t.Error("Generated ID seems too short for UUID format")
	}
}
