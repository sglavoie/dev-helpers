// Package mirror implements the one-way media mirror: an exact copy of a
// configured source directory onto a configured destination directory on
// another volume. The package holds no global state and reads no
// configuration: every input is passed in as a Config and every effect goes
// through the interfaces in Deps, so a mirror can be exercised in tests
// without touching real volumes or running rsync.
package mirror

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// PartialDir is the hidden directory holding resumable partial data. rsync
// excludes a relative --partial-dir from the transfer on its own, so it is
// never mirrored and never deleted from the destination.
const PartialDir = ".goback-partial"

// Config describes a single mirror operation. Both endpoints are absolute
// paths to directories; RsyncBinary is the rsync to run, looked up in PATH
// when it is not itself a path.
type Config struct {
	Source      string
	Destination string
	RsyncBinary string
}

// FileSystem is the read-only view of the filesystem the mirror needs. It has
// no way to create anything, which is what keeps a dry run from writing.
type FileSystem interface {
	Stat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Writable(path string) error

	// Resolve returns the path with every symlink in it followed. Stat and
	// Device both follow symlinks silently, so this is the only way to see
	// where an endpoint really is. It is called on existing paths only.
	Resolve(path string) (string, error)
}

// Capacity reports the free space of the filesystem holding a path.
type Capacity interface {
	AvailableBytes(path string) (uint64, error)
}

// Devices identifies the filesystem a path lives on. Two paths with the same
// device are on the same mounted filesystem.
type Devices interface {
	Device(path string) (string, error)

	// Identity names the directory a path is rather than the place it sits
	// at: the volume it lives on and its inode number. A directory keeps its
	// identity while it exists and no other directory can hold it at the same
	// time, so this is what tells a validated endpoint apart from another
	// directory moved to the same path.
	Identity(path string) (string, error)
}

// Clock supplies the current time.
type Clock interface {
	Now() time.Time
}

// CommandResult is the captured outcome of a command. Output is captured
// rather than streamed because the mirror parses it.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes a command without a shell.
type Runner interface {
	Run(ctx context.Context, command Command) (CommandResult, error)
}

// Deps holds every effect a preflight performs. None of them writes.
type Deps struct {
	FS       FileSystem
	Capacity Capacity
	Devices  Devices
	Clock    Clock
	Runner   Runner
}

// Approver asks for the explicit approval of a real mirror. It is asked for
// every real mirror, whatever confirmExec says, because a mirror deletes
// whatever the destination holds that the source does not.
type Approver interface {
	Approve(plan Plan) bool
}

// ApproverFunc adapts a plain function to an Approver.
type ApproverFunc func(plan Plan) bool

// Approve implements Approver.
func (f ApproverFunc) Approve(plan Plan) bool { return f(plan) }

// Locker serializes the real mirrors of one destination. Two mirrors of the
// same destination each review their own plan and then write over each other,
// so only one of them may ever reach a transfer.
type Locker interface {
	// Lock takes the exclusive lock of a destination and returns the
	// function that gives it back. It never waits: a mirror stops at an
	// approval prompt for as long as nobody answers it, so a second mirror
	// fails immediately instead of queueing behind a question.
	//
	// The destination is the resolved one, so two configured paths that are
	// symlink aliases of one directory take the same lock.
	Lock(destination string) (release func(), err error)
}

// RuleWriter stores the deletion boundary of a transfer where rsync can read
// it. The rules go to a file rather than to arguments because a filename may
// contain a newline, which no single argument can carry as one rule.
type RuleWriter interface {
	// WriteRules stores the rules and returns the path rsync must read plus
	// the function that removes it. The file lives outside both endpoints,
	// since anything inside the destination is content the mirror deletes.
	WriteRules(rules []byte) (path string, remove func(), err error)
}

// Bound is an endpoint a transfer is bound to. Path names the directory that
// was opened rather than the place it was found at, so it keeps meaning that
// directory however the configured path is changed afterwards. Release gives
// the directory back once the transfer is over.
type Bound struct {
	Path    string
	Release func()
}

// Binder binds a validated endpoint to the directory it is at that moment.
// rsync resolves its own arguments when it starts, which is later than any
// check the mirror can make, so a path string is not enough: an endpoint
// replaced between the last check and rsync's own open would redirect the
// transfer, and only a name that survives the replacement prevents it.
type Binder interface {
	// Bind opens path and returns the name of the directory it opened. The
	// identity is the one the preflight inspected, and the open must land on
	// exactly that directory: an ordinary directory substituted at the path
	// for the instant of the binding is refused rather than bound, which a
	// comparison between the opened directory and the path alone cannot do.
	// It also refuses an endpoint reached through a symlink and one that is
	// not a directory.
	Bind(path, identity string) (Bound, error)
}

