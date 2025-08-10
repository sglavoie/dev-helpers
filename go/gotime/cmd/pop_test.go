package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopStashBug(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.json")

	// Create a config manager
	configManager := config.NewManager(configPath)
	cfg := &models.Config{
		Entries: []models.Entry{},
		Stashes: []models.Stash{},
	}

	// Create an active entry with a specific start time
	originalStartTime := time.Now().Add(-2 * time.Hour)
	entry := models.Entry{
		ID:        "test-entry-1",
		ShortID:   1,
		Keyword:   "coding",
		Tags:      []string{"project-x"},
		StartTime: originalStartTime,
		EndTime:   nil,
		Active:    true,
		Stashed:   false,
	}
	cfg.Entries = append(cfg.Entries, entry)

	// Save initial config
	err := configManager.Save(cfg)
	require.NoError(t, err)

	// Stash the entry (simulating the stash command)
	for i := range cfg.Entries {
		if cfg.Entries[i].Active {
			// Stop the entry
			cfg.Entries[i].Stop()
			// Mark as stashed
			cfg.Entries[i].Stashed = true
		}
	}

	// Create the stash
	cfg.CreateStash([]string{"test-entry-1"})

	// Save after stashing
	err = configManager.Save(cfg)
	require.NoError(t, err)

	// Verify the entry was stashed properly
	assert.False(t, cfg.Entries[0].Active, "Entry should not be active after stashing")
	assert.True(t, cfg.Entries[0].Stashed, "Entry should be marked as stashed")
	assert.NotNil(t, cfg.Entries[0].EndTime, "Entry should have an end time after stashing")
	stashedEndTime := cfg.Entries[0].EndTime
	stashedDuration := cfg.Entries[0].Duration

	// Wait a moment to ensure time difference
	time.Sleep(100 * time.Millisecond)

	// Now pop the stash (this is where the bug occurs)
	err = runPopAll(cfg, configManager)
	require.NoError(t, err)

	// After pop, we should have:
	// 1. The original stashed entry should remain with its end time (completed)
	// 2. A new active entry should be created with the same keyword/tags but new start time

	// Check that we now have 2 entries
	assert.Equal(t, 2, len(cfg.Entries), "Should have 2 entries after pop: original completed and new active")

	// Find the original entry (should be completed/stopped)
	var originalEntry *models.Entry
	var newEntry *models.Entry

	for i := range cfg.Entries {
		if cfg.Entries[i].ID == "test-entry-1" {
			originalEntry = &cfg.Entries[i]
		} else if cfg.Entries[i].Active {
			newEntry = &cfg.Entries[i]
		}
	}

	// Verify original entry is preserved correctly
	require.NotNil(t, originalEntry, "Original entry should still exist")
	assert.False(t, originalEntry.Active, "Original entry should not be active")
	assert.False(t, originalEntry.Stashed, "Original entry should not be stashed")
	assert.Equal(t, originalStartTime, originalEntry.StartTime, "Original entry start time should be preserved")
	assert.Equal(t, stashedEndTime, originalEntry.EndTime, "Original entry end time should be preserved")
	assert.Equal(t, stashedDuration, originalEntry.Duration, "Original entry duration should be preserved")

	// Verify new entry is created correctly
	require.NotNil(t, newEntry, "New active entry should be created")
	assert.True(t, newEntry.Active, "New entry should be active")
	assert.False(t, newEntry.Stashed, "New entry should not be stashed")
	assert.Equal(t, "coding", newEntry.Keyword, "New entry should have same keyword")
	assert.Equal(t, []string{"project-x"}, newEntry.Tags, "New entry should have same tags")
	assert.NotEqual(t, originalStartTime, newEntry.StartTime, "New entry should have different start time")
	assert.True(t, newEntry.StartTime.After(originalStartTime), "New entry start time should be after original")
	assert.Nil(t, newEntry.EndTime, "New entry should not have end time")
}

