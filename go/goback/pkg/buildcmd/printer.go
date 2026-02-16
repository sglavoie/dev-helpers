package buildcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/inputs"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/viper"
)

func (r *builder) PrintCommandToRunWithConfirmation() {
	fmt.Println("The following command will be executed:", "\n", r.CommandString())

	if viper.GetBool("confirmExec") {
		confirms := inputs.AskYesNoQuestion("\nDo you wish to proceed?")
		if !confirms {
			os.Exit(0)
		}
	}
}
func (r *builder) PrintString() {
	fmt.Println(r.sb.String())
}

func (r *builder) WrapLongLinesWithBackslashes() {
	printer.WrapLongLinesWithBackslashes(r.sb, 80)
}

// getFlags returns the list of enabled rsync flag strings (e.g. "--archive").
func (r *builder) getFlags() []string {
	cfgPrefix := r.builderSettingsPrefix()
	var flags []string

	flagMap := []struct {
		key  string
		flag string
	}{
		{"archive", "--archive"},
		{"delete", "--delete"},
		{"deleteExcluded", "--delete-excluded"},
		{"dryRun", "--dry-run"},
		{"force", "--force"},
		{"hardLinks", "--hard-links"},
		{"ignoreErrors", "--ignore-errors"},
		{"pruneEmptyDirs", "--prune-empty-dirs"},
		{"verbose", "--verbose"},
	}

	for _, f := range flagMap {
		if viper.GetBool(cfgPrefix + f.key) {
			flags = append(flags, f.flag)
		}
	}

	if viper.GetBool("showProgress") {
		flags = append(flags, "--progress")
	}

	return flags
}

// getIncludePatterns returns the configured include patterns for this backup type.
func (r *builder) getIncludePatterns() []string {
	cfgPrefix := r.builderSettingsPrefix()
	return viper.GetStringSlice(cfgPrefix + "includedPatterns")
}

// getExcludePatterns returns the configured exclude patterns for this backup type.
func (r *builder) getExcludePatterns() []string {
	cfgPrefix := r.builderSettingsPrefix()
	return viper.GetStringSlice(cfgPrefix + "excludedPatterns")
}

// FormattedPreview prints a structured preview of the rsync command.
func (r *builder) FormattedPreview() {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Backup type: %s\n", r.builderType.String()))
	sb.WriteString(fmt.Sprintf("Profile:     %s\n", config.ActiveProfileName))

	flags := r.getFlags()
	if len(flags) > 0 {
		sb.WriteString("\nFlags:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", wrapFlags(flags, 70)))
	}

	includes := r.getIncludePatterns()
	if len(includes) > 0 {
		sb.WriteString("\nInclude patterns:\n")
		for _, p := range includes {
			sb.WriteString(fmt.Sprintf("  %s\n", p))
		}
	}

	excludes := r.getExcludePatterns()
	if len(excludes) > 0 {
		sb.WriteString("\nExclude patterns:\n")
		for _, p := range excludes {
			sb.WriteString(fmt.Sprintf("  %s\n", p))
		}
	}

	sb.WriteString(fmt.Sprintf("\nSource:      %s\n", r.updatedSrc))
	sb.WriteString(fmt.Sprintf("Destination: %s\n", r.updatedDestDir))

	// Full command at the bottom for copy-paste
	r.WrapLongLinesWithBackslashes()
	sb.WriteString(fmt.Sprintf("\nFull command:\n  %s\n", strings.ReplaceAll(r.sb.String(), "\n", "\n  ")))

	fmt.Print(sb.String())
}

// wrapFlags joins flags into lines that don't exceed maxWidth characters.
func wrapFlags(flags []string, maxWidth int) string {
	var lines []string
	var current string

	for _, f := range flags {
		if current == "" {
			current = f
		} else if len(current)+1+len(f) > maxWidth {
			lines = append(lines, current)
			current = f
		} else {
			current += " " + f
		}
	}
	if current != "" {
		lines = append(lines, current)
	}

	return strings.Join(lines, "\n  ")
}
