package cleanbackup

import (
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
func CleanType(backupType models.BackupTypes) error {
	prefix := config.ActiveProfilePrefix()
	dest := viper.GetString(prefix + "destination")
	scanDir := dest + "/" + backupType.String() + "/"

	if err := buildcmd.CheckSourceAccessible(scanDir); err != nil {
		return err
	}

	patterns := viper.GetStringSlice(prefix + "rsync." + backupType.String() + ".excludedPatterns")

	switch backupType.(type) {
	case models.Weekly, models.Monthly:
		dailyPatterns := viper.GetStringSlice(prefix + "rsync.daily.excludedPatterns")
		patterns = buildcmd.MergeUnique(patterns, dailyPatterns)
	}

	if len(patterns) == 0 {
		fmt.Printf("No exclude patterns configured for %s backups in profile %s\n",
			backupType.String(), config.ActiveProfileName)
		return nil
	}

	excluded, err := buildcmd.FindExcludedRoots(scanDir, patterns, 0)
	if err != nil {
		return fmt.Errorf("error scanning %s: %w", scanDir, err)
	}

	if len(excluded) == 0 {
		fmt.Printf("No excluded entries found in %s\n", scanDir)
		return nil
	}

	summary := fmt.Sprintf("Found %d excluded entries in %s", len(excluded), scanDir)
	content := summary + "\n\n" + strings.Join(excluded, "\n")
	printer.Pager(content, "Excluded entries")

	if !inputs.AskNoYesQuestion(fmt.Sprintf("Delete %d entries from %s?", len(excluded), scanDir)) {
		fmt.Println("Aborted")
		return nil
	}

	for _, entry := range excluded {
		fullPath := filepath.Join(scanDir, entry)
		if err := os.RemoveAll(fullPath); err != nil {
			fmt.Printf("Error deleting %s: %v\n", fullPath, err)
			continue
		}
		fmt.Printf("Deleted: %s\n", fullPath)
	}

	return nil
}

// CleanAll runs CleanType for daily, weekly (if configured), and monthly (if configured).
func CleanAll() error {
	if err := CleanType(models.Daily{}); err != nil {
		return err
	}
	if buildcmd.IsConfigured("weekly") {
		if err := CleanType(models.Weekly{}); err != nil {
			return err
		}
	}
	if buildcmd.IsConfigured("monthly") {
		if err := CleanType(models.Monthly{}); err != nil {
			return err
		}
	}
	return nil
}
