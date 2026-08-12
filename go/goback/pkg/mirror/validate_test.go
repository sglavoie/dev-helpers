package mirror

import (
	"io/fs"
	"testing"
)

func TestValidateAcceptsMountedVolumes(t *testing.T) {
	cfg, deps, _ := mountedSetup()

	endpoints, err := Validate(cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	if endpoints.Source != testSource || endpoints.Destination != testDestination {
		t.Fatalf("endpoints = %#v, want the configured pair", endpoints)
	}
	if endpoints.DestinationIdentity == "" {
		t.Fatal("DestinationIdentity is empty, want the identity of the validated destination")
	}
}

// The mirror never creates its destination, so an absent one is refused with an
// error that says what to do rather than being accepted as a leaf to create.
func TestValidateRejectsAbsentDestinationLeaf(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	delete(deps.FS.(fakeFS).files, testDestination)

	_, err := Validate(cfg, deps)
	requireErrorContains(t, err, "does not exist")
	requireErrorContains(t, err, "create it yourself")
}

func TestValidateNormalizesPaths(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	cfg.Source = "/Volumes/SanDisk/Media/"
	cfg.Destination = "/Volumes/Elements/./Media"

	endpoints, err := Validate(cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	if endpoints.Source != testSource || endpoints.Destination != testDestination {
		t.Fatalf("endpoints = %#v, want the cleaned pair", endpoints)
	}
}

func TestValidateRejectsUnsafePaths(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		destination string
		want        string
	}{
		{
			name:        "empty source",
			source:      "",
			destination: testDestination,
			want:        "both a mirror source and a mirror destination are required",
		},
		{
			name:        "empty destination",
			source:      testSource,
			destination: "",
			want:        "both a mirror source and a mirror destination are required",
		},
		{
			name:        "relative source",
			source:      "Media",
			destination: testDestination,
			want:        "is not an absolute path",
		},
		{
			name:        "relative destination",
			source:      testSource,
			destination: "../Media",
			want:        "is not an absolute path",
		},
		{
			name:        "identical paths",
			source:      testSource,
			destination: testSource,
			want:        "are the same path",
		},
		{
			name:        "identical after cleaning",
			source:      testSource,
			destination: "/Volumes/SanDisk/Media/",
			want:        "are the same path",
		},
		{
			name:        "destination inside source",
			source:      testSource,
			destination: "/Volumes/SanDisk/Media/copy",
			want:        "is inside the source",
		},
		{
			name:        "source inside destination",
			source:      "/Volumes/Elements/Media/inner",
			destination: testDestination,
			want:        "is inside the destination",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, _ := mountedSetup()
			cfg.Source = tc.source
			cfg.Destination = tc.destination

			_, err := Validate(cfg, deps)
			requireErrorContains(t, err, tc.want)
		})
	}
}

// A directory whose name merely starts with the other endpoint's name is not
// nested inside it.
func TestValidateAcceptsSiblingPrefixNames(t *testing.T) {
	deps := Deps{
		FS: fakeFS{files: map[string]fakeFile{
			"/mnt":        {dir: true},
			"/mnt/Media":  {dir: true, children: []string{"a"}},
			"/mnt/Media2": {dir: true},
		}},
		Devices: fakeDevices{devices: map[string]string{
			"/":           "system",
			"/mnt/Media":  "one",
			"/mnt/Media2": "two",
		}},
	}

	if _, err := Validate(Config{Source: "/mnt/Media", Destination: "/mnt/Media2"}, deps); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnusableSource(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(files map[string]fakeFile)
		want   string
	}{
		{
			name:   "missing source",
			mutate: func(files map[string]fakeFile) { delete(files, testSource) },
			want:   "does not exist",
		},
		{
			name:   "source is a file",
			mutate: func(files map[string]fakeFile) { files[testSource] = fakeFile{} },
			want:   "is not a directory",
		},
		{
			name:   "empty source",
			mutate: func(files map[string]fakeFile) { files[testSource] = fakeFile{dir: true} },
			want:   "is empty",
		},
		{
			name:   "unreadable source",
			mutate: func(files map[string]fakeFile) { files[testSource] = fakeFile{dir: true, readErr: fs.ErrPermission} },
			want:   "cannot read the mirror source",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, _ := mountedSetup()
			tc.mutate(deps.FS.(fakeFS).files)

			_, err := Validate(cfg, deps)
			requireErrorContains(t, err, tc.want)
		})
	}
}

