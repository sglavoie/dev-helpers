package buildcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/viper"
)

// effectiveSource returns the correct source directory for the given backup type.
// For daily backups this is the configured source; for weekly/monthly it is <dest>/daily/.
func effectiveSource(backupType models.BackupTypes) string {
	prefix := config.ActiveProfilePrefix()
	src := viper.GetString(prefix + "source")
	dest := viper.GetString(prefix + "destination")

	switch backupType.String() {
	case "weekly", "monthly":
		return dest + "/daily/"
	default:
		return src
	}
}

// listFiles runs rsync --list-only to enumerate files under source, optionally
// applying exclude patterns. It returns the list of relative file paths.
func listFiles(source string, excludePatterns []string) ([]string, error) {
	args := []string{"-r", "--list-only"}
	for _, p := range excludePatterns {
		args = append(args, fmt.Sprintf("--exclude=%s", p))
	}
	// Trailing slash ensures rsync lists the contents, not the directory itself.
	if !strings.HasSuffix(source, "/") {
		source += "/"
	}
	args = append(args, source)

	cmd := exec.Command("rsync", args...)
	out, err := cmd.Output()
	if err != nil {
		// rsync exits with 23 (partial transfer due to error, e.g. permission
		// denied on some files) or 24 (vanished source files) routinely when
		// scanning a home directory. The output is still valid for everything
		// it could access, so we tolerate these codes.
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code != 23 && code != 24 {
				return nil, fmt.Errorf("rsync --list-only failed (exit %d): %s", code, exitErr.Stderr)
			}
		} else {
			return nil, fmt.Errorf("rsync --list-only failed: %w", err)
		}
	}

	return parseListOnly(string(out)), nil
}

// parseListOnly extracts file paths from rsync --list-only output.
// Each line has the format: "drwxr-xr-x       4,096 2024/01/15 10:30:00 path/to/file"
// We take everything after the date+time columns.
func parseListOnly(output string) []string {
	var files []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// rsync --list-only output has 5 whitespace-separated fields before the path:
		// permissions, size, date, time, path
		// However size can include commas and the date/time are fixed format.
		// The safest approach: find the path after the date+time pattern.
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		// Rejoin from field index 4 onward (the filename may contain spaces)
		path := strings.Join(parts[4:], " ")
		if path == "." || path == "./" {
			continue
		}
		files = append(files, path)
	}
	return files
}

// depthExcludePattern returns an rsync exclude pattern that prevents descending
// beyond the given depth. For depth N it produces "*/" repeated N times followed
// by "*", e.g. depth 1 → "*/*", depth 2 → "*/*/*".
func depthExcludePattern(depth int) string {
	return strings.Repeat("*/", depth) + "*"
}

// findExcluded runs two rsync --list-only passes (with and without exclude patterns)
// and returns paths that are present in the unfiltered list but absent in the filtered one.
// When depth > 0 both passes are limited to that many directory levels.
func findExcluded(source string, patterns []string, depth int) ([]string, error) {
	var depthPatterns []string
	if depth > 0 {
		depthPatterns = []string{depthExcludePattern(depth)}
	}

	allFiles, err := listFiles(source, depthPatterns)
	if err != nil {
		return nil, err
	}

	filteredFiles, err := listFiles(source, append(depthPatterns, patterns...))
	if err != nil {
		return nil, err
	}

	filteredSet := make(map[string]struct{}, len(filteredFiles))
	for _, f := range filteredFiles {
		filteredSet[f] = struct{}{}
	}

	var excluded []string
	for _, f := range allFiles {
		if _, ok := filteredSet[f]; !ok {
			if !isDSStore(f) {
				excluded = append(excluded, f)
			}
		}
	}

	sort.Strings(excluded)
	return collapseToTopLevel(excluded), nil
}

// isDSStore reports whether path is a .DS_Store file: either exactly ".DS_Store"
// or ending with "/.DS_Store".
func isDSStore(path string) bool {
	return path == ".DS_Store" || strings.HasSuffix(path, "/.DS_Store")
}

