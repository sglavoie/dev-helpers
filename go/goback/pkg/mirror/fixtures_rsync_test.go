package mirror

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// awkwardNames are filenames that break anything parsing rsync's output by
// splitting on whitespace, expanding a shell, or assuming a name is printable.
var awkwardNames = []string{
	"a file with spaces.txt",
	`quotes'and"more.txt`,
	"dollar $HOME.txt",
	"glob*star?.txt",
	"semi;colon&pipe|.txt",
	"tab\there.txt",
	"newline\nhere.txt",
	"-leading-dash.txt",
}

func assertSymlink(t *testing.T, path, wantTarget string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("%s is not in the destination: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink, want the link copied rather than its target", path)
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if target != wantTarget {
		t.Fatalf("%s points at %q, want %q", path, target, wantTarget)
	}
}

func assertSameFile(t *testing.T, first, second string) {
	t.Helper()

	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatalf("%s and %s are separate files in the destination, want the hard link preserved", first, second)
	}
}

func deletionPaths(plan Plan) []string {
	paths := make([]string, 0, len(plan.Changes.Deletions))
	for _, deletion := range plan.Changes.Deletions {
		paths = append(paths, deletion.Path)
	}
	return paths
}

// An exact copy keeps what a plain recursive copy loses: dot files, the links
// themselves rather than what they point at, and the identity two hard links
// share.
func TestMirrorPreservesHiddenFilesSymlinksAndHardLinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, ".hidden"), "hidden")
	writeFile(t, filepath.Join(source, ".hidden dir", "inside.txt"), "nested")
	writeFile(t, filepath.Join(source, "original.txt"), "linked content")
	if err := os.Link(filepath.Join(source, "original.txt"), filepath.Join(source, "hard link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("original.txt", filepath.Join(source, "sym link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nowhere/at/all", filepath.Join(source, "broken link.txt")); err != nil {
		t.Fatal(err)
	}
	makeDir(t, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}

	assertFileContent(t, filepath.Join(destination, ".hidden"), "hidden")
	assertFileContent(t, filepath.Join(destination, ".hidden dir", "inside.txt"), "nested")
	assertSymlink(t, filepath.Join(destination, "sym link.txt"), "original.txt")
	assertSymlink(t, filepath.Join(destination, "broken link.txt"), "/nowhere/at/all")
	assertSameFile(t, filepath.Join(destination, "original.txt"), filepath.Join(destination, "hard link.txt"))
}

// rsync is executed without a shell and its itemization is read at fixed
// offsets, so a filename only has to survive rsync's own escaping.
func TestMirrorHandlesAwkwardFilenames(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	for _, name := range awkwardNames {
		writeFile(t, filepath.Join(source, name), "content of "+name)
		writeFile(t, filepath.Join(destination, "obsolete "+name), "delete me")
	}
	matchModTime(t, source, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}

	// Every awkward name was itemized as its own entry rather than being
	// split or swallowed, which is what the counts prove.
	approved := approver.plans[0]
	if approved.Changes.Created != len(awkwardNames) {
		t.Fatalf("created = %d, want one entry per awkward name (%d): %v", approved.Changes.Created, len(awkwardNames), approved.Changes)
	}
	if approved.Changes.Deleted != len(awkwardNames) {
		t.Fatalf("deleted = %d, want one entry per obsolete awkward name (%d): %v", approved.Changes.Deleted, len(awkwardNames), deletionPaths(approved))
	}

	// A newline is the one character rsync escapes, so the plan stays
	// line-oriented and the path is still recognizable.
	if !slices.Contains(deletionPaths(approved), `obsolete newline\#012here.txt`) {
		t.Fatalf("deletions = %q, want the newline reported as rsync escaped it", deletionPaths(approved))
	}

	for _, name := range awkwardNames {
		assertFileContent(t, filepath.Join(destination, name), "content of "+name)
		assertAbsent(t, filepath.Join(destination, "obsolete "+name))
	}
}

// The partial directory holds the resumable remains of an interrupted transfer.
// It belongs to neither side of the mirror: it is not copied from the source
// and it is not deleted from the destination for being absent from the source.
func TestMirrorNeverTransfersOrDeletesThePartialDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "kept.txt"), "hello")
	writeFile(t, filepath.Join(source, PartialDir, "source leftover.part"), "not mine to copy")
	writeFile(t, filepath.Join(destination, PartialDir, "interrupted.part"), "resume me")
	writeFile(t, filepath.Join(destination, "obsolete.txt"), "delete me")
	matchModTime(t, source, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}

	for _, path := range deletionPaths(approver.plans[0]) {
		if strings.Contains(path, PartialDir) {
			t.Fatalf("the plan would delete %q, want the partial directory left alone", path)
		}
	}
	assertFileContent(t, filepath.Join(destination, PartialDir, "interrupted.part"), "resume me")
	assertAbsent(t, filepath.Join(destination, PartialDir, "source leftover.part"))
	assertAbsent(t, filepath.Join(destination, "obsolete.txt"))
	assertFileContent(t, filepath.Join(destination, "kept.txt"), "hello")
}

