package restore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
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

func restore(ctx context.Context, plan Plan, copyFile func(context.Context, io.Writer, io.Reader) (int64, error)) error {
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

	files, err := inspect(plan.request)
	if err != nil {
		return err
	}
	defer files.close()
	if !plan.matches(files.plan) {
		return fmt.Errorf("restore paths or backup file changed since preview; run restore again")
	}
	stage, name, err := createStaging(files.dir)
	if err != nil {
		return err
	}
	defer stage.Close()
	defer unix.Unlinkat(int(files.dir.Fd()), name, 0)

	sourceHash := sha256.New()
	n, err := copyFile(ctx, io.MultiWriter(stage, sourceHash), files.source)
	if err != nil {
		return fmt.Errorf("copy backup file: %w", err)
	}
	if n != plan.Size {
		return fmt.Errorf("backup file changed while restoring (copied %d of %d bytes)", n, plan.Size)
	}
	// Read back both files: the recovered bytes must match the copied bytes,
	// and the backup must still contain those same bytes.
	for _, file := range []*os.File{stage, files.source} {
		if err := verifyContent(ctx, file, sourceHash.Sum(nil)); err != nil {
			return err
		}
	}
	info, err := files.source.Stat()
	if err != nil {
		return err
	}
	if !sameSource(plan.sourceInfo, info) {
		return fmt.Errorf("backup file changed while restoring; no recovered file published")
	}
	if err := stage.Chmod(plan.Mode); err != nil {
		return fmt.Errorf("preserve recovered file permissions: %w", err)
	}
	modified := unix.NsecToTimeval(plan.ModTime.UnixNano())
	if err := unix.Futimes(int(stage.Fd()), []unix.Timeval{modified, modified}); err != nil {
		return fmt.Errorf("preserve recovered file modification time: %w", err)
	}
	if err := stage.Sync(); err != nil {
		return fmt.Errorf("flush recovered file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Recheck configured paths after a possibly long copy. Publication itself
	// uses the bound directory, so a last-instant alias swap redirects nothing.
	current, err := inspect(plan.request)
	if err != nil {
		return err
	}
	defer current.close()
	if !plan.matches(current.plan) {
		return fmt.Errorf("restore paths or backup file changed while restoring; no recovered file published")
	}
	// A hard link publishes the staged inode atomically and fails if ANY entry
	// appeared at the target, including a dangling symlink. Rename would clobber it.
	if err := unix.Linkat(int(files.dir.Fd()), name, int(files.dir.Fd()), filepath.Base(plan.Destination), 0); err != nil {
		return fmt.Errorf("publish recovered file without overwriting: %w", err)
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
