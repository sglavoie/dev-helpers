package buildcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/viper"
)

func TestCleanupScanPreservesNamesAndIncludePrecedence(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	root := t.TempDir()
	names := []string{"two  spaces.txt", "two spaces.txt", " leading and trailing ", "tab\tname", "new\nline", `\#012`, "keep.txt", "a -> b"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder", "child"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("keep.txt", filepath.Join(root, "link -> name")); err != nil {
		t.Fatal(err)
	}
	filters := filterArgs([]string{"keep.txt", "two spaces.txt"}, []string{"*"})
	got, err := FindExcludedWithFilters(root, filters, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"two  spaces.txt", " leading and trailing ", "tab\tname", "new\nline", `\#012`, "a -> b", "folder/", "link -> name"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCleanupRefusesPartialListings(t *testing.T) {
	for _, code := range []string{"23", "24"} {
		t.Run(code, func(t *testing.T) {
			root := t.TempDir()
			script := "#!/bin/sh\nprintf '%s\\n' '-rw-r--r-- 0 2026/10/06 10:00:00 file'\nexit " + code + "\n"
			if err := os.WriteFile(filepath.Join(root, "rsync"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root)
			got, err := FindExcludedWithFilters(root, []string{"--exclude=*"}, 0)
			if err == nil || got != nil || !strings.Contains(err.Error(), "incomplete") {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}

func TestDerivedBackupsDoNotRequireOriginalSource(t *testing.T) {
	for _, kind := range []string{"weekly", "monthly"} {
		t.Run(kind, func(t *testing.T) {
			src, dest, _ := snapshotFixture(t)
			if err := os.RemoveAll(src); err != nil {
				t.Fatal(err)
			}
			daily := filepath.Join(dest, "daily")
			if err := os.Mkdir(daily, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(daily, "file"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			viper.Set("profiles.test.rsync."+kind+".archive", true)
			build := BuildWeekly
			if kind == "monthly" {
				build = BuildMonthly
			}
			b, err := build()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(b.CommandString(), daily+"/") {
				t.Fatal(b.CommandString())
			}
			requireAbsent(t, filepath.Join(dest, kind))
			if _, err := BuildDaily(); err == nil {
				t.Fatal("daily accepted missing original source")
			}
			if err := os.RemoveAll(daily); err != nil {
				t.Fatal(err)
			}
			if _, err := build(); err == nil {
				t.Fatal("derived backup accepted missing daily source")
			}
		})
	}
}

func TestDerivedFiltersInheritDailyExcludes(t *testing.T) {
	snapshotFixture(t)
	viper.Set("profiles.test.rsync.daily.excludedPatterns", []string{"*.tmp"})
	viper.Set("profiles.test.rsync.weekly.includedPatterns", []string{"keep.tmp"})
	want := []string{"--include=keep.tmp", "--exclude=*.tmp", "--exclude=*"}
	if got := FilterArgs(models.Weekly{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
