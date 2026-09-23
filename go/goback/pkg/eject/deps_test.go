package eject

import (
	"errors"
	"io/fs"
	"syscall"
	"testing"
	"time"
)

// fakeInfo is the metadata lstat would return for one path.
type fakeInfo struct {
	mode fs.FileMode
	sys  any
}

func (i fakeInfo) Name() string       { return "" }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fakeInfo) Sys() any           { return i.sys }

// rootDevice and driveDevice build directories on two distinct devices. The
// literals stay untyped because the Dev field's type differs between platforms.
func rootDevice() fakeInfo {
	return fakeInfo{mode: fs.ModeDir | 0o755, sys: &syscall.Stat_t{Dev: 1}}
}

func driveDevice() fakeInfo {
	return fakeInfo{mode: fs.ModeDir | 0o755, sys: &syscall.Stat_t{Dev: 7}}
}

// fakeFS answers lstat from a table, so mount detection is exercised without a
// drive and without depending on what /Volumes holds on the test machine.
func fakeFS(entries map[string]fakeInfo) osMounts {
	return osMounts{
		lstat: func(path string) (fs.FileInfo, error) {
			info, ok := entries[path]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return info, nil
		},
		device: statDevice,
	}
}

func TestOSMountsDetectsAMountPointByItsDevice(t *testing.T) {
	root := rootDevice()
	cases := []struct {
		name    string
		entries map[string]fakeInfo
		want    bool
	}{
		{
			name:    "a volume on its own device",
			entries: map[string]fakeInfo{"/Volumes": root, "/Volumes/SanDisk": driveDevice()},
			want:    true,
		},
		{
			name:    "an absent volume",
			entries: map[string]fakeInfo{"/Volumes": root},
		},
		{
			name:    "a leftover directory on the parent's device",
			entries: map[string]fakeInfo{"/Volumes": root, "/Volumes/SanDisk": rootDevice()},
		},
		{
			name: "a symlink",
			entries: map[string]fakeInfo{"/Volumes": root, "/Volumes/SanDisk": {
				mode: fs.ModeSymlink | 0o755, sys: &syscall.Stat_t{Dev: 7},
			}},
		},
		{
			name: "a regular file",
			entries: map[string]fakeInfo{"/Volumes": root, "/Volumes/SanDisk": {
				mode: 0o644, sys: &syscall.Stat_t{Dev: 7},
			}},
		},
		{
			name:    "a volume without filesystem metadata",
			entries: map[string]fakeInfo{"/Volumes": root, "/Volumes/SanDisk": {mode: fs.ModeDir}},
		},
		{
			name:    "a parent without filesystem metadata",
			entries: map[string]fakeInfo{"/Volumes": {mode: fs.ModeDir}, "/Volumes/SanDisk": driveDevice()},
		},
		{
			name:    "an unreadable parent",
			entries: map[string]fakeInfo{"/Volumes/SanDisk": driveDevice()},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fakeFS(tc.entries).Mounted("/Volumes/SanDisk"); got != tc.want {
				t.Fatalf("Mounted() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOSMountsTreatsAnLstatErrorAsNotMounted(t *testing.T) {
	mounts := osMounts{
		lstat:  func(string) (fs.FileInfo, error) { return nil, errors.New("permission denied") },
		device: statDevice,
	}
	if mounts.Mounted("/Volumes/SanDisk") {
		t.Fatal("Mounted() = true, want an unreadable volume reported as not mounted")
	}
}

// The real detector never reports the directory holding volumes, or the root
// of the filesystem, as a mounted volume of its own.
func TestOSDepsDoesNotTreatTheParentAsAMountPoint(t *testing.T) {
	if OSDeps().Mounts.Mounted("/Volumes/goback-test-volume-that-does-not-exist") {
		t.Fatal("an absent volume was reported as mounted")
	}
}
