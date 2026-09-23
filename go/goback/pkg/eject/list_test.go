package eject

import (
	"reflect"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
)

func profileRef(profile string, role config.EndpointRole, path string) config.Endpoint {
	return config.Endpoint{Profile: profile, Role: role, Path: path}
}

func mirrorRef(role config.EndpointRole, path string) config.Endpoint {
	return config.Endpoint{Mirror: true, Role: role, Path: path}
}

// SanDisk is shared by two profiles and the mirror, and every reference to it
// is kept even though the drive is listed once.
func TestMountedVolumesGroupsEveryReferenceByVolume(t *testing.T) {
	endpoints := []config.Endpoint{
		mirrorRef(config.RoleSource, "/Volumes/SanDisk/Media"),
		profileRef("media", config.RoleSource, "/Volumes/SanDisk/Media"),
		profileRef("macbook", config.RoleDestination, "/Volumes/SanDisk/macbook"),
		profileRef("macbook", config.RoleSource, "/Users/me"),
		profileRef("media", config.RoleDestination, "/Volumes/Elements/media"),
		mirrorRef(config.RoleDestination, "/Volumes/Elements/Media"),
	}

	got := MountedVolumes(endpoints, fakeMounts{mounted: map[string]bool{
		"/Volumes/Elements": true, "/Volumes/SanDisk": true,
	}})

	want := []MountedVolume{
		{Volume: "/Volumes/Elements", References: []config.Endpoint{
			profileRef("media", config.RoleDestination, "/Volumes/Elements/media"),
			mirrorRef(config.RoleDestination, "/Volumes/Elements/Media"),
		}},
		{Volume: "/Volumes/SanDisk", References: []config.Endpoint{
			profileRef("macbook", config.RoleDestination, "/Volumes/SanDisk/macbook"),
			profileRef("media", config.RoleSource, "/Volumes/SanDisk/Media"),
			mirrorRef(config.RoleSource, "/Volumes/SanDisk/Media"),
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MountedVolumes() = %#v, want %#v", got, want)
	}
}

func TestMountedVolumesOrderIsIndependentOfTheInputOrder(t *testing.T) {
	endpoints := []config.Endpoint{
		profileRef("b", config.RoleDestination, "/Volumes/Zeta/b"),
		profileRef("a", config.RoleDestination, "/Volumes/Alpha/a"),
		profileRef("b", config.RoleSource, "/Volumes/Alpha/b"),
		mirrorRef(config.RoleSource, "/Volumes/Alpha/m"),
	}
	reversed := make([]config.Endpoint, len(endpoints))
	for i, endpoint := range endpoints {
		reversed[len(endpoints)-1-i] = endpoint
	}

	mounts := alwaysMounted{}
	if a, b := MountedVolumes(endpoints, mounts), MountedVolumes(reversed, mounts); !reflect.DeepEqual(a, b) {
		t.Fatalf("MountedVolumes() depends on input order:\n%#v\n%#v", a, b)
	}
}

func TestMountedVolumesSkipsAbsentVolumesAndInternalPaths(t *testing.T) {
	endpoints := []config.Endpoint{
		profileRef("macbook", config.RoleSource, "/Users/me"),
		profileRef("macbook", config.RoleDestination, "/Volumes/SanDisk/macbook"),
		profileRef("local", config.RoleDestination, "/tmp/backups"),
		profileRef("media", config.RoleDestination, "/Volumes/Elements/media"),
	}

	got := MountedVolumes(endpoints, fakeMounts{mounted: map[string]bool{"/Volumes/SanDisk": true}})
	if len(got) != 1 || got[0].Volume != "/Volumes/SanDisk" {
		t.Fatalf("MountedVolumes() = %#v, want only the mounted SanDisk", got)
	}
}

func TestMountedVolumesIsEmptyWhenNothingIsMounted(t *testing.T) {
	endpoints := []config.Endpoint{profileRef("macbook", config.RoleDestination, "/Volumes/SanDisk/macbook")}
	if got := MountedVolumes(endpoints, fakeMounts{}); len(got) != 0 {
		t.Fatalf("MountedVolumes() = %#v, want none", got)
	}
	if got := MountedVolumes(nil, alwaysMounted{}); len(got) != 0 {
		t.Fatalf("MountedVolumes(nil) = %#v, want none", got)
	}
}

// Only a profile destination makes `goback eject --profile NAME` unmount the
// volume; a source or a mirror endpoint does not.
func TestDestinationProfilesIgnoresSourcesAndTheMirror(t *testing.T) {
	volume := MountedVolume{Volume: "/Volumes/SanDisk", References: []config.Endpoint{
		profileRef("media", config.RoleSource, "/Volumes/SanDisk/Media"),
		profileRef("zeta", config.RoleDestination, "/Volumes/SanDisk/zeta"),
		profileRef("alpha", config.RoleDestination, "/Volumes/SanDisk/alpha"),
		mirrorRef(config.RoleDestination, "/Volumes/SanDisk/Media"),
	}}

	if got := volume.DestinationProfiles(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("DestinationProfiles() = %#v, want alpha and zeta", got)
	}

	sourceOnly := MountedVolume{References: []config.Endpoint{
		profileRef("media", config.RoleSource, "/Volumes/SanDisk/Media"),
		mirrorRef(config.RoleSource, "/Volumes/SanDisk/Media"),
	}}
	if got := sourceOnly.DestinationProfiles(); len(got) != 0 {
		t.Fatalf("DestinationProfiles() = %#v, want none for sources and mirror endpoints", got)
	}
}

// Listing asks only whether each volume is mounted; diskutil is never involved.
func TestMountedVolumesChecksEachVolumeOnce(t *testing.T) {
	mounts := &countingMounts{}
	MountedVolumes([]config.Endpoint{
		profileRef("a", config.RoleSource, "/Volumes/SanDisk/a"),
		profileRef("a", config.RoleDestination, "/Volumes/SanDisk/b"),
	}, mounts)

	if !reflect.DeepEqual(mounts.asked, []string{"/Volumes/SanDisk"}) {
		t.Fatalf("asked about %#v, want SanDisk once", mounts.asked)
	}
}

type countingMounts struct{ asked []string }

func (m *countingMounts) Mounted(volume string) bool {
	m.asked = append(m.asked, volume)
	return true
}
