package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// listEject runs goback eject with the given flags against fake mounts and a
// runner that records any diskutil call, returning what was printed.
func listEject(t *testing.T, runner *recordingRunner, mounted []string, args ...string) (string, error) {
	t.Helper()

	parseFlags(t, ejectCmd, args...)
	var out bytes.Buffer
	ejectCmd.SetOut(&out)
	t.Cleanup(func() { ejectCmd.SetOut(nil) })

	err := runEjectWith(ejectCmd, ejectDeps(runner, mounted...))
	return out.String(), err
}

func assertNoDiskutil(t *testing.T, runner *recordingRunner) {
	t.Helper()
	if len(runner.volumes) != 0 {
		t.Fatalf("--list ran diskutil for %#v, want no call", runner.volumes)
	}
}

const listedDrives = `MOUNT POINT        REFERENCED BY                                           EJECT WITH
/Volumes/Elements  profile media destination (/Volumes/Elements/media)     goback eject --profile media
                   mirror destination (/Volumes/Elements/Media)
/Volumes/SanDisk   profile macbook destination (/Volumes/SanDisk/macbook)  goback eject --profile macbook
                   profile media source (/Volumes/SanDisk/Media)
                   mirror source (/Volumes/SanDisk/Media)

goback eject --all ejects every configured volume that is mounted, whichever profile or mirror references it.
`

// Listing needs no active profile, so it works on a machine whose hostname
// matches none, and it never asks diskutil to do anything.
func TestEjectListShowsMountedVolumesWithTheirReferences(t *testing.T) {
	configuredDrives(t)
	withActiveProfile(t, "")
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/Elements", "/Volumes/SanDisk"}, "--list")
	if err != nil {
		t.Fatal(err)
	}
	assertNoDiskutil(t, runner)
	if out != listedDrives {
		t.Fatalf("output =\n%s\nwant\n%s", out, listedDrives)
	}
}

func TestEjectListWithAllIsTheSameListing(t *testing.T) {
	configuredDrives(t)
	withActiveProfile(t, "")
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/Elements", "/Volumes/SanDisk"}, "--list", "--all")
	if err != nil {
		t.Fatal(err)
	}
	assertNoDiskutil(t, runner)
	if out != listedDrives {
		t.Fatalf("output =\n%s\nwant\n%s", out, listedDrives)
	}
}

func TestEjectListRejectsProfile(t *testing.T) {
	configuredDrives(t)
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/SanDisk"}, "--list", "--profile", "macbook")
	if err == nil || !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("error = %v, want --profile rejected with --list", err)
	}
	assertNoDiskutil(t, runner)
	if out != "" {
		t.Fatalf("output = %q, want nothing listed", out)
	}
}

func TestEjectListSkipsVolumesThatAreNotMounted(t *testing.T) {
	configuredDrives(t)
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/SanDisk"}, "--list")
	if err != nil {
		t.Fatal(err)
	}
	assertNoDiskutil(t, runner)
	if strings.Contains(out, "/Volumes/Elements") {
		t.Fatalf("output = %q, want the unplugged Elements left out", out)
	}
	if !strings.Contains(out, "/Volumes/SanDisk  profile macbook destination") {
		t.Fatalf("output = %q, want SanDisk listed", out)
	}
}

func TestEjectListSaysSoWhenNothingIsMounted(t *testing.T) {
	configuredDrives(t)
	runner := &recordingRunner{}

	out, err := listEject(t, runner, nil, "--list")
	if err != nil {
		t.Fatalf("error = %v, want an empty listing to succeed", err)
	}
	if out != "No configured volumes are currently mounted.\n" {
		t.Fatalf("output = %q, want the empty message", out)
	}
}

// A configuration pointing only at internal paths has no volume to list, even
// if something happens to be mounted.
func TestEjectListIgnoresInternalPaths(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("profiles.local.source", "/Users/me")
	viper.Set("profiles.local.destination", "/Users/me/backups")
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/SanDisk"}, "--list")
	if err != nil {
		t.Fatal(err)
	}
	if out != "No configured volumes are currently mounted.\n" {
		t.Fatalf("output = %q, want the empty message", out)
	}
}

