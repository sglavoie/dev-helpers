package buildcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// listFilteredFiles enumerates without changing the source. A partial listing
// cannot safely select files for deletion.
func listFilteredFiles(source string, filters []string) ([]string, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	args := append([]string{"-r", "--list-only"}, filters...)
	args = append(args, "--", strings.TrimSuffix(source, "/")+"/")
	cmd := exec.Command("rsync", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("rsync scan incomplete (exit %d); cannot determine excluded entries: %s", exitErr.ExitCode(), exitErr.Stderr)
		}
		return nil, fmt.Errorf("rsync --list-only failed: %w", err)
	}
	return parseListOnly(string(out))
}

// Match only the metadata prefix; consume exactly one separator before the
// filename. Whitespace within the filename is data, including leading spaces.
var listingLine = regexp.MustCompile(`^([bcdlps-])[rwxStTs-]{9}\s+[\d,]+\s+\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} (.*)$`)

func parseListOnly(output string) ([]string, error) {
	var files []string
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		match := listingLine.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("unrecognized rsync listing line: %q", line)
		}
		path := decodeRsyncName(match[2])
		if path == "." || path == "./" {
			continue
		}
		if !filepath.IsLocal(path) || strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("invalid path in rsync listing: %q", path)
		}
		if match[1] == "d" && !strings.HasSuffix(path, "/") {
			path += "/"
		}
		files = append(files, path)
	}
	return files, nil
}

// rsync escapes unsafe bytes as \#ooo and escapes a literal backslash when
// it could otherwise be mistaken for that notation. Decode in one pass.
func decodeRsyncName(name string) string {
	var out strings.Builder
	for i := 0; i < len(name); {
		if i+5 <= len(name) && name[i:i+2] == `\#` {
			if n, err := strconv.ParseUint(name[i+2:i+5], 8, 8); err == nil {
				out.WriteByte(byte(n))
				i += 5
				continue
			}
		}
		out.WriteByte(name[i])
		i++
	}
	return out.String()
}

// depthExcludePattern returns an rsync exclude pattern that prevents descending
// beyond the given depth. For depth N it produces "*/" repeated N times followed
// by "*", e.g. depth 1 → "*/*", depth 2 → "*/*/*".
func depthExcludePattern(depth int) string {
	return strings.Repeat("*/", depth) + "*"
}

// FindExcluded runs two rsync --list-only passes (with and without exclude patterns)
// and returns paths that are present in the unfiltered list but absent in the filtered one.
// It returns exact excluded roots, preserving parents that contain kept files.
// When depth > 0 both passes are limited to that many directory levels.
func FindExcluded(source string, patterns []string, depth int) ([]string, error) {
	return FindExcludedWithFilters(source, filterArgs(nil, patterns), depth)
}

// isDSStore reports whether path is a .DS_Store file: either exactly ".DS_Store"
// or ending with "/.DS_Store".
func isDSStore(path string) bool {
	return path == ".DS_Store" || strings.HasSuffix(path, "/.DS_Store")
}

// TestSinglePattern tests a single exclude pattern against the effective source
// for the given backup type and displays which files/directories it would exclude.
func TestSinglePattern(backupType models.BackupTypes, pattern string, subdir string, depth int) {
	source := effectiveSource(backupType)
	excluded, err := findExcludedInSubdir(source, filterArgs(nil, []string{pattern}), subdir, depth)
	if err != nil {
		fmt.Printf("Error testing pattern: %v\n", err)
		os.Exit(1)
	}
	source = previewScopeDescription(source, subdir)

	if len(excluded) == 0 {
		fmt.Printf("Pattern %q does not exclude any files from %s\n", pattern, source)
		return
	}

	summary := fmt.Sprintf("%d excluded entries matching pattern %q from %s", len(excluded), pattern, source)
	content := strings.Join(excluded, "\n")

	displayExcludedResults(summary, content, len(excluded))
}

// TestAllExcluded tests all configured exclude patterns for the given backup type
// and displays the combined list of excluded files/directories.
func TestAllExcluded(backupType models.BackupTypes, subdir string, depth int) {
	filters := FilterArgs(backupType)
	if len(filters) == 0 {
		fmt.Printf("No include/exclude patterns configured for %s backups in profile %s\n", backupType.String(), config.ActiveProfileName)
		return
	}

	source := effectiveSource(backupType)
	excluded, err := findExcludedInSubdir(source, filters, subdir, depth)
	if err != nil {
		fmt.Printf("Error testing patterns: %v\n", err)
		os.Exit(1)
	}
	source = previewScopeDescription(source, subdir)

	if len(excluded) == 0 {
		fmt.Printf("The %d configured patterns do not exclude any files from %s\n",
			len(filters), source)
		return
	}

	summary := fmt.Sprintf("%d excluded entries under %d configured filters from %s",
		len(excluded), len(filters), source)
	content := strings.Join(excluded, "\n")

	displayExcludedResults(summary, content, len(excluded))
}

