package mirror

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// InterruptedExitCode is the exit code an interrupted mirror is recorded with.
// rsync never exits with it, so an interruption stays distinguishable from any
// failure rsync can report.
const InterruptedExitCode = -1

// ErrInterrupted is the error of a mirror cancelled by a signal.
var ErrInterrupted = errors.New("the mirror was interrupted")

// Status classifies the outcome of a mirror.
type Status int

const (
	// StatusSkipped means no transfer was ever started: the preflight
	// failed or the approved state no longer held.
	StatusSkipped Status = iota
	// StatusUpToDate means the destination already matched the source, so
	// nothing was asked and nothing was run.
	StatusUpToDate
	// StatusDeclined means the user did not approve the transfer.
	StatusDeclined
	// StatusInterrupted means a signal cancelled the mirror.
	StatusInterrupted
	// StatusFailed means rsync ran and did not finish successfully.
	StatusFailed
	// StatusSucceeded means rsync ran and exited zero.
	StatusSucceeded
)

func (s Status) String() string {
	switch s {
	case StatusUpToDate:
		return "up to date"
	case StatusDeclined:
		return "declined"
	case StatusInterrupted:
		return "interrupted"
	case StatusFailed:
		return "failed"
	case StatusSucceeded:
		return "succeeded"
	default:
		return "skipped"
	}
}

// Result is the outcome of one mirror.
type Result struct {
	Status Status

	// Plan is the preflight the outcome is based on: the revalidated one
	// when the transfer was reached, the approved one otherwise.
	Plan Plan

	// Argv is the executed command, empty when nothing was executed.
	Argv []string

	ExitCode  int
	StartedAt time.Time
	Duration  time.Duration
	Err       error
}

// Attempted reports whether rsync was actually started, whatever it did
// afterwards. An interruption between the approval and the transfer leaves
// Argv empty and is not an attempt.
func (r Result) Attempted() bool {
	if len(r.Argv) == 0 {
		return false
	}
	return r.Status == StatusSucceeded || r.Status == StatusFailed || r.Status == StatusInterrupted
}

// CommandString renders the executed command for display and history.
func (r Result) CommandString() string {
	return FormatArgv(r.Argv)
}

// Summary is the one-line outcome shown to the user.
func (r Result) Summary() string {
	switch r.Status {
	case StatusUpToDate:
		return "The destination already matches the source: nothing was transferred and nothing was deleted."
	case StatusDeclined:
		return "Cancelled: nothing was created, transferred, or deleted."
	case StatusInterrupted:
		if !r.Attempted() {
			return "Interrupted: nothing was created, transferred, or deleted."
		}
		return fmt.Sprintf("Interrupted after %s. Partial data is kept in %s inside the destination, and the next mirror resumes it; the deletions were not performed.",
			r.Duration.Round(time.Second), PartialDir)
	case StatusFailed:
		return fmt.Sprintf("rsync exited with %d after %s. The destination was only partially updated, and its extra content was not deleted.",
			r.ExitCode, r.Duration.Round(time.Second))
	case StatusSucceeded:
		return fmt.Sprintf("The mirror finished in %s: %d created, %d updated, %d deleted.",
			r.Duration.Round(time.Second), r.Plan.Changes.Created, r.Plan.Changes.Updated, r.Plan.Changes.Deleted)
	default:
		return "The mirror did not run: nothing was created, transferred, or deleted."
	}
}

