package cleanbackup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/inputs"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/viper"
)

// CleanType retroactively applies current exclusion rules to the backup destination
// for the given backup type, prompting the user before deleting any entries.
func CleanType(backupType models.BackupTypes, dryRun bool) error {
	return cleanType(backupType, dryRun, inputs.AskNoYesQuestion, os.RemoveAll)
}

func cleanType(backupType models.BackupTypes, dryRun bool, approve func(string) bool, remove func(string) error) error {
	prefix := config.ActiveProfilePrefix()
	dest := viper.GetString(prefix + "destination")
	if dest == "" {
		return fmt.Errorf("destination not set for profile %q", config.ActiveProfileName)
	}
	scanDir := filepath.Join(dest, backupType.String())

	if err := buildcmd.CheckSourceAccessible(scanDir); err != nil {
		return err
	}

	filters := buildcmd.FilterArgs(backupType)

	if len(filters) == 0 {
		fmt.Printf("No include/exclude patterns configured for %s backups in profile %s\n",
			backupType.String(), config.ActiveProfileName)
		return nil
	}

	excluded, err := buildcmd.FindExcludedWithFilters(scanDir, filters, 0)
	if err != nil {
		return fmt.Errorf("error scanning %s: %w", scanDir, err)
	}

	if len(excluded) == 0 {
		fmt.Printf("No excluded entries found in %s\n", scanDir)
		return nil
	}

	summary := fmt.Sprintf("Found %d excluded entries in %s", len(excluded), scanDir)
	display := make([]string, len(excluded))
	for i, entry := range excluded {
		display[i] = fmt.Sprintf("%q", entry)
	}
	content := summary + "\n\n" + strings.Join(display, "\n")
	printer.Pager(content, "Excluded entries")

	if dryRun {
		fmt.Printf("Dry run: would delete %d entries; deleted 0, failed 0.\n", len(excluded))
		return nil
	}
	if !approve(fmt.Sprintf("Delete %d entries from %s?", len(excluded), scanDir)) {
		fmt.Println("Aborted")
		return nil
	}

	var failures []error
	deleted := 0
	for _, entry := range excluded {
		fullPath := filepath.Join(scanDir, entry)
		if err := remove(fullPath); err != nil {
			failures = append(failures, fmt.Errorf("delete %q: %w", fullPath, err))
			continue
		}
		deleted++
		fmt.Printf("Deleted: %q\n", fullPath)
	}
	fmt.Printf("Cleanup complete: deleted %d, failed %d.\n", deleted, len(failures))
	return errors.Join(failures...)
}

// CleanAll runs CleanType for daily, weekly (if configured), and monthly (if configured).
func CleanAll(dryRun bool) error {
	if err := CleanType(models.Daily{}, dryRun); err != nil {
		return err
	}
	if buildcmd.IsConfigured("weekly") {
		if err := CleanType(models.Weekly{}, dryRun); err != nil {
			return err
		}
	}
	if buildcmd.IsConfigured("monthly") {
		if err := CleanType(models.Monthly{}, dryRun); err != nil {
			return err
		}
	}
	return nil
}
