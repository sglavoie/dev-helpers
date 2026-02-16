package config

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type rsyncSupportedFlags struct {
	archive        bool
	delete         bool
	deleteExcluded bool
	dryRun         bool
	force          bool
	hardLinks      bool
	ignoreErrors   bool
	pruneEmptyDirs bool
	verbose        bool
}

type rsyncFlagsDaily struct {
	flags rsyncSupportedFlags
}

type rsyncFlagsWeekly struct {
	flags rsyncSupportedFlags
}

type rsync struct {
	daily  rsyncFlagsDaily
	weekly rsyncFlagsWeekly
}

type Config struct {
	confirmExec      bool
	ejectOnExit      bool
	excludedPatterns []string
	rsync            rsync
	srcDest          map[string]string
}

func (c *Config) Unmarshal() {
	err := viper.Unmarshal(&c)
	cobra.CheckErr(err)
}

type CliConfig struct {
	ConfigExtension string
}

var CfgFile string

// ProfileFlag holds the value of --profile / -p.
var ProfileFlag string

// AllProfiles is true when --all is passed.
var AllProfiles bool

// ActiveProfileName is the profile currently being processed.
var ActiveProfileName string

var cfg Config
var cliCfg CliConfig

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
func MustInitConfig(recreateInvalid bool, readConfig bool) {
	setCliCfg()
	setViperCfg()
	mustReadFile()

	if recreateInvalid {
		recreateInvalidFile()
	} else if readConfig {
		err := viper.ReadInConfig()
		cobra.CheckErr(err)
	}

	detectLegacyConfig()
	cfg.Unmarshal()
}

// detectLegacyConfig checks for old-style config (top-level "source" without "profiles")
// and exits with a helpful message if found.
func detectLegacyConfig() {
	if viper.IsSet("source") && !viper.IsSet("profiles") {
		fmt.Println("Legacy config format detected. goback now uses profiles.")
		fmt.Println()
		fmt.Println("Please restructure your ~/.goback.json to use the new format:")
		fmt.Println("  Move 'source', 'destination', and 'rsync' under a named profile in 'profiles'.")
		fmt.Println("  Add a 'hostname' field to machine-specific profiles (run `hostname` to get yours).")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println(`  {`)
		fmt.Println(`    "confirmExec": false,`)
		fmt.Println(`    "profiles": {`)
		fmt.Println(`      "myprofile": {`)
		fmt.Println(`        "hostname": "My-Machine.local",`)
		fmt.Println(`        "source": "/Users/me",`)
		fmt.Println(`        "destination": "/Volumes/Backup/myprofile",`)
		fmt.Println(`        "rsync": { ... }`)
		fmt.Println(`      }`)
		fmt.Println(`    }`)
		fmt.Println(`  }`)
		fmt.Println()
		fmt.Println("Run 'goback config reset' to generate a fresh config with the new structure,")
		fmt.Println("or manually edit your config with 'goback config edit'.")
		os.Exit(1)
	}
}

// ResolveProfiles determines which profiles to use based on flags.
// Called from PersistentPreRun after config is loaded.
func ResolveProfiles() {
	if ProfileFlag != "" && AllProfiles {
		fmt.Println("Error: --profile and --all are mutually exclusive")
		os.Exit(1)
	}

	if ProfileFlag != "" {
		// Validate the profile exists
		profiles := viper.GetStringMap("profiles")
		if _, ok := profiles[ProfileFlag]; !ok {
			fmt.Printf("Error: profile %q not found. Available profiles: %v\n", ProfileFlag, ProfileNames())
			os.Exit(1)
		}
		ActiveProfileName = ProfileFlag
		return
	}

	if AllProfiles {
		// ActiveProfileName will be set per-iteration in forEachProfile
		return
	}

	// Auto-detect: set ActiveProfileName to the first matching profile
	matching := MatchingProfiles()
	if len(matching) > 0 {
		ActiveProfileName = matching[0]
	} else if names := ProfileNames(); len(names) > 0 {
		// If no hostname match but only one profile, use it
		if len(names) == 1 {
			ActiveProfileName = names[0]
		} else {
			fmt.Println("Error: could not auto-detect profile for this machine.")
			fmt.Printf("No profile hostname matches %q.\n", mustHostname())
			fmt.Printf("Available profiles: %v\n", names)
			fmt.Println("Use --profile to specify one, or add a 'hostname' field to your profile.")
			os.Exit(1)
		}
	}
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

func setCliCfg() {
	cliCfg.ConfigExtension = ".json"
}

func setViperCfg() {
	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".goback" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("json")
		viper.SetConfigName(".goback")
		viper.SetConfigFile(home + "/.goback.json")
	}
}
