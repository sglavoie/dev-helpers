package cmd

import (
	"log"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/cobra"
)

// previewCmd simply prints the rsync command that would be executed
var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Print the rsync command that would be executed",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

func init() {
	previewCmd.PersistentFlags().String("test-pattern", "", "Test a single exclude pattern against the source directory")
	previewCmd.PersistentFlags().Bool("excluded", false, "Test all configured exclude patterns against the source directory")
	previewCmd.PersistentFlags().String("subdir", "", "Scope pattern testing to a subdirectory of the source (requires --test-pattern or --excluded)")
	previewCmd.PersistentFlags().Int("depth", 0, "Limit directory scan depth for pattern testing (requires --test-pattern or --excluded)")

	previewCmd.AddCommand(dailyCmdPreview)
	previewCmd.AddCommand(weeklyCmdPreview)
	previewCmd.AddCommand(monthlyCmdPreview)
	RootCmd.AddCommand(previewCmd)
}

func runPreview(cmd *cobra.Command, buildFn func(), backupType models.BackupTypes) {
	testPattern, _ := cmd.Flags().GetString("test-pattern")
	showExcluded, _ := cmd.Flags().GetBool("excluded")
	subdir, _ := cmd.Flags().GetString("subdir")
	depth, _ := cmd.Flags().GetInt("depth")

	forEachProfile(func() {
		switch {
		case testPattern != "" && showExcluded:
			log.Fatal("--test-pattern and --excluded are mutually exclusive")
		case subdir != "" && testPattern == "" && !showExcluded:
			log.Fatal("--subdir requires --test-pattern or --excluded")
		case depth > 0 && testPattern == "" && !showExcluded:
			log.Fatal("--depth requires --test-pattern or --excluded")
		case testPattern != "":
			buildcmd.TestSinglePattern(backupType, testPattern, subdir, depth)
		case showExcluded:
			buildcmd.TestAllExcluded(backupType, subdir, depth)
		default:
			buildFn()
		}
	})
}

var dailyCmdPreview = &cobra.Command{
	Use:   "daily",
	Short: "Preview command for daily backup",
	Run: func(cmd *cobra.Command, args []string) {
		runPreview(cmd, buildcmd.PrintCommandDaily, models.Daily{})
	},
}

var weeklyCmdPreview = &cobra.Command{
	Use:   "weekly",
	Short: "Preview command for weekly backup",
	Run: func(cmd *cobra.Command, args []string) {
		runPreview(cmd, buildcmd.PrintCommandWeekly, models.Weekly{})
	},
}

var monthlyCmdPreview = &cobra.Command{
	Use:   "monthly",
	Short: "Preview command for monthly backup",
	Run: func(cmd *cobra.Command, args []string) {
		runPreview(cmd, buildcmd.PrintCommandMonthly, models.Monthly{})
	},
}
