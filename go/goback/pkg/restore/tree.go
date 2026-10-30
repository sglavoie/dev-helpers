package restore

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sys/unix"
)

type treeEntry struct {
	path string // Relative to the selected directory; "." names its root.
	info fs.FileInfo
}

// Walk through directory descriptors, never following symlinks. Keep metadata,
// not open files, so the number of descriptors depends only on directory depth.
func scanTree(ctx context.Context, root *os.File) ([]treeEntry, error) {
	var entries []treeEntry
	var visit func(*os.File, string) error
	visit = func(dir *os.File, path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		before, err := dir.Stat()
		if err != nil {
			return err
		}
		entries = append(entries, treeEntry{path, before})
		names, err := dir.Readdirnames(-1)
		if err != nil {
			return fmt.Errorf("list %q: %w", path, err)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return err
			}
			childPath := filepath.Join(path, name)
			var stat unix.Stat_t
			if err := unix.Fstatat(int(dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				return fmt.Errorf("inspect %q: %w", childPath, err)
			}
			if kind := stat.Mode & unix.S_IFMT; kind != unix.S_IFREG && kind != unix.S_IFDIR {
				return fmt.Errorf("unsupported backup entry %q: only regular files and directories are supported; symlinks and special files are not supported", childPath)
			}
			child, err := openRelative(dir, name)
			if err != nil {
				return fmt.Errorf("open %q (symlinks are not supported): %w", childPath, err)
			}
			err = func() error {
				defer child.Close()
				var openedStat unix.Stat_t
				if err := unix.Fstat(int(child.Fd()), &openedStat); err != nil {
					return err
				}
				if stat.Dev != openedStat.Dev || stat.Ino != openedStat.Ino || stat.Mode != openedStat.Mode {
					return fmt.Errorf("backup entry %q changed during inspection", childPath)
				}
				info, err := child.Stat()
				if err != nil {
					return err
				}
				if info.IsDir() {
					return visit(child, childPath)
				}
				entries = append(entries, treeEntry{childPath, info})
				return nil
			}()
			if err != nil {
				return err
			}
		}
		after, err := dir.Stat()
		if err != nil {
			return err
		}
		if !sameSource(before, after) {
			return fmt.Errorf("backup directory %q changed during inspection", path)
		}
		return nil
	}
	if err := visit(root, "."); err != nil {
		return nil, err
	}
	return entries, nil
}

func sameTree(a, b []treeEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i, entry := range a {
		if entry.path != b[i].path || !sameSource(entry.info, b[i].info) {
			return false
		}
	}
	return true
}

func openExpected(root *os.File, entry treeEntry) (*os.File, error) {
	file, err := openRelative(root, entry.path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", entry.path, err)
	}
	info, err := file.Stat()
	if err != nil || !sameSource(entry.info, info) {
		file.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("backup entry %q changed while restoring", entry.path)
	}
	return file, nil
}
