package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
)

// profileResolution declares whether a command needs an active profile resolved
// before it runs. Commands that only read global configuration opt out so they
// stay usable on machines whose hostname matches no profile.
type profileResolution string

const (
	// profileRequired resolves an active profile, as every snapshot and
	// profile command does. It is the default for commands without an
	// annotation.
	profileRequired profileResolution = "required"

	// profileNotRequired loads the configuration without selecting a profile.
	profileNotRequired profileResolution = "not-required"

	// profileUnlessAll resolves a profile except when --all asks the command
	// to act on everything the configuration knows about, or when the
	// command's own --list flag asks it to only describe the configuration.
	profileUnlessAll profileResolution = "not-required-with-all"
)

const profileResolutionKey = "goback/profile-resolution"

// withProfileResolution builds the cobra annotations declaring a command's
// profile-resolution mode.
func withProfileResolution(mode profileResolution) map[string]string {
	return map[string]string{profileResolutionKey: string(mode)}
}

// needsProfileResolution reports whether the command about to run requires an
// active profile. The nearest annotated command in the parent chain wins.
func needsProfileResolution(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch profileResolution(c.Annotations[profileResolutionKey]) {
		case profileNotRequired:
			return false
		case profileUnlessAll:
			return !config.AllProfiles && !listRequested(cmd)
		case profileRequired:
			return true
		}
	}
	return true
}

// listRequested reports whether the command declares a --list flag and it was
// set.
func listRequested(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("list")
	return flag != nil && flag.Changed && flag.Value.String() == "true"
}
