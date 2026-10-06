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
	var rows []statusRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err)
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
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Profile != "beta" {
		t.Fatalf("filtered rows: %+v", rows)
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
	if len(rows) != 4 || rows[0].Freshness != "stale" || rows[0].Result != "failed (exit 23)" || *rows[0].LastSuccess != old || rows[1].Result != "interrupted" || rows[1].Freshness != "no recorded success" || rows[2].Freshness != "current" {
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
