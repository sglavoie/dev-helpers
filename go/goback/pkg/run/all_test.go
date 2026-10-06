package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"github.com/spf13/viper"
)

// Both rsync and a daily companion probe the lock from another process. The
// companion runs between the main daily command and the derived copies.
func TestAllBackupsHoldsLockAcrossTransfersAndCompanions(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "backup")
	for _, dir := range []string{source, dest} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "data"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	useCompanions(t, map[string]any{"id": "lock-check", "command": []string{os.Args[0], "-test.run=^TestDestinationLockHelper$", "--", "companion"}})
	viper.Set("profiles.test.source", source+"/")
	viper.Set("profiles.test.destination", dest)
	for _, kind := range []string{"daily", "weekly", "monthly"} {
		viper.Set("profiles.test.rsync."+kind+".archive", true)
	}
	t.Setenv("GOBACK_LOCK_DEST", dest)
	t.Setenv("GOBACK_LOCK_TEST_BIN", os.Args[0])
	if err := os.WriteFile(filepath.Join(root, "rsync"), []byte("#!/bin/sh\nexec \"$GOBACK_LOCK_TEST_BIN\" -test.run=^TestDestinationLockHelper$ -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	report := &Report{}
	if err := AllBackups(context.Background(), report); err != nil {
		t.Fatal(err)
	}
	if len(report.Mains) != 3 || len(report.Companions) != 1 || !report.CompletedBackup() {
		t.Fatalf("report: %+v", report)
	}
	_, release, err := destinationlock.Acquire(context.Background(), dest)
	if err != nil {
		t.Fatalf("lock leaked after sequence: %v", err)
	}
	release()
}

func TestDestinationLockHelper(t *testing.T) {
	dest := os.Getenv("GOBACK_LOCK_DEST")
	if dest == "" {
		return
	}
	if _, release, err := destinationlock.Acquire(context.Background(), dest); err == nil {
		release()
		fmt.Fprintln(os.Stderr, "destination was not locked")
		os.Exit(1)
	}
	last := os.Args[len(os.Args)-1]
	if last != "companion" {
		if err := os.WriteFile(filepath.Join(last, "data"), []byte("copied"), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func TestDecliningStopsOnlyTheCurrentSequence(t *testing.T) {
	for _, declined := range []string{"daily", "weekly"} {
		t.Run(declined, func(t *testing.T) {
			var actions []backupAction
			var called []string
			for _, kind := range []string{"daily", "weekly", "monthly"} {
				kind := kind
				actions = append(actions, backupAction{kind, func(ctx context.Context, report *Report) error {
					called = append(called, kind)
					status := MainSucceeded
					if kind == declined {
						status = MainDeclined
					}
					report.Mains = append(report.Mains, MainResult{BackupType: kind, Status: status})
					return nil
				}})
			}
			report := &Report{}
			if err := runSequence(context.Background(), report, actions); err != nil {
				t.Fatal(err)
			}
			if called[len(called)-1] != declined || len(report.Mains) != 3 {
				t.Fatalf("called %v, report %+v", called, report)
			}
			for _, result := range report.Mains[len(called):] {
				if result.Status != MainSkipped || !strings.Contains(mainDetails(result), declined+" declined") {
					t.Fatalf("missing skip explanation: %+v", result)
				}
			}
		})
	}
}