func TestPopSpecificEntries(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.json")

	// Create a config manager
	configManager := config.NewManager(configPath)
	cfg := &models.Config{
		Entries: []models.Entry{},
		Stashes: []models.Stash{},
	}

	// Create multiple active entries
	entry1StartTime := time.Now().Add(-3 * time.Hour)
	entry2StartTime := time.Now().Add(-2 * time.Hour)

	entry1 := models.Entry{
		ID:        "test-entry-1",
		ShortID:   1,
		Keyword:   "coding",
		Tags:      []string{"backend"},
		StartTime: entry1StartTime,
		EndTime:   nil,
		Active:    true,
		Stashed:   false,
	}

	entry2 := models.Entry{
		ID:        "test-entry-2",
		ShortID:   2,
		Keyword:   "meeting",
		Tags:      []string{"standup"},
		StartTime: entry2StartTime,
		EndTime:   nil,
		Active:    true,
		Stashed:   false,
	}

	cfg.Entries = append(cfg.Entries, entry1, entry2)

	// Stash all entries
	for i := range cfg.Entries {
		if cfg.Entries[i].Active {
			cfg.Entries[i].Stop()
			cfg.Entries[i].Stashed = true
		}
	}
	cfg.CreateStash([]string{"test-entry-1", "test-entry-2"})

	// Save after stashing
	err := configManager.Save(cfg)
	require.NoError(t, err)

	// Record stashed states
	entry1EndTime := cfg.Entries[0].EndTime
	entry1Duration := cfg.Entries[0].Duration

	// Wait a moment
	time.Sleep(100 * time.Millisecond)

	// Pop only the "coding" entry
	err = runPopSpecific(cfg, configManager, []string{"coding"})
	require.NoError(t, err)

	// Should have 3 entries: 2 original (1 still stashed, 1 completed) + 1 new active
	assert.Equal(t, 3, len(cfg.Entries), "Should have 3 entries after popping one")

	// Count entries by state
	var activeCount, stashedCount, completedCount int
	var newActiveEntry *models.Entry

	for i := range cfg.Entries {
		if cfg.Entries[i].Active {
			activeCount++
			if cfg.Entries[i].Keyword == "coding" {
				newActiveEntry = &cfg.Entries[i]
			}
		}
		if cfg.Entries[i].Stashed {
			stashedCount++
		}
		if !cfg.Entries[i].Active && !cfg.Entries[i].Stashed {
			completedCount++
		}
	}

	assert.Equal(t, 1, activeCount, "Should have 1 active entry")
	assert.Equal(t, 1, stashedCount, "Should have 1 stashed entry (meeting)")
	assert.Equal(t, 1, completedCount, "Should have 1 completed entry (original coding)")

	// Verify the new active entry
	require.NotNil(t, newActiveEntry, "Should have new active coding entry")
	assert.Equal(t, "coding", newActiveEntry.Keyword)
	assert.Equal(t, []string{"backend"}, newActiveEntry.Tags)
	assert.True(t, newActiveEntry.StartTime.After(entry1StartTime))

	// Verify original coding entry is preserved
	var originalCodingEntry *models.Entry
	for i := range cfg.Entries {
		if cfg.Entries[i].ID == "test-entry-1" {
			originalCodingEntry = &cfg.Entries[i]
			break
		}
	}

	require.NotNil(t, originalCodingEntry)
	assert.False(t, originalCodingEntry.Active)
	assert.False(t, originalCodingEntry.Stashed)
	assert.Equal(t, entry1StartTime, originalCodingEntry.StartTime)
	assert.Equal(t, entry1EndTime, originalCodingEntry.EndTime)
	assert.Equal(t, entry1Duration, originalCodingEntry.Duration)

	// Verify meeting entry is still stashed
	var meetingEntry *models.Entry
	for i := range cfg.Entries {
		if cfg.Entries[i].ID == "test-entry-2" {
			meetingEntry = &cfg.Entries[i]
			break
		}
	}

	require.NotNil(t, meetingEntry)
	assert.False(t, meetingEntry.Active)
	assert.True(t, meetingEntry.Stashed)
}
