package osutils

import (
	"os"
	"os/exec"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
)

func ExecShellCommand(command string) {
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		clihelpers.FatalExit(err.Error())
	}
}

func ExecWithOutputRedirects(cmdName string, args []string) error {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		clihelpers.FatalExit("Error running '%v' with '%v': %v", cmdName, args, err)
	}

	return nil
}
