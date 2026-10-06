package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/run"
	"github.com/spf13/viper"
)

func TestPreviewDoesNotEject(t *testing.T) {
	content := `{"ejectOnExit":true,"profiles":{"test":{"source":"/fixture","destination":"/fixture-backup","rsync":{"daily":{"archive":true}}}}}`
	output, err := profileCommand(t, content, "preview", "daily")
	if err != nil {
		t.Fatalf("preview: %v\n%s", err, output)
	}
	if strings.Contains(output, "eject") {
		t.Fatalf("preview attempted ejection:\n%s", output)
	}
}

func TestAutomaticEjectionRequiresCompletedBackups(t *testing.T) {
	for _, tc := range []struct {
		name      string
		main      run.MainResult
		wantEject bool
	}{
		{"success", run.MainResult{Status: run.MainSucceeded}, true},
		{"dry run", run.MainResult{Status: run.MainSucceeded, DryRun: true}, false},
		{"declined", run.MainResult{Status: run.MainDeclined}, false},
		{"skipped", run.MainResult{Status: run.MainSkipped}, false},
		{"failed", run.MainResult{Status: run.MainFailed, Err: errors.New("failed")}, false},
		{"interrupted", run.MainResult{Status: run.MainInterrupted, Err: errors.New("interrupted")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			oldProfile, oldActive, oldAll := config.ProfileFlag, config.ActiveProfileName, config.AllProfiles
			config.ProfileFlag, config.ActiveProfileName, config.AllProfiles = "", "", false
			t.Cleanup(func() {
				config.ProfileFlag, config.ActiveProfileName, config.AllProfiles = oldProfile, oldActive, oldAll
			})
			viper.Set("profiles.test.destination", "/fixture-backup")
			viper.Set("ejectOnExit", true)
			var ejected []string
			err := runBackupProfiles(context.Background(), func(_ context.Context, report *run.Report) error {
				report.Mains = append(report.Mains, tc.main)
				return tc.main.Err
			}, func(paths []string) error { ejected = append(ejected, paths...); return nil })
			if !errors.Is(err, tc.main.Err) {
				t.Fatalf("error = %v, want %v", err, tc.main.Err)
			}
			if tc.wantEject && !slices.Equal(ejected, []string{"/fixture-backup"}) {
				t.Fatalf("ejected = %v", ejected)
			}
			if !tc.wantEject && len(ejected) != 0 {
				t.Fatalf("ejected = %v", ejected)
			}
		})
	}
}
