package mirror

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// realExecDeps runs the real rsync and creates real directories, with only the
// approval faked so a test can answer the prompt.
func realExecDeps(approve bool) (ExecDeps, *fakeApprover, *bytes.Buffer) {
	approver := &fakeApprover{journal: &journal{}, approve: approve}
	var out bytes.Buffer
	return OSExecDeps(approver, &out, &out), approver, &out
}

func TestMirrorAgainstRealRsync(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	writeFile(t, filepath.Join(source, "sub dir", "nested.txt"), "y")
	makeDir(t, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	// An initial copy into an empty destination the user made themselves.
	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}
	assertFileContent(t, filepath.Join(destination, "a file.txt"), "hello")
	assertFileContent(t, filepath.Join(destination, "sub dir", "nested.txt"), "y")
	assertAbsent(t, filepath.Join(destination, PartialDir))

	// An update, a creation, and a destination-only entry that an exact
	// mirror must remove.
	writeFile(t, filepath.Join(source, "a file.txt"), "hello, a longer world")
	writeFile(t, filepath.Join(source, "added.txt"), "new")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "delete me")
	writeFile(t, filepath.Join(destination, "gone dir", "obsolete.txt"), "delete me too")

	result, err = Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}
	assertFileContent(t, filepath.Join(destination, "a file.txt"), "hello, a longer world")
	assertFileContent(t, filepath.Join(destination, "added.txt"), "new")
	assertAbsent(t, filepath.Join(destination, "obsolete.mov"))
	assertAbsent(t, filepath.Join(destination, "gone dir"))

	// A rerun that has nothing to do neither asks nor runs anything.
	asked := len(approver.plans)
	result, err = Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusUpToDate {
		t.Fatalf("status = %s, want up to date: %v", result.Status, result.Err)
	}
	if len(approver.plans) != asked {
		t.Fatalf("the approver was asked again for a destination that already matches the source")
	}
	if result.Attempted() {
		t.Fatal("an up-to-date destination reports a transfer, want none")
	}
}

func TestMirrorAgainstRealRsyncCreatesNothingWhenDeclined(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	makeDir(t, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(false)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusDeclined {
		t.Fatalf("status = %s, want declined", result.Status)
	}
	assertAbsent(t, filepath.Join(destination, "a file.txt"))
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the destination holds %d entries, want a declined mirror to write nothing", len(entries))
	}
}

// hookedStreamer makes the source change between the last preflight and the
// transfer, which is the only way to reach a failing real rsync: a source the
// preflight cannot read already fails the preflight.
type hookedStreamer struct {
	before func()
	inner  Streamer
}

func (s hookedStreamer) Stream(ctx context.Context, argv []string) (int, error) {
	s.before()
	return s.inner.Stream(ctx, argv)
}

// rsync refuses to delete when it could not read everything it was asked to
// send, and the mirror delays every deletion until after the transfer, so a
// failure leaves the destination stale rather than emptied.
func TestMirrorAgainstRealRsyncDeletesNothingWhenTheTransferFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every file, so an unreadable source cannot be simulated")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "readable.txt"), "fine")
	unreadable := filepath.Join(source, "unreadable.txt")
	writeFile(t, unreadable, "secret")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me until a run succeeds")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)
	exec.Streamer = hookedStreamer{
		before: func() {
			if err := os.Chmod(unreadable, 0o000); err != nil {
				t.Error(err)
			}
		},
		inner: exec.Streamer,
	}

	result, _ := Mirror(context.Background(), cfg, deps, exec, out)
	if result.Status != StatusFailed {
		t.Fatalf("status = %s, want failed: %v\n%s", result.Status, result.Err, out.String())
	}
	if result.ExitCode != 23 {
		t.Fatalf("ExitCode = %d, want rsync's own 23 for a partial transfer", result.ExitCode)
	}
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "keep me until a run succeeds")
	assertFileContent(t, filepath.Join(destination, "readable.txt"), "fine")
}

// rsync scans the destination itself, after the last walk the mirror
// performed, and the lock only excludes another goback. Content another writer
// adds in that window was never in any plan, so the real rsync must leave it
// alone while still deleting everything the plan did list.
func TestMirrorAgainstRealRsyncKeepsDestinationContentCreatedAtTransferStart(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "delete me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)
	exec.Streamer = hookedStreamer{
		before: func() { writeFile(t, filepath.Join(destination, "appeared.mov"), "nobody reviewed me") },
		inner:  exec.Streamer,
	}

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v\n%s", result.Status, result.Err, out.String())
	}
	assertFileContent(t, filepath.Join(destination, "a file.txt"), "hello")
	assertFileContent(t, filepath.Join(destination, "appeared.mov"), "nobody reviewed me")
	assertAbsent(t, filepath.Join(destination, "obsolete.mov"))
}

