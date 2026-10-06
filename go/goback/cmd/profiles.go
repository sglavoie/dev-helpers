package cmd

import (
	"errors"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
)

// forEachProfile selects and visits profiles without performing backup side
// effects. It collects failures so one profile does not prevent the others.
func forEachProfile(action func() error) error {
	profiles, err := config.SelectProfiles()
	if err != nil {
		return err
	}
	var failures []error
	for i, name := range profiles {
		config.ActiveProfileName = name
		if len(profiles) > 1 {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("=== Profile: %s ===\n", name)
		}
		if err := action(); err != nil {
			failures = append(failures, fmt.Errorf("profile %q: %w", name, err))
		}
	}
	return errors.Join(failures...)
}
