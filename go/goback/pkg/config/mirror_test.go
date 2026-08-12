package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadMirrorReadsCompleteBlock(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"/Volumes/Card/Media","destination":"/Volumes/Archive/Media"},"profiles":{"macbook":{}}}`)

	if !MirrorConfigured() {
		t.Fatal("MirrorConfigured() = false, want true for a config with a mirror block")
	}

	settings, err := LoadMirror()
	if err != nil {
		t.Fatal(err)
	}
	want := MirrorSettings{Source: "/Volumes/Card/Media", Destination: "/Volumes/Archive/Media"}
	if settings != want {
		t.Fatalf("LoadMirror() = %#v, want %#v", settings, want)
	}
}

func TestLoadMirrorTrimsWhitespace(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"  /Volumes/Card/Media  ","destination":"/Volumes/Archive/Media\n"}}`)

	settings, err := LoadMirror()
	if err != nil {
		t.Fatal(err)
	}
	want := MirrorSettings{Source: "/Volumes/Card/Media", Destination: "/Volumes/Archive/Media"}
	if settings != want {
		t.Fatalf("LoadMirror() = %#v, want %#v", settings, want)
	}
}

func TestLoadMirrorRejectsIncompleteBlocks(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantMissing []string
	}{
		{
			name:        "no mirror block",
			content:     `{"profiles":{"macbook":{"source":"/tmp","destination":"/Volumes/Backup/macbook"}}}`,
			wantMissing: []string{"mirror.source", "mirror.destination"},
		},
		{
			name:        "empty mirror block",
			content:     `{"mirror":{}}`,
			wantMissing: []string{"mirror.source", "mirror.destination"},
		},
		{
			name:        "destination only",
			content:     `{"mirror":{"destination":"/Volumes/Archive/Media"}}`,
			wantMissing: []string{"mirror.source"},
		},
		{
			name:        "source only",
			content:     `{"mirror":{"source":"/Volumes/Card/Media"}}`,
			wantMissing: []string{"mirror.destination"},
		},
		{
			name:        "blank source",
			content:     `{"mirror":{"source":"   ","destination":"/Volumes/Archive/Media"}}`,
			wantMissing: []string{"mirror.source"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loadConfig(t, tc.content)

			settings, err := LoadMirror()
			if err == nil {
				t.Fatalf("LoadMirror() = %#v, want an error", settings)
			}
			if settings != (MirrorSettings{}) {
				t.Fatalf("LoadMirror() returned %#v alongside an error, want the zero value", settings)
			}
			for _, key := range tc.wantMissing {
				if !strings.Contains(err.Error(), key) {
					t.Fatalf("error = %q, want it to name the missing key %q", err, key)
				}
			}
		})
	}
}

// A custom config that declares only one endpoint must not inherit the other
// from the compiled defaults: goback would otherwise mirror onto a volume its
// owner never configured.
func TestLoadMirrorDoesNotFallBackToCompiledPaths(t *testing.T) {
	loadConfig(t, `{"mirror":{"source":"/Volumes/Card/Media"}}`)

	if _, err := LoadMirror(); err == nil {
		t.Fatal("LoadMirror() succeeded with a partial mirror block, want an error")
	}
	if got := viper.GetString(MirrorKey + ".destination"); got != "" {
		t.Fatalf("mirror.destination = %q, want it to stay unset", got)
	}
}

func TestMirrorConfiguredWithoutBlock(t *testing.T) {
	loadConfig(t, `{"profiles":{"macbook":{"source":"/tmp"}}}`)

	if MirrorConfigured() {
		t.Fatal("MirrorConfigured() = true, want false for a config without a mirror block")
	}
}

func TestGeneratedDefaultsIncludeMirrorBlock(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	setDefaultValues()

	settings, err := LoadMirror()
	if err != nil {
		t.Fatal(err)
	}
	want := MirrorSettings{Source: DefaultMirrorSource, Destination: DefaultMirrorDestination}
	if settings != want {
		t.Fatalf("generated mirror block = %#v, want %#v", settings, want)
	}
	if !viper.GetBool("confirmExec") {
		t.Fatal("confirmExec = false, want the existing defaults to be untouched")
	}
}
