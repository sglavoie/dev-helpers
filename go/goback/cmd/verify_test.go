package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
)

func verifyCommand(t *testing.T, f restoreFixture, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	processArgs := append([]string{"-test.run=^TestProfileCommandProcess$", "--", "--config", f.config, "verify"}, args...)
	command := exec.CommandContext(ctx, os.Args[0], processArgs...)
	command.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1", "HOME="+f.home, "TMPDIR="+f.locks)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func verificationTree(t *testing.T, f restoreFixture) map[string]restoreTreeEntry {
	t.Helper()
	entries := restoreTree(t, f.root)
	for path := range entries {
		if path == f.locks || strings.HasPrefix(path, f.locks+string(filepath.Separator)) {
			delete(entries, path)
		}
	}
	return entries
}

func verifyJSON(t *testing.T, f restoreFixture, wantFailure bool) buildcmd.VerificationReport {
	t.Helper()
	before := verificationTree(t, f)
	stdout, stderr, err := verifyCommand(t, f, "daily", "--json")
	if (err != nil) != wantFailure {
		t.Fatalf("verify: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var report buildcmd.VerificationReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s\nstderr: %s", err, stdout, stderr)
	}
	if strings.Contains(stderr, "Usage:") {
		t.Fatalf("error included usage: %s", stderr)
	}
	if !reflect.DeepEqual(before, verificationTree(t, f)) {
		t.Fatal("verification modified source, backup, configuration, history, or logs")
	}
	return report
}

func requireVerificationRsync(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is required for verification integration tests")
	}
}