// A source holding only the partial directory is not empty on disk, but rsync
// excludes that directory, so the mirror it describes transfers nothing and
// deletes everything. The guard reads the source through the same exclusion,
// which is why the approver is never shown that plan.
func TestPartialDirectoryOnlySourceIsEffectivelyEmpty(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, PartialDir, "interrupted.part"), "resume me")
	writeFile(t, filepath.Join(destination, "precious.mov"), "keep me")
	matchModTime(t, source, destination)

	cfg, deps := realRsyncDeps(t, source, destination)

	// What rsync itself would do with such a source, established before the
	// guard is exercised: nothing to send, and the destination emptied.
	preflight, err := deps.Runner.Run(context.Background(), DryRunArgv(cfg))
	if err != nil {
		t.Fatal(err)
	}
	changes, err := ParseChanges(preflight.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	if changes.Created != 0 || changes.TransferBytes != 0 {
		t.Fatalf("changes = %#v, want rsync to transfer nothing out of a source holding only %s", changes, PartialDir)
	}
	if !slices.Contains(deletionPaths(Plan{Changes: changes}), "precious.mov") {
		t.Fatalf("deletions = %q, want the whole destination planned for deletion", deletionPaths(Plan{Changes: changes}))
	}

	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "is empty of everything the mirror would transfer")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(approver.plans) != 0 {
		t.Fatal("the approver was shown a plan that deletes the destination for an effectively empty source")
	}
	assertFileContent(t, filepath.Join(destination, "precious.mov"), "keep me")
}

// Deletions happen after the transfer, so the space they would free cannot pay
// for it. A destination that cannot hold the transfer is refused before anyone
// is asked to approve it.
func TestMirrorRefusesWhenTheDestinationCannotHoldTheTransfer(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "big.mov"), strings.Repeat("x", 4096))
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me until a run succeeds")

	cfg, deps := realRsyncDeps(t, source, destination)
	deps.Capacity = fakeCapacity{available: 512}
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if !errors.Is(err, ErrInsufficientSpace) {
		t.Fatalf("error = %v, want it to report the missing free space", err)
	}
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(approver.plans) != 0 {
		t.Fatal("the approver was asked to approve a transfer the destination cannot hold")
	}
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "keep me until a run succeeds")
	assertAbsent(t, filepath.Join(destination, "big.mov"))
}

// Nothing of the destination is ever created, so a destination whose parent is
// missing is a configuration mistake rather than a tree to materialize.
func TestMirrorRefusesADestinationWhoseParentIsMissing(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "missing", "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "create it yourself")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(approver.plans) != 0 {
		t.Fatal("the approver was asked about a destination that cannot be created")
	}
	assertAbsent(t, filepath.Join(root, "missing"))
}

// A destination nothing can be written into produces a perfectly good plan, so
// without a writability check the approval would be asked for a transfer that
// can only fail partway through.
func TestMirrorRefusesAnUnwritableDestination(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to every directory, so an unwritable destination cannot be simulated")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me")
	if err := os.Chmod(destination, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(destination, 0o755); err != nil {
			t.Error(err)
		}
	})

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "is not writable")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	if len(approver.plans) != 0 {
		t.Fatal("the approver was asked about a destination nothing can be written into")
	}
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "keep me")
}

// An interruption that arrives once the transfer is under way leaves the
// destination partially written and its extra content untouched, and rsync is
// not left running behind the command that was cancelled.
func TestMirrorAgainstRealRsyncReportsAnInterruptedTransfer(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	writeFile(t, filepath.Join(destination, "obsolete.mov"), "keep me until a run succeeds")
	matchModTime(t, source, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exec.Streamer = hookedStreamer{before: cancel, inner: exec.Streamer}

	result, err := Mirror(ctx, cfg, deps, exec, out)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("error = %v, want an interruption", err)
	}
	if result.Status != StatusInterrupted || result.ExitCode != InterruptedExitCode {
		t.Fatalf("result = %s / %d, want an interruption", result.Status, result.ExitCode)
	}
	if !result.Attempted() {
		t.Fatal("an interrupted transfer reports no attempt, want it recorded as one")
	}
	assertFileContent(t, filepath.Join(destination, "obsolete.mov"), "keep me until a run succeeds")
}

// requireCaseSensitiveFilesystem skips a test that cannot be represented on a
// case-insensitive volume, naming the filesystem as the reason.
func requireCaseSensitiveFilesystem(t *testing.T, dir string) {
	t.Helper()

	probe := filepath.Join(dir, "case-probe")
	writeFile(t, probe, "probe")
	if _, err := os.Stat(filepath.Join(dir, "CASE-PROBE")); err == nil {
		t.Skipf("unsupported filesystem: %s is case-insensitive, so a case-only rename cannot exist on it", dir)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
}

// A rename that only changes case is a creation and a deletion to rsync, and
// both have to reach the destination for it to match the source.
func TestMirrorAppliesACaseOnlyRename(t *testing.T) {
	root := t.TempDir()
	requireCaseSensitiveFilesystem(t, root)

	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "Movie.mov"), "same content")
	writeFile(t, filepath.Join(destination, "movie.mov"), "same content")
	matchModTime(t, source, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}
	if !slices.Contains(deletionPaths(approver.plans[0]), "movie.mov") {
		t.Fatalf("deletions = %q, want the lowercase name listed", deletionPaths(approver.plans[0]))
	}
	assertFileContent(t, filepath.Join(destination, "Movie.mov"), "same content")
	assertAbsent(t, filepath.Join(destination, "movie.mov"))
}

// A source that became empty between the preflight and the transfer would plan
// the deletion of the whole destination, so the revalidation refuses it.
func TestMirrorRefusesASourceThatEmptiedDuringTheApproval(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "only.txt"), "hello")
	writeFile(t, filepath.Join(destination, "precious.mov"), "keep me")

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, approver, out := realExecDeps(true)
	approver.whileDeciding = func() {
		if err := os.Remove(filepath.Join(source, "only.txt")); err != nil {
			t.Error(err)
		}
	}

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	requireErrorContains(t, err, "is empty")
	if result.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", result.Status)
	}
	assertFileContent(t, filepath.Join(destination, "precious.mov"), "keep me")
}