// swapForSymlink replaces a resolved endpoint with a symlink to somewhere else,
// the way a path can be redirected once the mirror has stopped looking at it.
// The directory itself is kept under a name of its own so a test can tell what
// reached the bound directory from what reached the replacement.
func swapForSymlink(t *testing.T, endpoint, kept, target string) {
	t.Helper()

	if err := os.Rename(endpoint, kept); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, endpoint); err != nil {
		t.Fatal(err)
	}
}

// The destination is bound to the directory it was validated as, so a symlink
// installed over it at the instant the transfer starts, which is later than
// every check the mirror can make, redirects nothing: rsync writes into and
// deletes from the bound directory, and the escape target is left alone.
func TestMirrorResolvedDestinationSwapAtTransferLaunchLeavesTheEscapeTargetUntouched(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	bound := filepath.Join(root, "bound destination")
	escape := filepath.Join(root, "escape")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "delete me")
	writeFile(t, filepath.Join(escape, "precious.mov"), "keep me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)
	exec.Streamer = hookedStreamer{
		before: func() { swapForSymlink(t, destination, bound, escape) },
		inner:  exec.Streamer,
	}

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v\n%s", result.Status, result.Err, out.String())
	}

	assertFileContent(t, filepath.Join(bound, "a file.txt"), "hello")
	assertAbsent(t, filepath.Join(bound, "obsolete.mov"))

	assertFileContent(t, filepath.Join(escape, "precious.mov"), "keep me")
	assertAbsent(t, filepath.Join(escape, "a file.txt"))
}

// The source is bound the same way, so a symlink installed over it at transfer
// launch cannot make rsync read somebody else's files into the destination.
func TestMirrorResolvedSourceSwapAtTransferLaunchTransfersTheBoundSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	bound := filepath.Join(root, "bound source")
	destination := filepath.Join(root, "destination")
	escape := filepath.Join(root, "escape")

	writeFile(t, filepath.Join(source, "mine.txt"), "from the bound source")
	writeFile(t, filepath.Join(escape, "not mine.txt"), "never asked for")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "delete me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)
	exec.Streamer = hookedStreamer{
		before: func() { swapForSymlink(t, source, bound, escape) },
		inner:  exec.Streamer,
	}

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v\n%s", result.Status, result.Err, out.String())
	}

	assertFileContent(t, filepath.Join(destination, "mine.txt"), "from the bound source")
	assertAbsent(t, filepath.Join(destination, "not mine.txt"))
	assertAbsent(t, filepath.Join(destination, "obsolete.mov"))

	assertFileContent(t, filepath.Join(escape, "not mine.txt"), "never asked for")
}

// An endpoint replaced by a symlink before the binding, rather than after it,
// is refused: the binding opens the endpoint without following it, so there is
// nothing to bind and nothing is written.
func TestMirrorRefusesAnEndpointLaunchedThroughASymlinkInstalledBeforeBinding(t *testing.T) {
	cases := []struct {
		name     string
		endpoint func(root, source, destination string) (string, string)
	}{
		{
			name: "source",
			endpoint: func(root, source, _ string) (string, string) {
				return source, filepath.Join(root, "bound source")
			},
		},
		{
			name: "destination",
			endpoint: func(root, _, destination string) (string, string) {
				return destination, filepath.Join(root, "bound destination")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			destination := filepath.Join(root, "destination")
			escape := filepath.Join(root, "escape")

			writeFile(t, filepath.Join(source, "a file.txt"), "hello")
			writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me")
			writeFile(t, filepath.Join(escape, "precious.mov"), "keep me too")

			cfg, deps := realRsyncDeps(t, source, destination)
			exec, _, out := realExecDeps(true)

			// The approval is the last moment the mirror is not looking:
			// what follows is the revalidation, whose own checks the
			// binding then has to hold up on its own.
			endpoint, kept := tc.endpoint(root, source, destination)
			exec.Binder = &hookedBinder{
				before: func() { swapForSymlink(t, endpoint, kept, escape) },
				inner:  exec.Binder,
			}

			result, err := Mirror(context.Background(), cfg, deps, exec, out)
			requireErrorContains(t, err, "nothing was written")
			if result.Status != StatusSkipped {
				t.Fatalf("status = %s, want skipped: %v", result.Status, result.Err)
			}
			assertFileContent(t, filepath.Join(escape, "precious.mov"), "keep me too")
			assertAbsent(t, filepath.Join(escape, "a file.txt"))
		})
	}
}

// hookedBinder replaces an endpoint just before it is bound, which is the one
// window a binding cannot close by binding.
type hookedBinder struct {
	before func()
	inner  Binder
	bound  int
}

func (b *hookedBinder) Bind(path, identity string) (Bound, error) {
	if b.bound == 0 {
		b.before()
	}
	b.bound++
	return b.inner.Bind(path, identity)
}

