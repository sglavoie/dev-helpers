package cmd

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/spf13/viper"
)

// recordingRunner stands in for diskutil so no test ever unmounts a drive.
type recordingRunner struct {
	volumes []string
	errs    map[string]error
}

func (r *recordingRunner) Run(argv []string) (string, error) {
	volume := argv[len(argv)-1]
	r.volumes = append(r.volumes, volume)
	return "", r.errs[volume]
}

type mountedVolumes map[string]bool

func (m mountedVolumes) Mounted(volume string) bool { return m[volume] }

func ejectDeps(runner *recordingRunner, mounted ...string) eject.Deps {
	set := mountedVolumes{}
	for _, volume := range mounted {
		set[volume] = true
	}
	return eject.Deps{Runner: runner, Mounts: set}
}

// configuredDrives is the shape of the real configuration: SanDisk holds a
// profile destination and is the mirror source, Elements holds another profile
// destination and is the mirror destination, and the macbook source is
// internal.
func configuredDrives(t *testing.T) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("profiles.macbook.source", "/Users/me")
	viper.Set("profiles.macbook.destination", "/Volumes/SanDisk/macbook")
	viper.Set("profiles.media.source", "/Volumes/SanDisk/Media")
	viper.Set("profiles.media.destination", "/Volumes/Elements/media")
	viper.Set(config.MirrorKey+".source", "/Volumes/SanDisk/Media")
	viper.Set(config.MirrorKey+".destination", "/Volumes/Elements/Media")
}

func withActiveProfile(t *testing.T, name string) {
	t.Helper()

	previous := config.ActiveProfileName
	config.ActiveProfileName = name
	t.Cleanup(func() { config.ActiveProfileName = previous })
}

// Each drive is ejected once even though SanDisk appears in three configured
// paths and Elements in two, and the internal source never reaches diskutil.
func TestEjectAllEjectsEachConfiguredDriveOnce(t *testing.T) {
	configuredDrives(t)
	withAllProfiles(t, true)
	withActiveProfile(t, "")

	runner := &recordingRunner{}
	var out bytes.Buffer
	if err := ejectWith(&out, ejectDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")); err != nil {
		t.Fatal(err)
	}

	want := []string{"/Volumes/Elements", "/Volumes/SanDisk"}
	if !reflect.DeepEqual(runner.volumes, want) {
		t.Fatalf("ejected %#v, want %#v", runner.volumes, want)
	}
	if !strings.Contains(out.String(), "2 ejected, 0 skipped, 0 failed") {
		t.Fatalf("output = %q, want the summary counts", out.String())
	}
}

// --all describes the whole configuration, so an unplugged drive is a skip and
// the command still succeeds.
func TestEjectAllSkipsDrivesThatAreNotPluggedIn(t *testing.T) {
	configuredDrives(t)
	withAllProfiles(t, true)
	withActiveProfile(t, "")

	runner := &recordingRunner{}
	var out bytes.Buffer
	if err := ejectWith(&out, ejectDeps(runner, "/Volumes/SanDisk")); err != nil {
		t.Fatalf("ejectWith() = %v, want an unplugged drive to be no failure", err)
	}

	if !reflect.DeepEqual(runner.volumes, []string{"/Volumes/SanDisk"}) {
		t.Fatalf("ejected %#v, want only the plugged-in drive", runner.volumes)
	}
	if !strings.Contains(out.String(), "/Volumes/Elements: not mounted, skipped") {
		t.Fatalf("output = %q, want the absent drive reported", out.String())
	}
}

// The mirror block is optional, and a configuration without one still ejects
// the drives its profiles point at.
func TestEjectAllWorksWithoutAMirrorBlock(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("profiles.macbook.source", "/Users/me")
	viper.Set("profiles.macbook.destination", "/Volumes/SanDisk/macbook")

	withAllProfiles(t, true)
	withActiveProfile(t, "")

	runner := &recordingRunner{}
	if err := ejectWith(&bytes.Buffer{}, ejectDeps(runner, "/Volumes/SanDisk")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.volumes, []string{"/Volumes/SanDisk"}) {
		t.Fatalf("ejected %#v, want the profile's drive", runner.volumes)
	}
}

// A configuration that points at no external volume at all says so instead of
// failing.
func TestEjectAllWithNoExternalVolumeConfigured(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("profiles.local.source", "/Users/me")
	viper.Set("profiles.local.destination", "/Users/me/backups")

	withAllProfiles(t, true)
	withActiveProfile(t, "")

	runner := &recordingRunner{}
	var out bytes.Buffer
	if err := ejectWith(&out, ejectDeps(runner, "/Volumes/SanDisk")); err != nil {
		t.Fatal(err)
	}
	if len(runner.volumes) != 0 {
		t.Fatalf("diskutil was called for %#v, want no call", runner.volumes)
	}
	if !strings.Contains(out.String(), "No configured volume under /Volumes") {
		t.Fatalf("output = %q, want it to report there was nothing to eject", out.String())
	}
}

// A mounted drive that refuses to eject is the only thing that makes the
// command fail, and it never stops the remaining drives.
func TestEjectAllFailsOnlyForAMountedDriveThatRefuses(t *testing.T) {
	configuredDrives(t)
	withAllProfiles(t, true)
	withActiveProfile(t, "")

	runner := &recordingRunner{errs: map[string]error{"/Volumes/Elements": errors.New("exit status 1")}}
	err := ejectWith(&bytes.Buffer{}, ejectDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk"))
	if err == nil || !strings.Contains(err.Error(), "/Volumes/Elements") {
		t.Fatalf("ejectWith() = %v, want the refused drive to fail the command", err)
	}
	if !reflect.DeepEqual(runner.volumes, []string{"/Volumes/Elements", "/Volumes/SanDisk"}) {
		t.Fatalf("ejected %#v, want the failure not to stop the next drive", runner.volumes)
	}
}

// Plain eject is unchanged: it acts on the active profile alone, leaving every
// other configured drive mounted.
func TestPlainEjectStillActsOnTheActiveProfileOnly(t *testing.T) {
	configuredDrives(t)
	withAllProfiles(t, false)
	withActiveProfile(t, "macbook")

	runner := &recordingRunner{}
	if err := ejectWith(&bytes.Buffer{}, ejectDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.volumes, []string{"/Volumes/SanDisk"}) {
		t.Fatalf("ejected %#v, want only the active profile's drive", runner.volumes)
	}
}
