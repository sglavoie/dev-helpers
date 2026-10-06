package buildcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
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
	mounts := eject.OSDeps().Mounts
	if err := validateSnapshotVolume(r.updatedSrc, src, mounts); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if err := validateSnapshotVolume(r.updatedDestDir, dest, mounts); err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	if src == dest {
		return fmt.Errorf("source and destination are the same: %s", src)
	}
	if containsPath(src, dest) || containsPath(dest, src) {
		return fmt.Errorf("source and destination directories overlap: %s and %s", src, dest)
	}
	return nil
}

// validateSnapshotVolume checks both the configured path and its resolved
// target. Local backups are allowed; paths under /Volumes must stay on a
// mounted volume. Reuse the same device-based detector as volume ejection.
func validateSnapshotVolume(path, resolved string, mounts eject.Mounts) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, endpoint := range []string{absolute, resolved} {
		// Preserve whitespace in volume names: it is part of the path.
		rel, err := filepath.Rel(eject.VolumesRoot, endpoint)
		if err != nil || rel == "." || !containsPath(eject.VolumesRoot, endpoint) {
			continue
		}
		name, _, _ := strings.Cut(rel, string(filepath.Separator))
		volume := filepath.Join(eject.VolumesRoot, name)
		if !mounts.Mounted(volume) {
			return fmt.Errorf("volume %s is not mounted; refusing to use %s", volume, path)
		}
		if !containsPath(volume, resolved) {
			return fmt.Errorf("path %s resolves outside its volume %s to %s", path, volume, resolved)
		}
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
