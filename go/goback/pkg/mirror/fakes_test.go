package mirror

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeFile describes one entry of a fake filesystem.
type fakeFile struct {
	dir         bool
	children    []string
	readErr     error
	notWritable bool
}

type fakeFS struct {
	files map[string]fakeFile

	// links maps a path to what it really is, the way a symlink does. Stat
	// still answers for the path itself, because the real Stat follows the
	// link too.
	links map[string]string
}

func (f fakeFS) Stat(path string) (fs.FileInfo, error) {
	file, ok := f.files[path]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
	}
	return fakeInfo{name: filepath.Base(path), dir: file.dir}, nil
}

func (f fakeFS) ReadDir(path string) ([]fs.DirEntry, error) {
	file, ok := f.files[path]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: path, Err: fs.ErrNotExist}
	}
	if file.readErr != nil {
		return nil, file.readErr
	}
	entries := make([]fs.DirEntry, 0, len(file.children))
	for _, child := range file.children {
		entry := fakeInfo{name: child}
		if described, ok := f.files[filepath.Join(path, child)]; ok {
			entry.dir = described.dir
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (f fakeFS) Writable(path string) error {
	file, ok := f.files[path]
	if !ok {
		return &fs.PathError{Op: "access", Path: path, Err: fs.ErrNotExist}
	}
	if file.notWritable {
		return fs.ErrPermission
	}
	return nil
}

func (f fakeFS) Resolve(path string) (string, error) {
	if _, ok := f.files[path]; !ok {
		return "", &fs.PathError{Op: "resolve", Path: path, Err: fs.ErrNotExist}
	}
	for current := path; ; current = filepath.Dir(current) {
		if target, ok := f.links[current]; ok {
			return filepath.Join(target, strings.TrimPrefix(path, current)), nil
		}
		if current == "/" || current == "." {
			return path, nil
		}
	}
}

type fakeInfo struct {
	name string
	dir  bool
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.dir }
func (i fakeInfo) Sys() any           { return nil }

func (i fakeInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir
	}
	return 0
}

func (i fakeInfo) Type() fs.FileMode          { return i.Mode().Type() }
func (i fakeInfo) Info() (fs.FileInfo, error) { return i, nil }

// fakeDevices resolves a path to the device of its nearest registered
// ancestor, the way a real mount point covers everything below it.
type fakeDevices struct {
	devices map[string]string

	// identities names the directory each path is, the way an inode number
	// does. A path answers for itself unless a test says another directory
	// took its place, which is a change no device and no resolution shows.
	identities map[string]string
}

func (d fakeDevices) Device(path string) (string, error) {
	for current := path; ; current = filepath.Dir(current) {
		if device, ok := d.devices[current]; ok {
			return device, nil
		}
		if current == "/" || current == "." {
			return "", fmt.Errorf("no device registered for %s", path)
		}
	}
}

func (d fakeDevices) Identity(path string) (string, error) {
	if identity, ok := d.identities[path]; ok {
		return identity, nil
	}
	if _, err := d.Device(path); err != nil {
		return "", err
	}
	return "directory of " + path, nil
}

type fakeCapacity struct {
	available uint64
	err       error
}

func (c fakeCapacity) AvailableBytes(string) (uint64, error) {
	return c.available, c.err
}

// fakeClock advances by a fixed step on every reading so a duration is
// deterministic.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	if c.now.IsZero() {
		c.now = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
		return c.now
	}
	c.now = c.now.Add(time.Second)
	return c.now
}

// fakeRunner answers the capability probe and the dry run separately, and
// records every argument vector it was asked to execute. A real mirror runs
// the preflight twice, so dryRuns answers each run in turn when it is set,
// which is how a destination that changes while the prompt is on screen is
// expressed.
type fakeRunner struct {
	version    CommandResult
	versionErr error
	dryRun     CommandResult
	dryRuns    []CommandResult
	dryRunErr  error
	calls      [][]string
}

