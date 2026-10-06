package config

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockSave(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	var overlap windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlap); err != nil {
		f.Close()
		return nil, fmt.Errorf("GoTime is being saved by another process; retry: %w", err)
	}
	return func() { f.Close() }, nil
}
