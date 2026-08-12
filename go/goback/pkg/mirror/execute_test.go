package mirror

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// upToDateOutput is what rsync itemizes when the destination already matches
// the source.
const upToDateOutput = "\nTotal transferred file size: 0 bytes\n"

// A real mirror is approved on the strength of the preflight, so it must show
// the same one a dry run shows, deletions included.
func TestMirrorShowsTheSamePreflightBeforeAsking(t *testing.T) {
	setup := mirrorSetup()

	if _, err := setup.run(context.Background()); err != nil {
		t.Fatal(err)
	}

	rendered := setup.out.String()
	for _, want := range []string{"dry run, nothing was written", testSource, testDestination, "deleted:  3", "dir gone/c.txt", "gone.txt"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("printed preflight = %q, want it to contain %q", rendered, want)
		}
	}
	if len(setup.approver.plans) != 1 {
		t.Fatalf("the approver was asked %d times, want exactly once", len(setup.approver.plans))
	}
	if setup.approver.plans[0].Changes.Deleted != 3 {
		t.Fatalf("the approved plan reported %d deletions, want the 3 that were printed", setup.approver.plans[0].Changes.Deleted)
	}
}

func TestMirrorDoesNothingWhenTheTransferIsDeclined(t *testing.T) {
	setup := mirrorSetup()
	setup.approver.approve = false

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatalf("declining returned %v, want a successful exit", err)
	}
	if result.Status != StatusDeclined {
		t.Fatalf("status = %s, want declined", result.Status)
	}
	if result.Attempted() {
		t.Fatal("a declined mirror reports an attempt, want none")
	}
	if got := setup.journal.steps; !slices.Equal(got, []string{"approve"}) {
		t.Fatalf("steps = %v, want the prompt and nothing else", got)
	}
}

// Asking whether to delete nothing and transfer nothing would train the user to
// answer yes.
func TestMirrorSkipsTheApprovalWhenTheDestinationIsUpToDate(t *testing.T) {
	setup := mirrorSetup()
	setup.runner.dryRun = CommandResult{Stdout: upToDateOutput}

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusUpToDate {
		t.Fatalf("status = %s, want up to date", result.Status)
	}
	if len(setup.journal.steps) != 0 {
		t.Fatalf("steps = %v, want nothing asked and nothing run", setup.journal.steps)
	}
}

// The mirror never creates its destination, so an absent one is refused before
// anything is asked, locked, or run.
func TestMirrorRefusesAnAbsentDestination(t *testing.T) {
	setup := mirrorSetup()
	delete(setup.deps.FS.(fakeFS).files, testDestination)

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "create it yourself")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(setup.journal.steps) != 0 {
		t.Fatalf("steps = %v, want nothing asked and nothing run", setup.journal.steps)
	}
}

func TestMirrorRunsTheRealCommandOnce(t *testing.T) {
	setup := mirrorSetup()

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(setup.streamer.calls) != 1 {
		t.Fatalf("rsync ran %d times, want exactly once", len(setup.streamer.calls))
	}

	argv := setup.streamer.calls[0]
	if !slices.Equal(argv, result.Argv) {
		t.Fatalf("executed %v but reported %v", argv, result.Argv)
	}
	if slices.Contains(argv, "--dry-run") {
		t.Fatalf("argv = %v, want no --dry-run in a real mirror", argv)
	}
	for _, want := range []string{"--archive", "--hard-links", "--acls", "--xattrs", "--crtimes", "--delete", "--delete-delay", "--partial-dir=" + PartialDir, "--info=progress2", "--human-readable", "--stats"} {
		if !slices.Contains(argv, want) {
			t.Fatalf("argv = %v, want it to contain %q", argv, want)
		}
	}
	if argv[len(argv)-2] != testSource+"/" || argv[len(argv)-1] != testDestination {
		t.Fatalf("endpoints = %v, want the source contents copied into the destination", argv[len(argv)-2:])
	}
	if result.Status != StatusSucceeded || result.ExitCode != 0 {
		t.Fatalf("result = %s / %d, want succeeded with exit code 0", result.Status, result.ExitCode)
	}
	if !result.Attempted() || result.Duration <= 0 {
		t.Fatalf("result = %#v, want a measured attempt", result)
	}
}

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

