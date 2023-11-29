package server

import "github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"

func GetUpdateCommand(cmds models.Commands, updatedCmd models.Command) models.Commands {
	cmd := cmds[updatedCmd.Id]
	cmd.Name = updatedCmd.Name
	cmd.Description = updatedCmd.Description
	cmd.Command = updatedCmd.Command
	cmds[updatedCmd.Id] = cmd
	return cmds
}
