package mirror

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

func (r *substitutingRunner) Run(ctx context.Context, command Command) (CommandResult, error) {
	argv := command.Argv
	if len(argv) == 2 && argv[1] == "--version" {
		return r.inner.Run(ctx, command)
	}

	rename(r.t, r.endpoint, r.kept)
	rename(r.t, r.substitute, r.endpoint)
	r.scans++
	defer func() {
		rename(r.t, r.endpoint, r.substitute)
		rename(r.t, r.kept, r.endpoint)
	}()

	return r.inner.Run(ctx, command)
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
