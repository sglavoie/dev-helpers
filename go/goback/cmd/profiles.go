package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/spf13/viper"
)

// forEachProfile runs the given action for each profile that should be processed
// based on the --profile, --all, or auto-detected profiles. It prints a header
// before each profile when running multiple profiles, and handles eject once
// after all profiles complete.
func forEachProfile(action func()) {
	profiles := profilesToRun()

	for i, name := range profiles {
		config.ActiveProfileName = name
		if len(profiles) > 1 {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("=== Profile: %s ===\n", name)
		}
		action()
	}

	if viper.GetBool("ejectOnExit") {
		eject.Eject()
	}
}

// profilesToRun returns the list of profile names to process based on flags.
func profilesToRun() []string {
	if config.ProfileFlag != "" {
		return []string{config.ProfileFlag}
	}
	if config.AllProfiles {
		return config.ProfileNames()
	}
	return config.MatchingProfiles()
}
