package mirror

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

// A drive can be unplugged and another one mounted under the same name while
// the prompt waits, which no path check would notice.
func TestMirrorRefusesToWriteWhenADriveWasReplacedDuringTheApproval(t *testing.T) {
	cases := []struct {
		name    string
		replace func(devices map[string]string)
		want    string
	}{
		{
			name:    "destination replaced",
			replace: func(devices map[string]string) { devices["/Volumes/Elements"] = "another-drive" },
			want:    testDestination + " is not on the filesystem it was on",
		},
		{
			name:    "source replaced",
			replace: func(devices map[string]string) { devices["/Volumes/SanDisk"] = "another-drive" },
			want:    testSource + " is not on the filesystem it was on",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setup := mirrorSetup()
			devices := setup.deps.Devices.(fakeDevices).devices
			setup.approver.whileDeciding = func() { tc.replace(devices) }

			result, err := setup.run(context.Background())
			requireErrorContains(t, err, tc.want)
			if result.Status != StatusSkipped {
				t.Fatalf("status = %s, want skipped", result.Status)
			}
			if got := setup.journal.steps; !slices.Equal(got, []string{"approve"}) {
				t.Fatalf("steps = %v, want nothing created and nothing run", got)
			}
		})
	}
}

// A destination that gained content while the prompt waited would have that
// content deleted without anyone ever seeing it listed.
func TestMirrorRefusesDeletionsThatWereNeverReviewed(t *testing.T) {
	setup := mirrorSetup()
	setup.runner.dryRuns = []CommandResult{
		{Stdout: mountedRsyncOutput},
		{Stdout: mountedRsyncOutput + "*deleting   brand new.mov\n"},
	}

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "brand new.mov")
	requireErrorContains(t, err, "run the mirror again")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(setup.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
	}
}

// mutatingRunner changes the destination while the revalidation scan is
// running. That window belongs to no plan: stillApproved only compares the two
// plans it was given, and the transfer is a third rsync process that scans the
// destination again on its own.
type mutatingRunner struct {
	inner   *fakeRunner
	mutate  func()
	dryRuns int
}

func (r *mutatingRunner) Run(ctx context.Context, argv []string) (CommandResult, error) {
	result, err := r.inner.Run(ctx, argv)
	if len(argv) > 1 && argv[1] != "--version" {
		r.dryRuns++
		if r.dryRuns == 2 {
			r.mutate()
		}
	}
	return result, err
}

// addEntry adds a file to a directory of the fake filesystem the way another
// process writing into the destination does.
func addEntry(fsys fakeFS, dir, name string) {
	parent := fsys.files[dir]
	parent.children = append(parent.children, name)
	fsys.files[dir] = parent
	fsys.files[filepath.Join(dir, name)] = fakeFile{}
}

func removeEntry(fsys fakeFS, dir, name string) {
	parent := fsys.files[dir]
	parent.children = slices.DeleteFunc(parent.children, func(child string) bool { return child == name })
	fsys.files[dir] = parent
	delete(fsys.files, filepath.Join(dir, name))
}

// The reviewed plans say nothing about content that appears once the
// revalidation has read the destination, and the real rsync would find it and
// delete it as destination-only content nobody ever saw.
func TestMirrorRefusesDestinationContentThatAppearedAfterRevalidation(t *testing.T) {
	cases := []struct {
		name    string
		dir     string
		entry   string
		wanting string
	}{
		{name: "at the top level", dir: testDestination, entry: "brand new.mov", wanting: "brand new.mov"},
		{name: "inside a subdirectory", dir: testDestination + "/sub", entry: "brand new.mov", wanting: "sub/brand new.mov"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setup := mirrorSetup()
			fsys := setup.deps.FS.(fakeFS)
			addEntry(fsys, testDestination, "sub")
			fsys.files[testDestination+"/sub"] = fakeFile{dir: true}
			setup.deps.Runner = &mutatingRunner{inner: setup.runner, mutate: func() { addEntry(fsys, tc.dir, tc.entry) }}

			result, err := setup.run(context.Background())
			requireErrorContains(t, err, tc.wanting)
			requireErrorContains(t, err, "after the plan was revalidated")
			requireErrorContains(t, err, "run the mirror again")
			if result.Status != StatusSkipped {
				t.Fatalf("status = %s, want skipped", result.Status)
			}
			if len(setup.streamer.calls) != 0 {
				t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
			}
			if _, ok := fsys.files[filepath.Join(tc.dir, tc.entry)]; !ok {
				t.Fatal("the entry that appeared is gone, want it preserved")
			}
		})
	}
}

// A destination that lost content since the revalidation has nothing
// unreviewed in it: there is simply less to delete than the plan listed.
func TestMirrorAllowsDestinationContentThatVanishedAfterRevalidation(t *testing.T) {
	setup := mirrorSetup()
	fsys := setup.deps.FS.(fakeFS)
	addEntry(fsys, testDestination, "gone.txt")
	setup.deps.Runner = &mutatingRunner{inner: setup.runner, mutate: func() { removeEntry(fsys, testDestination, "gone.txt") }}

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}
}

// rsync writes its own partial data into the destination and never deletes it,
// so a resumable transfer left behind by an earlier run is not unreviewed
// content.
func TestMirrorIgnoresThePartialDirectoryAfterRevalidation(t *testing.T) {
	setup := mirrorSetup()
	fsys := setup.deps.FS.(fakeFS)
	setup.deps.Runner = &mutatingRunner{inner: setup.runner, mutate: func() {
		addEntry(fsys, testDestination, PartialDir)
	}}

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}
}