// Streamer runs a command with its output attached to the terminal instead of
// captured, and reports the exit code it finished with. Cancelling ctx
// terminates the command. A returned error means the command did not run to
// completion; an exit code alone is not an error.
type Streamer interface {
	Stream(ctx context.Context, command Command) (int, error)
}

// ExecDeps holds the effects only a real mirror performs. They are separate
// from Deps so that every write, and the approval that authorizes it, is
// visible in the signature of what needs them.
type ExecDeps struct {
	Approver Approver
	Locker   Locker
	Rules    RuleWriter
	Binder   Binder
	Streamer Streamer
}

// OSExecDeps returns the real-machine execution dependencies, writing the
// transfer's own output to the given writers.
func OSExecDeps(approver Approver, stdout, stderr io.Writer) ExecDeps {
	return ExecDeps{
		Approver: approver,
		Locker:   osLocker{dir: os.TempDir()},
		Rules:    osRuleWriter{dir: os.TempDir()},
		Binder:   osBinder{},
		Streamer: osStreamer{stdout: stdout, stderr: stderr},
	}
}

// lockPerm is the mode of a lock file. It carries no content and is only ever
// opened by its owner.
const lockPerm = 0o600

// osLocker locks a destination through flock on a file named after it. The
// lock file lives outside both endpoints, because anything inside the
// destination is content the mirror itself deletes. It is machine-local, which
// is the exclusion a single-user CLI can promise: it serializes the mirrors
// started on this machine under this user's temporary directory.
type osLocker struct {
	dir string
}

func (l osLocker) Lock(destination string) (func(), error) {
	sum := sha256.Sum256([]byte(destination))
	path := filepath.Join(l.dir, fmt.Sprintf("goback-mirror-%x.lock", sum[:8]))

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, lockPerm)
	if err != nil {
		return nil, fmt.Errorf("cannot open the mirror lock %s: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another goback mirror is already running for %s: wait for it to finish, since two mirrors of one destination would each delete what the other just wrote", destination)
		}
		return nil, fmt.Errorf("cannot lock the mirror destination %s through %s: %w", destination, path, err)
	}

	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

// osRuleWriter writes the deletion boundary to a throwaway file next to the
// lock. os.CreateTemp gives it a unique name and mode 0600, so two mirrors
// running at once never read each other's rules.
type osRuleWriter struct {
	dir string
}

func (w osRuleWriter) WriteRules(rules []byte) (string, func(), error) {
	file, err := os.CreateTemp(w.dir, "goback-mirror-rules-*")
	if err != nil {
		return "", nil, fmt.Errorf("cannot write the deletion rules of the mirror: %w", err)
	}

	remove := func() { os.Remove(file.Name()) }
	if _, err := file.Write(rules); err != nil {
		file.Close()
		remove()
		return "", nil, fmt.Errorf("cannot write the deletion rules of the mirror to %s: %w", file.Name(), err)
	}
	if err := file.Close(); err != nil {
		remove()
		return "", nil, fmt.Errorf("cannot write the deletion rules of the mirror to %s: %w", file.Name(), err)
	}
	return file.Name(), remove, nil
}

// volumeIdentityDir is the macOS namespace naming a directory by the volume it
// lives on and its inode number instead of by the path it was reached through.
// A name under it keeps meaning the same directory when the path is renamed,
// replaced by another directory, or replaced by a symlink, which is what binds
// a transfer to the endpoints that were validated.
const volumeIdentityDir = "/.vol"

// osBinder binds an endpoint by opening it and naming it by identity. The
// directory stays open until the transfer is over, so its inode cannot be
// reused by something else while rsync is reaching the endpoint through it.
type osBinder struct{}

func (osBinder) Bind(path, identity string) (Bound, error) {
	file, err := openDirectory(path)
	if err != nil {
		return Bound{}, err
	}
	bound, opened, err := boundName(file, path)
	if err != nil {
		file.Close()
		return Bound{}, err
	}

	// The open is the moment the transfer's endpoint is settled, so it is
	// also where the directory the preflight inspected has to be recognized.
	// An ordinary directory moved onto the path for the instant of the
	// binding and moved away again passes every path check made before and
	// after it, and only this comparison refuses it.
	if opened != identity {
		bound.Release()
		return Bound{}, fmt.Errorf("the mirror endpoint %s is the directory %s and not %s, which is the one the preflight inspected, so nothing was written: a directory substituted at that path, even only while it is opened, would make the transfer read from, write to, and delete in a directory nobody reviewed", path, opened, identity)
	}
	return bound, nil
}

