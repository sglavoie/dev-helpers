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
	// command's own --list or --volume flag names what to act on without a
	// profile.
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
			return !config.AllProfiles && !profileFreeRequested(cmd)
		case profileRequired:
			return true
		}
	}
	return true
}

// profileFreeRequested reports whether the command declares a --list or
// --volume flag and it was set.
func profileFreeRequested(cmd *cobra.Command) bool {
	if flag := cmd.Flags().Lookup("list"); flag != nil && flag.Changed && flag.Value.String() == "true" {
		return true
	}
	flag := cmd.Flags().Lookup("volume")
	return flag != nil && flag.Changed
}