// Keep the rsync root unchanged: rebasing it changes anchored filters and loses
// exclusions inherited from parent directories. Paths remain source-relative;
// depth is measured from the selected subdirectory.
func findExcludedInSubdir(source string, filters []string, subdir string, depth int) ([]string, error) {
	if source == "" {
		return nil, fmt.Errorf("source not set for profile %q", config.ActiveProfileName)
	}
	if depth < 0 {
		return nil, fmt.Errorf("--depth must be greater than or equal to 0")
	}
	root, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	scope := subdir
	if !filepath.IsAbs(scope) {
		scope = filepath.Join(root, scope)
	}
	rel, err := filepath.Rel(root, scope)
	if err != nil || !containsPath(root, scope) {
		return nil, fmt.Errorf("--subdir must be inside the backup source %s", source)
	}
	if err := CheckSourceAccessible(scope); err != nil {
		return nil, err
	}
	scanDepth := depth
	if depth > 0 && rel != "." {
		scanDepth += strings.Count(filepath.ToSlash(rel), "/") + 1
	}
	excluded, err := FindExcludedWithFilters(root, filters, scanDepth)
	if err != nil || rel == "." {
		return excluded, err
	}
	prefix := filepath.ToSlash(rel) + "/"
	var scoped []string
	for _, path := range excluded {
		switch {
		case strings.HasPrefix(path, prefix):
			scoped = append(scoped, path)
		case strings.HasSuffix(path, "/") && strings.HasPrefix(prefix, path):
			// An excluded ancestor means the entire selected directory is excluded.
			return []string{prefix}, nil
		}
	}
	return scoped, nil
}

func previewScopeDescription(source, subdir string) string {
	if subdir != "" {
		return fmt.Sprintf("%s (subdirectory %q)", source, subdir)
	}
	return source
}

// FindExcludedRoots returns the root entries of
// excluded subtrees instead of collapsing to top-level path components.
// For example, if sglavoie/.cache/ and sglavoie/node_modules/ are excluded,
// it returns those paths rather than collapsing both to sglavoie/.
func FindExcludedRoots(source string, patterns []string, depth int) ([]string, error) {
	return FindExcludedWithFilters(source, filterArgs(nil, patterns), depth)
}

// FindExcludedWithFilters returns exact roots omitted by the ordered backup
// filters. Both scans must succeed before any candidates are returned.
func FindExcludedWithFilters(source string, filters []string, depth int) ([]string, error) {
	if depth < 0 {
		return nil, fmt.Errorf("--depth must be greater than or equal to 0")
	}
	var depthArgs []string
	if depth > 0 {
		depthArgs = []string{"--exclude=" + depthExcludePattern(depth)}
	}
	allFiles, err := listFilteredFiles(source, depthArgs)
	if err != nil {
		return nil, err
	}
	filteredFiles, err := listFilteredFiles(source, append(depthArgs, filters...))
	if err != nil {
		return nil, err
	}
	kept := make(map[string]bool, len(filteredFiles))
	for _, f := range filteredFiles {
		kept[f] = true
	}
	var excluded []string
	for _, f := range allFiles {
		if !kept[f] && !isDSStore(f) {
			excluded = append(excluded, f)
		}
	}
	sort.Strings(excluded)
	return rootsOnly(excluded), nil
}

// rootsOnly filters a sorted list of paths to only include root entries —
// paths that are not children of another excluded path. Since the input is
// sorted, a child always follows its parent, so we only need to track the
// most recent root.
func rootsOnly(paths []string) []string {
	var roots []string
	var lastRoot string
	for _, p := range paths {
		if lastRoot != "" && strings.HasPrefix(p, lastRoot) {
			continue
		}
		roots = append(roots, p)
		if strings.HasSuffix(p, "/") {
			lastRoot = p
		} else {
			lastRoot = ""
		}
	}
	return roots
}

// CheckSourceAccessible verifies that the source directory exists and is readable.
func CheckSourceAccessible(source string) error {
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
