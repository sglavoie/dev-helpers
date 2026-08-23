package cmd

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// writeTestConfig saves entries into a config file of its own and returns a
// manager for it together with the temporary directory holding it, which the
// caller is responsible for removing.
func writeTestConfig(t *testing.T, prefix string, entries []models.Entry) (*config.Manager, string) {
	t.Helper()

	tmpDir, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	configManager := config.NewManager(filepath.Join(tmpDir, "test_config.json"))
	if err := configManager.Save(&models.Config{Entries: entries}); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	return configManager, tmpDir
}

// taggedTestEntries returns four completed entries with repeated keywords and
// overlapping tags, which is what the tag and delete commands are exercised on.
func taggedTestEntries() []models.Entry {
	return []models.Entry{
		{
			ID:       "entry1",
			ShortID:  1,
			Keyword:  "meeting",
			Tags:     []string{"work", "important"},
			Duration: 3600,
		},
		{
			ID:       "entry2",
			ShortID:  2,
			Keyword:  "coding",
			Tags:     []string{"work", "project1"},
			Duration: 7200,
		},
		{
			ID:       "entry3",
			ShortID:  3,
			Keyword:  "documentation",
			Tags:     []string{"project1", "writing"},
			Duration: 1800,
		},
		{
			ID:       "entry4",
			ShortID:  4,
			Keyword:  "meeting",
			Tags:     []string{"important", "client"},
			Duration: 1200,
		},
	}
}
