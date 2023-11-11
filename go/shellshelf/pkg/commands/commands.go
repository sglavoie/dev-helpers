package commands

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
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

func Encode(cmdStr string) string {
	return base64.StdEncoding.EncodeToString([]byte(cmdStr))
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

func IsCommandNameAlreadyTaken(commands map[string]models.Command, name string) bool {
	for _, cmd := range commands {
		if cmd.Name == name {
			return true
		}
	}
	return false
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

func LoadDecoded() (map[string]models.Command, error) {
	commands, err := Load()
	if err != nil {
		return nil, err
	}

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

func Save(commands map[string]models.Command) error {
	viper.Set("commands", commands)
	return viper.WriteConfig()
}
