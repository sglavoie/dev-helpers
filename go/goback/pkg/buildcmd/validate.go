package buildcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/viper"
)

// validateBeforeRun only reads paths. Destination creation belongs to execution,
// after confirmation, and is never performed for a dry run.
func (r *builder) validateBeforeRun() error {
	entries, err := os.ReadDir(r.updatedSrc)
	if err != nil {
		return fmt.Errorf("cannot read source directory %s: %w", r.updatedSrc, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("source directory %s is empty", r.updatedSrc)
	}
	src, err := filepath.EvalSymlinks(r.updatedSrc)
	if err != nil {
		return err
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return err
	}

	dest, err := filepath.EvalSymlinks(r.updatedDestDir)
	if os.IsNotExist(err) {
		// Snapshot destinations have an existing configured parent. A dangling
		// symlink is not an absent destination and must not be followed later.
		if _, statErr := os.Lstat(r.updatedDestDir); !os.IsNotExist(statErr) {
			return fmt.Errorf("cannot resolve destination %s: %w", r.updatedDestDir, err)
		}
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(r.updatedDestDir))
		if parentErr != nil {
			return parentErr
		}
		dest = filepath.Join(parent, filepath.Base(r.updatedDestDir))
	} else if err != nil {
		return err
	} else {
		info, err := os.Stat(dest)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("destination is not a directory: %s", dest)
		}
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	if src == dest {
		return fmt.Errorf("source and destination are the same: %s", src)
	}
	if containsPath(src, dest) || containsPath(dest, src) {
		return fmt.Errorf("source and destination directories overlap: %s and %s", src, dest)
	}
	return nil
}

func containsPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// validateSettings checks configuration without requiring mounted endpoints.
func (r *builder) validateSettings() error {
	prefix := config.ActiveProfilePrefix()
	if _, _, err := configuredPaths(r.builderType.String() != "daily"); err != nil {
		return err
	}
	if !viper.IsSet(r.builderSettingsPrefix() + "archive") {
		return fmt.Errorf("no rsync.%s configuration found for profile %q (%srsync.%s.archive is required)", r.builderType.String(), config.ActiveProfileName, prefix, r.builderType.String())
	}
	return nil
}
