package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/spf13/cobra"
)

var ejectCmd = &cobra.Command{
	Use:   "eject",
	Short: "Eject the disk linked to the backup",
	Run: func(cmd *cobra.Command, args []string) {
		eject.Eject()
	},
}

func init() {
	RootCmd.AddCommand(ejectCmd)
}