// escapeTarget is a directory on the system disk: where an endpoint replaced by
// a symlink would send the transfer.
const escapeTarget = "/Users/sglavoie/escape"

// Validation resolves the endpoints at one instant, while rsync acts later on
// paths of its own and follows symlinks. An endpoint replaced in between must
// never reach a write.
func TestMirrorRefusesAnEndpointSwappedForASymlinkAfterRevalidation(t *testing.T) {
	cases := []struct {
		name    string
		swapped string
	}{
		{name: "source leaf", swapped: testSource},
		{name: "destination leaf", swapped: testDestination},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setup := mirrorSetup()
			fsys := setup.deps.FS.(fakeFS)
			setup.deps.Runner = &mutatingRunner{inner: setup.runner, mutate: func() {
				fsys.links[tc.swapped] = escapeTarget
			}}

			result, err := setup.run(context.Background())
			requireErrorContains(t, err, escapeTarget)
			requireErrorContains(t, err, "nothing was written")
			if result.Status != StatusSkipped {
				t.Fatalf("status = %s, want skipped", result.Status)
			}
			if got := setup.journal.steps; !slices.Equal(got, []string{"approve"}) {
				t.Fatalf("steps = %v, want nothing run", got)
			}
			if len(setup.streamer.calls) != 0 {
				t.Fatalf("ran %v, want no write-capable effect", setup.streamer.calls)
			}
		})
	}
}

// rsync is given the resolved endpoints rather than the configured ones, so it
// acts on the directories validation accepted rather than on paths that can be
// made to mean something else afterwards.
func TestMirrorRunsOnResolvedEndpointsThatNoSymlinkSwapCanRedirectAfterRevalidation(t *testing.T) {
	setup := mirrorSetup()
	fsys := setup.deps.FS.(fakeFS)
	fsys.links[testSource] = "/Volumes/SanDisk/Real"
	fsys.links[testDestination] = "/Volumes/Elements/Other"

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}

	argv := result.Argv
	if argv[len(argv)-2] != "/Volumes/SanDisk/Real/" || argv[len(argv)-1] != "/Volumes/Elements/Other" {
		t.Fatalf("endpoints = %v, want the resolved directories rather than the configured paths", argv[len(argv)-2:])
	}
}

// The transfer is a separate rsync that scans the destination after every walk
// the mirror performed, so it is given the boundary of what the mirror saw:
// content that appears in that last window is outside the boundary and survives
// instead of being deleted unreviewed.
func TestMirrorRestrictsDeletionsToTheDestinationItWalkedAtTransferStart(t *testing.T) {
	setup := mirrorSetup()
	fsys := setup.deps.FS.(fakeFS)
	addEntry(fsys, testDestination, "gone.txt")
	addEntry(fsys, testDestination, "sub")
	fsys.files[testDestination+"/sub"] = fakeFile{dir: true}
	addEntry(fsys, testDestination+"/sub", "nested*star.txt")
	addEntry(fsys, testDestination, PartialDir)
	fsys.files[testDestination+"/"+PartialDir] = fakeFile{dir: true}
	setup.streamer.beforeStreaming = func() { addEntry(fsys, testDestination, "appeared.mov") }

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}

	// rsync owns the partial directory and never deletes it, so it is not
	// part of the boundary either.
	want := "R /gone.txt\x00R /sub\x00" + `R /sub/nested\*star.txt` + "\x00P /**\x00"
	if got := string(setup.rules.rules); got != want {
		t.Fatalf("deletion rules = %q, want %q", got, want)
	}
	if setup.rules.written != 1 || setup.rules.removed != 1 {
		t.Fatalf("the rules were written %d times and removed %d times, want exactly one of each", setup.rules.written, setup.rules.removed)
	}

	argv := setup.streamer.calls[0]
	for _, flag := range []string{"--from0", "--filter=merge " + setup.rules.path} {
		if !slices.Contains(argv, flag) {
			t.Fatalf("argv = %v, want it to carry %q", argv, flag)
		}
	}
}

// Deletions the user already reviewed that are no longer needed are not a
// reason to refuse: the destination only got closer to the source.
func TestMirrorAcceptsAShrunkenDeletionList(t *testing.T) {
	setup := mirrorSetup()
	setup.runner.dryRuns = []CommandResult{
		{Stdout: mountedRsyncOutput},
		{Stdout: ">f.s....... a file.txt\n*deleting   gone.txt\n\nTotal transferred file size: 2,048 bytes\n"},
	}

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}
	if result.Plan.Changes.Deleted != 1 {
		t.Fatalf("the executed plan reported %d deletions, want the revalidated one", result.Plan.Changes.Deleted)
	}
}

// A destination that became synchronized while the prompt waited has nothing
// left to do, and running rsync anyway would only be a way to be surprised.
func TestMirrorStopsWhenRevalidationFindsNothingLeftToDo(t *testing.T) {
	setup := mirrorSetup()
	setup.runner.dryRuns = []CommandResult{
		{Stdout: mountedRsyncOutput},
		{Stdout: upToDateOutput},
	}

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusUpToDate {
		t.Fatalf("status = %s, want up to date", result.Status)
	}
	if len(setup.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
	}
}

func TestMirrorStopsWhenTheSourceDisappearsDuringTheApproval(t *testing.T) {
	setup := mirrorSetup()
	files := setup.deps.FS.(fakeFS).files
	setup.approver.whileDeciding = func() { delete(files, testSource) }

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "no longer in the state that was approved")
	requireErrorContains(t, err, "does not exist")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(setup.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
	}
}
