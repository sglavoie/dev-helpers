package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func GetCommandWithEditor(editor string) (string, error) {

	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			return "", errors.New("no editor specified")
		}
	}

	tempFile, err := os.CreateTemp("", "shellshelf-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			return
		}
	}(tempFile.Name())

	input, err := OpenEditorAndGetInput(editor, tempFile.Name())
	if err != nil {
		return "", err
	}

	return input, nil
}

func OpenEditorAndGetInput(editor, tempFilePath string) (string, error) {
	args := getArgsForEditor(editor, tempFilePath)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to open editor: %v", err)
	}

	// Read the content of the temporary file
	content, err := os.ReadFile(tempFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %v", err)
	}

	return string(content), nil
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
