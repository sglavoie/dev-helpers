package eject

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/viper"
)

// fakeRunner records every argument vector it is handed and answers from the
// tables, so a test can prove both what diskutil was asked to do and what it
// was never asked to do.
type fakeRunner struct {
	argvs  [][]string
	output map[string]string
	errs   map[string]error
}

func (r *fakeRunner) Run(argv []string) (string, error) {
	r.argvs = append(r.argvs, argv)
	volume := argv[len(argv)-1]
	return r.output[volume], r.errs[volume]
}

// volumes returns the volume of each recorded call, in call order.
func (r *fakeRunner) volumes() []string {
	var called []string
	for _, argv := range r.argvs {
		called = append(called, argv[len(argv)-1])
	}
	return called
}

type fakeMounts struct {
	mounted map[string]bool
}

func (m fakeMounts) Mounted(volume string) bool { return m.mounted[volume] }

func fakeDeps(runner *fakeRunner, mounted ...string) Deps {
	set := make(map[string]bool, len(mounted))
	for _, volume := range mounted {
		set[volume] = true
	}
	return Deps{Runner: runner, Mounts: fakeMounts{mounted: set}}
}

func TestAllRunsDiskutilEjectOnEveryMountedVolume(t *testing.T) {
	runner := &fakeRunner{}
	deps := fakeDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")

	report := All([]string{"/Volumes/SanDisk/macbook", "/Volumes/Elements/Media"}, deps)

	want := [][]string{
		{"diskutil", "eject", "/Volumes/Elements"},
		{"diskutil", "eject", "/Volumes/SanDisk"},
	}
	if !reflect.DeepEqual(runner.argvs, want) {
		t.Fatalf("diskutil calls = %#v, want %#v", runner.argvs, want)
	}
	if report.Failed() {
		t.Fatalf("report failed, want success: %+v", report.Outcomes)
	}
	if err := report.Err(); err != nil {
		t.Fatal(err)
	}
}

// SanDisk is both a destination of one profile and the source of the mirror,
// and Elements is both a profile destination and the mirror destination, yet
// each drive may only be ejected once.
func TestAllEjectsEachConfiguredVolumeExactlyOnce(t *testing.T) {
	runner := &fakeRunner{}
	deps := fakeDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")

	endpoints := []string{
		"/Users/me",
		"/Volumes/SanDisk/macbook",
		"/Volumes/SanDisk/Media",
		"/Volumes/Elements/media",
		"/Volumes/Elements/Media",
	}
	report := All(endpoints, deps)

	want := []string{"/Volumes/Elements", "/Volumes/SanDisk"}
	if got := runner.volumes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ejected %#v, want %#v", got, want)
	}
	if len(report.Outcomes) != 2 {
		t.Fatalf("report has %d outcomes, want 2: %+v", len(report.Outcomes), report.Outcomes)
	}
}

// diskutil must never see an internal path, whatever the configuration says.
func TestAllNeverPassesAPathOutsideVolumesToDiskutil(t *testing.T) {
	runner := &fakeRunner{}
	deps := Deps{Runner: runner, Mounts: alwaysMounted{}}

	report := All([]string{"/Users/me", "/tmp/backups", "/", "/Volumes", "/VolumesBackup/drive", "relative"}, deps)

	if len(runner.argvs) != 0 {
		t.Fatalf("diskutil was called with %#v, want no call at all", runner.argvs)
	}
	if len(report.Outcomes) != 0 {
		t.Fatalf("report has %+v, want no outcome", report.Outcomes)
	}
	if err := report.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil when nothing was ejectable", err)
	}
}

type alwaysMounted struct{}

func (alwaysMounted) Mounted(string) bool { return true }

