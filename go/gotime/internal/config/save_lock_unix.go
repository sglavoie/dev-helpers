//go:build unix

package config

import (
	"fmt"
	"os"
	"syscall"
)

// Lock only the compare-and-replace, never the time spent in an editor.
// Keep the lock file: unlinking it would let two writers lock different inodes.
func lockSave(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("GoTime is being saved by another process; retry: %w", err)
	}
	return func() { f.Close() }, nil
}