// A volume referenced only as a source or by the mirror cannot be ejected on
// its own through a profile, so no profile command is suggested for it.
func TestEjectListSuggestsNoProfileCommandForSourcesOrTheMirror(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("profiles.media.source", "/Volumes/Photos/Library")
	viper.Set("profiles.media.destination", "/Users/me/backups")
	viper.Set("mirror.source", "/Volumes/Archive/In")
	viper.Set("mirror.destination", "/Volumes/Archive/Out")
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/Archive", "/Volumes/Photos"}, "--list")
	if err != nil {
		t.Fatal(err)
	}
	want := `MOUNT POINT       REFERENCED BY                                   EJECT WITH
/Volumes/Archive  mirror source (/Volumes/Archive/In)             -
                  mirror destination (/Volumes/Archive/Out)
/Volumes/Photos   profile media source (/Volumes/Photos/Library)  -

goback eject --all ejects every configured volume that is mounted, whichever profile or mirror references it.
`
	if out != want {
		t.Fatalf("output =\n%s\nwant\n%s", out, want)
	}
}

// Two profiles backing up to the same drive each get their own command.
func TestEjectListSuggestsEveryProfileWithADestinationOnTheVolume(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("profiles.work.destination", "/Volumes/SanDisk/work")
	viper.Set("profiles.home.destination", "/Volumes/SanDisk/home")
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/SanDisk"}, "--list")
	if err != nil {
		t.Fatal(err)
	}
	want := `MOUNT POINT       REFERENCED BY                                     EJECT WITH
/Volumes/SanDisk  profile home destination (/Volumes/SanDisk/home)  goback eject --profile home
                  profile work destination (/Volumes/SanDisk/work)  goback eject --profile work
`
	if !strings.HasPrefix(out, want) {
		t.Fatalf("output =\n%s\nwant it to start with\n%s", out, want)
	}
}

// Suggested commands keep an explicit --config so they act on the same
// configuration, quoted so they can be pasted into a shell, and listing never
// rewrites that configuration.
func TestEjectListFollowsAndQuotesACustomConfigFile(t *testing.T) {
	path := customConfig(t, `{
		"profiles": {"it's mine": {"source": "/Users/me", "destination": "/Volumes/Custom Out/macbook"}}
	}`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}

	out, err := listEject(t, runner, []string{"/Volumes/Custom Out"}, "--list", "--config", path)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDiskutil(t, runner)

	quoted := shellQuote(path)
	for _, want := range []string{
		"/Volumes/Custom Out  profile it's mine destination (/Volumes/Custom Out/macbook)",
		`goback eject --profile 'it'\''s mine' --config ` + quoted,
		"goback eject --all --config " + quoted + " ejects every configured volume",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output =\n%s\nwant it to contain %q", out, want)
		}
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("the custom configuration was rewritten:\n%s", after)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"macbook":            "macbook",
		"/Users/me/cfg.json": "/Users/me/cfg.json",
		"My Drive":           "'My Drive'",
		"it's":               `'it'\''s'`,
		"":                   "''",
		"$HOME":              "'$HOME'",
	}
	for word, want := range cases {
		if got := shellQuote(word); got != want {
			t.Fatalf("shellQuote(%q) = %q, want %q", word, got, want)
		}
	}
}

// --list is a read-only request, so it needs no profile even without --all.
func TestEjectListSkipsProfileResolution(t *testing.T) {
	withAllProfiles(t, false)
	parseFlags(t, ejectCmd, "--list")

	if needsProfileResolution(ejectCmd) {
		t.Fatal("eject --list requires profile resolution, want it usable without a matching profile")
	}
}

// Without --list the flag changes nothing: plain eject still acts on the
// active profile's drive.
func TestEjectWithoutListStillEjects(t *testing.T) {
	configuredDrives(t)
	withAllProfiles(t, false)
	withActiveProfile(t, "macbook")
	runner := &recordingRunner{}

	if _, err := listEject(t, runner, []string{"/Volumes/Elements", "/Volumes/SanDisk"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.volumes) != 1 || runner.volumes[0] != "/Volumes/SanDisk" {
		t.Fatalf("ejected %#v, want only the active profile's drive", runner.volumes)
	}
}

func TestEjectHelpDescribesList(t *testing.T) {
	for _, want := range []string{"With --list, eject nothing", "rejects --profile", "that profile's destination only"} {
		if !strings.Contains(ejectCmd.Long, want) {
			t.Fatalf("eject help = %q, want it to mention %q", ejectCmd.Long, want)
		}
	}
	if ejectCmd.Flags().Lookup("list") == nil {
		t.Fatal("goback eject declares no --list flag")
	}
}
