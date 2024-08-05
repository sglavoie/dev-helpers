package editor

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/exec"
	"strings"
)

func OpenFileWithEditor(filePath string) {
	args := getArgsForEditor(getDefaultEditorName(), filePath)
	err := execWithOutputRedirects(args[0], args[1:])
	if err != nil {
		cobra.CheckErr(err)
	}
}

func getDefaultEditorName() string {
	editor := viper.GetString("editor")
	if editor != "" {
		return editor
	}

	editor = os.Getenv("EDITOR")
	if editor == "" {
		cobra.CheckErr("EDITOR environment variable not set")
	}
	return editor
}

func getArgsForEditor(editor, tempFilePath string) []string {
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
	if err != nil {
		cobra.CheckErr(err)
	}

	return nil
}