// A drive that is already unplugged is in the state the caller asked for.
func TestAllSkipsAbsentVolumesWithoutFailing(t *testing.T) {
	runner := &fakeRunner{}
	deps := fakeDeps(runner, "/Volumes/SanDisk")

	report := All([]string{"/Volumes/SanDisk/macbook", "/Volumes/Elements/Media"}, deps)

	if got := runner.volumes(); !reflect.DeepEqual(got, []string{"/Volumes/SanDisk"}) {
		t.Fatalf("ejected %#v, want only the mounted volume", got)
	}

	absent := report.Outcomes[0]
	if absent.Volume != "/Volumes/Elements" || absent.Status != StatusAbsent {
		t.Fatalf("outcome = %+v, want /Volumes/Elements absent", absent)
	}
	if report.Failed() || report.Err() != nil {
		t.Fatalf("an absent volume failed the report: %v", report.Err())
	}

	ejected, skipped, failed := report.Counts()
	if ejected != 1 || skipped != 1 || failed != 0 {
		t.Fatalf("counts = %d ejected, %d skipped, %d failed, want 1/1/0", ejected, skipped, failed)
	}
}

// One drive refusing to eject must not leave the others mounted.
func TestAllAttemptsEveryMountedVolumeAfterAFailure(t *testing.T) {
	runner := &fakeRunner{
		errs:   map[string]error{"/Volumes/Elements": errors.New("exit status 1")},
		output: map[string]string{"/Volumes/Elements": "Unmount of disk4 failed: at least one volume is in use\n"},
	}
	deps := fakeDeps(runner, "/Volumes/Backup", "/Volumes/Elements", "/Volumes/SanDisk")

	report := All([]string{"/Volumes/SanDisk/a", "/Volumes/Elements/b", "/Volumes/Backup/c"}, deps)

	want := []string{"/Volumes/Backup", "/Volumes/Elements", "/Volumes/SanDisk"}
	if got := runner.volumes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ejected %#v, want every mounted volume attempted: %#v", got, want)
	}
	if !report.Failed() {
		t.Fatal("report succeeded, want the refused volume to fail it")
	}

	err := report.Err()
	if err == nil || !strings.Contains(err.Error(), "/Volumes/Elements") {
		t.Fatalf("Err() = %v, want it to name /Volumes/Elements", err)
	}
	if strings.Contains(err.Error(), "/Volumes/SanDisk") {
		t.Fatalf("Err() = %v, want only the failed volume named", err)
	}

	ejected, skipped, failed := report.Counts()
	if ejected != 2 || skipped != 0 || failed != 1 {
		t.Fatalf("counts = %d ejected, %d skipped, %d failed, want 2/0/1", ejected, skipped, failed)
	}
}

// Every mounted volume failing is still one attempt each.
func TestAllReportsEveryFailure(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{
		"/Volumes/Elements": errors.New("exit status 1"),
		"/Volumes/SanDisk":  errors.New("exit status 1"),
	}}
	deps := fakeDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")

	report := All([]string{"/Volumes/SanDisk/a", "/Volumes/Elements/b"}, deps)

	if err := report.Err(); err == nil || !strings.Contains(err.Error(), "/Volumes/Elements, /Volumes/SanDisk") {
		t.Fatalf("Err() = %v, want both volumes named", err)
	}
}

func TestRenderShowsEveryOutcomeAndTheCounts(t *testing.T) {
	runner := &fakeRunner{
		errs:   map[string]error{"/Volumes/Elements": errors.New("exit status 1")},
		output: map[string]string{"/Volumes/Elements": "Unmount of disk4 failed\n"},
	}
	deps := fakeDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")

	var out bytes.Buffer
	All([]string{"/Volumes/SanDisk/a", "/Volumes/Elements/b", "/Volumes/Backup/c"}, deps).Render(&out)

	for _, want := range []string{
		"/Volumes/Backup: not mounted, skipped",
		"/Volumes/Elements: failed to eject: exit status 1",
		"Unmount of disk4 failed",
		"/Volumes/SanDisk: ejected",
		"1 ejected, 1 skipped, 1 failed",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want it to contain %q", out.String(), want)
		}
	}
}