// Mirror performs a real mirror. It runs and prints the same preflight as a
// dry run, takes the destination's exclusive lock, asks for an explicit
// approval, binds both endpoints to the directories that approval was about,
// revalidates everything it rested on through those bindings, and only then
// transfers. Nothing outside the preflight is executed and nothing is written
// before both the approval and the revalidation succeeded.
func Mirror(ctx context.Context, cfg Config, deps Deps, exec ExecDeps, out io.Writer) (Result, error) {
	approved, err := DryRun(ctx, cfg, deps)
	if err != nil {
		return skipped(err)
	}
	approved.Render(out)

	if approved.Changes.Empty() {
		return Result{Status: StatusUpToDate, Plan: approved}, nil
	}

	// Two mirrors of one destination each review their own plan and then
	// delete what the other just wrote, and neither user is shown the
	// other's changes. The lock is taken on the resolved destination, so two
	// configured paths that are symlink aliases of one directory exclude
	// each other. It is taken before the prompt, so a second mirror is
	// refused while somebody is still deciding rather than after they
	// answered, and it is held until the transfer is over. stillApproved
	// refuses a destination that resolves elsewhere by then, so the lock held
	// is always the one of the directory the transfer writes into.
	release, err := exec.Locker.Lock(approved.Endpoints.DestinationResolved)
	if err != nil {
		return skipped(err)
	}
	defer release()

	if !exec.Approver.Approve(approved) {
		return Result{Status: StatusDeclined, Plan: approved}, nil
	}
	if ctx.Err() != nil {
		return Result{Status: StatusInterrupted, Plan: approved, Err: ErrInterrupted}, ErrInterrupted
	}

	// The binding below opens whatever the destination path holds now, and
	// everything after it is about that directory. A path that came to hold
	// another ordinary directory while the prompt was on screen is therefore
	// refused before it is bound rather than compared once it is: from here on
	// the mirror would be revalidating a directory it never locked.
	if err := destinationStillLocked(deps.Devices, approved.Endpoints); err != nil {
		return skipped(err)
	}

	// Both endpoints are bound before anything observes them again, and every
	// observation from here on is made through those bindings: the final
	// preflight, the destination walks that bound its deletions, and the
	// transfer all name the endpoints by identity rather than by place. A
	// preflight given path strings reports on whatever the paths hold while
	// rsync reads them, so a directory substituted for the length of that scan
	// alone could otherwise describe one directory while the transfer, which
	// opens its own arguments later than any check the mirror can make, acts on
	// another. Requiring the recorded identity is what keeps the binding itself
	// from landing on a directory substituted for the instant it is opened.
	boundSource, err := exec.Binder.Bind(approved.Endpoints.SourceResolved, approved.Endpoints.SourceIdentity)
	if err != nil {
		return skipped(err)
	}
	defer boundSource.Release()

	boundDestination, err := exec.Binder.Bind(approved.Endpoints.DestinationResolved, approved.Endpoints.DestinationIdentity)
	if err != nil {
		return skipped(err)
	}
	defer boundDestination.Release()

	destinationName := boundDestination.Path

	// The destination is listed before the revalidation scan starts, so
	// that everything it gains from here on is visible as content no plan
	// ever reviewed, including whatever appears while that scan runs.
	reviewed, err := destinationEntries(deps.FS, destinationName)
	if err != nil {
		return skipped(err)
	}

	// The user spent real time at the prompt, during which a drive can be
	// unplugged, remounted, or replaced, and the destination can gain
	// content nobody reviewed. Everything the approval rested on is
	// therefore measured again, and the transfer runs against the second
	// plan rather than the first.
	current, err := dryRunBound(ctx, cfg, deps, boundSource.Path, destinationName)
	if err != nil {
		if ctx.Err() != nil {
			return Result{Status: StatusInterrupted, Plan: approved, Err: ErrInterrupted}, ErrInterrupted
		}
		return skipped(fmt.Errorf("the endpoints are no longer in the state that was approved, so nothing was written: %w", err))
	}
	if err := stillApproved(approved, current); err != nil {
		return skipped(err)
	}
	if current.Changes.Empty() {
		return Result{Status: StatusUpToDate, Plan: current}, nil
	}
	deletable, err := destinationUnchanged(deps.FS, destinationName, reviewed)
	if err != nil {
		return skipped(err)
	}

	// The binding settles which directories rsync acts on; this settles that
	// the paths the configuration names still lead to them, which a path
	// repointed at another directory inside the same volume would not. A
	// replacement from here on changes neither.
	if err := endpointsStillResolve(deps.FS, current.Endpoints); err != nil {
		return skipped(err)
	}
	resolvedDestination := current.Endpoints.DestinationResolved
	if err := resolvesTo(deps.FS, resolvedDestination, resolvedDestination); err != nil {
		return skipped(err)
	}

	// The transfer scans the destination itself, later than any walk the
	// mirror performed, so it is given the boundary of what the mirror did
	// see: it may delete those paths and nothing else.
	rules, removeRules, err := exec.Rules.WriteRules(DeletionRules(sortedPaths(deletable)))
	if err != nil {
		return skipped(err)
	}
	defer removeRules()

	return transfer(ctx, cfg, deps, exec, current, boundSource, boundDestination, rules)
}

