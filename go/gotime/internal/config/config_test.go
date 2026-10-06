package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestStaleWriterCannotOverwriteNewData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	first, second := NewManager(path), NewManager(path)
	a, err := first.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.Load()
	if err != nil {
		t.Fatal(err)
	}
	a.AddEntry(models.NewEntry("first", nil, 1))
	if err := first.Save(a); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	b.AddEntry(models.NewEntry("second", nil, 1))
	if err := second.Save(b); err == nil {
		t.Fatal("stale writer overwrote newer data")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("data changed after rejected save")
	}
	b, err = second.Load()
	if err != nil {
		t.Fatal(err)
	}
	b.AddEntry(models.NewEntry("second", nil, 1))
	if err := second.Save(b); err != nil {
		t.Fatal(err)
	}
	result, _ := first.Load()
	if len(result.Entries) != 2 {
		t.Fatal("retry lost an entry")
	}
}

func TestAtomicSavePreservesSymlinkAndPermissions(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "data.json"), filepath.Join(dir, "link.json")
	if err := os.WriteFile(target, []byte(`{"entries":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manager := NewManager(link)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AddEntry(models.NewEntry("saved", nil, 1))
	if err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("save replaced the symlink")
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("save changed permissions")
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, ".gotime-*.tmp")); len(matches) != 0 {
		t.Fatal("temporary files left behind")
	}
}

func TestCompetingSaveLockFailsWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	manager := NewManager(path)
	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	unlock, err := lockSave(path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	cfg.AddEntry(models.NewEntry("blocked", nil, 1))
	if err := manager.Save(cfg); err == nil {
		t.Fatal("competing writer acquired the lock")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("locked save changed data")
	}
}

func TestNewDirectoriesUnderSymlinkKeepTheSameIdentity(t *testing.T) {
	dir := t.TempDir()
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manager := NewManager(filepath.Join(alias, "new", "nested", "data.json"))
	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AddEntry(models.NewEntry("saved", nil, 1))
	if err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
}