// rsync opens its arguments itself, after the last thing the mirror can check,
// so both endpoints are bound to the directories they are before the transfer
// is built, and the transfer is given those bindings rather than paths.
func TestMirrorBindsBothEndpointsBeforeTheTransfer(t *testing.T) {
	setup := mirrorSetup()
	fsys := setup.deps.FS.(fakeFS)
	fsys.links[testSource] = "/Volumes/SanDisk/Real"
	setup.binder.name = func(path string) string { return "/bound" + path }

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}

	want := []string{"/Volumes/SanDisk/Real", testDestination}
	if !slices.Equal(setup.binder.bound, want) {
		t.Fatalf("bound %v, want the resolved endpoints %v", setup.binder.bound, want)
	}

	argv := result.Argv
	if argv[len(argv)-2] != "/bound/Volumes/SanDisk/Real/" || argv[len(argv)-1] != "/bound"+testDestination {
		t.Fatalf("endpoints = %v, want the bound directories rather than their paths", argv[len(argv)-2:])
	}
	if setup.binder.released != 2 {
		t.Fatalf("released %d endpoints, want both given back once the mirror is over", setup.binder.released)
	}
}

// An endpoint that cannot be bound is an endpoint nothing can prove anything
// about, so the transfer never starts.
func TestMirrorStopsWhenAnEndpointCannotBeBound(t *testing.T) {
	setup := mirrorSetup()
	setup.binder.err = errFake

	result, err := setup.run(context.Background())
	if !errors.Is(err, errFake) {
		t.Fatalf("error = %v, want the binding failure", err)
	}
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(setup.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
	}
	if setup.rules.written != 0 {
		t.Fatal("the deletion rules were written for a transfer that never started")
	}
}

