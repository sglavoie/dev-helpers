package buildcmd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func compareVerificationFile(ctx context.Context, source, destination string) (string, error) {
	sourceHash, err := verificationChecksum(ctx, source)
	if err != nil {
		return "unreadable", fmt.Errorf("read source: %w", err)
	}
	info, err := os.Lstat(destination)
	if os.IsNotExist(err) || errors.Is(err, unix.ENOTDIR) {
		return "missing", nil
	}
	if err != nil {
		return "unreadable", fmt.Errorf("inspect backup: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "different", nil
	}
	backupHash, err := verificationChecksum(ctx, destination)
	if err != nil {
		return "unreadable", fmt.Errorf("read backup: %w", err)
	}
	if sourceHash != backupHash {
		return "different", nil
	}
	return "", nil
}

func verificationChecksum(ctx context.Context, path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	before, err := os.Lstat(path)
	if err != nil {
		return digest, err
	}
	if !before.Mode().IsRegular() {
		return digest, fmt.Errorf("entry is no longer a regular file: %s", path)
	}
	// Do not follow a file replaced by a symlink, or block if it becomes a FIFO.
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return digest, fmt.Errorf("open %s: %w", path, err)
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return digest, err
	}
	if !sameVerificationFile(before, opened) {
		return digest, fmt.Errorf("file changed before reading: %s", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, verificationReader{ctx, file}); err != nil {
		return digest, err
	}
	after, err := file.Stat()
	if err != nil {
		return digest, err
	}
	current, err := os.Lstat(path)
	if err != nil {
		return digest, err
	}
	if !sameVerificationFile(before, after) || !sameVerificationFile(before, current) {
		return digest, fmt.Errorf("file changed while reading: %s", path)
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func sameVerificationFile(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

type verificationReader struct {
	ctx  context.Context
	file *os.File
}

func (r verificationReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.file.Read(p)
}
