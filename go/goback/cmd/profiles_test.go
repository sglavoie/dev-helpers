package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Run the actual command tree in a fresh process so os.Exit, Cobra flags, and
// Viper globals behave exactly as they do in the installed CLI.
func TestProfileCommandProcess(t *testing.T) {
	if os.Getenv("GOBACK_TEST_COMMAND") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		os.Exit(2)
	}
	RootCmd.SetArgs(os.Args[separator+1:])
	Execute()
	os.Exit(0)
}

func profileCommand(t *testing.T, content string, args ...string) (string, error) {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, "fixture.json")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	processArgs := append([]string{"-test.run=^TestProfileCommandProcess$", "--", "--config", configPath}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], processArgs...)
	cmd.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1", "HOME="+home)
	output, err := cmd.CombinedOutput()
	after, readErr := os.ReadFile(configPath)
	if readErr != nil || string(after) != content {
		t.Fatalf("command changed its configuration: %v", readErr)
	}
	return string(output), err
}

func TestProfileSelectionThroughCommandExecution(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	profile := func(host string, media bool) map[string]any {
		return map[string]any{"hostname": host, "backupMedia": media, "source": "/fixture-source", "destination": "/fixture-destination"}
	}
	tests := []struct {
		name     string
		profiles map[string]any
		flags    []string
		want     []string
		absent   []string
		wantErr  string
	}{
		{name: "sole mismatched hostname", profiles: map[string]any{"default": profile(hostname+".different", false)}, want: []string{"Profile:     default"}},
		{name: "sole missing hostname", profiles: map[string]any{"default": profile("", false)}, want: []string{"Profile:     default"}},
		{name: "hostname match", profiles: map[string]any{"machine": profile(hostname, false), "other": profile("", false)}, want: []string{"Profile:     machine"}, absent: []string{"Profile:     other"}},
		{name: "explicit selection", profiles: map[string]any{"machine": profile(hostname, false), "other": profile("", false)}, flags: []string{"--profile", "other"}, want: []string{"Profile:     other"}, absent: []string{"Profile:     machine"}},
		{name: "all profiles", profiles: map[string]any{"a": profile("", false), "b": profile("", false)}, flags: []string{"--all"}, want: []string{"Profile:     a", "Profile:     b"}},
		{name: "shared media", profiles: map[string]any{"machine": profile(hostname, true), "media": profile("", false), "remote": profile(hostname+".different", false)}, want: []string{"Profile:     machine", "Profile:     media"}, absent: []string{"Profile:     remote"}},
		{name: "empty profiles", profiles: map[string]any{}, wantErr: "no backup profiles configured"},
		{name: "empty profiles with all", profiles: map[string]any{}, flags: []string{"--all"}, wantErr: "no backup profiles configured"},
		{name: "ambiguous profiles", profiles: map[string]any{"a": profile("", false), "b": profile("", false)}, wantErr: "could not auto-detect profile"},
		{name: "unknown profile", profiles: map[string]any{"default": profile("", false)}, flags: []string{"--profile", "missing"}, wantErr: `profile "missing" not found`},
		{name: "conflicting flags", profiles: map[string]any{"default": profile("", false)}, flags: []string{"--profile", "default", "--all"}, wantErr: "mutually exclusive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := json.Marshal(map[string]any{"profiles": tt.profiles, "ejectOnExit": false})
			if err != nil {
				t.Fatal(err)
			}
			output, err := profileCommand(t, string(content), append([]string{"preview", "daily"}, tt.flags...)...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(output, tt.wantErr) {
					t.Fatalf("error = %v, output = %q, want failure containing %q", err, output, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("command failed: %v\n%s", err, output)
			}
			for _, want := range tt.want {
				if strings.Count(output, want) != 1 {
					t.Fatalf("output = %q, want exactly one %q", output, want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(output, absent) {
					t.Fatalf("output = %q, want no %q", output, absent)
				}
			}
		})
	}
}

func TestRunDailyReportsFailureForSoleUnmatchedProfile(t *testing.T) {
	// Deliberately omit the source so reaching the backup fails before any
	// filesystem writes or child commands. Previously this silently exited 0.
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"", hostname + ".different"} {
		content, err := json.Marshal(map[string]any{"profiles": map[string]any{"default": map[string]any{"hostname": host}}})
		if err != nil {
			t.Fatal(err)
		}
		output, err := profileCommand(t, string(content), "run", "daily")
		if err == nil || !strings.Contains(output, `source not set for profile "default"`) || !strings.Contains(output, "skipped") {
			t.Fatalf("hostname = %q, error = %v, output = %q, want a reported backup failure", host, err, output)
		}
	}
}

func TestMisplacedCompanionsFailButConfigRepairRemainsAccessible(t *testing.T) {
	content := `{"profiles":{"default":{}},"dailyCompanions":[{"id":"photos","command":["photos-backup","daily"]}]}`
	output, err := profileCommand(t, content, "run", "daily")
	for _, want := range []string{"top-level dailyCompanions", "fixture.json", "profiles.<name>.dailyCompanions"} {
		if err == nil || !strings.Contains(output, want) {
			t.Fatalf("error = %v, output = %q, want failure naming %q", err, output, want)
		}
	}
	output, err = profileCommand(t, content, "config", "print", "--raw")
	if err != nil || strings.TrimSpace(output) != content {
		t.Fatalf("config print failed: %v\n%s", err, output)
	}
	customConfig(t, content)
	if err := RootCmd.PersistentPreRunE(editCmd, nil); err != nil {
		t.Fatalf("config edit pre-run rejected a repairable config: %v", err)
	}
}

func TestPreviewShowsCompanionsInTheirProfile(t *testing.T) {
	content := `{"profiles":{"default":{"dailyCompanions":[{"id":"photos","command":["photos-backup","daily"],"dryRunArgs":["--dry-run"]}]}}}`
	output, err := profileCommand(t, content, "preview", "daily")
	if err != nil || !strings.Contains(output, "photos-backup daily") || !strings.Contains(output, "photos-backup daily --dry-run") {
		t.Fatalf("preview failed to show companions: %v\n%s", err, output)
	}
}

func TestGlobalCommandsDoNotRequireProfiles(t *testing.T) {
	output, err := profileCommand(t, `{"profiles":{}}`, "eject", "--all")
	if err != nil || !strings.Contains(output, "No configured volume") {
		t.Fatalf("eject --all failed: %v\n%s", err, output)
	}
	output, err = profileCommand(t, `{"profiles":{}}`, "mirror", "--dry-run")
	if err == nil || !strings.Contains(output, "mirror.source") || strings.Contains(output, "no backup profiles") {
		t.Fatalf("mirror failed at profile selection: %v\n%s", err, output)
	}
}