// Two mirrors of one destination would each delete what the other just wrote,
// and neither user would be shown the other's changes.
func TestMirrorRefusesAConcurrentMirrorOfTheSameDestination(t *testing.T) {
	locks := t.TempDir()
	first := mirrorSetup()
	first.exec.Locker = osLocker{dir: locks}
	second := mirrorSetup()
	second.exec.Locker = osLocker{dir: locks}

	holding, answer := make(chan struct{}), make(chan struct{})
	first.approver.whileDeciding = func() {
		close(holding)
		select {
		case <-answer:
		case <-time.After(10 * time.Second):
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := first.run(context.Background())
		done <- err
	}()
	<-holding

	result, err := second.run(context.Background())
	requireErrorContains(t, err, "another goback mirror is already running")
	requireErrorContains(t, err, testDestination)
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(second.approver.plans) != 0 {
		t.Fatal("the second mirror asked for an approval, want it refused before anyone is asked to decide")
	}
	if len(second.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", second.streamer.calls)
	}

	close(answer)
	if err := <-done; err != nil {
		t.Fatalf("the first mirror = %v, want it unaffected", err)
	}
	if len(first.streamer.calls) != 1 {
		t.Fatalf("the first mirror ran rsync %d times, want exactly once", len(first.streamer.calls))
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

// The lock is per destination, so an unrelated mirror is not made to wait for
// an approval prompt it has nothing to do with.
func TestMirrorAllowsConcurrentMirrorsOfDifferentDestinations(t *testing.T) {
	locks := t.TempDir()
	first := mirrorSetup()
	first.exec.Locker = osLocker{dir: locks}
	second := mirrorSetup()
	second.exec.Locker = osLocker{dir: locks}
	second.cfg.Destination = "/Volumes/Elements/Other"

	holding, answer := make(chan struct{}), make(chan struct{})
	first.approver.whileDeciding = func() {
		close(holding)
		select {
		case <-answer:
		case <-time.After(10 * time.Second):
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := first.run(context.Background())
		done <- err
	}()
	<-holding

	result, err := second.run(context.Background())
	if err != nil {
		t.Fatalf("the second mirror = %v, want another destination to be free to run", err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}

	close(answer)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// The lock is given back whatever the mirror did, so the next one is not
// refused by a lock nobody holds.
func TestMirrorReleasesTheDestinationLock(t *testing.T) {
	setup := mirrorSetup()
	setup.approver.approve = false

	if _, err := setup.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(setup.locker.locked, []string{testDestination}) {
		t.Fatalf("locked %v, want the destination locked once", setup.locker.locked)
	}
	if setup.locker.released != 1 {
		t.Fatalf("the lock was given back %d times, want exactly once", setup.locker.released)
	}
}

// destinationAlias is an in-volume symlink to the same directory
// testDestination names: one directory reached through two configured paths.
const destinationAlias = "/Volumes/Elements/Alias"

func aliasSetup() *execSetup {
	setup := mirrorSetup()
	fsys := setup.deps.FS.(fakeFS)
	addEntry(fsys, "/Volumes/Elements", filepath.Base(destinationAlias))
	fsys.files[destinationAlias] = fakeFile{dir: true}
	fsys.links[destinationAlias] = testDestination
	setup.cfg.Destination = destinationAlias
	return setup
}

// The lock names the destination, so it has to name the directory rather than
// the path it was configured under: two aliases of one directory that hashed
// differently would take two locks and mirror the same content at once.
func TestMirrorLocksTheCanonicalDestination(t *testing.T) {
	setup := aliasSetup()

	result, err := setup.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}
	if !slices.Equal(setup.locker.locked, []string{testDestination}) {
		t.Fatalf("locked %v, want the resolved destination %q", setup.locker.locked, testDestination)
	}
}

// Two mirrors started through different aliases of one directory would each
// delete what the other just wrote, exactly as two mirrors of the same
// configured path would.
func TestMirrorRefusesAConcurrentMirrorOfAResolvedAliasOfTheDestination(t *testing.T) {
	locks := t.TempDir()
	first := mirrorSetup()
	first.exec.Locker = osLocker{dir: locks}
	second := aliasSetup()
	second.exec.Locker = osLocker{dir: locks}

	holding, answer := make(chan struct{}), make(chan struct{})
	first.approver.whileDeciding = func() {
		close(holding)
		select {
		case <-answer:
		case <-time.After(10 * time.Second):
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := first.run(context.Background())
		done <- err
	}()
	<-holding

	result, err := second.run(context.Background())
	requireErrorContains(t, err, "another goback mirror is already running")
	requireErrorContains(t, err, testDestination)
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(second.approver.plans) != 0 {
		t.Fatal("the aliased mirror asked for an approval, want it refused before anyone is asked to decide")
	}
	if len(second.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", second.streamer.calls)
	}

	close(answer)
	if err := <-done; err != nil {
		t.Fatalf("the first mirror = %v, want it unaffected", err)
	}
}

// The lock is taken on the destination the approved plan resolved to, so an
// alias repointed at another directory while the prompt is on screen would make
// the transfer write somewhere the mirror never locked.
func TestMirrorRefusesAnAliasRetargetedDuringApproval(t *testing.T) {
	setup := aliasSetup()
	fsys := setup.deps.FS.(fakeFS)
	setup.approver.whileDeciding = func() { fsys.links[destinationAlias] = "/Volumes/Elements/Other" }

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "when the mirror took its lock")
	requireErrorContains(t, err, testDestination)
	requireErrorContains(t, err, "/Volumes/Elements/Other")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if !slices.Equal(setup.locker.locked, []string{testDestination}) {
		t.Fatalf("locked %v, want the destination the approved plan resolved to", setup.locker.locked)
	}
	if setup.rules.written != 0 || len(setup.streamer.calls) != 0 {
		t.Fatalf("wrote %d rule files, ran %v, want the refusal before any of them", setup.rules.written, setup.streamer.calls)
	}
}

// Two mirrors of one directory reached through different names must exclude
// each other. An alias retargeted during the approval takes the lock of the
// directory it named then and would transfer into the one it names now, which is
// a directory another mirror holds the lock of.
func TestMirrorRefusesARetargetedAliasHoldingAnotherDirectorysLock(t *testing.T) {
	locks := t.TempDir()

	first := mirrorSetup()
	first.exec.Locker = osLocker{dir: locks}

	// The alias names an unrelated directory when the second mirror starts, so
	// it legitimately takes a different lock than the first mirror's.
	second := aliasSetup()
	second.exec.Locker = osLocker{dir: locks}
	secondFS := second.deps.FS.(fakeFS)
	secondFS.links[destinationAlias] = "/Volumes/Elements/Other"

	holding, answer := make(chan struct{}), make(chan struct{})
	first.approver.whileDeciding = func() {
		close(holding)
		select {
		case <-answer:
		case <-time.After(10 * time.Second):
		}
	}
	second.approver.whileDeciding = func() { secondFS.links[destinationAlias] = testDestination }

	done := make(chan error, 1)
	go func() {
		_, err := first.run(context.Background())
		done <- err
	}()
	<-holding

	result, err := second.run(context.Background())
	requireErrorContains(t, err, "when the mirror took its lock")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(second.approver.plans) != 1 {
		t.Fatalf("the second mirror was asked %d times, want it to have taken its own lock and asked once", len(second.approver.plans))
	}
	if len(second.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v for a destination locked by another mirror, want nothing run", second.streamer.calls)
	}
	if second.rules.written != 0 {
		t.Fatal("the deletion rules were written for a transfer that never started")
	}

	close(answer)
	if err := <-done; err != nil {
		t.Fatalf("the first mirror = %v, want it unaffected", err)
	}
	if len(first.streamer.calls) != 1 {
		t.Fatalf("the first mirror ran rsync %d times, want it to be the only transfer into %s", len(first.streamer.calls), testDestination)
	}
}

// otherDestination is an unrelated directory on the destination volume: the one
// a substitution puts where the configured destination was.
const otherDestination = "/Volumes/Elements/Other"

// The lock names a path while the transfer acts on a directory, so a path that
// came to hold another directory during the approval would carry the transfer
// into a directory the lock was never taken for. No device and no resolution
// changes when one ordinary directory replaces another.
func TestMirrorRefusesADestinationDirectoryReplacedDuringApproval(t *testing.T) {
	setup := mirrorSetup()
	identities := setup.deps.Devices.(fakeDevices).identities
	setup.approver.whileDeciding = func() { identities[testDestination] = "directory of " + otherDestination }

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "no longer the directory it was when the mirror took its lock")
	requireErrorContains(t, err, testDestination)
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if setup.rules.written != 0 || len(setup.streamer.calls) != 0 {
		t.Fatalf("wrote %d rule files, ran %v, want the refusal before any of them", setup.rules.written, setup.streamer.calls)
	}
}

// substitutedDirectoryBinder makes a path hold another directory for exactly
// the time one binding takes and gives the original back immediately after, so
// every check the mirror makes before and after the binding sees the endpoint it
// validated and only the binding itself can tell.
type substitutedDirectoryBinder struct {
	devices  fakeDevices
	endpoint string
	identity string
	inner    Binder
}

func (b *substitutedDirectoryBinder) Bind(path, identity string) (Bound, error) {
	if path != b.endpoint {
		return b.inner.Bind(path, identity)
	}

	original, had := b.devices.identities[path]
	b.devices.identities[path] = b.identity
	defer func() {
		if had {
			b.devices.identities[path] = original
			return
		}
		delete(b.devices.identities, path)
	}()

	return b.inner.Bind(path, identity)
}

// A directory that occupies the configured destination only while the binding
// runs would be transferred into under the lock of the path it borrowed, while
// the mirror that is really mirroring it holds its own lock: two transfers into
// one directory, each believing it excluded the other.
func TestMirrorRefusesABoundDestinationHeldUnderAnotherLock(t *testing.T) {
	locks := t.TempDir()

	// The mirror of the substitute directory holds its real lock for as long
	// as nobody answers its prompt.
	holder := mirrorSetup()
	holder.exec.Locker = osLocker{dir: locks}
	holder.cfg.Destination = otherDestination

	substituted := mirrorSetup()
	substituted.exec.Locker = osLocker{dir: locks}
	substituted.exec.Binder = &substitutedDirectoryBinder{
		devices:  substituted.deps.Devices.(fakeDevices),
		endpoint: testDestination,
		identity: "directory of " + otherDestination,
		inner:    substituted.binder,
	}

	holding, answer := make(chan struct{}), make(chan struct{})
	holder.approver.whileDeciding = func() {
		close(holding)
		select {
		case <-answer:
		case <-time.After(10 * time.Second):
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := holder.run(context.Background())
		done <- err
	}()
	<-holding

	result, err := substituted.run(context.Background())
	requireErrorContains(t, err, "the one the preflight inspected")
	requireErrorContains(t, err, testDestination)
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(substituted.approver.plans) != 1 {
		t.Fatalf("the substituted mirror was asked %d times, want it to have taken its own lock and asked once", len(substituted.approver.plans))
	}
	if len(substituted.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v for a directory locked by another mirror, want nothing run", substituted.streamer.calls)
	}
	if substituted.rules.written != 0 {
		t.Fatal("the deletion rules were written for a transfer that never started")
	}

	close(answer)
	if err := <-done; err != nil {
		t.Fatalf("the mirror of %s = %v, want it unaffected", otherDestination, err)
	}
	if len(holder.streamer.calls) != 1 {
		t.Fatalf("the mirror of %s ran rsync %d times, want it to be the only transfer into it", otherDestination, len(holder.streamer.calls))
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

// The preflight is what produces a deletion plan, so a failing one must stop
// everything before a prompt is even shown.
func TestMirrorSkipsEverythingWhenThePreflightFails(t *testing.T) {
	setup := mirrorSetup()
	delete(setup.deps.FS.(fakeFS).files, testSource)

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "does not exist")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(setup.journal.steps) != 0 {
		t.Fatalf("steps = %v, want nothing asked and nothing run", setup.journal.steps)
	}
}

func TestMirrorReportsAFailedTransfer(t *testing.T) {
	setup := mirrorSetup()
	setup.streamer.exitCode = 23

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "exit code 23")
	if result.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
	if result.ExitCode != 23 {
		t.Fatalf("ExitCode = %d, want rsync's own 23", result.ExitCode)
	}
	if !result.Attempted() {
		t.Fatal("a failed transfer reports no attempt, want it recorded as one")
	}
	// Deletion is delayed until the transfer succeeded, so a failure never
	// removes anything from the destination.
	if !slices.Contains(result.Argv, "--delete-delay") {
		t.Fatalf("argv = %v, want deletions delayed until after a successful transfer", result.Argv)
	}
}

func TestMirrorReportsACommandThatCouldNotBeRun(t *testing.T) {
	setup := mirrorSetup()
	setup.streamer.err = errFake

	result, err := setup.run(context.Background())
	requireErrorContains(t, err, "could not be run")
	if result.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
}

func TestMirrorReportsAnInterruptedTransfer(t *testing.T) {
	setup := mirrorSetup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setup.streamer.cancel = cancel

	result, err := setup.run(ctx)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("error = %v, want an interruption", err)
	}
	if result.Status != StatusInterrupted {
		t.Fatalf("status = %s, want interrupted", result.Status)
	}
	if result.ExitCode != InterruptedExitCode {
		t.Fatalf("ExitCode = %d, want %d", result.ExitCode, InterruptedExitCode)
	}
	if !result.Attempted() {
		t.Fatal("an interrupted transfer reports no attempt, want it recorded as one")
	}
}

// A signal that arrives while the prompt is on screen must not be answered by
// starting the transfer it was meant to stop.
func TestMirrorInterruptedAtThePromptNeverStarts(t *testing.T) {
	setup := mirrorSetup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setup.approver.whileDeciding = cancel

	result, err := setup.run(ctx)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("error = %v, want an interruption", err)
	}
	if result.Status != StatusInterrupted {
		t.Fatalf("status = %s, want interrupted", result.Status)
	}
	if result.Attempted() {
		t.Fatal("a mirror interrupted before rsync started reports an attempt, want none")
	}
	if len(setup.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
	}
}

// cancellingRunner interrupts the mirror while it is revalidating what was
// approved, which is where a large source spends real time.
type cancellingRunner struct {
	inner   *fakeRunner
	cancel  context.CancelFunc
	dryRuns int
}

func (r *cancellingRunner) Run(ctx context.Context, argv []string) (CommandResult, error) {
	if len(argv) > 1 && argv[1] != "--version" {
		r.dryRuns++
		if r.dryRuns == 2 {
			r.cancel()
		}
	}
	return r.inner.Run(ctx, argv)
}

func TestMirrorInterruptedDuringRevalidationNeverStarts(t *testing.T) {
	setup := mirrorSetup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setup.deps.Runner = &cancellingRunner{inner: setup.runner, cancel: cancel}

	result, err := setup.run(ctx)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("error = %v, want an interruption", err)
	}
	if result.Status != StatusInterrupted {
		t.Fatalf("status = %s, want interrupted", result.Status)
	}
	if len(setup.streamer.calls) != 0 {
		t.Fatalf("rsync ran %v, want nothing run", setup.streamer.calls)
	}
}

// Ejecting a drive is a separate, explicit operation: a mirror never unmounts
// anything, however it ended.
func TestMirrorNeverEjectsAnything(t *testing.T) {
	setup := mirrorSetup()

	if _, err := setup.run(context.Background()); err != nil {
		t.Fatal(err)
	}

	executed := append(slices.Clone(setup.runner.calls), setup.streamer.calls...)
	for _, argv := range executed {
		if argv[0] != "rsync" {
			t.Fatalf("the mirror executed %v, want nothing but rsync", argv)
		}
		for _, arg := range argv {
			for _, forbidden := range []string{"diskutil", "eject", "umount", "unmount"} {
				if strings.Contains(arg, forbidden) {
					t.Fatalf("the mirror executed %v, want no %q in it", argv, forbidden)
				}
			}
		}
	}
}

func TestResultSummaryDescribesEveryOutcome(t *testing.T) {
	cases := []struct {
		name   string
		result Result
		want   string
	}{
		{name: "skipped", result: Result{Status: StatusSkipped}, want: "did not run"},
		{name: "up to date", result: Result{Status: StatusUpToDate}, want: "already matches"},
		{name: "declined", result: Result{Status: StatusDeclined}, want: "Cancelled"},
		{name: "interrupted before starting", result: Result{Status: StatusInterrupted}, want: "nothing was created"},
		{name: "interrupted mid transfer", result: Result{Status: StatusInterrupted, Argv: []string{"rsync"}}, want: PartialDir},
		{name: "failed", result: Result{Status: StatusFailed, Argv: []string{"rsync"}}, want: "only partially updated"},
		{name: "succeeded", result: Result{Status: StatusSucceeded, Argv: []string{"rsync"}}, want: "finished in"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.result.Summary()
			if !strings.Contains(got, tc.want) {
				t.Fatalf("Summary() = %q, want it to mention %q", got, tc.want)
			}
		})
	}
}
