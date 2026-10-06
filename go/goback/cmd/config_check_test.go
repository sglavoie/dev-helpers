package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCheck(t *testing.T) {
	const validProfile = `{"source":"/Volumes/Offline/source/","destination":"/Volumes/Offline/backup","rsync":{"daily":{"archive":true}}}`
	for _, tc := range []struct{ name, content, want string }{
		{"offline", `{"profiles":{"a":` + validProfile + `,"b":` + validProfile + `}}`, "Configuration OK:"},
		{"mirror only", `{"mirror":{"source":"/offline/source","destination":"/offline/dest"}}`, "Configuration OK:"},
		{"weekly without original source", `{"profiles":{"a":{"destination":"/offline/dest","rsync":{"weekly":{"archive":false}}}}}`, "Configuration OK:"},
		{"typo", `{"profiles":{"a":{"rsync":{"daily":{"archive":true,"excludedPaterns":["*.tmp"]}}}}}`, "excludedpaterns"},
		{"boolean type", `{"confirmExec":"false"}`, "confirmexec"},
		{"pattern type", `{"profiles":{"a":{"rsync":{"daily":{"archive":true,"excludedPatterns":"*.tmp"}}}}}`, "excludedpatterns"},
		{"pattern element type", `{"profiles":{"a":{"rsync":{"daily":{"archive":true,"excludedPatterns":[1]}}}}}`, "excludedpatterns"},
		{"backup type", `{"profiles":{"a":{"rsync":{"daliy":{"archive":true}}}}}`, "unknown backup type"},
		{"missing source", `{"profiles":{"a":{"destination":"/offline/dest","rsync":{"daily":{"archive":true}}}}}`, "profiles.a.source is required"},
		{"missing archive", `{"profiles":{"a":{"source":"/offline/src","destination":"/offline/dest","rsync":{"daily":{"delete":false}}}}}`, "profiles.a.rsync.daily.archive is required"},
		{"missing destination", `{"profiles":{"a":{"rsync":{"weekly":{"archive":true}}}}}`, "profiles.a.destination is required"},
		{"empty", `{}`, "configure at least one"},
		{"empty misspelled object", `{"profiels":{}}`, "profiels"},
		{"empty profile", `{"profiles":{"a":{}}}`, "profiles.a.destination is required"},
		{"empty backup", `{"profiles":{"a":{"rsync":{"daily":{}}}}}`, "profiles.a.rsync.daily.archive is required"},
		{"null boolean", `{"confirmExec":null}`, "must not be null"},
		{"null pattern", `{"profiles":{"a":{"rsync":{"daily":{"archive":true,"excludedPatterns":[null]}}}}}`, "must not be null"},
		{"incomplete mirror", `{"mirror":{"source":"/offline/src"}}`, "mirror.destination is required"},
		{"malformed", `{"profiles":`, "configuration"},
		{"invalid companion", `{"profiles":{"a":{"dailyCompanions":[{"id":"x","command":["echo"],"dryRunArg":["--dry-run"]}]}}}`, "unknown key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := profileCommand(t, tc.content, "config", "check")
			if (err == nil) != (tc.want == "Configuration OK:") || !strings.Contains(strings.ToLower(out), strings.ToLower(tc.want)) {
				t.Fatalf("check: %v\n%s", err, out)
			}
			if strings.Contains(out, "recreate") || strings.Contains(out, "Is the drive mounted") {
				t.Fatalf("check prompted or accessed endpoints: %s", out)
			}
		})
	}
}

func TestConfigCheckMissingFileDoesNotCreateAnything(t *testing.T) {
	home := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestProfileCommandProcess$", "--", "config", "check")
	command.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1", "HOME="+home)
	out, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(out), filepath.Join(home, ".goback.json")) || strings.Contains(string(out), "create one") {
		t.Fatalf("missing file: %v\n%s", err, out)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("check wrote to home: %v, %v", entries, err)
	}
}

func TestBackupCommandsRejectConfigurationTyposBeforeExecution(t *testing.T) {
	content := `{"profiles":{"a":{"source":"/offline/src","destination":"/offline/dest","rsync":{"daily":{"archive":true,"excludedPaterns":["*.tmp"]}}}}}`
	for _, args := range [][]string{{"run", "daily"}, {"run", "daily", "--dry-run"}, {"preview", "daily"}, {"preview", "daily", "--excluded"}, {"mirror", "--dry-run"}, {"clean", "backup", "daily"}} {
		out, err := profileCommand(t, content, args...)
		if err == nil || !strings.Contains(out, "excludedpaterns") || strings.Contains(out, "will be executed") {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	out, err := profileCommand(t, content, "config", "print", "--raw", "--no-pager")
	if err != nil || !strings.Contains(out, "excludedPaterns") {
		t.Fatalf("repair access: %v\n%s", err, out)
	}
}