func transfer(ctx context.Context, cfg Config, deps Deps, exec ExecDeps, plan Plan, source, destination Bound, rules string) (Result, error) {
	bound := Config{
		Source:      source.Path,
		Destination: destination.Path,
		RsyncBinary: cfg.RsyncBinary,
	}

	result := Result{
		Plan:      plan,
		Argv:      TransferArgv(bound, rules),
		StartedAt: deps.Clock.Now(),
	}

	code, err := exec.Streamer.Stream(ctx, result.Argv)
	result.Duration = deps.Clock.Now().Sub(result.StartedAt)

	switch {
	case ctx.Err() != nil:
		result.Status = StatusInterrupted
		result.ExitCode = InterruptedExitCode
		result.Err = ErrInterrupted
	case err != nil:
		result.Status = StatusFailed
		result.ExitCode = code
		result.Err = fmt.Errorf("the mirror could not be run: %w", err)
	case code != 0:
		result.Status = StatusFailed
		result.ExitCode = code
		result.Err = fmt.Errorf("the mirror failed with exit code %d", code)
	default:
		result.Status = StatusSucceeded
	}
	return result, result.Err
}

// stillApproved compares the revalidated preflight with the approved one. It
// is what stands between a reviewed deletion list and a destination that
// quietly became a different drive, changed which directory it names, or gained
// content, while the prompt was on screen.
func stillApproved(approved, current Plan) error {
	if approved.Endpoints.SourceDevice != current.Endpoints.SourceDevice {
		return fmt.Errorf("the source %s is not on the filesystem it was on during the preflight: the drive was unmounted or replaced, so nothing was written", approved.Endpoints.Source)
	}
	if approved.Endpoints.DestinationDevice != current.Endpoints.DestinationDevice {
		return fmt.Errorf("the destination %s is not on the filesystem it was on during the preflight: the drive was unmounted or replaced, so nothing was written", approved.Endpoints.Destination)
	}

	// The lock was taken on the destination the approved plan resolved to,
	// while everything from here on acts on the revalidated one. An in-volume
	// alias retargeted during the prompt keeps the same device and can keep the
	// same deletion list, so nothing else notices that the transfer would write
	// into a directory whose lock another mirror may be holding.
	if approved.Endpoints.DestinationResolved != current.Endpoints.DestinationResolved {
		return fmt.Errorf("the destination %s named %s when the mirror took its lock and names %s now, so nothing was written: the transfer would write into a directory this mirror never locked, which another mirror may be writing to",
			approved.Endpoints.Destination, approved.Endpoints.DestinationResolved, current.Endpoints.DestinationResolved)
	}

	// The lock names a path, while the transfer acts on a directory. A path
	// whose directory was replaced by another ordinary one keeps its
	// resolution and its device, so this is the only comparison that keeps the
	// held lock and the transferred directory the same thing. The anchor is
	// already bound by now, so what it still reports is a leaf that appeared or
	// vanished while the prompt was on screen.
	if approved.Endpoints.DestinationIdentity != current.Endpoints.DestinationIdentity {
		return replacedDestination(approved.Endpoints.Destination)
	}

	unreviewed := unreviewedDeletions(approved, current)
	if len(unreviewed) > 0 {
		return fmt.Errorf("the destination gained %d entries that the approved plan did not list (%s), so nothing was written: run the mirror again to review them",
			len(unreviewed), strings.Join(firstFew(unreviewed, 3), ", "))
	}
	return nil
}

// destinationStillLocked refuses when the destination path stopped holding the
// directory the lock was taken for. It runs before the binding, because the
// binding opens what the path holds now and the whole revalidation is then made
// through it: a directory substituted during the prompt would otherwise be
// bound, inspected, and transferred into under a lock another mirror may hold.
// No device and no resolution changes when one ordinary directory replaces
// another, so the identity is the only thing that reports it.
func destinationStillLocked(devices Devices, approved Endpoints) error {
	identity, err := devices.Identity(approved.DestinationResolved)
	if err != nil {
		return fmt.Errorf("cannot identify the directory the destination %s is, so nothing was written: %w", approved.Destination, err)
	}
	if identity != approved.DestinationIdentity {
		return replacedDestination(approved.Destination)
	}
	return nil
}

func replacedDestination(destination string) error {
	return fmt.Errorf("the destination %s is no longer the directory it was when the mirror took its lock, so nothing was written: another directory occupies that path now, and transferring into it would write under a lock taken for a different directory, which another mirror may be holding",
		destination)
}

