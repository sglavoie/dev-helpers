// Package restore recovers files and directory trees into an existing recovery directory.
// Planning only reads; execution publishes a verified copy without overwriting.
package restore

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
)

// Request.Path is relative to BackupRoot/BackupType, not to the live source.
// Destination is an existing directory; the recovered entry keeps its basename.
type Request struct {
	BackupRoot  string
	BackupType  string
	Path        string
	Destination string
	Recursive   bool
}

// Plan describes the exact copy that was inspected. Its private identities are
// compared again under the backup lock before execution.
type Plan struct {
	Source      string
	Destination string
	Size        int64
	Mode        fs.FileMode
	ModTime     time.Time
	IsDir       bool
	Files       int
	Directories int
	request     Request
	root        string
	directory   string
	rootInfo    fs.FileInfo
	dirInfo     fs.FileInfo
	sourceInfo  fs.FileInfo
	entries     []treeEntry
}

// Prepare validates a restore without creating directories, locks, or history.
func Prepare(req Request) (Plan, error) {
	return PrepareContext(context.Background(), req)
}

// PrepareContext also allows cancellation while inspecting a large directory tree.
func PrepareContext(ctx context.Context, req Request) (Plan, error) {
	files, err := inspect(ctx, req)
	if err != nil {
		return Plan{}, err
	}
	defer files.close()
	return files.plan, nil
}

type opened struct {
	plan              Plan
	root, source, dir *os.File
}

func (f *opened) close() {
	for _, file := range []*os.File{f.source, f.dir, f.root} {
		if file != nil {
			file.Close()
		}
	}
}

func inspect(ctx context.Context, req Request) (_ *opened, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch req.BackupType {
	case "daily", "weekly", "monthly":
	default:
		return nil, fmt.Errorf("backup type must be daily, weekly, or monthly")
	}
	if !filepath.IsLocal(req.Path) || filepath.Clean(req.Path) == "." {
		return nil, fmt.Errorf("file must be a relative path inside the %s backup", req.BackupType)
	}
	for _, part := range strings.Split(req.Path, string(filepath.Separator)) {
		if part == ".." {
			return nil, fmt.Errorf("file path must not contain '..'")
		}
	}
	if req.BackupRoot == "" {
		return nil, fmt.Errorf("backup destination is not configured")
	}
	if req.Destination == "" {
		return nil, fmt.Errorf("--to must name an existing recovery directory")
	}
	f := &opened{}
	defer func() {
		if err != nil {
			f.close()
		}
	}()
	f.plan.request = req
	f.plan.root, err = resolveDirectory(req.BackupRoot)
	if err != nil {
		return nil, fmt.Errorf("backup destination: %w", err)
	}
	f.plan.directory, err = resolveDirectory(req.Destination)
	if err != nil {
		return nil, fmt.Errorf("--to must name an existing recovery directory: %w", err)
	}
	if destinationlock.Within(f.plan.root, f.plan.directory) || destinationlock.Within(f.plan.directory, f.plan.root) {
		return nil, fmt.Errorf("recovery directory must not overlap the backup destination %s", f.plan.root)
	}
	f.root, err = openDirectory(f.plan.root)
	if err != nil {
		return nil, err
	}
	f.dir, err = openDirectory(f.plan.directory)
	if err != nil {
		return nil, err
	}
	f.plan.rootInfo, err = f.root.Stat()
	if err != nil {
		return nil, err
	}
	f.plan.dirInfo, err = f.dir.Stat()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(req.BackupType, req.Path)
	f.plan.Source = filepath.Join(f.plan.root, path)
	f.source, err = openRelative(f.root, path)
	if err != nil {
		return nil, fmt.Errorf("open backup file %q (symlinks are not supported): %w", f.plan.Source, err)
	}
	f.plan.sourceInfo, err = f.source.Stat()
	if err != nil {
		return nil, err
	}
	if f.plan.sourceInfo.IsDir() && !req.Recursive {
		return nil, fmt.Errorf("backup path %q must be a regular file; use --recursive to recover a directory", f.plan.Source)
	}
	if !f.plan.sourceInfo.Mode().IsRegular() && !f.plan.sourceInfo.IsDir() {
		return nil, fmt.Errorf("backup path %q must be a regular file", f.plan.Source)
	}
	f.plan.Destination = filepath.Join(f.plan.directory, filepath.Base(req.Path))
	if err := requireAbsent(f.dir, filepath.Base(req.Path)); err != nil {
		return nil, err
	}
	f.plan.Size = f.plan.sourceInfo.Size()
	f.plan.Mode = f.plan.sourceInfo.Mode().Perm()
	f.plan.ModTime = f.plan.sourceInfo.ModTime()
	f.plan.IsDir = f.plan.sourceInfo.IsDir()
	f.plan.Files = 1
	if f.plan.IsDir {
		f.plan.entries, err = scanTree(ctx, f.source)
		if err != nil {
			return nil, fmt.Errorf("inspect backup directory: %w", err)
		}
		f.plan.Size, f.plan.Files = 0, 0
		for _, entry := range f.plan.entries {
			if entry.info.IsDir() {
				f.plan.Directories++
			} else {
				f.plan.Files++
				f.plan.Size += entry.info.Size()
			}
		}
	}
	return f, nil
}

func resolveDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", path)
	}
	if err := eject.ValidateMountedPath(absolute, resolved, eject.OSDeps().Mounts); err != nil {
		return "", err
	}
	return resolved, nil
}

func (p Plan) matches(other Plan) bool {
	return p.Source == other.Source && p.Destination == other.Destination &&
		os.SameFile(p.rootInfo, other.rootInfo) && os.SameFile(p.dirInfo, other.dirInfo) &&
		sameSource(p.sourceInfo, other.sourceInfo) && sameTree(p.entries, other.entries)
}

func sameSource(before, after fs.FileInfo) bool {
	return os.SameFile(before, after) && before.Size() == after.Size() &&
		before.Mode() == after.Mode() && before.ModTime().Equal(after.ModTime())
}