// substitutingBinder puts another ordinary directory at an endpoint for exactly
// the time one binding takes and restores the validated one immediately
// afterwards. Every check the mirror makes before and after the binding
// therefore sees the endpoint it validated, and only what the binding itself
// opened can tell the substitution apart.
type substitutingBinder struct {
	endpoint   string
	substitute string
	kept       string
	on         int
	inner      Binder
	t          *testing.T

	calls       int
	substituted bool
}

func (b *substitutingBinder) Bind(path, identity string) (Bound, error) {
	b.calls++
	if b.calls != b.on {
		return b.inner.Bind(path, identity)
	}

	rename(b.t, b.endpoint, b.kept)
	rename(b.t, b.substitute, b.endpoint)
	b.substituted = true
	defer func() {
		rename(b.t, b.endpoint, b.substitute)
		rename(b.t, b.kept, b.endpoint)
	}()

	return b.inner.Bind(path, identity)
}

func rename(t *testing.T, from, to string) {
	t.Helper()

	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

// A source directory replaced by an ordinary directory for the instant it is
// bound, and restored before anything looks at the path again, must not reach
// rsync: the transfer would copy a stranger's files into the destination
// although every path check the mirror makes passes.
func TestMirrorRefusesSourceDirectorySubstitutedOnlyDuringBinding(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	substitute := filepath.Join(root, "substitute")

	writeFile(t, filepath.Join(source, "mine.txt"), "from the validated source")
	writeFile(t, filepath.Join(substitute, "not mine.txt"), "never asked for")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	// The source is the first endpoint bound.
	binder := &substitutingBinder{
		endpoint:   source,
		substitute: substitute,
		kept:       filepath.Join(root, "validated source"),
		on:         1,
		inner:      exec.Binder,
		t:          t,
	}
	exec.Binder = binder

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "nothing was written")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped: %v\n%s", result.Status, result.Err, out.String())
	}
	if !binder.substituted {
		t.Fatal("the source was never substituted, want the binding exercised")
	}

	assertFileContent(t, filepath.Join(substitute, "not mine.txt"), "never asked for")
	assertFileContent(t, filepath.Join(source, "mine.txt"), "from the validated source")
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "keep me")
	assertAbsent(t, filepath.Join(destination, "not mine.txt"))
	assertAbsent(t, filepath.Join(destination, "mine.txt"))
}

// A destination directory replaced the same way must not reach rsync either:
// the transfer would write into it and delete everything it holds that the
// source does not, under a plan computed for another directory entirely.
func TestMirrorRefusesDestinationDirectorySubstitutedOnlyDuringBinding(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	substitute := filepath.Join(root, "substitute")

	writeFile(t, filepath.Join(source, "mine.txt"), "from the source")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "delete me eventually")
	writeFile(t, filepath.Join(substitute, "precious.mov"), "nobody reviewed me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	// The destination is bound after the source.
	binder := &substitutingBinder{
		endpoint:   destination,
		substitute: substitute,
		kept:       filepath.Join(root, "validated destination"),
		on:         2,
		inner:      exec.Binder,
		t:          t,
	}
	exec.Binder = binder

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "nothing was written")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped: %v\n%s", result.Status, result.Err, out.String())
	}
	if !binder.substituted {
		t.Fatal("the destination was never substituted, want the binding exercised")
	}

	assertFileContent(t, filepath.Join(substitute, "precious.mov"), "nobody reviewed me")
	assertAbsent(t, filepath.Join(substitute, "mine.txt"))
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "delete me eventually")
	assertAbsent(t, filepath.Join(destination, "mine.txt"))
}

// The mirror never creates its destination, so no directory it made can be
// substituted before its identity is captured: there is no creation and no
// capture. An absent destination is refused by the preflight instead, and this
// is the production-rsync proof that the refusal happens before rsync is
// started — nothing is created at the configured path and no transfer runs.
func TestMirrorReplacementBeforeInitialIdentityCaptureIsNeverPassedToRsync(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "from the source")
	writeFile(t, filepath.Join(source, "sub dir", "nested.txt"), "nested in the source")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)
	exec.Streamer = hookedStreamer{
		before: func() { t.Error("rsync was started although the destination does not exist") },
		inner:  exec.Streamer,
	}

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "create it yourself")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped: %v\n%s", result.Status, result.Err, out.String())
	}
	if result.Attempted() {
		t.Fatalf("argv = %v, want no transfer", result.Argv)
	}
	if len(approver.plans) > 0 {
		t.Fatal("the approver was asked about a destination that does not exist")
	}

	assertAbsent(t, destination)
}

// substitutingRunner puts another ordinary directory at an endpoint for exactly
// the time a preflight's rsync spends inspecting it and gives the validated one
// back the moment that scan is over. Validation, the destination walks, and the
// binding all see the endpoint they were configured for, so a preflight that
// describes the substitute is a preflight that was not tied to the directory the
// transfer acts on.
type substitutingRunner struct {
	t          *testing.T
	endpoint   string
	substitute string
	kept       string
	inner      Runner

	scans int
}

