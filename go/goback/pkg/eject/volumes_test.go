package eject

import (
	"reflect"
	"strings"
	"testing"
)

// Ejecting the wrong thing is unrecoverable, so the only paths that may ever
// become a diskutil argument are the ones strictly beneath /Volumes.
func TestVolumeOfAcceptsOnlyPathsBeneathVolumes(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "volume root", path: "/Volumes/SanDisk", want: "/Volumes/SanDisk"},
		{name: "directory on a volume", path: "/Volumes/SanDisk/macbook", want: "/Volumes/SanDisk"},
		{name: "nested directory", path: "/Volumes/Elements/Media/2026", want: "/Volumes/Elements"},
		{name: "trailing slash", path: "/Volumes/SanDisk/", want: "/Volumes/SanDisk"},
		{name: "surrounding whitespace", path: "  /Volumes/SanDisk/Media  ", want: "/Volumes/SanDisk"},
		{name: "repeated separators", path: "/Volumes//SanDisk///Media", want: "/Volumes/SanDisk"},
		{name: "volume name with spaces", path: "/Volumes/My Drive/backups", want: "/Volumes/My Drive"},
		{name: "volumes itself", path: "/Volumes", want: ""},
		{name: "volumes with a trailing slash", path: "/Volumes/", want: ""},
		{name: "escaping back to volumes", path: "/Volumes/SanDisk/..", want: ""},
		{name: "escaping above volumes", path: "/Volumes/SanDisk/../../Users", want: ""},
		{name: "home directory", path: "/Users/me/Documents", want: ""},
		{name: "root", path: "/", want: ""},
		{name: "a similarly named directory", path: "/VolumesBackup/SanDisk", want: ""},
		{name: "relative path", path: "Volumes/SanDisk", want: ""},
		{name: "empty", path: "", want: ""},
		{name: "whitespace only", path: "   ", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := VolumeOf(tc.path)
			if ok != (tc.want != "") {
				t.Fatalf("VolumeOf(%q) ok = %v, want %v", tc.path, ok, tc.want != "")
			}
			if got != tc.want {
				t.Fatalf("VolumeOf(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestVolumesDeduplicatesAndSorts(t *testing.T) {
	paths := []string{
		"/Users/me",
		"/Volumes/SanDisk/macbook",
		"/Volumes/SanDisk/Media",
		"/Volumes/Elements/media",
		"/Volumes/Elements/Media",
		"/Volumes/SanDisk",
	}

	want := []string{"/Volumes/Elements", "/Volumes/SanDisk"}
	if got := Volumes(paths); !reflect.DeepEqual(got, want) {
		t.Fatalf("Volumes() = %#v, want %#v", got, want)
	}
}

// The order must come from the volume names rather than from the order the
// configuration happened to list its profiles in.
func TestVolumesOrderIsIndependentOfTheInputOrder(t *testing.T) {
	forward := Volumes([]string{"/Volumes/SanDisk/a", "/Volumes/Elements/b", "/Volumes/Backup/c"})
	reverse := Volumes([]string{"/Volumes/Backup/c", "/Volumes/Elements/b", "/Volumes/SanDisk/a"})

	want := []string{"/Volumes/Backup", "/Volumes/Elements", "/Volumes/SanDisk"}
	if !reflect.DeepEqual(forward, want) || !reflect.DeepEqual(reverse, want) {
		t.Fatalf("Volumes() = %#v and %#v, want %#v for both", forward, reverse, want)
	}
}

func TestVolumesDropsEverythingOutsideVolumes(t *testing.T) {
	got := Volumes([]string{"/Users/me", "/tmp/backups", "/Volumes", "", "relative/path"})
	if len(got) != 0 {
		t.Fatalf("Volumes() = %#v, want none", got)
	}
}

func TestConfiguredVolumeAcceptsANameOrAMountPoint(t *testing.T) {
	paths := []string{"/Users/me", "/Volumes/SanDisk/macbook", "/Volumes/Elements/Media", "/Volumes/My Drive/x"}

	for arg, want := range map[string]string{
		"Elements":           "/Volumes/Elements",
		" Elements ":         "/Volumes/Elements",
		"/Volumes/Elements":  "/Volumes/Elements",
		"/Volumes/Elements/": "/Volumes/Elements",
		"My Drive":           "/Volumes/My Drive",
	} {
		got, err := ConfiguredVolume(arg, paths)
		if err != nil || got != want {
			t.Fatalf("ConfiguredVolume(%q) = %q, %v, want %q", arg, got, err, want)
		}
	}
}

// Only a volume the configuration references may be named, and only by its
// name or its mount point, never by a path inside it or outside /Volumes.
func TestConfiguredVolumeRefusesAnythingElse(t *testing.T) {
	paths := []string{"/Users/me", "/Volumes/SanDisk/macbook", "/Volumes/Elements/Media"}

	for _, arg := range []string{
		"Backup", "/Volumes/Backup", "elements", "", " ", ".", "..", "/Volumes", "/Volumes/",
		"Elements/Media", "/Volumes/Elements/Media", "../Elements", "/Volumes/SanDisk/../Elements/x",
		"/Users/me", "/", "Macintosh HD",
	} {
		got, err := ConfiguredVolume(arg, paths)
		if err == nil {
			t.Fatalf("ConfiguredVolume(%q) = %q, want it refused", arg, got)
		}
	}

	_, err := ConfiguredVolume("Backup", paths)
	if err == nil || !strings.Contains(err.Error(), "/Volumes/Elements, /Volumes/SanDisk") {
		t.Fatalf("error = %v, want it to list the configured volumes", err)
	}
	if _, err := ConfiguredVolume("Elements", []string{"/Users/me"}); err == nil || !strings.Contains(err.Error(), "no volume") {
		t.Fatalf("error = %v, want it to say nothing is configured", err)
	}
}