// rsync excludes the partial directory from the transfer, so a source holding
// only it reads as non-empty while the mirror it describes is an empty one that
// would delete the whole destination.
func TestValidateRejectsAPartialDirectoryOnlySource(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	deps.FS.(fakeFS).files[testSource] = fakeFile{dir: true, children: []string{PartialDir}}

	_, err := Validate(cfg, deps)
	requireErrorContains(t, err, "is empty of everything the mirror would transfer")
	requireErrorContains(t, err, PartialDir)
}

// The partial directory is what makes an interrupted transfer resumable, so its
// presence beside ordinary content is normal and must stay valid.
func TestValidateAcceptsAPartialDirectoryBesideSourceContent(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	deps.FS.(fakeFS).files[testSource] = fakeFile{dir: true, children: []string{PartialDir, "a file.txt"}}

	if _, err := Validate(cfg, deps); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnusableDestination(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(files map[string]fakeFile)
		want   string
	}{
		{
			name:   "destination is absent",
			mutate: func(files map[string]fakeFile) { delete(files, testDestination) },
			want:   "create it yourself",
		},
		{
			name:   "mount point is absent",
			mutate: func(files map[string]fakeFile) { delete(files, "/Volumes/Elements") },
			want:   "cannot resolve the mount point",
		},
		{
			name:   "destination is a file",
			mutate: func(files map[string]fakeFile) { files[testDestination] = fakeFile{} },
			want:   "exists but is not a directory",
		},
		{
			name:   "read-only destination",
			mutate: func(files map[string]fakeFile) { files[testDestination] = fakeFile{dir: true, notWritable: true} },
			want:   testDestination + " is not writable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, _ := mountedSetup()
			tc.mutate(deps.FS.(fakeFS).files)

			_, err := Validate(cfg, deps)
			requireErrorContains(t, err, tc.want)
		})
	}
}

func TestValidateRejectsSharedFilesystem(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	deps.Devices.(fakeDevices).devices["/Volumes/Elements"] = "sandisk"

	_, err := Validate(cfg, deps)
	requireErrorContains(t, err, "on the same filesystem")
}

// An unplugged drive leaves an empty directory behind under /Volumes that is
// really on the system disk. Mirroring onto it would fill the internal disk;
// mirroring from it would plan the deletion of the whole destination.
func TestValidateRejectsStaleVolumeDirectories(t *testing.T) {
	cases := []struct {
		name   string
		volume string
	}{
		{name: "stale source volume", volume: "/Volumes/SanDisk"},
		{name: "stale destination volume", volume: "/Volumes/Elements"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, _ := mountedSetup()
			deps.Devices.(fakeDevices).devices[tc.volume] = "system"

			_, err := Validate(cfg, deps)
			requireErrorContains(t, err, "is not a mounted volume")
			requireErrorContains(t, err, tc.volume)
		})
	}
}

// Stat, Device, and rsync all follow symlinks, so an endpoint that resolves
// onto the system disk passes every device check while rsync would write, and
// delete, outside the configured volume.
func TestValidateRejectsSymlinkedEndpointsEscapingTheVolume(t *testing.T) {
	const escape = "/Users/sglavoie/escape"

	cases := []struct {
		name   string
		mutate func(deps Deps)
	}{
		{
			name: "source leaf resolves onto the system disk",
			mutate: func(deps Deps) {
				deps.FS.(fakeFS).links[testSource] = escape
				deps.Devices.(fakeDevices).devices[testSource] = "system"
			},
		},
		{
			name: "destination leaf resolves onto the system disk",
			mutate: func(deps Deps) {
				deps.FS.(fakeFS).links[testDestination] = escape
				deps.Devices.(fakeDevices).devices[testDestination] = "system"
			},
		},
		{
			name: "destination parent resolves onto the system disk",
			mutate: func(deps Deps) {
				deps.FS.(fakeFS).links["/Volumes/Elements"] = escape
			},
		},
		{
			name: "source parent resolves onto the system disk",
			mutate: func(deps Deps) {
				deps.FS.(fakeFS).links["/Volumes/SanDisk"] = escape
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, _ := mountedSetup()
			tc.mutate(deps)

			_, err := Validate(cfg, deps)
			requireErrorContains(t, err, "outside the drive it was configured for")
		})
	}
}

// A symlink that stays inside the validated volume is not an escape: it points
// at content the mirror is allowed to write and delete anyway.
func TestValidateAcceptsSymlinksInsideTheVolume(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	deps.FS.(fakeFS).links[testDestination] = "/Volumes/Elements/Other"

	if _, err := Validate(cfg, deps); err != nil {
		t.Fatal(err)
	}
}
