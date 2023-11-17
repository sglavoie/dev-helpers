package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/viper"
)

func GetCommandWithEditor() (string, error) {
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

	input, err := OpenEditorAndGetInput(tempFile.Name())
	if err != nil {
		return "", err
	}

	return input, nil
}

func GetParsedCommand(input string) (models.Command, error) {
	fieldSeparator := getFieldSeparator()
	lines := strings.Split(input, "\n")
	parsedCmd := models.Command{
		Tags: []string{},
	}

	currField := ""
	for _, line := range lines {
		if strings.HasPrefix(line, fieldSeparator) {
			switch line {
			case strings.TrimSpace(getHeaderLineFromTemplate(fieldSeparator, "name:")):
				currField = "name"
			case strings.TrimSpace(getHeaderLineFromTemplate(fieldSeparator, "description:")):
				currField = "description"
			case strings.TrimSpace(getHeaderLineFromTemplate(fieldSeparator, "tags:")):
				currField = "tags"
			case strings.TrimSpace(getHeaderLineFromTemplate(fieldSeparator, "command:")):
				currField = "command"
			}
			continue
		}
		parsedCmd = getUpdatedParsedCommandField(parsedCmd, currField, line)
		continue
	}

	parsedCmd = getTrimmedParsedCommandFields(parsedCmd)

	return parsedCmd, nil
}

func OpenEditorAndGetInput(tempFilePath string) (string, error) {
	args := getArgsForEditor(getDefaultEditorName(), tempFilePath)
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

func getDefaultEditorName() string {
	editor := viper.GetString("editor")
	if editor != "" {
		return editor
	}

	editor = os.Getenv("EDITOR")
	if editor == "" {
		clihelpers.FatalExit("No editor specified")
	}
	return editor
}

func getFieldSeparator() string {
	return "+~~~~+~~~~+"
}

func getHeaderLineFromTemplate(s, title string) string {
	return fmt.Sprintf("%s %s\n", s, title)
}

func getStringToWriteToTempFile(command models.Command) string {
	fieldSeparator := getFieldSeparator()
	editorStr := getHeaderLineFromTemplate(fieldSeparator, "do not edit these line separators")
	editorStr += getHeaderLineFromTemplate(fieldSeparator, "fields can span multiple lines")
	editorStr += getHeaderLineFromTemplate(fieldSeparator, "name:")
	editorStr += fmt.Sprintf(command.Name) + "\n"
	editorStr += getHeaderLineFromTemplate(fieldSeparator, "description:")
	editorStr += fmt.Sprintf(command.Description) + "\n"
	editorStr += getHeaderLineFromTemplate(fieldSeparator, "tags:")
	for _, tag := range command.Tags {
		editorStr += fmt.Sprintf("%s\n", tag)
	}
	if len(command.Tags) == 0 {
		editorStr += "\n"
	}
	editorStr += getHeaderLineFromTemplate(fieldSeparator, "command:")
	editorStr += fmt.Sprintf(command.Command) + "\n" + fieldSeparator

	return editorStr
}

func getUpdatedCommandFields(command models.Command) (string, error) {
	tempFile, err := os.CreateTemp("", "shellshelf-*.tmp")
	if err != nil {
		return "", errors.New("failed to create temp file: " + err.Error())
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			fmt.Println("Failed to remove temp file:", err)
			return
		}
	}(tempFile.Name())

	strCmd := getStringToWriteToTempFile(command)
	if _, err := tempFile.WriteString(strCmd); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %v\n", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %v\n", err)
	}

	args := getArgsForEditor(getDefaultEditorName(), tempFile.Name())
	fmt.Println("args:", args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to open editor: %v\n", err)
	}

	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %v\n", err)
	}

	return string(content), nil
}

func getTrimmedParsedCommandFields(cmd models.Command) models.Command {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Description = strings.TrimSpace(cmd.Description)
	cmd.Command = strings.TrimSpace(cmd.Command)
	for i, tag := range cmd.Tags {
		cmd.Tags[i] = strings.TrimSpace(tag)
	}
	return cmd
}

func getUpdatedParsedCommandField(cmd models.Command, field, value string) models.Command {
	// Fields can be multi-line, so append to existing content
	switch field {
	case "name":
		cmd.Name += "\n" + strings.TrimSpace(value)
	case "description":
		cmd.Description += "\n" + strings.TrimSpace(value)
	case "tags":
		tags := strings.Split(value, ",")
		for _, tag := range tags {
			if tag == "" {
				continue
			}
			cmd.Tags = append(cmd.Tags, strings.TrimSpace(tag))
		}
	case "command":
		cmd.Command += "\n" + strings.TrimSpace(value)
	}
	return cmd
}
