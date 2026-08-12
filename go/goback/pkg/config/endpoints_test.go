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
