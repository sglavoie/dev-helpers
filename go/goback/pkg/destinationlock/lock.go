// Package destinationlock coordinates goback writers on this machine.
package destinationlock

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

type contextKey struct{}
type lease struct {
	path string
	info os.FileInfo
}

// Acquire locks an existing directory, resolving aliases. A context holding the
// same lock can be reused by sequential steps until its release function runs.
// It must not be shared with concurrent writers.
func Acquire(ctx context.Context, path string) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ctx, nil, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return ctx, nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return ctx, nil, err
	}
	if !info.IsDir() {
		return ctx, nil, fmt.Errorf("lock destination is not a directory: %s", path)
	}
	if held, ok := ctx.Value(contextKey{}).(lease); ok {
		if held.path != resolved || !os.SameFile(held.info, info) {
			return ctx, nil, fmt.Errorf("destination changed while locked: %s", path)
		}
		return ctx, func() {}, nil
	}
	release, err := LockResolved(os.TempDir(), resolved)
	if err != nil {
		return ctx, nil, err
	}
	return context.WithValue(ctx, contextKey{}, lease{resolved, info}), release, nil
}

// LockResolved accepts an already canonical absolute path. Shared ancestor
// locks and an exclusive leaf lock exclude overlapping destinations while
// allowing siblings to run independently. Files stay outside backup content
// and are never unlinked, so waiting processes cannot lock a different inode.
func LockResolved(dir, destination string) (func(), error) {
	if !filepath.IsAbs(destination) {
		return nil, fmt.Errorf("lock destination must be absolute: %s", destination)
	}
	destination = filepath.Clean(destination)
	paths := []string{destination}
	for p := filepath.Dir(destination); p != paths[len(paths)-1]; p = filepath.Dir(p) {
		paths = append(paths, p)
	}
	var files []*os.File
	var once sync.Once
	release := func() {
		once.Do(func() {
			for i := len(files) - 1; i >= 0; i-- {
				_ = syscall.Flock(int(files[i].Fd()), syscall.LOCK_UN)
				_ = files[i].Close()
			}
		})
	}
	for i := len(paths) - 1; i >= 0; i-- {
		sum := sha256.Sum256([]byte(paths[i]))
		name := filepath.Join(dir, fmt.Sprintf("goback-destination-%x.lock", sum[:16]))
		file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			release()
			return nil, fmt.Errorf("open destination lock: %w", err)
		}
		mode := syscall.LOCK_SH
		if i == 0 {
			mode = syscall.LOCK_EX
		}
		if err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB); err != nil {
			_ = file.Close()
			release()
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return nil, fmt.Errorf("another goback operation is already running for %s or an overlapping destination", destination)
			}
			return nil, fmt.Errorf("lock %s: %w", destination, err)
		}
		files = append(files, file)
	}
	return release, nil
}

// Within reports whether a resolved endpoint stays under its locked root.
func Within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
