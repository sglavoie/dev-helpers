package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/schedule"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var scheduleCmd = &cobra.Command{
	Use: "schedule", Short: "Generate a macOS backup schedule for review",
	Args: cobra.NoArgs,
}

var scheduleDailyCmd = &cobra.Command{
	Use: "daily", Short: "Print a daily macOS LaunchAgent plist without installing it",
	Long: `Print a daily macOS LaunchAgent plist to stdout for review and installation.
It runs one fixed profile with --non-interactive --quiet at the specified local
time. Configuration is read without prompts or changes; drives may be offline.
The executable, config, HOME, PATH, and log paths are explicit. No shell is used.
Create the log directory before loading the agent. This command creates no
files and never calls launchctl. See docs/scheduling.md for setup and removal.`,
	Args: cobra.NoArgs, SilenceUsage: true,
	RunE: printDailySchedule,
}

func init() {
	scheduleDailyCmd.Flags().String("at", "09:00", "Daily local time in HH:MM format")
	scheduleDailyCmd.Flags().String("label", "local.goback.daily", "LaunchAgent label (use a distinct label for each schedule)")
	scheduleDailyCmd.Flags().String("binary", "", "Absolute path of the installed goback executable (default: this executable)")
	scheduleDailyCmd.Flags().String("exec-path", os.Getenv("PATH"), "Explicit PATH for rsync and companions; colon-separated absolute directories")
	scheduleDailyCmd.Flags().String("log-dir", "", "Absolute directory for stdout/stderr logs (default: $HOME/Library/Logs/goback)")
	scheduleCmd.AddCommand(scheduleDailyCmd)
	RootCmd.AddCommand(scheduleCmd)
}

func printDailySchedule(cmd *cobra.Command, _ []string) error {
	if config.AllProfiles {
		return fmt.Errorf("schedule daily does not support --all; generate one schedule per profile")
	}
	profiles, err := config.SelectProfiles()
	if err != nil {
		return err
	}
	if len(profiles) != 1 {
		return fmt.Errorf("schedule daily requires one profile; select it with --profile NAME")
	}
	config.ActiveProfileName = profiles[0]
	if !buildcmd.IsConfigured("daily") {
		return fmt.Errorf("profile %q has no daily backup configured", profiles[0])
	}
	if buildcmd.IsDryRun("daily") {
		return fmt.Errorf("profile %q configures daily as a dry run; disable rsync.daily.dryRun before scheduling backups", profiles[0])
	}
	for _, key := range []string{"source", "destination"} {
		if !filepath.IsAbs(viper.GetString(config.ActiveProfilePrefix() + key)) {
			return fmt.Errorf("profiles.%s.%s must be an absolute path for scheduling", profiles[0], key)
		}
	}
	companions, err := config.DailyCompanions()
	if err != nil {
		return err
	}
	for _, companion := range companions {
		if program := companion.Command[0]; strings.ContainsRune(program, filepath.Separator) && !filepath.IsAbs(program) {
			return fmt.Errorf("companion %q executable must be an absolute path or a program name resolved through PATH", companion.ID)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binary, _ := cmd.Flags().GetString("binary")
	if binary == "" {
		binary, err = os.Executable()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(binary) {
		return fmt.Errorf("goback binary must be an absolute path")
	}
	info, err := os.Stat(binary)
	if err != nil {
		return fmt.Errorf("goback executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("goback binary must be an executable regular file: %s", binary)
	}
	configPath, err := filepath.Abs(viper.ConfigFileUsed())
	if err != nil {
		return err
	}
	logDir, _ := cmd.Flags().GetString("log-dir")
	if logDir == "" {
		logDir = filepath.Join(home, "Library", "Logs", "goback")
	}
	label, _ := cmd.Flags().GetString("label")
	at, _ := cmd.Flags().GetString("at")
	execPath, _ := cmd.Flags().GetString("exec-path")
	data, err := schedule.Daily(schedule.Options{
		Binary: binary, Config: configPath, Profile: profiles[0], Home: home,
		Label: label, At: at, ExecPath: execPath, LogDir: logDir,
	})
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(data)
	return err
}
