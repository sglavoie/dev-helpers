package config

import (
	"os"
	"strings"
	"testing"
)

func resetProfileState(t *testing.T) {
	t.Helper()

	ProfileFlag = ""
	AllProfiles = false
	ActiveProfileName = ""
	t.Cleanup(func() {
		ProfileFlag = ""
		AllProfiles = false
		ActiveProfileName = ""
	})
}

func TestResolveProfilesWithProfileFlag(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/tmp"},"media":{}}}`)
	resetProfileState(t)

	ProfileFlag = "media"
	if err := ResolveProfiles(); err != nil {
		t.Fatal(err)
	}
	if ActiveProfileName != "media" {
		t.Fatalf("ActiveProfileName = %q, want %q", ActiveProfileName, "media")
	}
}

func TestResolveProfilesRejectsUnknownProfile(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/tmp"}}}`)
	resetProfileState(t)

	ProfileFlag = "nope"
	err := ResolveProfiles()
	if err == nil {
		t.Fatal("ResolveProfiles() = nil, want an error for an unknown profile")
	}
	if !strings.Contains(err.Error(), `profile "nope" not found`) {
		t.Fatalf("error = %q, want it to name the unknown profile", err)
	}
}

func TestResolveProfilesRejectsProfileAndAll(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/tmp"}}}`)
	resetProfileState(t)

	ProfileFlag = "macbook"
	AllProfiles = true
	err := ResolveProfiles()
	if err == nil {
		t.Fatal("ResolveProfiles() = nil, want an error for --profile with --all")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %q, want it to report the mutually exclusive flags", err)
	}
}

func TestResolveProfilesWithAllLeavesNoActiveProfile(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/tmp"},"media":{}}}`)
	resetProfileState(t)

	AllProfiles = true
	if err := ResolveProfiles(); err != nil {
		t.Fatal(err)
	}
	if ActiveProfileName != "" {
		t.Fatalf("ActiveProfileName = %q, want it left to the per-profile iteration", ActiveProfileName)
	}
}

func TestResolveProfilesMatchesHostname(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Skip("hostname unavailable")
	}
	loadConfig(t, `{"profiles":{"macbook":{"hostname":"`+hostname+`"},"other":{"hostname":"someone-else.local"}}}`)
	resetProfileState(t)

	if err := ResolveProfiles(); err != nil {
		t.Fatal(err)
	}
	if ActiveProfileName != "macbook" {
		t.Fatalf("ActiveProfileName = %q, want the profile matching this hostname", ActiveProfileName)
	}
}

func TestResolveProfilesUsesTheOnlyProfileWithoutHostnameMatch(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"hostname":"someone-else.local"}}}`)
	resetProfileState(t)

	if err := ResolveProfiles(); err != nil {
		t.Fatal(err)
	}
	if ActiveProfileName != "macbook" {
		t.Fatalf("ActiveProfileName = %q, want the single configured profile", ActiveProfileName)
	}
}

func TestResolveProfilesFailsWhenAmbiguous(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"hostname":"someone-else.local"},"other":{"hostname":"another.local"}}}`)
	resetProfileState(t)

	err := ResolveProfiles()
	if err == nil {
		t.Fatal("ResolveProfiles() = nil, want an error when no profile matches this machine")
	}
	if !strings.Contains(err.Error(), "could not auto-detect profile") {
		t.Fatalf("error = %q, want it to report the failed auto-detection", err)
	}
}

func TestResolveProfilesClearsStaleSelectionOnFailure(t *testing.T) {
	loadConfig(t, `{"profiles":{}}`)
	resetProfileState(t)
	ActiveProfileName = "previous"
	if err := ResolveProfiles(); err == nil {
		t.Fatal("empty profiles accepted")
	}
	if ActiveProfileName != "" {
		t.Fatalf("active profile = %q after failed selection", ActiveProfileName)
	}
}

func TestDetectLegacyConfig(t *testing.T) {
	loadConfig(t, `{"source":"/Users/me","destination":"/Volumes/Backup"}`)

	err := detectLegacyConfig()
	if err == nil {
		t.Fatal("detectLegacyConfig() = nil, want an error for a top-level source without profiles")
	}
	if !strings.Contains(err.Error(), "legacy config format detected") {
		t.Fatalf("error = %q, want it to report the legacy format", err)
	}
}

// A config with a mirror block but no top-level source is not legacy.
func TestDetectLegacyConfigAcceptsMirrorOnlyKeys(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"/Volumes/SanDisk/Media","destination":"/Volumes/Elements/Media"},"profiles":{"macbook":{"source":"/tmp"}}}`)

	if err := detectLegacyConfig(); err != nil {
		t.Fatal(err)
	}
}
