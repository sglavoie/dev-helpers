package buildcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// Modern rsync emits eleven columns; macOS openrsync emits nine. A tab
// separator avoids confusing summary padding with leading filename spaces.
var verificationItem = regexp.MustCompile(`^[<>ch.][fdLDS]([a-zA-Z.+? ]{7}|[a-zA-Z.+? ]{9})$`)

func compareVerification(ctx context.Context, output, source, destination string) ([]VerificationDifference, error) {
	differences := []VerificationDifference{}
	var problems []error
	sourceRoot := verificationSourceRoot(source)
	for _, line := range strings.Split(output, "\n") {
		if err := ctx.Err(); err != nil {
			problems = append(problems, err)
			break
		}
		if line == "" {
			continue
		}
		// Refuse unexpected output rather than silently missing a difference.
		item, name, found := strings.Cut(line, "\t")
		if !found || !verificationItem.MatchString(item) || strings.ContainsRune(item, '?') {
			problems = append(problems, fmt.Errorf("unrecognized rsync verification output: %q", line))
			continue
		}
		path := decodeRsyncName(name)
		if !filepath.IsLocal(path) || strings.ContainsRune(path, 0) {
			problems = append(problems, fmt.Errorf("invalid path in rsync verification: %q", path))
			continue
		}
		if item[1] == 'f' {
			status, err := compareVerificationFile(ctx, sourceRoot+"/"+path, filepath.Join(destination, path))
			if err != nil {
				problems = append(problems, fmt.Errorf("verify %q: %w", path, err))
			}
			if status != "" {
				differences = append(differences, VerificationDifference{Path: path, Status: status})
			}
			continue
		}
		created := item[2:] == strings.Repeat("+", len(item)-2)
		// Metadata changes alone do not indicate damaged contents. For these
		// nonregular entries, c covers link targets and device identities.
		if !created && item[2] != 'c' && item[3] != 's' {
			continue
		}
		status := "different"
		if created {
			// rsync also uses +++++++++ for replacement of a different entry type.
			// Lstat the leaf without a trailing slash so a dangling symlink exists.
			_, err := os.Lstat(filepath.Join(destination, filepath.Clean(path)))
			if os.IsNotExist(err) || errors.Is(err, unix.ENOTDIR) {
				status = "missing"
			} else if err != nil {
				problems = append(problems, fmt.Errorf("inspect backup entry %q: %w", path, err))
			}
		}
		differences = append(differences, VerificationDifference{Path: path, Status: status})
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].Path < differences[j].Path })
	return differences, errors.Join(problems...)
}

// Rsync includes the source basename unless a trailing slash, /. or /.. asks
// for its contents. Do not clean away symlinks followed by .. in configured paths.
func verificationSourceRoot(source string) string {
	lastSlash := strings.LastIndex(source, "/")
	base := source[lastSlash+1:]
	if base == "" || base == "." || base == ".." {
		return source
	}
	return source[:lastSlash+1]
}
