package commands

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/editor"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
)

func Add(cmds map[string]models.Command, cmd models.Command) map[string]models.Command {
	maxID, err := GetMaxID(cmds)
	if err != nil {
		clihelpers.FatalExit("Error getting max ID: %v", err)
	}

	newCmdId := strconv.Itoa(maxID + 1)

	RunCheckOnDecodedCommand(cmd)
	cmd.Command = Encode(cmd.Command)
	cmd.Id = newCmdId
	cmds[newCmdId] = cmd

	return cmds
}

func AreAllCommandIDsValid(commands map[string]models.Command, ids []string) error {
	for _, id := range ids {
		if _, ok := commands[id]; !ok {
			return fmt.Errorf("command ID %s not found", id)
		}
	}
	return nil
}

func Decode(encodedCmd string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCmd)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}

func EditAllFields(cmd models.Command) (models.Command, error) {
	updatedCmdDetails, err := editor.GetUpdatedCommandFields(cmd)
	if err != nil {
		return cmd, err
	}

	parsedCmd, err := editor.GetParsedCommand(updatedCmdDetails)
	if err != nil {
		return cmd, err
	}

	return parsedCmd, nil
}

func Encode(cmdStr string) string {
	return base64.StdEncoding.EncodeToString([]byte(cmdStr))
}

func EncodeAll(commands map[string]models.Command) map[string]models.Command {
	for id, cmd := range commands {
		cmd.Command = Encode(cmd.Command)
		commands[id] = cmd
	}
	return commands
}

func GetById(commands map[string]models.Command, id string) (models.Command, error) {
	if cmd, ok := commands[id]; ok {
		return cmd, nil
	}
	return models.Command{}, fmt.Errorf("command ID '%s' not found", id)
}

func GetByName(commands map[string]models.Command, name string) (models.Command, error) {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd, nil
		}
	}
	return models.Command{}, fmt.Errorf("command name '%s' not found", name)
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

func IsCommandNameAlreadyTaken(commands map[string]models.Command, name string) (bool, []string) {
	var ids []string
	for id, cmd := range commands {
		if cmd.Name == name {
			ids = append(ids, id)
		}
	}
	return len(ids) > 0, ids
}

func LoadDecoded(commands map[string]models.Command) (map[string]models.Command, error) {
	for id, cmd := range commands {
		decodedCmd, err := Decode(cmd.Command)
		if err != nil {
			return nil, err
		}
		cmd.Command = decodedCmd
		commands[id] = cmd
	}

	return commands, nil
}

func RemoveById(commands map[string]models.Command, ids string) map[string]models.Command {
	delete(commands, ids)
	return commands
}

func RemoveByIds(commands map[string]models.Command, ids []string) map[string]models.Command {
	for _, id := range ids {
		delete(commands, id)
	}
	return commands
}

func RunCheckOnDecodedCommand(decodedCmd models.Command) {
	if strings.TrimSpace(decodedCmd.Name) == "" {
		clihelpers.FatalExit("command name cannot be empty")
	}

	if strings.TrimSpace(decodedCmd.Command) == "" {
		clihelpers.FatalExit("command content cannot be empty")
	}
}

func SetCommand(cfg *models.Config, cmd models.Command, cmdId string) {
	cfg.Commands[cmdId] = cmd
}
