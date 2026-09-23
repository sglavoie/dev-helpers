package eject

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Runner executes diskutil without a shell. It returns the combined output
// whether the command succeeded or not, because a failure is only actionable
// with what diskutil said about it.
type Runner interface {
	Run(argv []string) (string, error)
}

// Mounts reports whether a volume mount point is currently available.
type Mounts interface {
	Mounted(volume string) bool
}

// Deps holds every effect ejecting performs, so a test never reaches diskutil
// and never depends on a drive being plugged in.
type Deps struct {
	Runner Runner
	Mounts Mounts
}

// OSDeps returns the dependencies backed by the real machine.
func OSDeps() Deps {
	return Deps{Runner: osRunner{}, Mounts: osMounts{lstat: os.Lstat, device: statDevice}}
}

type osRunner struct{}

func (osRunner) Run(argv []string) (string, error) {
	output, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	return string(output), err
}

// osMounts detects a mount point by its filesystem: a volume is mounted only
// when its root is a real directory, not a symlink, on a different device than
// the directory holding it. That rules out the empty directory a drive can
// leave behind under /Volumes after an unclean unmount.
type osMounts struct {
	lstat  func(string) (fs.FileInfo, error)
	device func(fs.FileInfo) (uint64, bool)
}

func (m osMounts) Mounted(volume string) bool {
	info, err := m.lstat(volume)
	if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	parent, err := m.lstat(filepath.Dir(volume))
	if err != nil {
		return false
	}

	volumeDevice, ok := m.device(info)
	if !ok {
		return false
	}
	parentDevice, ok := m.device(parent)
	return ok && volumeDevice != parentDevice
}

// statDevice returns the device a file lives on, and false when the platform
// provides no such metadata.
func statDevice(info fs.FileInfo) (uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, false
	}
	return uint64(stat.Dev), true
}
