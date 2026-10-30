package restore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"golang.org/x/sys/unix"
)

// Restore copies and checks content before publishing. It retains ordinary Unix
// permissions and modification time, and records no backup history. The explicit
// destination and refusal to overwrite make interactive confirmation unnecessary.
func Restore(ctx context.Context, plan Plan) error {
	return restore(ctx, plan, copyContext)
}

func restore(ctx context.Context, plan Plan, copyFile copier) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if plan.rootInfo == nil || plan.dirInfo == nil || plan.sourceInfo == nil {
		return fmt.Errorf("restore requires a prepared plan")
	}
	_, releaseBackup, err := destinationlock.Acquire(ctx, plan.root)
	if err != nil {
		return err
	}
	defer releaseBackup()
	_, releaseRecovery, err := destinationlock.Acquire(ctx, plan.directory)
	if err != nil {
		return err
	}
	defer releaseRecovery()

	files, err := inspect(ctx, plan.request)
	if err != nil {
		return err
	}
	defer files.close()
	if !plan.matches(files.plan) {
		return fmt.Errorf("restore paths or backup contents changed since preview; run restore again")
	}
	if plan.IsDir {
		return restoreDirectory(ctx, plan, files, copyFile)
	}
	stage, name, err := createStaging(files.dir)
	if err != nil {
		return err
	}
	defer stage.Close()
	defer unix.Unlinkat(int(files.dir.Fd()), name, 0)

	digest, err := copyContent(ctx, files.source, stage, plan.sourceInfo, copyFile)
	if err != nil {
		return err
	}
	if err := finishFile(ctx, files.source, stage, plan.sourceInfo, digest); err != nil {
		return err
	}
	if err := recheck(ctx, plan); err != nil {
		return err
	}
	// A hard link publishes the staged inode atomically and fails if ANY entry
	// appeared at the target, including a dangling symlink. Rename would clobber it.
	if err := unix.Linkat(int(files.dir.Fd()), name, int(files.dir.Fd()), filepath.Base(plan.Destination), 0); err != nil {
		return fmt.Errorf("publish recovered file without overwriting: %w", err)
	}
	return nil
}

func recheck(ctx context.Context, plan Plan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Recheck configured paths after a possibly long copy. Publication itself
	// uses the bound directory, so a last-instant alias swap redirects nothing.
	current, err := inspect(ctx, plan.request)
	if err != nil {
		return err
	}
	defer current.close()
	if !plan.matches(current.plan) {
		return fmt.Errorf("restore paths or backup contents changed while restoring; no recovered entry published")
	}
	return ctx.Err()
}

type copier func(context.Context, io.Writer, io.Reader) (int64, error)

func copyContent(ctx context.Context, source, stage *os.File, info fs.FileInfo, copyFile copier) ([]byte, error) {
	hash := sha256.New()
	n, err := copyFile(ctx, io.MultiWriter(stage, hash), source)
	if err != nil {
		return nil, fmt.Errorf("copy backup file: %w", err)
	}
	if n != info.Size() {
		return nil, fmt.Errorf("backup file changed while restoring (copied %d of %d bytes)", n, info.Size())
	}
	return hash.Sum(nil), nil
}

func finishFile(ctx context.Context, source, stage *os.File, expected fs.FileInfo, digest []byte) error {
	// Read both files back after copying: the recovered and stored bytes must
	// still match the bytes we copied. Directory restores defer this until all
	// files have been copied, so later I/O cannot hide corruption of an early file.
	for _, file := range []*os.File{stage, source} {
		if err := verifyContent(ctx, file, digest); err != nil {
			return err
		}
	}
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if !sameSource(expected, info) {
		return fmt.Errorf("backup file changed while restoring; no recovered file published")
	}
	return preserveMetadata(stage, expected)
}

func preserveMetadata(file *os.File, info fs.FileInfo) error {
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("preserve recovered permissions: %w", err)
	}
	modified := unix.NsecToTimeval(info.ModTime().UnixNano())
	if err := unix.Futimes(int(file.Fd()), []unix.Timeval{modified, modified}); err != nil {
		return fmt.Errorf("preserve recovered modification time: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("flush recovered entry: %w", err)
	}
	return nil
}

func verifyContent(ctx context.Context, file *os.File, expected []byte) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := copyContext(ctx, hash, file); err != nil {
		return fmt.Errorf("verify file content: %w", err)
	}
	if !bytes.Equal(expected, hash.Sum(nil)) {
		return fmt.Errorf("content verification failed; no recovered file published")
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func copyContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, contextReader{ctx, src})
}