func (r *fakeRunner) Run(ctx context.Context, argv []string) (CommandResult, error) {
	r.calls = append(r.calls, argv)
	if err := ctx.Err(); err != nil {
		return CommandResult{}, err
	}
	if len(argv) == 2 && argv[1] == "--version" {
		return r.version, r.versionErr
	}
	if len(r.dryRuns) > 0 {
		next := r.dryRuns[0]
		if len(r.dryRuns) > 1 {
			r.dryRuns = r.dryRuns[1:]
		}
		return next, r.dryRunErr
	}
	return r.dryRun, r.dryRunErr
}

func (r *fakeRunner) dryRunCall(t *testing.T) []string {
	t.Helper()
	for _, call := range r.calls {
		if len(call) != 2 || call[1] != "--version" {
			return call
		}
	}
	t.Fatal("rsync was never asked to run a dry run")
	return nil
}

const rsyncVersionOutput = `rsync  version 3.4.4  protocol version 32
Copyright (C) 1996-2026 by Andrew Tridgell, Wayne Davison, and others.
Web site: https://rsync.samba.org/
Capabilities:
    64-bit files, 64-bit inums, 64-bit timestamps, 64-bit long ints,
    socketpairs, symlinks, symtimes, hardlinks, hardlink-specials,
    hardlink-symlinks, IPv6, atimes, batchfiles, inplace, append, ACLs,
    xattrs, optional secluded-args, iconv, no prealloc, stop-at, crtimes
Optimizations:
    no SIMD-roll, no asm-roll, openssl-crypto, no asm-MD5
`

// mountedRsyncOutput is the itemization and statistics of a run that creates
// one file, updates another, and deletes a directory with a file in it.
const mountedRsyncOutput = `>f.s....... a file.txt
cd+++++++++ sub/
>f+++++++++ sub/b.txt
*deleting   dir gone/c.txt
*deleting   dir gone/
*deleting   gone.txt

Number of files: 4 (reg: 2, dir: 2)
Number of created files: 2 (reg: 1, dir: 1)
Number of deleted files: 3 (reg: 2, dir: 1)
Number of regular files transferred: 2
Total file size: 6 bytes
Total transferred file size: 2,048 bytes
Literal data: 0 bytes
`

const (
	testSource      = "/Volumes/SanDisk/Media"
	testDestination = "/Volumes/Elements/Media"
)

// mountedSetup is a healthy pair of mounted volumes: a readable non-empty
// source and an existing destination on another device.
func mountedSetup() (Config, Deps, *fakeRunner) {
	runner := &fakeRunner{
		version: CommandResult{Stdout: rsyncVersionOutput},
		dryRun:  CommandResult{Stdout: mountedRsyncOutput},
	}
	deps := Deps{
		FS: fakeFS{files: map[string]fakeFile{
			"/Volumes/SanDisk":        {dir: true, children: []string{"Media"}},
			testSource:                {dir: true, children: []string{"a file.txt"}},
			"/Volumes/Elements":       {dir: true, children: []string{"Media"}},
			testDestination:           {dir: true},
			"/Volumes/Elements/Other": {dir: true},
		}, links: map[string]string{}},
		Capacity: fakeCapacity{available: 1 << 30},
		Devices: fakeDevices{
			devices: map[string]string{
				"/":                 "system",
				"/Volumes/SanDisk":  "sandisk",
				"/Volumes/Elements": "elements",
			},
			identities: map[string]string{},
		},
		Clock:  &fakeClock{},
		Runner: runner,
	}
	return Config{Source: testSource, Destination: testDestination, RsyncBinary: "rsync"}, deps, runner
}

// journal records the order of the effects a real mirror performs, which is
// what makes "nothing is created or run before the approval" testable.
type journal struct {
	steps []string
}

func (j *journal) record(step string) {
	j.steps = append(j.steps, step)
}

type fakeApprover struct {
	journal *journal
	approve bool
	plans   []Plan

	// whileDeciding runs inside the prompt, which is where a drive gets
	// unplugged or a destination gains content in the real world.
	whileDeciding func()
}

func (a *fakeApprover) Approve(plan Plan) bool {
	a.journal.record("approve")
	a.plans = append(a.plans, plan)
	if a.whileDeciding != nil {
		a.whileDeciding()
	}
	return a.approve
}

// fakeLocker answers the destination lock without touching the filesystem. The
// real lock is exercised by the concurrency tests, which use osLocker itself.
type fakeLocker struct {
	err      error
	locked   []string
	released int
}

