package eject

import (
	"os"
	"os/exec"
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
	return Deps{Runner: osRunner{}, Mounts: osMounts{}}
}

type osRunner struct{}

func (osRunner) Run(argv []string) (string, error) {
	output, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	return string(output), err
}

type osMounts struct{}

func (osMounts) Mounted(volume string) bool {
	info, err := os.Stat(volume)
	return err == nil && info.IsDir()
}
