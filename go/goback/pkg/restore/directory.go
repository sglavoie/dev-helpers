package restore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func restoreDirectory(ctx context.Context, plan Plan, files *opened, copyFile copier) (err error) {
	// Stage beside the final name. On macOS, moving a read-only directory to
	// another parent requires write access to that directory (to change "..").
	// Renaming within this parent lets us preserve even read-only root modes
	// before publication, rather than fixing permissions on a visible tree.
	stage, name, err := createStagingDirectory(files.dir)
	if err != nil {
		return err
	}
	defer stage.Close()
	published := false
	defer func() {
		if !published {
			if cleanupErr := removeStagingDirectory(files.dir, name, stage); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("remove restore staging directory: %w", cleanupErr))
			}
		}
	}()

	digests := make(map[string][]byte, plan.Files)
	for _, entry := range plan.entries[1:] {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.info.IsDir() {
			parent, err := openAt(stage, filepath.Dir(entry.path), true)
			if err != nil {
				return err
			}
			dir, err := makeDirectory(parent, filepath.Base(entry.path))
			parent.Close()
			if err != nil {
				return fmt.Errorf("stage directory %q: %w", entry.path, err)
			}
			dir.Close()
			continue
		}
		digest, err := stageFile(ctx, files.source, stage, entry, copyFile)
		if err != nil {
			return fmt.Errorf("restore %q: %w", entry.path, err)
		}
		digests[entry.path] = digest
	}

	// Reject unexpected staged entries as well as changed file bytes. Never
	// publish a partial tree or a file that was not part of the inspected plan.
	staged, err := scanTree(ctx, stage)
	if err != nil {
		return fmt.Errorf("inspect staged directory: %w", err)
	}
	if len(staged) != len(plan.entries) {
		return fmt.Errorf("staged directory contents changed; no recovered directory published")
	}
	for i, entry := range plan.entries {
		if entry.path != staged[i].path || entry.info.IsDir() != staged[i].info.IsDir() {
			return fmt.Errorf("staged directory contents changed; no recovered directory published")
		}
		if !entry.info.IsDir() {
			if err := finishTreeFile(ctx, files.source, stage, entry, digests[entry.path]); err != nil {
				return fmt.Errorf("verify %q: %w", entry.path, err)
			}
		}
	}
	// Apply directory metadata last, deepest first: creating children changes
	// parent mtimes, and original directory modes may remove our write access.
	for i := len(plan.entries) - 1; i >= 0; i-- {
		entry := plan.entries[i]
		if !entry.info.IsDir() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		dir, err := openAt(stage, entry.path, true)
		if err != nil {
			return err
		}
		err = preserveMetadata(dir, entry.info)
		dir.Close()
		if err != nil {
			return fmt.Errorf("preserve directory %q: %w", entry.path, err)
		}
	}
	if err := recheck(ctx, plan); err != nil {
		return err
	}
	// Ordinary rename can replace an empty directory created during the copy.
	// Use the platform's atomic no-replace operation, with no unsafe fallback.
	if err := publishDirectory(int(files.dir.Fd()), name, int(files.dir.Fd()), filepath.Base(plan.Destination)); err != nil {
		return fmt.Errorf("publish recovered directory without overwriting (filesystem must support exclusive rename): %w", err)
	}
	published = true
	return nil
}

func stageFile(ctx context.Context, sourceRoot, stageRoot *os.File, entry treeEntry, copyFile copier) ([]byte, error) {
	source, err := openExpected(sourceRoot, entry)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	parent, err := openAt(stageRoot, filepath.Dir(entry.path), true)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	fd, err := unix.Openat(int(parent.Fd()), filepath.Base(entry.path), unix.O_CREAT|unix.O_EXCL|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	stage := os.NewFile(uintptr(fd), entry.path)
	defer stage.Close()
	return copyContent(ctx, source, stage, entry.info, copyFile)
}

func finishTreeFile(ctx context.Context, sourceRoot, stageRoot *os.File, entry treeEntry, digest []byte) error {
	source, err := openExpected(sourceRoot, entry)
	if err != nil {
		return err
	}
	defer source.Close()
	stage, err := openRelative(stageRoot, entry.path)
	if err != nil {
		return err
	}
	defer stage.Close()
	return finishFile(ctx, source, stage, entry.info, digest)
}
