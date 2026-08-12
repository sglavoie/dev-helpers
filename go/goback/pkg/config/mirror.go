package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// MirrorKey is the top-level configuration key of the one-way media mirror.
const MirrorKey = "mirror"

// DefaultMirrorSource and DefaultMirrorDestination are written to generated
// configuration files. They are never used as fallbacks when reading a config.
const (
	DefaultMirrorSource      = "/Volumes/SanDisk/Media"
	DefaultMirrorDestination = "/Volumes/Elements/Media"
)

// MirrorSettings holds the configured endpoints of the one-way media mirror.
type MirrorSettings struct {
	Source      string
	Destination string
}

// MirrorConfigured reports whether the configuration declares a mirror block.
func MirrorConfigured() bool {
	return viper.IsSet(MirrorKey)
}

// LoadMirror returns the configured mirror endpoints. It fails when the mirror
// block is missing or incomplete instead of falling back to compiled paths, so
// a custom configuration can never mirror endpoints its owner did not declare.
func LoadMirror() (MirrorSettings, error) {
	settings := readMirror()

	var missing []string
	if settings.Source == "" {
		missing = append(missing, MirrorKey+".source")
	}
	if settings.Destination == "" {
		missing = append(missing, MirrorKey+".destination")
	}
	if len(missing) > 0 {
		return MirrorSettings{}, fmt.Errorf("%s not set in %s.\n\nAdd a top-level %q block to your configuration, for example:\n  %q: {\n    \"source\": %q,\n    \"destination\": %q\n  }",
			strings.Join(missing, " and "), viper.ConfigFileUsed(), MirrorKey, MirrorKey, DefaultMirrorSource, DefaultMirrorDestination)
	}

	return settings, nil
}

// readMirror returns the configured endpoints as-is, with surrounding
// whitespace removed and without requiring either of them.
func readMirror() MirrorSettings {
	return MirrorSettings{
		Source:      strings.TrimSpace(viper.GetString(MirrorKey + ".source")),
		Destination: strings.TrimSpace(viper.GetString(MirrorKey + ".destination")),
	}
}
