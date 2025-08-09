package models

import (
	"time"

	"github.com/google/uuid"
)

// Stash represents a collection of stashed entries
type Stash struct {
	ID        string    `json:"id"`         // Unique stash identifier
	EntryIDs  []string  `json:"entry_ids"`  // IDs of stashed entries
	CreatedAt time.Time `json:"created_at"` // When stash was created
}

// Config represents the application configuration and state
type Config struct {
	Entries     []Entry `json:"entries"`
	NextShortID int     `json:"next_short_id"`
	LastKeyword string  `json:"last_entry_keyword"`
	Stashes     []Stash `json:"stashes"` // Currently only one supported
}

// Entry represents a time tracking entry
type Entry struct {
	ID        string     `json:"id"`         // UUIDv7 for permanent reference
	ShortID   int        `json:"short_id"`   // for recent entries
	Keyword   string     `json:"keyword"`    // Primary categorization
	Tags      []string   `json:"tags"`       // Secondary categorization
	StartTime time.Time  `json:"start_time"` // When tracking started
	EndTime   *time.Time `json:"end_time"`   // When tracking stopped (nil if active)
	Duration  int        `json:"duration"`   // Cached seconds for completed entries
	Active    bool       `json:"active"`     // Currently running flag
	Stashed   bool       `json:"stashed"`    // Indicates if entry is part of a stash
}

// NewEntry creates a new entry with generated ID
func NewEntry(keyword string, tags []string, shortID int) *Entry {
	return &Entry{
		ID:        uuid.NewString(),
		ShortID:   shortID,
		Keyword:   keyword,
		Tags:      tags,
		StartTime: time.Now(),
		EndTime:   nil,
		Duration:  0,
		Active:    true,
		Stashed:   false,
	}
}

// Stop stops the entry and calculates duration
func (e *Entry) Stop() {
	if e.Active {
		now := time.Now()
		e.EndTime = &now
		e.Duration = int(now.Sub(e.StartTime).Seconds())
		e.Active = false
	}
}

// GetCurrentDuration returns the current duration in seconds
func (e *Entry) GetCurrentDuration() int {
	if e.Active {
		return int(time.Since(e.StartTime).Seconds())
	}
	return e.Duration
}

// HasTag checks if the entry has a specific tag
func (e *Entry) HasTag(tag string) bool {
	for _, t := range e.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasAnyTag checks if the entry has any of the specified tags
func (e *Entry) HasAnyTag(tags []string) bool {
	for _, tag := range tags {
		if e.HasTag(tag) {
			return true
		}
	}
	return false
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Entries:     []Entry{},
		NextShortID: 1,
		LastKeyword: "",
		Stashes:     []Stash{},
	}
}

// GetActiveEntries returns all active entries
func (c *Config) GetActiveEntries() []Entry {
	var active []Entry
	for _, entry := range c.Entries {
		if entry.Active {
			active = append(active, entry)
		}
	}
	return active
}

// GetEntryByShortID finds an entry by its short ID
func (c *Config) GetEntryByShortID(shortID int) *Entry {
	for i := range c.Entries {
		if c.Entries[i].ShortID == shortID {
			return &c.Entries[i]
		}
	}
	return nil
}

// GetEntryByID finds an entry by its UUID
func (c *Config) GetEntryByID(id string) *Entry {
	for i := range c.Entries {
		if c.Entries[i].ID == id {
			return &c.Entries[i]
		}
	}
	return nil
}

// GetEntriesByKeyword returns all entries matching a keyword
func (c *Config) GetEntriesByKeyword(keyword string) []Entry {
	var matches []Entry
	for _, entry := range c.Entries {
		if entry.Keyword == keyword {
			matches = append(matches, entry)
		}
	}
	return matches
}

// GetEntriesPtrsByKeyword returns pointers to all entries matching a keyword
func (c *Config) GetEntriesPtrsByKeyword(keyword string) []*Entry {
	var matches []*Entry
	for i := range c.Entries {
		if c.Entries[i].Keyword == keyword {
			matches = append(matches, &c.Entries[i])
		}
	}
	return matches
}

// GetNonStashedEntriesPtrsByKeyword returns pointers to non-stashed entries matching a keyword
func (c *Config) GetNonStashedEntriesPtrsByKeyword(keyword string) []*Entry {
	var matches []*Entry
	for i := range c.Entries {
		if c.Entries[i].Keyword == keyword && !c.Entries[i].Stashed {
			matches = append(matches, &c.Entries[i])
		}
	}
	return matches
}

// HasActiveEntryForKeyword checks if there's already an active entry for the given keyword
func (c *Config) HasActiveEntryForKeyword(keyword string) bool {
	for _, entry := range c.Entries {
		if entry.Keyword == keyword && entry.Active {
			return true
		}
	}
	return false
}

// GetLastEntryByKeyword returns the most recent entry for a keyword
func (c *Config) GetLastEntryByKeyword(keyword string) *Entry {
	var lastEntry *Entry
	for i := range c.Entries {
		entry := &c.Entries[i]
		if entry.Keyword == keyword {
			if lastEntry == nil || entry.StartTime.After(lastEntry.StartTime) {
				lastEntry = entry
			}
		}
	}
	return lastEntry
}

