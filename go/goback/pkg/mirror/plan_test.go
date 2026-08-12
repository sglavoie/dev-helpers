package mirror

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDryRunReportsThePlan(t *testing.T) {
	cfg, deps, runner := mountedSetup()

	plan, err := DryRun(context.Background(), cfg, deps)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Changes.Created != 2 || plan.Changes.Updated != 1 || plan.Changes.Deleted != 3 {
		t.Fatalf("changes = %#v, want 2 created, 1 updated, 3 deleted", plan.Changes)
	}
	if plan.Changes.TransferBytes != 2048 {
		t.Fatalf("TransferBytes = %d, want 2048", plan.Changes.TransferBytes)
	}
	if plan.AvailableBytes != 1<<30 {
		t.Fatalf("AvailableBytes = %d, want the free space of the destination", plan.AvailableBytes)
	}
	if plan.Duration <= 0 {
		t.Fatalf("Duration = %s, want the measured length of the preflight", plan.Duration)
	}
	if !slices.Contains(runner.dryRunCall(t), "--dry-run") {
		t.Fatalf("rsync ran %v, want --dry-run", runner.dryRunCall(t))
	}
}

// Nothing may run before the endpoints are known to be safe, because rsync
// itself is what would produce a deletion plan.
func TestDryRunValidatesBeforeRunningAnything(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(deps Deps)
		want   string
	}{
		{
			name:   "missing source",
			mutate: func(deps Deps) { delete(deps.FS.(fakeFS).files, testSource) },
			want:   "does not exist",
		},
		{
			name:   "empty source",
			mutate: func(deps Deps) { deps.FS.(fakeFS).files[testSource] = fakeFile{dir: true} },
			want:   "is empty",
		},
		{
			name:   "unmounted source",
			mutate: func(deps Deps) { deps.Devices.(fakeDevices).devices["/Volumes/SanDisk"] = "system" },
			want:   "is not a mounted volume",
		},
		{
			name:   "unmounted destination",
			mutate: func(deps Deps) { deps.Devices.(fakeDevices).devices["/Volumes/Elements"] = "system" },
			want:   "is not a mounted volume",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, runner := mountedSetup()
			tc.mutate(deps)

			plan, err := DryRun(context.Background(), cfg, deps)
			requireErrorContains(t, err, tc.want)
			if len(runner.calls) != 0 {
				t.Fatalf("rsync ran %v, want nothing executed before validation succeeds", runner.calls)
			}
			if len(plan.Changes.Deletions) != 0 {
				t.Fatalf("deletions = %#v, want none from a failed preflight", plan.Changes.Deletions)
			}
		})
	}
}

// Deletions are delayed until the transfer finished, so the space they free
// cannot pay for the transfer that precedes them.
func TestDryRunRejectsATransferLargerThanTheFreeSpace(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	deps.Capacity = fakeCapacity{available: 2047}

	_, err := DryRun(context.Background(), cfg, deps)
	if !errors.Is(err, ErrInsufficientSpace) {
		t.Fatalf("error = %v, want it to wrap ErrInsufficientSpace", err)
	}
	requireErrorContains(t, err, "deletions happen after the transfer")
}

func TestDryRunAcceptsATransferThatExactlyFits(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	deps.Capacity = fakeCapacity{available: 2048}

	if _, err := DryRun(context.Background(), cfg, deps); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunReportsAFailedRsync(t *testing.T) {
	cfg, deps, runner := mountedSetup()
	runner.dryRun = CommandResult{ExitCode: 23, Stderr: "rsync error: some files could not be transferred"}

	_, err := DryRun(context.Background(), cfg, deps)
	requireErrorContains(t, err, "exit code 23")
	requireErrorContains(t, err, "some files could not be transferred")
}

func TestDryRunReportsCancellation(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DryRun(ctx, cfg, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want a cancelled context to be reported", err)
	}
}

func TestDryRunRejectsUnparsableOutput(t *testing.T) {
	cfg, deps, runner := mountedSetup()
	runner.dryRun = CommandResult{Stdout: "*deleting   everything.mov\n"}

	_, err := DryRun(context.Background(), cfg, deps)
	requireErrorContains(t, err, "Total transferred file size:")
}

func TestRenderShowsEveryDeletion(t *testing.T) {
	cfg, deps, _ := mountedSetup()
	plan, err := DryRun(context.Background(), cfg, deps)
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	plan.Render(&out)
	rendered := out.String()

	for _, want := range []string{
		"dry run, nothing was written",
		testSource,
		testDestination,
		"created:  2",
		"updated:  1",
		"deleted:  3",
		"2.0 KiB (2048 bytes)",
		"dir gone/c.txt",
		"dir gone/",
		"gone.txt",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered plan = %q, want it to contain %q", rendered, want)
		}
	}
}

func TestRenderReportsAnUpToDateDestination(t *testing.T) {
	cfg, deps, runner := mountedSetup()
	runner.dryRun = CommandResult{Stdout: "\nTotal transferred file size: 0 bytes\n"}

	plan, err := DryRun(context.Background(), cfg, deps)
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	plan.Render(&out)
	if !strings.Contains(out.String(), "already up to date") {
		t.Fatalf("rendered plan = %q, want it to say the destination is up to date", out.String())
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{in: 0, want: "0 B"},
		{in: 512, want: "512 B"},
		{in: 1024, want: "1.0 KiB"},
		{in: 2048, want: "2.0 KiB"},
		{in: 1 << 20, want: "1.0 MiB"},
		{in: 3 * (1 << 30), want: "3.0 GiB"},
		{in: 5 * (1 << 40), want: "5.0 TiB"},
	}

	for _, tc := range cases {
		if got := HumanBytes(tc.in); got != tc.want {
			t.Fatalf("HumanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
