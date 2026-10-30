package eject

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateMountedPath refuses stale /Volumes directories and aliases escaping
// their configured volume. Local paths are allowed. Both paths must name the
// same endpoint, with resolved containing no symlinks.
func ValidateMountedPath(path, resolved string, mounts Mounts) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	within := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	}
	for _, endpoint := range []string{absolute, resolved} {
		rel, err := filepath.Rel(VolumesRoot, endpoint)
		if err != nil || rel == "." || !within(VolumesRoot, endpoint) {
			continue
		}
		// Whitespace is part of the volume name.
		name, _, _ := strings.Cut(rel, string(filepath.Separator))
		volume := filepath.Join(VolumesRoot, name)
		if !mounts.Mounted(volume) {
			return fmt.Errorf("volume %s is not mounted; refusing to use %s", volume, path)
		}
		if !within(volume, resolved) {
			return fmt.Errorf("path %s resolves outside its volume %s to %s", path, volume, resolved)
		}
	}
	return nil
}
