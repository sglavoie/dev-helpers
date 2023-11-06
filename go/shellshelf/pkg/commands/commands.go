package commands

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Add(cmds map[string]models.Command, cmd models.Command) (map[string]models.Command, error) {
	maxID, err := GetMaxID(cmds)
	if err != nil {
		return cmds, err
	}

	cmds[strconv.Itoa(maxID+1)] = cmd

	return cmds, nil
}

func Decode(encodedCmd string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCmd)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}

func Encode(cmdStr string) string {
	return base64.StdEncoding.EncodeToString([]byte(cmdStr))
}

func GetFlagString(cmd *cobra.Command, flagName string) (string, error) {
	value, err := cmd.Flags().GetString(flagName)
	if err != nil {
		return "", fmt.Errorf("error retrieving %s: %v", flagName, err)
	}
	return value, nil
}

func GetFlagStringSlice(cmd *cobra.Command, flagName string) ([]string, error) {
	value, err := cmd.Flags().GetStringSlice(flagName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving %s: %v", flagName, err)
	}
	return value, nil
}

func GetMaxID(commands map[string]models.Command) (int, error) {
	maxID := 0
	for idStr := range commands {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return 0, fmt.Errorf("failed to parse ID: %v", err)
		}
		if id > maxID {
			maxID = id
		}
	}
	return maxID, nil
}

func Load() (map[string]models.Command, error) {
	if !viper.IsSet("commands") {
		return nil, errors.New("'commands' key not found in config")
	}

	var commands map[string]models.Command
	err := viper.UnmarshalKey("commands", &commands)
	if err != nil {
		return nil, err
	}

	return commands, nil
}

func Save(commands map[string]models.Command) error {
	viper.Set("commands", commands)
	return viper.WriteConfig()
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