func TestRenderSaysSoWhenNothingIsEjectable(t *testing.T) {
	var out bytes.Buffer
	Report{}.Render(&out)

	if !strings.Contains(out.String(), "No configured volume under /Volumes") {
		t.Fatalf("output = %q, want it to report there was nothing to eject", out.String())
	}
}

func setProfileDestination(t *testing.T, destination string) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)

	previous := config.ActiveProfileName
	config.ActiveProfileName = "macbook"
	t.Cleanup(func() { config.ActiveProfileName = previous })

	viper.Set("profiles.macbook.destination", destination)
}

// Plain eject still acts on the active profile's destination alone.
func TestEjectUsesTheActiveProfileDestination(t *testing.T) {
	setProfileDestination(t, "/Volumes/SanDisk/macbook")
	runner := &fakeRunner{}

	if err := Eject(io.Discard, fakeDeps(runner, "/Volumes/SanDisk")); err != nil {
		t.Fatal(err)
	}
	if got := runner.volumes(); !reflect.DeepEqual(got, []string{"/Volumes/SanDisk"}) {
		t.Fatalf("ejected %#v, want only the active profile's volume", got)
	}
}

func TestEjectWithoutADestinationRefusesBeforeDiskutil(t *testing.T) {
	setProfileDestination(t, "")
	runner := &fakeRunner{}

	err := Eject(io.Discard, fakeDeps(runner, "/Volumes/SanDisk"))
	if err == nil || err.Error() != "destination not set" {
		t.Fatalf("Eject() = %v, want it refused for a missing destination", err)
	}
	if len(runner.argvs) != 0 {
		t.Fatalf("diskutil was called with %#v, want no call", runner.argvs)
	}
}

// A profile backing up to an internal path has no drive to eject, and asking
// diskutil about it is exactly what must never happen.
func TestEjectRefusesADestinationOutsideVolumes(t *testing.T) {
	setProfileDestination(t, "/Users/me/backups")
	runner := &fakeRunner{}

	err := Eject(io.Discard, fakeDeps(runner, "/Volumes/SanDisk"))
	if err == nil || !strings.Contains(err.Error(), "/Volumes") {
		t.Fatalf("Eject() = %v, want it refused as not being on a volume", err)
	}
	if len(runner.argvs) != 0 {
		t.Fatalf("diskutil was called with %#v, want no call", runner.argvs)
	}
}

func TestEjectReportsAnAbsentVolumeAsSuccess(t *testing.T) {
	setProfileDestination(t, "/Volumes/SanDisk/macbook")
	runner := &fakeRunner{}

	var out bytes.Buffer
	if err := Eject(&out, fakeDeps(runner)); err != nil {
		t.Fatalf("Eject() = %v, want an unplugged drive to be no failure", err)
	}
	if !strings.Contains(out.String(), "not mounted, skipped") {
		t.Fatalf("output = %q, want the skip reported", out.String())
	}
}

// The snapshot ejectOnExit path keeps ejecting each destination volume once.
func TestEjectPathsEjectsEachDestinationVolumeOnce(t *testing.T) {
	runner := &fakeRunner{}
	deps := fakeDeps(runner, "/Volumes/Elements", "/Volumes/SanDisk")

	destinations := []string{"/Volumes/SanDisk/macbook", "/Volumes/SanDisk/media", "/Volumes/Elements/media"}
	if err := EjectPaths(io.Discard, destinations, deps); err != nil {
		t.Fatal(err)
	}

	want := []string{"/Volumes/Elements", "/Volumes/SanDisk"}
	if got := runner.volumes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ejected %#v, want %#v", got, want)
	}
}

func TestEjectPathsReportsAFailure(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{"/Volumes/SanDisk": errors.New("exit status 1")}}

	err := EjectPaths(io.Discard, []string{"/Volumes/SanDisk/macbook"}, fakeDeps(runner, "/Volumes/SanDisk"))
	if err == nil || !strings.Contains(err.Error(), "/Volumes/SanDisk") {
		t.Fatalf("EjectPaths() = %v, want the failure reported", err)
	}
}
