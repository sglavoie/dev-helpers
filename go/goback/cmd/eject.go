package cmd

import (
	"io"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/spf13/cobra"
)

var ejectCmd = &cobra.Command{
	Use:   "eject",
	Short: "Eject the disk linked to the backup",
	Long: "Eject the disk linked to the backup.\n\n" +
		"With --all, eject every external volume the configuration points at instead: each profile's source and destination and the mirror's endpoints, restricted to paths under /Volumes and ejected once each. A volume that is not mounted is skipped without failing, one volume failing never stops the others, and the command exits nonzero only when a mounted volume could not be ejected.",
	Annotations: withProfileResolution(profileUnlessAll),
	Run: func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(runEject(cmd))
	},
}

func runEject(cmd *cobra.Command) error {
	return ejectWith(cmd.OutOrStdout(), eject.OSDeps())
}

// ejectWith ejects the active profile's volume, or with --all every volume the
// configuration knows about. --all needs no profile, so it also works on a
// machine whose hostname matches none.
func ejectWith(out io.Writer, deps eject.Deps) error {
	if config.AllProfiles {
		report := eject.All(config.ConfiguredEndpoints(), deps)
		report.Render(out)
		return report.Err()
	}
	return eject.Eject(out, deps)
}

func init() {
	RootCmd.AddCommand(ejectCmd)
}
