package run

import (
	"context"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"github.com/spf13/viper"
)

type backupAction struct {
	kind string
	run  func(context.Context, *Report) error
}

// AllBackups holds one destination lock across the entire sequence, including
// companions and the intervals between daily and its derived copies.
func AllBackups(ctx context.Context, report *Report) error {
	actions := []backupAction{{"daily", DailyBackup}}
	if buildcmd.IsConfigured("weekly") {
		actions = append(actions, backupAction{"weekly", WeeklyBackup})
	}
	if buildcmd.IsConfigured("monthly") {
		actions = append(actions, backupAction{"monthly", MonthlyBackup})
	}
	for _, action := range actions {
		if !buildcmd.IsDryRun(action.kind) {
			locked, release, err := destinationlock.Acquire(ctx, viper.GetString(config.ActiveProfilePrefix()+"destination"))
			if err != nil {
				return err
			}
			defer release()
			ctx = locked
			break
		}
	}
	return runSequence(ctx, report, actions)
}

func runSequence(ctx context.Context, report *Report, actions []backupAction) error {
	for i, action := range actions {
		before := len(report.Mains)
		err := action.run(ctx, report)
		declined := len(report.Mains) > before && report.Mains[len(report.Mains)-1].Status == MainDeclined
		if err != nil || declined {
			reason := fmt.Sprintf("%s did not complete; remaining backups skipped", action.kind)
			if declined {
				reason = fmt.Sprintf("%s declined; remaining backups skipped", action.kind)
			}
			for _, remaining := range actions[i+1:] {
				report.Mains = append(report.Mains, MainResult{BackupType: remaining.kind, Status: MainSkipped,
					DryRun: buildcmd.IsDryRun(remaining.kind), SkipReason: reason})
			}
			return err
		}
	}
	return nil
}
