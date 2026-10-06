package cleanlogs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/diagnostics"
)

// Clean trims current diagnostics and legacy home-directory logs. Candidates
// are collected before any deletion, so a scan error leaves every log intact.
func Clean(w io.Writer, keep, daily, weekly, monthly int, dryRun bool) error {
	for _, n := range []int{keep, daily, weekly, monthly} {
		if n < 0 {
			return fmt.Errorf("log retention counts must be greater than or equal to 0")
		}
	}
	paths, err := diagnostics.LogPaths()
	if err != nil {
		return err
	}
	candidates := append([]string(nil), paths[min(keep, len(paths)):]...)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		return err
	}
	for _, group := range []struct {
		kind string
		keep int
	}{{"daily", daily}, {"weekly", weekly}, {"monthly", monthly}} {
		legacy, err := legacyPaths(home, entries, group.kind)
		if err != nil {
			return err
		}
		candidates = append(candidates, legacy[min(group.keep, len(legacy)):]...)
	}
	if len(candidates) == 0 {
		fmt.Fprintln(w, "No logs to remove.")
		return nil
	}
	var failures []error
	seen := make(map[string]bool)
	for _, path := range candidates {
		if seen[path] {
			continue
		}
		seen[path] = true
		if dryRun {
			fmt.Fprintf(w, "Would remove %q\n", path)
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("remove log %q: %w", path, err))
			continue
		}
		fmt.Fprintf(w, "Removed %q\n", path)
	}
	return errors.Join(failures...)
}

func legacyPaths(home string, entries []os.DirEntry, kind string) ([]string, error) {
	type logFile struct {
		path     string
		modified time.Time
	}
	var files []logFile
	for _, entry := range entries {
		name := entry.Name()
		suffix, match := strings.CutPrefix(name, ".goback")
		if !entry.Type().IsRegular() || !match || strings.Contains(suffix, ".") || !strings.Contains(suffix, kind) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, logFile{filepath.Join(home, name), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modified.Equal(files[j].modified) {
			return files[i].path > files[j].path
		}
		return files[i].modified.After(files[j].modified)
	})
	var paths []string
	for _, file := range files {
		paths = append(paths, file.path)
	}
	return paths, nil
}
