package config

import (
	"strings"

	"github.com/spf13/viper"
)

// EndpointRole is which side of a transfer a configured path is on.
type EndpointRole string

const (
	RoleSource      EndpointRole = "source"
	RoleDestination EndpointRole = "destination"
)

// Endpoint is one configured path together with what it belongs to: either the
// named profile or, when Mirror is set, the top-level mirror block.
type Endpoint struct {
	Profile string
	Mirror  bool
	Role    EndpointRole
	Path    string
}

// Owner describes what the endpoint belongs to, such as "profile macbook" or
// "mirror".
func (e Endpoint) Owner() string {
	if e.Mirror {
		return MirrorKey
	}
	return "profile " + e.Profile
}

// ConfiguredEndpointRefs returns every configured path with its owner and
// role: the source and destination of each profile in sorted profile order,
// followed by the mirror source and destination. Empty values are skipped but,
// unlike ConfiguredEndpoints, a path referenced several times is kept once per
// reference. It only reads configuration: it never touches the paths.
func ConfiguredEndpointRefs() []Endpoint {
	var endpoints []Endpoint
	add := func(endpoint Endpoint) {
		if endpoint.Path != "" {
			endpoints = append(endpoints, endpoint)
		}
	}

	for _, name := range ProfileNames() {
		prefix := "profiles." + name + "."
		add(Endpoint{Profile: name, Role: RoleSource, Path: strings.TrimSpace(viper.GetString(prefix + "source"))})
		add(Endpoint{Profile: name, Role: RoleDestination, Path: strings.TrimSpace(viper.GetString(prefix + "destination"))})
	}

	mirror := readMirror()
	add(Endpoint{Mirror: true, Role: RoleSource, Path: mirror.Source})
	add(Endpoint{Mirror: true, Role: RoleDestination, Path: mirror.Destination})
	return endpoints
}

// ConfiguredEndpoints returns every path the configuration points at: the
// source and destination of each profile in sorted profile order, followed by
// the mirror source and destination when they are configured. Empty values are
// skipped and duplicates are dropped, keeping first occurrences, so the result
// is deterministic. It only reads configuration: it never touches the paths.
func ConfiguredEndpoints() []string {
	refs := ConfiguredEndpointRefs()
	unique := make([]string, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref.Path] {
			continue
		}
		seen[ref.Path] = true
		unique = append(unique, ref.Path)
	}
	return unique
}
