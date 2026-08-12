package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// customConfig writes a throwaway configuration file and points --config at it,
// which is what an invocation that must not touch the real ~/.goback.json does.
func customConfig(t *testing.T, content string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "custom.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)

	previous := config.CfgFile
	config.CfgFile = path
	t.Cleanup(func() { config.CfgFile = previous })

	if err := config.MustInitConfig(true, true); err != nil {
		t.Fatal(err)
	}
	return path
}

const customMirrorConfig = `{
	"mirror": {"source": "/Volumes/Custom In/Media", "destination": "/Volumes/Custom Out/Media"},
	"profiles": {"macbook": {"source": "/Users/me", "destination": "/Volumes/Custom Out/macbook"}}
}`

// A custom configuration is the only thing that decides what gets mirrored: the
// compiled defaults exist to generate a configuration, never to stand in for a
// missing one.
func TestMirrorReadsACustomConfigFileWithoutRewritingIt(t *testing.T) {
	path := customConfig(t, customMirrorConfig)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	parseFlags(t, mirrorCmd, "--dry-run")

	err = runMirror(mirrorCmd)
	if err == nil {
		t.Fatal("the mirror ran against an absent drive, want it refused")
	}
	if !strings.Contains(err.Error(), "/Volumes/Custom In/Media") {
		t.Fatalf("error = %q, want it to name the configured source", err)
	}
	if strings.Contains(err.Error(), config.DefaultMirrorSource) {
		t.Fatalf("error = %q, want the compiled default never used as a fallback", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("the custom configuration was rewritten:\n%s", after)
	}
}

// eject --all reads the same custom configuration, so a throwaway config file
// is enough to exercise the whole aggregation without a drive in sight.
func TestEjectAllFollowsACustomConfigFile(t *testing.T) {
	customConfig(t, customMirrorConfig)
	withAllProfiles(t, true)
	withActiveProfile(t, "")

	runner := &recordingRunner{}
	var out bytes.Buffer
	if err := ejectWith(&out, ejectDeps(runner, "/Volumes/Custom In", "/Volumes/Custom Out")); err != nil {
		t.Fatal(err)
	}

	want := []string{"/Volumes/Custom In", "/Volumes/Custom Out"}
	if !reflect.DeepEqual(runner.volumes, want) {
		t.Fatalf("ejected %#v, want %#v", runner.volumes, want)
	}
}

// The help is the only place a user reads what a mirror does before running
// one, so it has to state the destructive part, the confirmation, and the fact
// that nothing is unmounted afterwards.
func TestMirrorHelpStatesTheDestructiveContract(t *testing.T) {
	for _, want := range []string{
		"deleting whatever the destination holds that the source does not",
		"--dry-run",
		"confirmation that defaults to No",
		"Neither drive is ever ejected",
	} {
		if !strings.Contains(mirrorCmd.Long, want) {
			t.Fatalf("mirror help = %q, want it to mention %q", mirrorCmd.Long, want)
		}
	}

	flag := mirrorCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("goback mirror declares no --dry-run flag")
	}
	if !strings.Contains(flag.Usage, "without writing anything") {
		t.Fatalf("--dry-run usage = %q, want it to say nothing is written", flag.Usage)
	}
}

// eject --all is the only command that acts on drives it was not pointed at, so
// its help has to say which ones and when it fails.
func TestEjectHelpStatesWhatAllCovers(t *testing.T) {
	for _, want := range []string{
		"each profile's source and destination and the mirror's endpoints",
		"restricted to paths under /Volumes",
		"A volume that is not mounted is skipped without failing",
		"exits nonzero only when a mounted volume could not be ejected",
	} {
		if !strings.Contains(ejectCmd.Long, want) {
			t.Fatalf("eject help = %q, want it to mention %q", ejectCmd.Long, want)
		}
	}
}

// Verbose and quiet contradict each other, and cobra is what enforces it.
func TestRootRejectsVerboseWithQuiet(t *testing.T) {
	parseFlags(t, RootCmd, "--verbose", "--quiet")

	err := RootCmd.ValidateFlagGroups()
	if err == nil {
		t.Fatal("--verbose with --quiet was accepted, want the pair rejected")
	}
	for _, want := range []string{"verbose", "quiet"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to name --%s", err, want)
		}
	}
}

// walkCommands returns every registered command, subcommands included.
func walkCommands(cmd *cobra.Command) []*cobra.Command {
	commands := []*cobra.Command{cmd}
	for _, child := range cmd.Commands() {
		commands = append(commands, walkCommands(child)...)
	}
	return commands
}

// Only the mirror opts out of profile resolution outright, and only eject opts
// out conditionally. Anything else silently opting out would make a snapshot
// command run without the profile it reads its paths from.
func TestOnlyGlobalCommandsSkipProfileResolution(t *testing.T) {
	for _, all := range []bool{false, true} {
		withAllProfiles(t, all)

		for _, cmd := range walkCommands(RootCmd) {
			path := cmd.CommandPath()
			want := true
			switch {
			case path == "goback mirror":
				want = false
			case path == "goback eject":
				want = !all
			}
			if got := needsProfileResolution(cmd); got != want {
				t.Fatalf("needsProfileResolution(%q) = %v with --all=%v, want %v", path, got, all, want)
			}
		}
	}
}

// goback mirror is registered on the root command, which is the only thing
// making it reachable at all.
func TestMirrorIsRegisteredOnce(t *testing.T) {
	var found int
	for _, cmd := range RootCmd.Commands() {
		if cmd.Name() == "mirror" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("goback mirror is registered %d times, want exactly once", found)
	}
}
