package cmd

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/spf13/cobra"
)

var ejectCmd = &cobra.Command{
	Use:   "eject",
	Short: "Eject the disk linked to the backup",
	Long: "Eject the disk linked to the backup.\n\n" +
		"Plain eject, or eject --profile NAME, unmounts the volume holding that profile's destination only.\n\n" +
		"With --all, eject every external volume the configuration points at instead: each profile's source and destination and the mirror's endpoints, restricted to paths under /Volumes and ejected once each. A volume that is not mounted is skipped without failing, one volume failing never stops the others, and the command exits nonzero only when a mounted volume could not be ejected.\n\n" +
		"With --list, eject nothing: print every configured volume that is currently mounted, what references it, and the commands that would eject it. --list needs no profile, rejects --profile, and treats --all as redundant.",
	Annotations: withProfileResolution(profileUnlessAll),
	Run: func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(runEject(cmd))
	},
}

func runEject(cmd *cobra.Command) error {
	return runEjectWith(cmd, eject.OSDeps())
}

// runEjectWith dispatches to the listing before anything that could eject, so
// --list never reaches diskutil.
func runEjectWith(cmd *cobra.Command, deps eject.Deps) error {
	list, err := cmd.Flags().GetBool("list")
	if err != nil {
		return err
	}
	if !list {
		return ejectWith(cmd.OutOrStdout(), deps)
	}

	if flag := cmd.Flags().Lookup("profile"); flag != nil && flag.Changed {
		return fmt.Errorf("--profile does not apply to %s --list: the listing always covers every profile", cmd.CommandPath())
	}
	var configPath string
	if flag := cmd.Flags().Lookup("config"); flag != nil && flag.Changed {
		configPath = config.CfgFile
	}
	listMountedWith(cmd.OutOrStdout(), deps.Mounts, configPath)
	return nil
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

// listMountedWith prints every mounted volume the configuration references,
// one row per reference. configPath, when set, is carried into the suggested
// commands so they act on the same configuration.
func listMountedWith(out io.Writer, mounts eject.Mounts, configPath string) {
	volumes := eject.MountedVolumes(config.ConfiguredEndpointRefs(), mounts)
	if len(volumes) == 0 {
		fmt.Fprintln(out, "No configured volumes are currently mounted.")
		return
	}

	var table bytes.Buffer
	w := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MOUNT POINT\tREFERENCED BY\tEJECT WITH")
	for _, volume := range volumes {
		profiles := volume.DestinationProfiles()
		commands := make([]string, len(profiles))
		for i, profile := range profiles {
			commands[i] = ejectCommand(configPath, "--profile", profile)
		}
		if len(commands) == 0 {
			commands = []string{"-"}
		}

		rows := max(len(volume.References), len(commands))
		for i := range rows {
			mount, reference, command := "", "", ""
			if i == 0 {
				mount = volume.Volume
			}
			if i < len(volume.References) {
				ref := volume.References[i]
				reference = fmt.Sprintf("%s %s (%s)", ref.Owner(), ref.Role, ref.Path)
			}
			if i < len(commands) {
				command = commands[i]
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", mount, reference, command)
		}
	}
	w.Flush()
	// A row without a command still pads its reference cell, so the padding is
	// trimmed rather than printed as trailing whitespace.
	for _, line := range strings.Split(strings.TrimSuffix(table.String(), "\n"), "\n") {
		fmt.Fprintln(out, strings.TrimRight(line, " "))
	}

	fmt.Fprintf(out, "\n%s ejects every configured volume that is mounted, whichever profile or mirror references it.\n",
		ejectCommand(configPath, "--all"))
}

// ejectCommand builds a shell-safe goback eject invocation.
func ejectCommand(configPath string, args ...string) string {
	words := append([]string{"goback", "eject"}, args...)
	if configPath != "" {
		words = append(words, "--config", configPath)
	}
	for i, word := range words {
		words[i] = shellQuote(word)
	}
	return strings.Join(words, " ")
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote returns word unchanged when a POSIX shell would read it as is,
// and single-quoted otherwise.
func shellQuote(word string) string {
	if shellSafe.MatchString(word) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func init() {
	ejectCmd.Flags().Bool("list", false, "List the configured volumes that are mounted, without ejecting anything")
	RootCmd.AddCommand(ejectCmd)
}
