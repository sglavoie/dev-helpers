package config

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/viper"
)

var CfgFile string

// ProfileFlag holds the value of --profile / -p.
var ProfileFlag string

// AllProfiles is true when --all is passed.
var AllProfiles bool

// ActiveProfileName is the profile currently being processed.
var ActiveProfileName string

// ActiveProfilePrefix returns the viper key prefix for the active profile,
// e.g. "profiles.macbook." when ActiveProfileName is "macbook".
func ActiveProfilePrefix() string {
	return "profiles." + ActiveProfileName + "."
}

// ProfileNames returns a sorted list of all profile keys under "profiles".
func ProfileNames() []string {
	all := viper.GetStringMap("profiles")
	names := make([]string, 0, len(all))
	for k := range all {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// MatchingProfiles returns the profiles that should run on the current machine.
// It finds the machine profile whose hostname matches os.Hostname(), then
// appends "media" (or any non-hostname profile with backupMedia triggered)
// if the matched profile has backupMedia set to true.
func MatchingProfiles() []string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	var matched []string
	var mediaProfiles []string

	for _, name := range ProfileNames() {
		prefix := "profiles." + name + "."
		profileHostname := viper.GetString(prefix + "hostname")

		if profileHostname != "" && profileHostname == hostname {
			matched = append(matched, name)
			if viper.GetBool(prefix + "backupMedia") {
				// Find profiles that have no hostname (shared profiles like "media")
				for _, other := range ProfileNames() {
					otherPrefix := "profiles." + other + "."
					if viper.GetString(otherPrefix+"hostname") == "" {
						mediaProfiles = append(mediaProfiles, other)
					}
				}
			}
		}
	}

	// Append media/shared profiles, avoiding duplicates
	seen := make(map[string]bool)
	for _, name := range matched {
		seen[name] = true
	}
	for _, name := range mediaProfiles {
		if !seen[name] {
			matched = append(matched, name)
			seen[name] = true
		}
	}

	return matched
}

// MustInitConfig reads in config file.
func MustInitConfig(recreateInvalid bool, readConfig bool) error {
	setViperCfg()
	mustReadFile()

	if recreateInvalid {
		recreateInvalidFile()
	} else if readConfig {
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	return detectLegacyConfig()
}

// detectLegacyConfig checks for old-style config (top-level "source" without "profiles")
// and returns an error with a helpful message if found.
func detectLegacyConfig() error {
	if viper.IsSet("source") && !viper.IsSet("profiles") {
		return fmt.Errorf("legacy config format detected. goback now uses profiles.\n\n" +
			"Please restructure your ~/.goback.json to use the new format:\n" +
			"  Move 'source', 'destination', and 'rsync' under a named profile in 'profiles'.\n" +
			"  Add a 'hostname' field to machine-specific profiles (run `hostname` to get yours).\n\n" +
			"Example:\n" +
			"  {\n" +
			`    "confirmExec": false,` + "\n" +
			`    "profiles": {` + "\n" +
			`      "myprofile": {` + "\n" +
			`        "hostname": "My-Machine.local",` + "\n" +
			`        "source": "/Users/me",` + "\n" +
			`        "destination": "/Volumes/Backup/myprofile",` + "\n" +
			`        "rsync": { ... }` + "\n" +
			"      }\n" +
			"    }\n" +
			"  }\n\n" +
			"Run 'goback config reset' to generate a fresh config with the new structure,\n" +
			"or manually edit your config with 'goback config edit'.")
	}
	return nil
}

// ResolveProfiles determines which profiles to use based on flags.
// Called from PersistentPreRunE after config is loaded.
func ResolveProfiles() error {
	if ProfileFlag != "" && AllProfiles {
		return fmt.Errorf("--profile and --all are mutually exclusive")
	}

	if ProfileFlag != "" {
		// Validate the profile exists
		profiles := viper.GetStringMap("profiles")
		if _, ok := profiles[ProfileFlag]; !ok {
			return fmt.Errorf("profile %q not found. Available profiles: %v", ProfileFlag, ProfileNames())
		}
		ActiveProfileName = ProfileFlag
		return nil
	}

	if AllProfiles {
		// ActiveProfileName will be set per-iteration in forEachProfile
		return nil
	}

	// Auto-detect: set ActiveProfileName to the first matching profile
	matching := MatchingProfiles()
	if len(matching) > 0 {
		ActiveProfileName = matching[0]
		fmt.Fprintf(os.Stderr, "Using profile: %s\n", ActiveProfileName)
	} else if names := ProfileNames(); len(names) > 0 {
		// If no hostname match but only one profile, use it
		if len(names) == 1 {
			ActiveProfileName = names[0]
			fmt.Fprintf(os.Stderr, "Using profile: %s\n", ActiveProfileName)
		} else {
			return fmt.Errorf("could not auto-detect profile for this machine.\nNo profile hostname matches %q.\nAvailable profiles: %v\nUse --profile to specify one, or add a 'hostname' field to your profile.", mustHostname(), names)
		}
	}
	return nil
}

func mustHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "(unknown)"
	}
	return h
}

func recreateInvalidFile() {
	if err := viper.ReadInConfig(); err != nil {
		askToRecreateInvalidFile()
	}
}

func setViperCfg() {
	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}

		// Search config in home directory with name ".goback" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("json")
		viper.SetConfigName(".goback")
		viper.SetConfigFile(home + "/.goback.json")
	}
}
