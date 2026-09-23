package eject

import (
	"cmp"
	"slices"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
)

// MountedVolume is a mounted volume together with every configured endpoint
// that lives on it.
type MountedVolume struct {
	Volume     string
	References []config.Endpoint
}

// DestinationProfiles returns, sorted and once each, the profiles whose
// destination lives on the volume: the ones `goback eject --profile NAME`
// would unmount it for.
func (v MountedVolume) DestinationProfiles() []string {
	var profiles []string
	for _, ref := range v.References {
		if !ref.Mirror && ref.Role == config.RoleDestination {
			profiles = append(profiles, ref.Profile)
		}
	}
	slices.Sort(profiles)
	return slices.Compact(profiles)
}

// MountedVolumes groups the endpoints by the /Volumes/<name> mount point they
// live on and keeps the volumes that are currently mounted, sorted by mount
// point. Endpoints outside /Volumes are dropped. It only checks whether each
// volume is mounted and never ejects anything.
func MountedVolumes(endpoints []config.Endpoint, mounts Mounts) []MountedVolume {
	grouped := make(map[string][]config.Endpoint)
	for _, endpoint := range endpoints {
		volume, ok := VolumeOf(endpoint.Path)
		if !ok {
			continue
		}
		grouped[volume] = append(grouped[volume], endpoint)
	}

	var volumes []MountedVolume
	for volume, refs := range grouped {
		if !mounts.Mounted(volume) {
			continue
		}
		slices.SortStableFunc(refs, compareEndpoints)
		volumes = append(volumes, MountedVolume{Volume: volume, References: refs})
	}
	slices.SortFunc(volumes, func(a, b MountedVolume) int { return cmp.Compare(a.Volume, b.Volume) })
	return volumes
}

// compareEndpoints orders profile references before mirror ones, then by
// profile name, source before destination, and finally by path.
func compareEndpoints(a, b config.Endpoint) int {
	return cmp.Or(
		compareBool(a.Mirror, b.Mirror),
		cmp.Compare(a.Profile, b.Profile),
		compareBool(a.Role == config.RoleDestination, b.Role == config.RoleDestination),
		cmp.Compare(a.Path, b.Path),
	)
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}
