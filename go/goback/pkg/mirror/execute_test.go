package mirror

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
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
	if argv[len(argv)-2] != testSource+"/" || argv[len(argv)-1] != "." || result.Dir != testDestination {
		t.Fatalf("endpoints = %v, want the source contents copied into the destination", argv[len(argv)-2:])
	}
	if result.Status != StatusSucceeded || result.ExitCode != 0 {
		t.Fatalf("result = %s / %d, want succeeded with exit code 0", result.Status, result.ExitCode)
	}
	if !result.Attempted() || result.Duration <= 0 {
		t.Fatalf("result = %#v, want a measured attempt", result)
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

func (r *cancellingRunner) Run(ctx context.Context, command Command) (CommandResult, error) {
	argv := command.Argv
	if len(argv) > 1 && argv[1] != "--version" {
		r.dryRuns++
		if r.dryRuns == 2 {
			r.cancel()
		}
	}
	return r.inner.Run(ctx, command)
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
		{name: "interrupted mid transfer", result: Result{Status: StatusInterrupted, Command: Command{Argv: []string{"rsync"}}}, want: PartialDir},
		{name: "failed", result: Result{Status: StatusFailed, Command: Command{Argv: []string{"rsync"}}}, want: "only partially updated"},
		{name: "succeeded", result: Result{Status: StatusSucceeded, Command: Command{Argv: []string{"rsync"}}}, want: "finished in"},
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
