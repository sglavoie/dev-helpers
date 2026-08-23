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