func (r *substitutingRunner) Run(ctx context.Context, argv []string) (CommandResult, error) {
	if len(argv) == 2 && argv[1] == "--version" {
		return r.inner.Run(ctx, argv)
	}

	rename(r.t, r.endpoint, r.kept)
	rename(r.t, r.substitute, r.endpoint)
	r.scans++
	defer func() {
		rename(r.t, r.endpoint, r.substitute)
		rename(r.t, r.kept, r.endpoint)
	}()

	return r.inner.Run(ctx, argv)
}

// A source that another ordinary directory stands in for while every preflight
// scans it would have the user approve a plan computed from a stranger's files:
// here the substitute holds what the destination already has, so the approved
// plan deletes nothing, while mirroring the real source would delete the file
// the substitute made look wanted. The final preflight inspects the bound
// source, so the deletion it finds is one nobody reviewed and nothing is
// written.
func TestMirrorRefusesSourceDirectorySubstitutedOnlyDuringFinalPreflight(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	substitute := filepath.Join(root, "substitute")

	writeFile(t, filepath.Join(source, "mine.txt"), "from the validated source")
	writeFile(t, filepath.Join(substitute, "mine.txt"), "from the validated source")
	writeFile(t, filepath.Join(substitute, "wanted.mov"), "what the destination holds")
	writeFile(t, filepath.Join(destination, "wanted.mov"), "what the destination holds")

	cfg, deps := realRsyncDeps(t, source, destination)
	runner := &substitutingRunner{
		t:          t,
		endpoint:   source,
		substitute: substitute,
		kept:       filepath.Join(root, "validated source"),
		inner:      deps.Runner,
	}
	deps.Runner = runner
	exec, _, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "nothing was written")
	requireErrorContains(t, err, "wanted.mov")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped: %v\n%s", result.Status, result.Err, out.String())
	}
	if runner.scans < 2 {
		t.Fatalf("the source was substituted for %d preflights, want the final one exercised", runner.scans)
	}

	assertFileContent(t, filepath.Join(destination, "wanted.mov"), "what the destination holds")
	assertAbsent(t, filepath.Join(destination, "mine.txt"))
	assertFileContent(t, filepath.Join(source, "mine.txt"), "from the validated source")
	assertFileContent(t, filepath.Join(substitute, "wanted.mov"), "what the destination holds")
}

// A destination that another ordinary directory stands in for while every
// preflight scans it is the destructive half of the same substitution: the
// approved plan describes an empty stand-in and deletes nothing, while the
// transfer would delete everything the real destination holds. The final
// preflight inspects the bound destination, so those deletions are unreviewed
// and neither directory is touched.
func TestMirrorRefusesDestinationDirectorySubstitutedOnlyDuringFinalPreflight(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	substitute := filepath.Join(root, "substitute")

	writeFile(t, filepath.Join(source, "mine.txt"), "from the source")
	writeFile(t, filepath.Join(destination, "precious.mov"), "delete me only after a review")
	if err := os.MkdirAll(substitute, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, deps := realRsyncDeps(t, source, destination)
	runner := &substitutingRunner{
		t:          t,
		endpoint:   destination,
		substitute: substitute,
		kept:       filepath.Join(root, "validated destination"),
		inner:      deps.Runner,
	}
	deps.Runner = runner
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "nothing was written")
	requireErrorContains(t, err, "precious.mov")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped: %v\n%s", result.Status, result.Err, out.String())
	}
	if runner.scans < 2 {
		t.Fatalf("the destination was substituted for %d preflights, want the final one exercised", runner.scans)
	}
	if len(approver.plans) != 1 || approver.plans[0].Changes.Deleted != 0 {
		t.Fatalf("the approved plan listed %d deletions, want the substitute's none", approver.plans[0].Changes.Deleted)
	}

	assertFileContent(t, filepath.Join(destination, "precious.mov"), "delete me only after a review")
	assertAbsent(t, filepath.Join(destination, "mine.txt"))
	assertAbsent(t, filepath.Join(substitute, "mine.txt"))
}

// A source entry the preflight itself cannot read fails before the prompt, so
// no approval is ever asked for a plan rsync could not compute.
func TestMirrorAgainstRealRsyncStopsWhenThePreflightCannotReadTheSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every file, so an unreadable source cannot be simulated")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "readable.txt"), "fine")
	unreadable := filepath.Join(source, "unreadable.txt")
	writeFile(t, unreadable, "secret")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "preflight failed with exit code 23")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(approver.plans) != 0 {
		t.Fatal("the approver was asked to approve a preflight that failed")
	}
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "keep me")
}
