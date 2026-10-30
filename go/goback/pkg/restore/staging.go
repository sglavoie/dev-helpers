package restore

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func makeDirectory(parent *os.File, name string) (*os.File, error) {
	if err := unix.Mkdirat(int(parent.Fd()), name, 0700); err != nil {
		return nil, err
	}
	return openAt(parent, name, true)
}

func createStagingDirectory(parent *os.File) (*os.File, string, error) {
	for i := 0; i < 10; i++ {
		name, err := stagingName()
		if err != nil {
			return nil, "", err
		}
		dir, err := makeDirectory(parent, name)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			// mkdir may have succeeded before opening the new directory failed.
			_ = unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
			return nil, "", fmt.Errorf("create recovery staging directory: %w", err)
		}
		return dir, name, nil
	}
	return nil, "", fmt.Errorf("could not create a unique recovery staging directory")
}

func removeStagingDirectory(parent *os.File, name string, dir *os.File) error {
	if err := dir.Chmod(0700); err != nil {
		return err
	}
	// The staging root may already have been scanned during verification.
	if _, err := dir.Seek(0, 0); err != nil {
		return err
	}
	if err := removeContents(dir); err != nil {
		return err
	}
	return unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
}

// Clean through the bound directory even if --to was renamed during the copy.
// Do not follow symlinks; make directories traversable again if their restored
// permissions prevent cleanup. This only touches our private staging tree.
func removeContents(dir *os.File) error {
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		var stat unix.Stat_t
		if err := unix.Fstatat(int(dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
		flags := 0
		if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
			if err := unix.Fchmodat(int(dir.Fd()), name, 0700, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				return err
			}
			child, err := openAt(dir, name, true)
			if err != nil {
				return err
			}
			err = removeContents(child)
			child.Close()
			if err != nil {
				return err
			}
			flags = unix.AT_REMOVEDIR
		}
		if err := unix.Unlinkat(int(dir.Fd()), name, flags); err != nil {
			return err
		}
	}
	return nil
}
