package cleanbackup

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/viper"
)

func fixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	viper.Reset()
	previous := config.ActiveProfileName
	config.ActiveProfileName = "test"
	t.Cleanup(func() { viper.Reset(); config.ActiveProfileName = previous })
	root := t.TempDir()
	daily := filepath.Join(root, "daily")
	if err := os.Mkdir(daily, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"keep.txt", "two  spaces.txt", "two spaces.txt"} {
		if err := os.WriteFile(filepath.Join(daily, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	viper.Set("profiles.test.destination", root)
	viper.Set("profiles.test.rsync.daily.includedPatterns", []string{"keep.txt"})
	viper.Set("profiles.test.rsync.daily.excludedPatterns", []string{"*.txt"})
	return daily
}

func TestCleanupDryRunNeverPromptsOrDeletes(t *testing.T) {
	daily := fixture(t)
	err := cleanType(models.Daily{}, true, func(string) bool { t.Fatal("prompted"); return true }, func(string) error { t.Fatal("deleted"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(daily)
	if err != nil || len(entries) != 3 {
		t.Fatalf("entries = %v, error = %v", entries, err)
	}
}

func TestCleanupDeletesExactNamesAndPreservesIncludes(t *testing.T) {
	daily := fixture(t)
	if err := cleanType(models.Daily{}, false, func(string) bool { return true }, os.RemoveAll); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(daily)
	if err != nil || len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Fatalf("entries = %v, error = %v", entries, err)
	}
}

func TestCleanupReturnsFailuresAndContinues(t *testing.T) {
	daily := fixture(t)
	denied := errors.New("permission denied")
	calls := 0
	err := cleanType(models.Daily{}, false, func(string) bool { return true }, func(path string) error {
		calls++
		if filepath.Base(path) == "two  spaces.txt" {
			return denied
		}
		return os.RemoveAll(path)
	})
	if !errors.Is(err, denied) || calls != 2 {
		t.Fatalf("error = %v, calls = %d", err, calls)
	}
	if _, err := os.Stat(filepath.Join(daily, "two spaces.txt")); !os.IsNotExist(err) {
		t.Fatal("later deletion did not run")
	}
}
