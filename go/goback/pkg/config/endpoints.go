package config

import (
	"strings"

	"github.com/spf13/viper"
)

// ConfiguredEndpoints returns every path the configuration points at: the
// source and destination of each profile in sorted profile order, followed by
// the mirror source and destination when they are configured. Empty values are
// skipped and duplicates are dropped, keeping first occurrences, so the result
// is deterministic. It only reads configuration: it never touches the paths.
func ConfiguredEndpoints() []string {
	var endpoints []string
	for _, name := range ProfileNames() {
		prefix := "profiles." + name + "."
		endpoints = append(endpoints,
			strings.TrimSpace(viper.GetString(prefix+"source")),
			strings.TrimSpace(viper.GetString(prefix+"destination")),
		)
	}

	mirror := readMirror()
	endpoints = append(endpoints, mirror.Source, mirror.Destination)

	unique := make([]string, 0, len(endpoints))
	seen := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint == "" || seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		unique = append(unique, endpoint)
	}
	return unique
}
