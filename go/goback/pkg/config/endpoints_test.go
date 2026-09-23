package config

import (
	"reflect"
	"testing"
)

func TestConfiguredEndpointsIncludesProfilesAndMirror(t *testing.T) {
	loadConfig(t, `{
		"mirror": {"source": "/Volumes/SanDisk/Media", "destination": "/Volumes/Elements/Media"},
		"profiles": {
			"media": {"source": "/Volumes/SanDisk/Media", "destination": "/Volumes/Elements/media"},
			"macbook": {"source": "/Users/me", "destination": "/Volumes/SanDisk/macbook"}
		}
	}`)

	want := []string{
		"/Users/me",
		"/Volumes/SanDisk/macbook",
		"/Volumes/SanDisk/Media",
		"/Volumes/Elements/media",
		"/Volumes/Elements/Media",
	}
	if got := ConfiguredEndpoints(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfiguredEndpoints() = %#v, want %#v", got, want)
	}
}

func TestConfiguredEndpointsWithoutMirrorBlock(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/Users/me","destination":"/Volumes/SanDisk/macbook"}}}`)

	want := []string{"/Users/me", "/Volumes/SanDisk/macbook"}
	if got := ConfiguredEndpoints(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfiguredEndpoints() = %#v, want %#v", got, want)
	}
}

func TestConfiguredEndpointsSkipsEmptyValues(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"  "},"profiles":{"default":{"source":"","destination":""}}}`)

	if got := ConfiguredEndpoints(); len(got) != 0 {
		t.Fatalf("ConfiguredEndpoints() = %#v, want none", got)
	}
}

func TestConfiguredEndpointsDeduplicatesRepeatedPaths(t *testing.T) {
	loadConfig(t, `{
		"mirror": {"source": "/Volumes/SanDisk/Media", "destination": "/Volumes/Elements/Media"},
		"profiles": {"media": {"source": "/Volumes/SanDisk/Media", "destination": "/Volumes/Elements/Media"}}
	}`)

	want := []string{"/Volumes/SanDisk/Media", "/Volumes/Elements/Media"}
	if got := ConfiguredEndpoints(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfiguredEndpoints() = %#v, want %#v", got, want)
	}
}

func TestConfiguredEndpointRefsKeepsOwnersAndRoles(t *testing.T) {
	loadConfig(t, `{
		"mirror": {"source": "/Volumes/SanDisk/Media", "destination": "/Volumes/Elements/Media"},
		"profiles": {
			"media": {"source": "/Volumes/SanDisk/Media", "destination": "/Volumes/Elements/media"},
			"macbook": {"source": "/Users/me", "destination": "/Volumes/SanDisk/macbook"}
		}
	}`)

	want := []Endpoint{
		{Profile: "macbook", Role: RoleSource, Path: "/Users/me"},
		{Profile: "macbook", Role: RoleDestination, Path: "/Volumes/SanDisk/macbook"},
		{Profile: "media", Role: RoleSource, Path: "/Volumes/SanDisk/Media"},
		{Profile: "media", Role: RoleDestination, Path: "/Volumes/Elements/media"},
		{Mirror: true, Role: RoleSource, Path: "/Volumes/SanDisk/Media"},
		{Mirror: true, Role: RoleDestination, Path: "/Volumes/Elements/Media"},
	}
	if got := ConfiguredEndpointRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfiguredEndpointRefs() = %#v, want %#v", got, want)
	}
}

func TestConfiguredEndpointRefsSkipsEmptyValues(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"  "},"profiles":{"default":{"source":"","destination":" /Volumes/SanDisk/x "}}}`)

	want := []Endpoint{{Profile: "default", Role: RoleDestination, Path: "/Volumes/SanDisk/x"}}
	if got := ConfiguredEndpointRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfiguredEndpointRefs() = %#v, want %#v", got, want)
	}
}

func TestConfiguredEndpointRefsWithOnlyAMirror(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"/Volumes/A/x","destination":"/Volumes/B/y"}}`)

	want := []Endpoint{
		{Mirror: true, Role: RoleSource, Path: "/Volumes/A/x"},
		{Mirror: true, Role: RoleDestination, Path: "/Volumes/B/y"},
	}
	if got := ConfiguredEndpointRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfiguredEndpointRefs() = %#v, want %#v", got, want)
	}
}

func TestEndpointOwner(t *testing.T) {
	if got := (Endpoint{Profile: "macbook"}).Owner(); got != "profile macbook" {
		t.Fatalf("Owner() = %q, want profile macbook", got)
	}
	if got := (Endpoint{Mirror: true}).Owner(); got != "mirror" {
		t.Fatalf("Owner() = %q, want mirror", got)
	}
}
