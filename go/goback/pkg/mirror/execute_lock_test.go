package mirror

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

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
	if argv[len(argv)-2] != "/bound/Volumes/SanDisk/Real/" || argv[len(argv)-1] != "." || result.Dir != "/bound"+testDestination {
		t.Fatalf("endpoints = %v, want the bound directories rather than their paths", argv[len(argv)-2:])
	}
	preflight := setup.runner.commands[len(setup.runner.commands)-1]
	if preflight.Dir != result.Dir || !slices.Equal(preflight.Argv[len(preflight.Argv)-2:], argv[len(argv)-2:]) {
		t.Fatalf("preflight = %v, transfer = %v, want the same bound endpoints", preflight, result.Command)
	}
	if setup.streamer.commands[0].Dir != result.Dir {
		t.Fatalf("streamed command = %v, want the recorded working directory %q", setup.streamer.commands[0], result.Dir)
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
