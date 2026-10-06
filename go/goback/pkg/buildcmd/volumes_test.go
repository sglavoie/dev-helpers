package buildcmd

import (
	"slices"
	"strings"
	"testing"
)

type snapshotMounts []string

func (m snapshotMounts) Mounted(volume string) bool { return slices.Contains(m, volume) }

func TestSnapshotVolumeValidation(t *testing.T) {
	for _, tc := range []struct {
		name, path, resolved string
		mounted              snapshotMounts
		wantError            string
	}{
		{"local backup", "/Users/me/backup/daily", "/Users/me/backup/daily", nil, ""},
		{"similar prefix", "/Volumes-old/Backup/daily", "/Volumes-old/Backup/daily", nil, ""},
		{"mounted destination", "/Volumes/Backup/daily", "/Volumes/Backup/daily", snapshotMounts{"/Volumes/Backup"}, ""},
		{"stale destination", "/Volumes/Backup/daily", "/Volumes/Backup/daily", nil, "not mounted"},
		{"stale source", "/Volumes/Source/data", "/Volumes/Source/data", nil, "not mounted"},
		{"local alias to stale volume", "/Users/me/backup/daily", "/Volumes/Backup/daily", nil, "not mounted"},
		{"local alias to mounted volume", "/Users/me/backup/daily", "/Volumes/Backup/daily", snapshotMounts{"/Volumes/Backup"}, ""},
		{"escaping symlink", "/Volumes/Backup/daily", "/Users/me/data", snapshotMounts{"/Volumes/Backup"}, "outside its volume"},
		{"escaping sibling prefix", "/Volumes/Backup/daily", "/Volumes/Backup-old/daily", snapshotMounts{"/Volumes/Backup", "/Volumes/Backup-old"}, "outside its volume"},
		{"symlink within volume", "/Volumes/Backup/alias", "/Volumes/Backup/data", snapshotMounts{"/Volumes/Backup"}, ""},
		{"space in volume name", "/Volumes/Backup /daily", "/Volumes/Backup /daily", snapshotMounts{"/Volumes/Backup "}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSnapshotVolume(tc.path, tc.resolved, tc.mounted)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}
