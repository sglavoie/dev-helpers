package restore

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Open each component relative to an already-open directory. No component may
// become a symlink between inspection and open, and a renamed parent cannot
// redirect later operations to a different directory.
func openDirectory(path string) (*os.File, error) {
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	root := os.NewFile(uintptr(fd), "/")
	defer root.Close()
	return openAt(root, strings.TrimPrefix(path, "/"), true)
}

func openRelative(root *os.File, path string) (*os.File, error) {
	return openAt(root, path, false)
}

func openAt(root *os.File, path string, directory bool) (*os.File, error) {
	parent := root
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		if directory || i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		fd, err := unix.Openat(int(parent.Fd()), part, flags, 0)
		if parent != root {
			parent.Close()
		}
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", part, err)
		}
		parent = os.NewFile(uintptr(fd), part)
	}
	return parent, nil
}

func requireAbsent(dir *os.File, name string) error {
	var info unix.Stat_t
	err := unix.Fstatat(int(dir.Fd()), name, &info, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("refusing to overwrite existing recovery path %q", name)
	}
	return fmt.Errorf("inspect recovery path %q: %w", name, err)
}

func createStaging(dir *os.File) (*os.File, string, error) {
	for i := 0; i < 10; i++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := fmt.Sprintf(".goback-restore-%x", random)
		fd, err := unix.Openat(int(dir.Fd()), name, unix.O_CREAT|unix.O_EXCL|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return nil, "", fmt.Errorf("create recovery staging file: %w", err)
		}
		return os.NewFile(uintptr(fd), name), name, nil
	}
	return nil, "", fmt.Errorf("could not create a unique recovery staging file")
}
