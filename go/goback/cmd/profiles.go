package cmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/spf13/viper"
)

// forEachProfile runs the given action for each profile that should be processed
// based on the --profile, --all, or auto-detected profiles. It prints a header
// before each profile when running multiple profiles, and handles eject once
// after all profiles complete. If any profile fails, it prints the error and
// continues to the next profile. After all profiles complete, it exits with
// code 1 if any profile failed.
func forEachProfile(action func() error) {
	profiles := profilesToRun()
	ejectOnExit := viper.GetBool("ejectOnExit")

	var destinations []string
	anyFailed := false
	for i, name := range profiles {
		config.ActiveProfileName = name
		if len(profiles) > 1 {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("=== Profile: %s ===\n", name)
		}
		if err := action(); err != nil {
			fmt.Fprintf(os.Stderr, "error: profile %q: %v\n", name, err)
			anyFailed = true
			continue
		}
		if ejectOnExit {
			dest := viper.GetString(config.ActiveProfilePrefix() + "destination")
			if dest != "" {
				destinations = append(destinations, dest)
			}
		}
	}

	if ejectOnExit {
		eject.EjectPaths(destinations)
	}

	if anyFailed {
		os.Exit(1)
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