// GetLastNonStashedEntryByKeyword returns the most recent non-stashed entry for a keyword
func (c *Config) GetLastNonStashedEntryByKeyword(keyword string) *Entry {
	var lastEntry *Entry
	for i := range c.Entries {
		entry := &c.Entries[i]
		if entry.Keyword == keyword && !entry.Stashed {
			if lastEntry == nil || entry.StartTime.After(lastEntry.StartTime) {
				lastEntry = entry
			}
		}
	}
	return lastEntry
}

// GetLastEntry returns the most recent entry
func (c *Config) GetLastEntry() *Entry {
	if len(c.Entries) == 0 {
		return nil
	}

	lastEntry := &c.Entries[0]
	for i := range c.Entries {
		entry := &c.Entries[i]
		if entry.StartTime.After(lastEntry.StartTime) {
			lastEntry = entry
		}
	}
	return lastEntry
}

// AddEntry adds a new entry and updates short IDs
func (c *Config) AddEntry(entry *Entry) {
	c.Entries = append(c.Entries, *entry)
	c.LastKeyword = entry.Keyword
	c.updateShortIDs()
}

// UpdateShortIDs reassigns short IDs to the most recent entries (exported for config package)
func (c *Config) UpdateShortIDs() {
	c.updateShortIDs()
}

// updateShortIDs reassigns short IDs to the most recent entries
func (c *Config) updateShortIDs() {
	// Sort entries by start time (most recent first)
	entries := make([]*Entry, len(c.Entries))
	for i := range c.Entries {
		entries[i] = &c.Entries[i]
	}

	// Simple bubble sort by start time (descending)
	for i := 0; i < len(entries)-1; i++ {
		for j := 0; j < len(entries)-i-1; j++ {
			if entries[j].StartTime.Before(entries[j+1].StartTime) {
				entries[j], entries[j+1] = entries[j+1], entries[j]
			}
		}
	}

	// Reset all short IDs
	for i := range c.Entries {
		c.Entries[i].ShortID = 0
	}

	// Assign short IDs to most recent entries
	for i := 0; i < len(entries) && i < 1_000; i++ {
		entries[i].ShortID = i + 1
	}
}

// RemoveEntry removes an entry by its UUID
func (c *Config) RemoveEntry(id string) bool {
	for i, entry := range c.Entries {
		if entry.ID == id {
			// If the entry is stashed, also remove it from any stashes
			if entry.Stashed {
				c.removeFromStashes(id)
			}
			c.Entries = append(c.Entries[:i], c.Entries[i+1:]...)
			c.updateShortIDs()
			return true
		}
	}
	return false
}

// removeFromStashes removes an entry ID from all stashes and cleans up empty stashes
func (c *Config) removeFromStashes(entryID string) {
	for i := len(c.Stashes) - 1; i >= 0; i-- {
		stash := &c.Stashes[i]
		// Remove the entry ID from this stash
		for j, id := range stash.EntryIDs {
			if id == entryID {
				stash.EntryIDs = append(stash.EntryIDs[:j], stash.EntryIDs[j+1:]...)
				break
			}
		}
		// If stash is now empty, remove it
		if len(stash.EntryIDs) == 0 {
			c.Stashes = append(c.Stashes[:i], c.Stashes[i+1:]...)
		}
	}
}

// HasActiveStash returns true if there is an active stash
func (c *Config) HasActiveStash() bool {
	return len(c.Stashes) > 0
}

// GetActiveStash returns the active stash (only one supported)
func (c *Config) GetActiveStash() *Stash {
	if len(c.Stashes) > 0 {
		return &c.Stashes[0]
	}
	return nil
}

// CreateStash creates a new stash from the given entry IDs
func (c *Config) CreateStash(entryIDs []string) *Stash {
	stash := Stash{
		ID:        uuid.NewString(),
		EntryIDs:  entryIDs,
		CreatedAt: time.Now(),
	}
	c.Stashes = []Stash{stash} // Only one stash supported
	return &c.Stashes[0]
}

// RemoveStash removes a stash by its ID
func (c *Config) RemoveStash(stashID string) bool {
	for i, stash := range c.Stashes {
		if stash.ID == stashID {
			c.Stashes = append(c.Stashes[:i], c.Stashes[i+1:]...)
			return true
		}
	}
	return false
}

// GetStashedEntries returns all stashed entries
func (c *Config) GetStashedEntries() []Entry {
	var stashed []Entry
	for _, entry := range c.Entries {
		if entry.Stashed {
			stashed = append(stashed, entry)
		}
	}
	return stashed
}

// GetStashedEntriesPtr returns pointers to all stashed entries
func (c *Config) GetStashedEntriesPtr() []*Entry {
	var stashed []*Entry
	for i := range c.Entries {
		if c.Entries[i].Stashed {
			stashed = append(stashed, &c.Entries[i])
		}
	}
	return stashed
}

// GetNonStashedEntries returns all entries that are not stashed
func (c *Config) GetNonStashedEntries() []Entry {
	var nonStashed []Entry
	for _, entry := range c.Entries {
		if !entry.Stashed {
			nonStashed = append(nonStashed, entry)
		}
	}
	return nonStashed
}
