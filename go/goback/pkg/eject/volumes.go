package eject

import (
	"path/filepath"
	"slices"
	"strings"
)

// VolumesRoot is where macOS mounts external volumes.
const VolumesRoot = "/Volumes"

// VolumeOf returns the /Volumes/<name> mount point the given path lives on. It
// reports false for anything that is not strictly beneath /Volumes: a relative
// path, an internal path, and /Volumes itself all have no volume to eject. It
// reads configuration only and never touches the filesystem.
func VolumeOf(path string) (string, bool) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if !strings.HasPrefix(cleaned, VolumesRoot+"/") {
		return "", false
	}

	name, _, _ := strings.Cut(strings.TrimPrefix(cleaned, VolumesRoot+"/"), "/")
	if name == "" {
		return "", false
	}
	return VolumesRoot + "/" + name, true
}

// Volumes returns the mount points of every given path that lives under
// /Volumes, each one once and sorted, so the same configuration always ejects
// the same volumes in the same order. Paths elsewhere are dropped, which is
// what keeps an internal path from ever reaching diskutil.
func Volumes(paths []string) []string {
	volumes := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		volume, ok := VolumeOf(path)
		if !ok || seen[volume] {
			continue
		}
		seen[volume] = true
		volumes = append(volumes, volume)
	}
	slices.Sort(volumes)
	return volumes
}
