package buildcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
)

func TestDerivedSourceHistoryShowsEarlierSuccessAndLatestFailure(t *testing.T) {
	old := time.Now().Add(-9*24*time.Hour - time.Minute).Format("2006-01-02 15:04:05")
	for _, code := range []int{0, 23, -1} {
		out := dailyHistoryDescription([]db.SummaryRow{
			{Profile: "other", BackupType: "daily", LastSuccess: nil},
			{Profile: "test", BackupType: "weekly", LastSuccess: nil},
			{Profile: "test", BackupType: "daily", LastSuccess: &old, ExitCode: code, LatestAttempt: "2026-10-06 12:00:00"},
		}, "test")
		if !strings.Contains(out, old) || !strings.Contains(out, "9 days ago") {
			t.Fatal(out)
		}
		if strings.Contains(out, "partially updated") != (code != 0) {
			t.Fatal(out)
		}
		if code == -1 && !strings.Contains(out, "interrupted") {
			t.Fatal(out)
		}
	}
	out := dailyHistoryDescription(nil, "test")
	if !strings.Contains(out, "never recorded") {
		t.Fatal(out)
	}
	out = dailyHistoryDescription([]db.SummaryRow{{Profile: "test", BackupType: "daily", ExitCode: 23}}, "test")
	if !strings.Contains(out, "never recorded") || !strings.Contains(out, "partially updated") {
		t.Fatal(out)
	}
}

func TestSourceHistoryIsReadOnlyAndUnavailableHistoryDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	previous := config.ActiveProfileName
	config.ActiveProfileName = "test"
	t.Cleanup(func() { config.ActiveProfileName = previous })
	for _, b := range []*RsyncBuilder{commandToRunWeekly("", "/offline"), commandToRunMonthly("", "/offline")} {
		if out := b.dailySourceHistory(); !strings.Contains(out, "never recorded") {
			t.Fatal(out)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("history lookup created files", entries, err)
	}
	if err := os.WriteFile(filepath.Join(home, ".goback.db"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if out := commandToRunWeekly("", "/offline").dailySourceHistory(); !strings.Contains(out, "history unavailable") {
		t.Fatal(out)
	}
	if out := commandToRunDaily("/offline", "/offline").dailySourceHistory(); out != "" {
		t.Fatal(out)
	}
}
