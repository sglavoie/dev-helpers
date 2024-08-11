package editor

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func OpenFileWithEditor(filePath string) {
	args := args(defaultName(), filePath)
	err := execWithOutputRedirects(args[0], args[1:])
	cobra.CheckErr(err)
}

func defaultName() string {
	editor := viper.GetString("editor")
	if editor != "" {
		return editor
	}

	editor = os.Getenv("EDITOR")
	if editor == "" {
		log.Fatal("EDITOR environment variable not set")
	}
	return editor
}

func args(editor, tempFilePath string) []string {
	var args []string
	if editor == "code" {
		args = strings.Fields(editor)
		args = append(args, "--new-window", "--wait", tempFilePath)
	} else {
		args = strings.Fields(editor)
		args = append(args, tempFilePath)
	}

	return args
}

func execWithOutputRedirects(cmdName string, args []string) error {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	cobra.CheckErr(err)
	return nil
}
