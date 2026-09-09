package mirror

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Command describes both the arguments and working directory of a process.
// Dir is empty when the child should inherit the caller's working directory.
type Command struct {
	Argv []string
	Dir  string
}

// String includes the working directory because a relative destination such
// as "." is meaningful only together with the directory the child starts in.
func (c Command) String() string {
	if c.Dir == "" {
		return FormatArgv(c.Argv)
	}
	return fmt.Sprintf("[working directory: %q] %s", c.Dir, FormatArgv(c.Argv))
}

func (c Command) process(ctx context.Context) (*exec.Cmd, error) {
	if len(c.Argv) == 0 {
		return nil, fmt.Errorf("cannot execute an empty command")
	}
	// Resolve against the caller's directory before assigning Cmd.Dir. In
	// particular, ./rsync must not become relative to the backup destination.
	program, err := exec.LookPath(c.Argv[0])
	if err != nil {
		return nil, fmt.Errorf("cannot find executable %q: %w", c.Argv[0], err)
	}
	program, err = filepath.Abs(program)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve executable %q: %w", c.Argv[0], err)
	}
	cmd := exec.CommandContext(ctx, program, c.Argv[1:]...)
	cmd.Dir = c.Dir
	return cmd, nil
}
