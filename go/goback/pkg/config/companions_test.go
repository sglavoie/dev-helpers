package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func loadConfig(t *testing.T, content string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".goback.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
}

func profileConfig(companions string) string {
	return `{"profiles":{"macbook":{"source":"/tmp","dailyCompanions":` + companions + `}}}`
}

func TestProfileCompanionsParsesFullEntry(t *testing.T) {
	loadConfig(t, profileConfig(`[
		{
			"id": "apple-photos",
			"name": "Apple Photos",
			"command": ["photos-backup", "daily"],
			"dryRunArgs": ["--dry-run"]
		}
	]`))

	companions, err := ProfileCompanions("macbook")
	if err != nil {
		t.Fatal(err)
	}
	want := []Companion{{
		ID:         "apple-photos",
		Name:       "Apple Photos",
		Command:    []string{"photos-backup", "daily"},
		DryRunArgs: []string{"--dry-run"},
	}}
	if !reflect.DeepEqual(companions, want) {
		t.Fatalf("companions = %#v, want %#v", companions, want)
	}
}

func TestDailyCompanionsUsesActiveProfile(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"dailyCompanions":[{"id":"a","command":["true"]}]},"media":{}}}`)

	ActiveProfileName = "macbook"
	t.Cleanup(func() { ActiveProfileName = "" })

	companions, err := DailyCompanions()
	if err != nil {
		t.Fatal(err)
	}
	if len(companions) != 1 || companions[0].ID != "a" {
		t.Fatalf("companions = %#v, want the single companion of the active profile", companions)
	}

	ActiveProfileName = "media"
	companions, err = DailyCompanions()
	if err != nil {
		t.Fatal(err)
	}
	if len(companions) != 0 {
		t.Fatalf("companions = %#v, want none for a profile without companions", companions)
	}
}

func TestProfileCompanionsDefaultsNameToID(t *testing.T) {
	loadConfig(t, profileConfig(`[{"id":"apple-photos","command":["photos-backup"]}]`))

	companions, err := ProfileCompanions("macbook")
	if err != nil {
		t.Fatal(err)
	}
	if companions[0].Name != "apple-photos" {
		t.Fatalf("name = %q, want the id", companions[0].Name)
	}
	if companions[0].DryRunArgs != nil {
		t.Fatalf("dryRunArgs = %#v, want nil", companions[0].DryRunArgs)
	}
}

func TestProfileCompanionsWithoutProfileOrKey(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/tmp"}}}`)

	for _, profile := range []string{"", "macbook", "unknown"} {
		companions, err := ProfileCompanions(profile)
		if err != nil {
			t.Fatalf("profile %q: %v", profile, err)
		}
		if companions != nil {
			t.Fatalf("profile %q: companions = %#v, want nil", profile, companions)
		}
	}
}

func TestArgvAppendsDryRunArgs(t *testing.T) {
	c := Companion{
		Command:    []string{"photos-backup", "daily"},
		DryRunArgs: []string{"--dry-run"},
	}

	if got, want := c.Argv(false), []string{"photos-backup", "daily"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Argv(false) = %#v, want %#v", got, want)
	}
	if got, want := c.Argv(true), []string{"photos-backup", "daily", "--dry-run"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Argv(true) = %#v, want %#v", got, want)
	}
	if got, want := c.Command, []string{"photos-backup", "daily"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Argv mutated Command to %#v, want %#v", got, want)
	}
}

func TestProfileCompanionsRejectsInvalidEntries(t *testing.T) {
	cases := []struct {
		name       string
		companions string
		wantErr    string
	}{
		{
			name:       "not a list",
			companions: `{"id":"a","command":["true"]}`,
			wantErr:    "must be a list of companion objects",
		},
		{
			name:       "entry is not an object",
			companions: `["photos-backup daily"]`,
			wantErr:    "must be an object",
		},
		{
			name:       "shell string command",
			companions: `[{"id":"a","command":"photos-backup daily"}]`,
			wantErr:    "companions are executed without a shell",
		},
		{
			name:       "shell string in first argument",
			companions: `[{"id":"a","command":["photos-backup daily"]}]`,
			wantErr:    "looks like a shell string",
		},
		{
			name:       "shell metacharacter in program",
			companions: `[{"id":"a","command":["photos-backup;rm -rf /"]}]`,
			wantErr:    "looks like a shell string",
		},
		{
			name:       "shell string dry-run args",
			companions: `[{"id":"a","command":["true"],"dryRunArgs":"--dry-run"}]`,
			wantErr:    "companions are executed without a shell",
		},
		{
			name:       "missing command",
			companions: `[{"id":"a"}]`,
			wantErr:    "command is required",
		},
		{
			name:       "empty command",
			companions: `[{"id":"a","command":[]}]`,
			wantErr:    "command is required",
		},
		{
			name:       "non-string argument",
			companions: `[{"id":"a","command":["true",3]}]`,
			wantErr:    "command[1] must be a string",
		},
		{
			name:       "empty argument",
			companions: `[{"id":"a","command":["true",""]}]`,
			wantErr:    "command[1] must not be empty",
		},
		{
			name:       "missing id",
			companions: `[{"command":["true"]}]`,
			wantErr:    "id is required",
		},
		{
			name:       "id with a slash",
			companions: `[{"id":"apple/photos","command":["true"]}]`,
			wantErr:    "may only contain letters",
		},
		{
			name:       "id with a space",
			companions: `[{"id":"apple photos","command":["true"]}]`,
			wantErr:    "may only contain letters",
		},
		{
			name:       "duplicate id",
			companions: `[{"id":"a","command":["true"]},{"id":"a","command":["false"]}]`,
			wantErr:    `duplicate companion id "a"`,
		},
		{
			name:       "unknown key",
			companions: `[{"id":"a","command":["true"],"shell":true}]`,
			wantErr:    "unknown key",
		},
		{
			name:       "empty name",
			companions: `[{"id":"a","name":"  ","command":["true"]}]`,
			wantErr:    "name must not be empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loadConfig(t, profileConfig(tc.companions))

			companions, err := ProfileCompanions("macbook")
			if err == nil {
				t.Fatalf("ProfileCompanions returned %#v, want an error", companions)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), "profiles.macbook.dailyCompanions") {
				t.Fatalf("error = %q, want it to name the configuration key", err)
			}
		})
	}
}

func TestProfileCompanionsAcceptsMixedCaseKeys(t *testing.T) {
	loadConfig(t, profileConfig(`[{"ID":"a","Name":"A","COMMAND":["true"],"dryrunargs":["-n"]}]`))

	companions, err := ProfileCompanions("macbook")
	if err != nil {
		t.Fatal(err)
	}
	want := []Companion{{ID: "a", Name: "A", Command: []string{"true"}, DryRunArgs: []string{"-n"}}}
	if !reflect.DeepEqual(companions, want) {
		t.Fatalf("companions = %#v, want %#v", companions, want)
	}
}