// openDirectory opens a path as the directory it is at that instant. O_NOFOLLOW
// refuses an endpoint that became a symlink rather than following it, and
// O_DIRECTORY refuses one that is no longer a directory.
func openDirectory(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open the mirror endpoint %s, so nothing was written: %w", path, err)
	}
	return file, nil
}

// boundName names an opened directory by identity and reports the identity it
// holds. The name keeps meaning that directory however the path it was opened
// through is changed afterwards, and the fd stays open until the binding is
// released so the inode cannot be reused underneath it.
func boundName(file *os.File, path string) (Bound, string, error) {
	info, err := file.Stat()
	if err != nil {
		return Bound{}, "", fmt.Errorf("cannot identify the mirror endpoint %s, so nothing was written: %w", path, err)
	}
	opened, err := identityOf(info)
	if err != nil {
		return Bound{}, "", fmt.Errorf("cannot identify the mirror endpoint %s, so nothing was written: %w", path, err)
	}

	bound := volumeIdentityDir + "/" + opened
	if err := sameDirectory(bound, info); err != nil {
		return Bound{}, "", fmt.Errorf("the volume holding %s does not name its directories through %s, so the transfer cannot be bound to the directory that was validated and nothing was written: %w", path, volumeIdentityDir, err)
	}

	return Bound{Path: bound, Release: func() { file.Close() }}, opened, nil
}

// identityOf renders the volume and inode number of a directory as the name
// the binding and the preflight both compare and as the name /.vol addresses
// it by.
func identityOf(info fs.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("the filesystem does not report inode numbers")
	}
	return fmt.Sprintf("%d/%d", stat.Dev, stat.Ino), nil
}

// sameDirectory reports whether path is the very directory info describes
// rather than another one that happens to sit at the same place.
func sameDirectory(path string, info fs.FileInfo) error {
	other, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(other, info) {
		return fmt.Errorf("%s is a different directory", path)
	}
	return nil
}

// killGrace is how long rsync has to exit after the context was cancelled and
// it was sent SIGTERM, before it is killed. Whatever it was transferring stays
// in PartialDir, so the next mirror resumes it.
const killGrace = 10 * time.Second

type osStreamer struct {
	stdout io.Writer
	stderr io.Writer
}

func (s osStreamer) Stream(ctx context.Context, command Command) (int, error) {
	cmd, err := command.process(ctx)
	if err != nil {
		return 1, err
	}
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = killGrace

	err = cmd.Run()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}

// OSDeps returns the dependencies backed by the real machine.
func OSDeps() Deps {
	return Deps{
		FS:       osFileSystem{},
		Capacity: osCapacity{},
		Devices:  osDevices{},
		Clock:    osClock{},
		Runner:   osRunner{},
	}
}

type osFileSystem struct{}

func (osFileSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (osFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (osFileSystem) Writable(path string) error {
	return syscall.Access(path, unixWriteOK)
}

func (osFileSystem) Resolve(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// unixWriteOK is the W_OK mode of access(2).
const unixWriteOK = 0x2

type osCapacity struct{}

func (osCapacity) AvailableBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("cannot read free space of %s: %w", path, err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}

type osDevices struct{}

func (osDevices) Device(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("cannot identify the filesystem of %s", path)
	}
	return fmt.Sprintf("%d", stat.Dev), nil
}

func (osDevices) Identity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	identity, err := identityOf(info)
	if err != nil {
		return "", fmt.Errorf("cannot identify the directory %s: %w", path, err)
	}
	return identity, nil
}

type osClock struct{}

func (osClock) Now() time.Time {
	return time.Now()
}

type osRunner struct{}

func (osRunner) Run(ctx context.Context, command Command) (CommandResult, error) {
	cmd, err := command.process(ctx)
	if err != nil {
		return CommandResult{}, err
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}

	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case ctx.Err() != nil:
		return result, ctx.Err()
	case errors.As(err, &exitErr):
		result.ExitCode = exitErr.ExitCode()
	default:
		return result, err
	}
	return result, nil
}