func (l *fakeLocker) Lock(destination string) (func(), error) {
	if l.err != nil {
		return nil, l.err
	}
	l.locked = append(l.locked, destination)
	return func() { l.released++ }, nil
}

// fakeRuleWriter keeps the deletion boundary in memory so a test can read the
// rules rsync would have been given.
type fakeRuleWriter struct {
	path    string
	rules   []byte
	written int
	removed int
	err     error
}

func (w *fakeRuleWriter) WriteRules(rules []byte) (string, func(), error) {
	if w.err != nil {
		return "", nil, w.err
	}
	w.rules = rules
	w.written++
	return w.path, func() { w.removed++ }, nil
}

// fakeBinder models a machine where a directory never moves: the name of a
// bound endpoint is the path it was bound from, unless a test gives it a name
// of its own to prove the transfer runs on what was bound rather than on the
// path. Like the real binder it refuses a path holding another directory than
// the one the preflight inspected, which is what a test expresses by changing
// the identity fakeDevices answers for that path.
type fakeBinder struct {
	devices  fakeDevices
	name     func(string) string
	err      error
	bound    []string
	released int
}

func (b *fakeBinder) Bind(path, identity string) (Bound, error) {
	if b.err != nil {
		return Bound{}, b.err
	}
	b.bound = append(b.bound, path)

	opened, err := b.devices.Identity(path)
	if err != nil {
		return Bound{}, err
	}
	if opened != identity {
		return Bound{}, fmt.Errorf("the mirror endpoint %s is the directory %s and not %s, which is the one the preflight inspected, so nothing was written", path, opened, identity)
	}

	name := path
	if b.name != nil {
		name = b.name(path)
	}
	return Bound{Path: name, Release: func() { b.released++ }}, nil
}

type fakeStreamer struct {
	journal  *journal
	calls    [][]string
	exitCode int
	err      error

	// cancel simulates a signal arriving while rsync is transferring.
	cancel context.CancelFunc

	// beforeStreaming runs at the instant the transfer starts, which is the
	// last moment another writer can add destination-only content the mirror
	// never walked.
	beforeStreaming func()
}

func (s *fakeStreamer) Stream(ctx context.Context, argv []string) (int, error) {
	s.journal.record("stream")
	if s.beforeStreaming != nil {
		s.beforeStreaming()
	}
	s.calls = append(s.calls, argv)
	if s.cancel != nil {
		s.cancel()
	}
	if ctx.Err() != nil {
		return InterruptedExitCode, nil
	}
	return s.exitCode, s.err
}

// execSetup is a healthy pair of mounted volumes plus an approver that says
// yes, an rsync that succeeds, and a record of the order everything happened
// in.
type execSetup struct {
	cfg      Config
	deps     Deps
	exec     ExecDeps
	runner   *fakeRunner
	approver *fakeApprover
	locker   *fakeLocker
	rules    *fakeRuleWriter
	binder   *fakeBinder
	streamer *fakeStreamer
	journal  *journal
	out      strings.Builder
}

func mirrorSetup() *execSetup {
	cfg, deps, runner := mountedSetup()
	shared := &journal{}
	setup := &execSetup{
		cfg:      cfg,
		deps:     deps,
		runner:   runner,
		approver: &fakeApprover{journal: shared, approve: true},
		locker:   &fakeLocker{},
		rules:    &fakeRuleWriter{path: "/tmp/goback-mirror-rules-test"},
		binder:   &fakeBinder{devices: deps.Devices.(fakeDevices)},
		streamer: &fakeStreamer{journal: shared},
		journal:  shared,
	}
	setup.exec = ExecDeps{
		Approver: setup.approver,
		Locker:   setup.locker,
		Rules:    setup.rules,
		Binder:   setup.binder,
		Streamer: setup.streamer,
	}
	return setup
}

func (s *execSetup) run(ctx context.Context) (Result, error) {
	return Mirror(ctx, s.cfg, s.deps, s.exec, &s.out)
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want one mentioning %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want it to mention %q", err, want)
	}
}

var errFake = errors.New("fake failure")
