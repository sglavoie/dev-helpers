package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/viper"
)

const overviewConfig = `{"profiles":{
 "alpha":{"hostname":"unmatched-alpha","source":"/offline/source","destination":"/offline/backup","rsync":{"daily":{"archive":true},"monthly":{"archive":true}}},
 "beta":{"hostname":"unmatched-beta","destination":"/offline/other","rsync":{"weekly":{"archive":true}}}
},"mirror":{"source":"/offline/mirror-in","destination":"/offline/mirror-out"}}`

func TestOverviewWorksOfflineWithoutChangingConfigOrCreatingHistory(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config.json")
	if err := os.WriteFile(path, []byte(overviewConfig), 0600); err != nil {
		t.Fatal(err)
	}
	out := globalCommand(t, home, "--config", path, "profiles")
	for _, want := range []string{"alpha", "beta", "AUTO-SELECTED", "Automatic selection unavailable", "/offline/source"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	out = globalCommand(t, home, "--config", path, "status", "--json")
	var report statusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	rows := report.Backups
	if report.DBPath != filepath.Join(home, ".goback.db") || report.DBExists {
		t.Fatalf("history: %+v", report)
	}
	if len(rows) != 4 {
		t.Fatalf("rows: %+v", rows)
	}
	for _, row := range rows {
		if row.Result != "never run" || row.LastSuccess != nil || row.LatestAttempt != nil || row.ExitCode != nil {
			t.Fatalf("row: %+v", row)
		}
	}
	out = globalCommand(t, home, "--config", path, "status", "--profile", "beta", "--json")
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	rows = report.Backups
	if len(rows) != 1 || rows[0].Profile != "beta" {
		t.Fatalf("filtered rows: %+v", rows)
	}
	out = globalCommand(t, home, "--config", path, "status")
	if !strings.Contains(out, "History: "+filepath.Join(home, ".goback.db")+" (no backups recorded yet)") {
		t.Fatalf("missing history line: %s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".goback.db")); !os.IsNotExist(err) {
		t.Fatal("overview created history")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != overviewConfig {
		t.Fatal("overview changed configuration")
	}
}

func TestStatusPreservesFailureAndLastSuccessWhileMarkingStale(t *testing.T) {
	customConfig(t, overviewConfig)
	previous := config.ProfileFlag
	config.ProfileFlag = ""
	t.Cleanup(func() { config.ProfileFlag = previous })
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	old := now.Add(-72 * time.Hour).Format("2006-01-02 15:04:05")
	recent := now.Add(-time.Hour).Format("2006-01-02 15:04:05")
	rows, err := configuredStatus(config.ProfileNames(), []db.SummaryRow{
		{Profile: "alpha", BackupType: "daily", LatestAttempt: recent, ExitCode: 23, LastSuccess: &old},
		{Profile: "alpha", BackupType: "monthly", LatestAttempt: recent, ExitCode: -1},
		{Profile: "beta", BackupType: "weekly", LatestAttempt: recent, LastSuccess: &recent},
		{Profile: "retired", BackupType: "daily", LatestAttempt: recent, LastSuccess: &recent},
	}, now, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	oldISO := now.Add(-72 * time.Hour).Format(time.RFC3339)
	if len(rows) != 4 || rows[0].Freshness != "stale" || rows[0].Result != "failed (exit 23)" || *rows[0].LastSuccess != oldISO || *rows[0].lastSuccessRaw != old || rows[1].Result != "interrupted" || rows[1].Freshness != "no recorded success" || rows[2].Freshness != "current" {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestProfilesMarksTheSameDefaultSelectionAsRuns(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	content := strings.ReplaceAll(overviewConfig, "unmatched-alpha", hostname)
	customConfig(t, content)
	viper.Set("profiles.alpha.backupMedia", true)
	viper.Set("profiles.beta.hostname", "")
	selected, err := config.DefaultProfiles()
	if err != nil || strings.Join(selected, ",") != "alpha,beta" {
		t.Fatalf("selected %v: %v", selected, err)
	}
}

func TestOverviewRejectsInvalidFlagsAndConfigWithoutPrompts(t *testing.T) {
	for _, args := range [][]string{{"status", "--older-than", "-1h"}, {"status", "--older-than", "0s"}, {"profiles", "--profile", "missing"}, {"status", "--profile", "alpha", "--all"}} {
		if out, err := profileCommand(t, overviewConfig, args...); err == nil {
			t.Fatalf("accepted %v: %s", args, out)
		}
	}
	for _, cmd := range []string{"profiles", "status"} {
		out, err := profileCommand(t, `{"profiles":`, cmd)
		if err == nil || strings.Contains(out, "recreate") {
			t.Fatalf("%s: %v, %s", cmd, err, out)
		}
	}
}

func TestStatusJSONReadsRecordedHistoryReadOnlyWithLocalOffsets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "config.json")
	if err := os.WriteFile(path, []byte(overviewConfig), 0600); err != nil {
		t.Fatal(err)
	}
	daily := time.Date(2026, 10, 6, 11, 21, 0, 0, time.Local)
	db.RecordBackup(db.HistoryEntry{CreatedAt: daily, Profile: "alpha", BackupType: "daily"})
	db.RecordBackup(db.HistoryEntry{CreatedAt: daily.Add(time.Hour), Profile: "alpha", BackupType: "daily", ExitCode: 23})
	db.RecordBackup(db.HistoryEntry{CreatedAt: daily, Profile: db.MirrorProfile, BackupType: "mirror", ExitCode: -1})
	history := filepath.Join(home, ".goback.db")
	before, err := os.ReadFile(history)
	if err != nil {
		t.Fatal(err)
	}
	out := globalCommand(t, home, "--config", path, "status", "--json")
	var report statusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if report.DBPath != history || !report.DBExists {
		t.Fatalf("history: %+v", report)
	}
	if _, err := time.Parse(time.RFC3339, report.GeneratedAt); err != nil {
		t.Fatalf("generated_at %q: %v", report.GeneratedAt, err)
	}
	byType := map[string]statusRow{}
	for _, row := range report.Backups {
		byType[row.Profile+"/"+row.BackupType] = row
	}
	alpha := byType["alpha/daily"]
	if alpha.LastSuccess == nil || *alpha.LastSuccess != daily.Format(time.RFC3339) ||
		alpha.LatestAttempt == nil || *alpha.LatestAttempt != daily.Add(time.Hour).Format(time.RFC3339) ||
		alpha.ExitCode == nil || *alpha.ExitCode != 23 {
		t.Fatalf("alpha/daily: %+v", alpha)
	}
	if !strings.HasSuffix(*alpha.LastSuccess, daily.Format("-07:00")) && !strings.HasSuffix(*alpha.LastSuccess, "Z") {
		t.Fatalf("timestamp lacks the local offset: %s", *alpha.LastSuccess)
	}
	mirror := byType["global/mirror"]
	if mirror.Result != "interrupted" || mirror.LastSuccess != nil {
		t.Fatalf("mirror: %+v", mirror)
	}
	if monthly := byType["alpha/monthly"]; monthly.Result != "never run" || monthly.LatestAttempt != nil {
		t.Fatalf("monthly: %+v", monthly)
	}
	after, err := os.ReadFile(history)
	if err != nil || string(after) != string(before) {
		t.Fatal("status changed the history database")
	}
	text := globalCommand(t, home, "--config", path, "status")
	if !strings.Contains(text, "History: "+history+"\n") || !strings.Contains(text, "2026-10-06 11:21:00") {
		t.Fatalf("table: %s", text)
	}
}

func TestStatusErrorsOmitTheUsageText(t *testing.T) {
	out, err := profileCommand(t, `{"profiles":{"alpha":{"rsync":{"daily":{"archive":true,"retired":true}}}}}`, "status", "--json")
	if err == nil || strings.Contains(out, "Usage:") || !strings.Contains(out, "retired") {
		t.Fatalf("%v: %s", err, out)
	}
}