func writeVerificationConfig(t *testing.T, f restoreFixture, source string, settings map[string]any) {
	t.Helper()
	content, err := json.Marshal(map[string]any{
		"confirmExec": true, "ejectOnExit": true, "showProgress": true,
		"profiles": map[string]any{"recover": map[string]any{
			"source": source, "destination": f.backup,
			"rsync":           map[string]any{"daily": settings},
			"dailyCompanions": []any{map[string]any{"id": "must-not-run", "command": []string{"goback-test-nonexistent-companion"}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.config, content, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCLIDetectsSameSizeAndTimeCorruptionWithoutChangingAnything(t *testing.T) {
	requireVerificationRsync(t)
	f := restoreCLIFixture(t)
	original := filepath.Join(f.source, "notes.txt")
	if err := os.WriteFile(original, []byte("good data"), 0640); err != nil {
		t.Fatal(err)
	}
	if out, err := restoreCommand(t, f, "run", "daily"); err != nil {
		t.Fatalf("backup: %v\n%s", err, out)
	}
	writeVerificationConfig(t, f, f.source+"/", map[string]any{"archive": true, "delete": true, "ignoreErrors": true, "verbose": true})
	report := verifyJSON(t, f, false)
	if !report.Completed || len(report.Differences) != 0 || report.Profile != "recover" || report.BackupType != "daily" {
		t.Fatalf("matching report: %+v", report)
	}
	backup := filepath.Join(f.backup, "daily", "notes.txt")
	info, err := os.Stat(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("BAD! data"), info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(backup, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(backup)
	if err != nil || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		t.Fatal("fixture did not preserve size and modification time")
	}
	report = verifyJSON(t, f, true)
	want := []buildcmd.VerificationDifference{{Path: "notes.txt", Status: "different"}}
	if !report.Completed || !reflect.DeepEqual(report.Differences, want) || len(report.Errors) != 0 {
		t.Fatalf("corruption report: %+v", report)
	}
	stdout, stderr, err := verifyCommand(t, f, "daily")
	if err == nil || !strings.Contains(stdout, `different: "notes.txt"`) || !strings.Contains(stderr, "1 difference") {
		t.Fatalf("text report: %v\n%s\n%s", err, stdout, stderr)
	}
}

func TestVerifyCLIUsesBackupFiltersAndSourceLayout(t *testing.T) {
	requireVerificationRsync(t)
	for _, suffix := range []string{"/", "", "/."} {
		t.Run("source suffix "+suffix, func(t *testing.T) {
			f := restoreCLIFixture(t)
			for _, dir := range []string{"nested", "empty"} {
				if err := os.Mkdir(filepath.Join(f.source, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"file", "nested/kept.txt", "excluded.tmp", "nested/excluded.tmp", " leading\nspace", `literal\#012`, "-> arrow"} {
				if err := os.WriteFile(filepath.Join(f.source, name), []byte("data"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink("file", filepath.Join(f.source, "link")); err != nil {
				t.Fatal(err)
			}
			source := f.source + suffix
			writeVerificationConfig(t, f, source, map[string]any{"archive": true, "excludedPatterns": []string{"*.tmp"}})
			// Use the real backup command, with confirmation/ejection/companions
			// disabled just for fixture creation.
			data, _ := os.ReadFile(f.config)
			var config map[string]any
			if err := json.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			config["confirmExec"], config["ejectOnExit"] = false, false
			delete(config["profiles"].(map[string]any)["recover"].(map[string]any), "dailyCompanions")
			data, _ = json.Marshal(config)
			if err := os.WriteFile(f.config, data, 0600); err != nil {
				t.Fatal(err)
			}
			if out, err := restoreCommand(t, f, "run", "daily"); err != nil {
				t.Fatalf("backup: %v\n%s", err, out)
			}
			base := filepath.Join(f.backup, "daily")
			prefix := ""
			if suffix == "" {
				prefix = filepath.Base(f.source) + "/"
				base = filepath.Join(base, filepath.Base(f.source))
			}
			// Retained destination-only files and metadata changes are ignored.
			if err := os.WriteFile(filepath.Join(base, "retained"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filepath.Join(base, "file"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(filepath.Join(base, "file"), time.Unix(0, 0), time.Unix(0, 0)); err != nil {
				t.Fatal(err)
			}
			verifyJSON(t, f, false)
			for _, name := range []string{"nested/kept.txt", " leading\nspace", `literal\#012`, "-> arrow", "empty"} {
				if err := os.Remove(filepath.Join(base, name)); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(filepath.Join(base, "link")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("wrong", filepath.Join(base, "link")); err != nil {
				t.Fatal(err)
			}
			report := verifyJSON(t, f, true)
			if !report.Completed || len(report.Differences) != 6 {
				t.Fatalf("missing/changed report: %+v", report)
			}
			for _, difference := range report.Differences {
				want := "missing"
				if difference.Path == prefix+"link" {
					want = "different"
				}
				if difference.Status != want || !strings.HasPrefix(difference.Path, prefix) || strings.Contains(difference.Path, "excluded") {
					t.Fatalf("wrong difference: %+v", difference)
				}
			}
		})
	}
}

func TestVerifyCLIHonorsOrderedIncludesAnchoredExcludesAndPruning(t *testing.T) {
	requireVerificationRsync(t)
	f := restoreCLIFixture(t)
	for _, path := range []string{"keep/important.txt", "keep/also.txt", "keep/omit.txt", "ignored.txt", "empty/.tmp"} {
		full := filepath.Join(f.source, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// First-match rules: the explicit include survives the later exclusion.
	settings := map[string]any{"archive": true, "pruneEmptyDirs": true,
		"includedPatterns": []string{"*/", "/keep/important.txt", "/keep/also.txt"},
		"excludedPatterns": []string{"/keep/important.txt", "/keep/omit.txt"}}
	writeVerificationConfig(t, f, f.source+"/", settings)
	putBackupFile(t, f, "daily", "keep/important.txt", "source")
	putBackupFile(t, f, "daily", "keep/also.txt", "source")
	verifyJSON(t, f, false)
	if err := os.WriteFile(filepath.Join(f.backup, "daily/keep/important.txt"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	report := verifyJSON(t, f, true)
	if !report.Completed || !reflect.DeepEqual(report.Differences, []buildcmd.VerificationDifference{{Path: "keep/important.txt", Status: "different"}}) {
		t.Fatalf("filters changed backup selection: %+v", report)
	}
}

func TestVerifyCLIReportsUnreadableSourceOrBackup(t *testing.T) {
	requireVerificationRsync(t)
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-000 files")
	}
	for _, side := range []string{"source", "backup"} {
		t.Run(side, func(t *testing.T) {
			f := restoreCLIFixture(t)
			original := filepath.Join(f.source, "private")
			if err := os.WriteFile(original, []byte("read this"), 0600); err != nil {
				t.Fatal(err)
			}
			putBackupFile(t, f, "daily", "private", "read this")
			path := original
			if side == "backup" {
				path = filepath.Join(f.backup, "daily", "private")
			}
			if err := os.Chmod(path, 0000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			// The tree snapshot helper cannot read this fixture either.
			stdout, stderr, err := verifyCommand(t, f, "daily", "--json")
			var report buildcmd.VerificationReport
			if err == nil || json.Unmarshal([]byte(stdout), &report) != nil || report.Completed || len(report.Errors) == 0 || !strings.Contains(strings.ToLower(stderr), "permission denied") || !strings.Contains(stderr, "private") || len(report.Differences) != 1 || report.Differences[0].Status != "unreadable" {
				t.Fatalf("unreadable report: %+v, %v\n%s\n%s", report, err, stdout, stderr)
			}
		})
	}
}

func TestVerifyCLIHandlesEmptyAndUnavailableSources(t *testing.T) {
	requireVerificationRsync(t)
	f := restoreCLIFixture(t)
	putBackupFile(t, f, "daily", "retained", "old")
	report := verifyJSON(t, f, false)
	if !report.Completed || len(report.Differences) != 0 {
		t.Fatalf("empty source: %+v", report)
	}
	if err := os.Remove(f.source); err != nil {
		t.Fatal(err)
	}
	report = verifyJSON(t, f, true)
	if report.Completed || len(report.Errors) == 0 {
		t.Fatalf("offline source: %+v", report)
	}
}

func TestVerifyCLIReportsTypeChangesWithoutFollowingBackupLinks(t *testing.T) {
	requireVerificationRsync(t)
	f := restoreCLIFixture(t)
	if err := os.Mkdir(filepath.Join(f.source, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"file", "nested/child"} {
		if err := os.WriteFile(filepath.Join(f.source, path), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	putBackupFile(t, f, "daily", "nested", "wrong type")
	if err := os.Symlink(filepath.Join(f.source, "file"), filepath.Join(f.backup, "daily/file")); err != nil {
		t.Fatal(err)
	}
	report := verifyJSON(t, f, true)
	want := []buildcmd.VerificationDifference{{Path: "file", Status: "different"}, {Path: "nested/", Status: "different"}, {Path: "nested/child", Status: "missing"}}
	if !report.Completed || !reflect.DeepEqual(report.Differences, want) {
		t.Fatalf("type-change report: %+v", report)
	}
}

func TestVerifyCLIWithSystemRsync(t *testing.T) {
	// On macOS this exercises Apple's openrsync, whose itemization is shorter.
	if _, err := os.Stat("/usr/bin/rsync"); err != nil {
		t.Skip("system rsync is not available")
	}
	bin := t.TempDir()
	if err := os.Symlink("/usr/bin/rsync", filepath.Join(bin, "rsync")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	f := restoreCLIFixture(t)
	for _, name := range []string{"file", "line\nbreak", `literal\#012`} {
		if err := os.WriteFile(filepath.Join(f.source, name), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(f.source, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := restoreCommand(t, f, "run", "daily"); err != nil {
		t.Fatalf("system rsync backup: %v\n%s", err, out)
	}
	verifyJSON(t, f, false)
	if err := os.WriteFile(filepath.Join(f.backup, "daily/file"), []byte("FAIL"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.backup, "daily/line\nbreak")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.backup, "daily/empty")); err != nil {
		t.Fatal(err)
	}
	report := verifyJSON(t, f, true)
	want := []buildcmd.VerificationDifference{{Path: "empty/", Status: "missing"}, {Path: "file", Status: "different"}, {Path: "line\nbreak", Status: "missing"}}
	if !report.Completed || !reflect.DeepEqual(report.Differences, want) {
		t.Fatalf("system rsync report: %+v", report)
	}
}

func TestVerifyCLIRequiresOneSelectedProfile(t *testing.T) {
	f := restoreCLIFixture(t)
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	profile := map[string]any{"hostname": hostname, "source": f.source + "/", "destination": f.backup, "rsync": map[string]any{"daily": map[string]any{"archive": true}}}
	content, err := json.Marshal(map[string]any{"profiles": map[string]any{"a": profile, "b": profile}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.config, content, 0600); err != nil {
		t.Fatal(err)
	}
	putBackupFile(t, f, "daily", "retained", "data")
	stdout, stderr, err := verifyCommand(t, f, "daily")
	if err == nil || !strings.Contains(stderr, "verify requires one profile") {
		t.Fatalf("ambiguous profile: %v\n%s\n%s", err, stdout, stderr)
	}
	requireVerificationRsync(t)
	stdout, stderr, err = verifyCommand(t, f, "daily", "--profile", "b", "--json")
	var report buildcmd.VerificationReport
	if err != nil || json.Unmarshal([]byte(stdout), &report) != nil || report.Profile != "b" || !report.Completed {
		t.Fatalf("explicit profile: %v\n%s\n%s", err, stdout, stderr)
	}
}

func TestVerifyCLIRejectsInvalidRequestsAndNeverRepairsConfiguration(t *testing.T) {
	for _, args := range [][]string{nil, {"weekly"}, {"daily", "extra"}, {"daily", "--all"}, {"daily", "--profile", "missing"}} {
		f := restoreCLIFixture(t)
		before := verificationTree(t, f)
		stdout, stderr, err := verifyCommand(t, f, args...)
		if err == nil || strings.Contains(stdout+stderr, "create one") || strings.Contains(stdout+stderr, "recreate") {
			t.Fatalf("invalid request %q: %v\n%s\n%s", args, err, stdout, stderr)
		}
		if !reflect.DeepEqual(before, verificationTree(t, f)) {
			t.Fatalf("invalid request %q modified the filesystem", args)
		}
	}
	for _, content := range []string{"missing", `{"profiles":`, `{"profiels":{}}`} {
		f := restoreCLIFixture(t)
		if content == "missing" {
			if err := os.Remove(f.config); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(f.config, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		before := verificationTree(t, f)
		stdout, stderr, err := verifyCommand(t, f, "daily", "--json")
		if err == nil || strings.Contains(stdout+stderr, "create one") || strings.Contains(stdout+stderr, "recreate") {
			t.Fatalf("configuration error prompted: %v\n%s\n%s", err, stdout, stderr)
		}
		if !reflect.DeepEqual(before, verificationTree(t, f)) {
			t.Fatal("verification created or repaired configuration")
		}
	}
}