// endpointsStillResolve refuses to write when an endpoint no longer resolves to
// what the last validation accepted. The endpoints are bound by the time it
// runs, so it reports a replacement the binding has already made harmless
// rather than racing to prevent one: it is the difference between the
// directories the transfer acts on and what the configured paths have come to
// mean.
func endpointsStillResolve(fsys FileSystem, endpoints Endpoints) error {
	if err := resolvesTo(fsys, endpoints.Source, endpoints.SourceResolved); err != nil {
		return err
	}
	return resolvesTo(fsys, endpoints.Destination, endpoints.DestinationResolved)
}

func resolvesTo(fsys FileSystem, path, want string) error {
	resolved, err := fsys.Resolve(path)
	if err != nil {
		return fmt.Errorf("cannot resolve %s, so nothing was written: %w", path, err)
	}
	if resolved != want {
		return fmt.Errorf("%s now resolves to %s instead of %s, so nothing was written: an endpoint replaced by a symlink after the mirror was validated would make it write and delete somewhere nobody approved", path, resolved, want)
	}
	return nil
}

// unreviewedDeletions returns the paths the revalidated plan would delete that
// the approved plan never showed the user.
func unreviewedDeletions(approved, current Plan) []string {
	reviewed := make(map[string]struct{}, len(approved.Changes.Deletions))
	for _, deletion := range approved.Changes.Deletions {
		reviewed[deletion.Path] = struct{}{}
	}

	var unreviewed []string
	for _, deletion := range current.Changes.Deletions {
		if _, ok := reviewed[deletion.Path]; !ok {
			unreviewed = append(unreviewed, deletion.Path)
		}
	}
	return unreviewed
}

// destinationUnchanged refuses the transfer when the destination gained
// entries since the revalidated plan was computed, and returns the entries it
// found, which are the only ones the transfer is then allowed to delete. The
// transfer is a separate rsync process that scans the destination once more on
// its own, and the lock only excludes another goback: content that appeared in
// the meantime would be deleted although neither reviewed plan ever listed it.
func destinationUnchanged(fsys FileSystem, destination string, reviewed map[string]struct{}) (map[string]struct{}, error) {
	current, err := destinationEntries(fsys, destination)
	if err != nil {
		return nil, err
	}

	var appeared []string
	for entry := range current {
		if _, ok := reviewed[entry]; !ok {
			appeared = append(appeared, entry)
		}
	}
	if len(appeared) == 0 {
		return current, nil
	}

	slices.Sort(appeared)
	return nil, fmt.Errorf("the destination %s gained %d entries after the plan was revalidated (%s), so nothing was written: run the mirror again to review them",
		destination, len(appeared), strings.Join(firstFew(appeared, 3), ", "))
}

// sortedPaths orders the walked destination paths so a transfer is given the
// same rules, in the same order, on every run.
func sortedPaths(entries map[string]struct{}) []string {
	paths := make([]string, 0, len(entries))
	for entry := range entries {
		paths = append(paths, entry)
	}
	slices.Sort(paths)
	return paths
}

// destinationEntries lists every path the destination holds, relative to it.
func destinationEntries(fsys FileSystem, destination string) (map[string]struct{}, error) {
	entries := make(map[string]struct{})
	if err := listInto(fsys, destination, "", entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func listInto(fsys FileSystem, dir, prefix string, into map[string]struct{}) error {
	children, err := fsys.ReadDir(dir)
	if os.IsNotExist(err) {
		// A directory can be removed by whoever owns its content while
		// the walk is under way, and one that is no longer there holds
		// nothing.
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read the mirror destination %s: %w", dir, err)
	}

	for _, child := range children {
		if prefix == "" && child.Name() == PartialDir {
			// rsync owns the partial directory: it is neither
			// transferred nor deleted, so what it holds is not part
			// of what a plan reviews.
			continue
		}
		relative := filepath.Join(prefix, child.Name())
		into[relative] = struct{}{}
		if child.IsDir() {
			if err := listInto(fsys, filepath.Join(dir, child.Name()), relative, into); err != nil {
				return err
			}
		}
	}
	return nil
}

func firstFew(paths []string, n int) []string {
	if len(paths) <= n {
		return paths
	}
	return append(paths[:n:n], "...")
}

func skipped(err error) (Result, error) {
	return Result{Status: StatusSkipped, Err: err}, err
}