// collapseToTopLevel reduces a sorted list of excluded paths to unique
// top-level entries. Any path containing a "/" is collapsed to its first
// component (as a directory with trailing slash), and duplicates are removed.
// Top-level files (no internal "/") are kept as-is.
func collapseToTopLevel(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var result []string

	for _, p := range paths {
		top := topLevelEntry(p)
		key := strings.TrimSuffix(top, "/")
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, top)
		}
	}

	sort.Strings(result)
	return result
}

// topLevelEntry extracts the first path component from a path. A path with no
// "/" (e.g. "file.txt") or only a trailing "/" (e.g. ".cache/") is already
// top-level and returned unchanged. A deeper path like "CacheClip/audio/foo"
// returns "CacheClip/".
func topLevelEntry(path string) string {
	idx := strings.Index(path, "/")
	if idx == -1 || idx == len(path)-1 {
		return path
	}
	return path[:idx+1]
}

// TestSinglePattern tests a single exclude pattern against the effective source
// for the given backup type and displays which files/directories it would exclude.
func TestSinglePattern(backupType models.BackupTypes, pattern string, subdir string, depth int) {
	source := effectiveSource(backupType)
	source = applySubdir(source, subdir)

	if err := checkSourceAccessible(source); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	excluded, err := findExcluded(source, []string{pattern}, depth)
	if err != nil {
		fmt.Printf("Error testing pattern: %v\n", err)
		os.Exit(1)
	}

	if len(excluded) == 0 {
		fmt.Printf("Pattern %q does not exclude any files from %s\n", pattern, source)
		return
	}

	summary := fmt.Sprintf("%d top-level files/directories excluded by pattern %q from %s", len(excluded), pattern, source)
	content := strings.Join(excluded, "\n")

	displayExcludedResults(summary, content, len(excluded))
}

// TestAllExcluded tests all configured exclude patterns for the given backup type
// and displays the combined list of excluded files/directories.
func TestAllExcluded(backupType models.BackupTypes, subdir string, depth int) {
	cfgPrefix := config.ActiveProfilePrefix() + "rsync." + backupType.String() + "."
	patterns := viper.GetStringSlice(cfgPrefix + "excludedPatterns")

	if len(patterns) == 0 {
		fmt.Printf("No exclude patterns configured for %s backups in profile %s\n",
			backupType.String(), config.ActiveProfileName)
		return
	}

	source := effectiveSource(backupType)
	source = applySubdir(source, subdir)

	if err := checkSourceAccessible(source); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	excluded, err := findExcluded(source, patterns, depth)
	if err != nil {
		fmt.Printf("Error testing patterns: %v\n", err)
		os.Exit(1)
	}

	if len(excluded) == 0 {
		fmt.Printf("The %d configured patterns do not exclude any files from %s\n",
			len(patterns), source)
		return
	}

	summary := fmt.Sprintf("%d top-level files/directories excluded by %d configured patterns from %s",
		len(excluded), len(patterns), source)
	content := strings.Join(excluded, "\n")

	displayExcludedResults(summary, content, len(excluded))
}

// applySubdir scopes a source path to a subdirectory. If subdir is empty the
// source is returned unchanged. If subdir is an absolute path it replaces the
// source entirely; otherwise it is joined as a relative path.
func applySubdir(source, subdir string) string {
	if subdir == "" {
		return source
	}
	if filepath.IsAbs(subdir) {
		return subdir
	}
	return filepath.Join(source, subdir)
}

// checkSourceAccessible verifies that the source directory exists and is readable.
func checkSourceAccessible(source string) error {
	path := strings.TrimSuffix(source, "/")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Source directory %s is not accessible. Is the drive mounted?", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("Source path %s is not a directory", path)
	}
	return nil
}

// displayExcludedResults prints or pages the excluded file list depending on size.
func displayExcludedResults(summary string, content string, count int) {
	const pagerThreshold = 50

	if count >= pagerThreshold {
		pagerContent := summary + "\n\n" + content
		printer.Pager(pagerContent, "Excluded files")
	} else {
		fmt.Println(summary)
		fmt.Println()
		fmt.Println(content)
	}
}
