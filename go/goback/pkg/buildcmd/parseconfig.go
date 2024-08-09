package buildcmd

import (
	"github.com/spf13/viper"
	"strings"
	"time"
)

func addBooleanFlags(sb *strings.Builder) {
	if viper.GetBool("rsyncFlags.archive") {
		sb.WriteString(" --archive")
	}
	if viper.GetBool("rsyncFlags.delete") {
		sb.WriteString(" --delete")
	}
	if viper.GetBool("rsyncFlags.deleteExcluded") {
		sb.WriteString(" --delete-excluded")
	}
	if viper.GetBool("rsyncFlags.dryRun") {
		sb.WriteString(" --dry-run")
	}
	if viper.GetBool("rsyncFlags.force") {
		sb.WriteString(" --force")
	}
	if viper.GetBool("rsyncFlags.hardLinks") {
		sb.WriteString(" --hard-links")
	}
	if viper.GetBool("rsyncFlags.ignoreErrors") {
		sb.WriteString(" --ignore-errors")
	}
	if viper.GetBool("rsyncFlags.pruneEmptyDirs") {
		sb.WriteString(" --prune-empty-dirs")
	}
	if viper.GetBool("rsyncFlags.verbose") {
		sb.WriteString(" --verbose")
	}

	if viper.GetBool("showProgress") {
		sb.WriteString(" --progress")
	}
}

func addLogFile(sb *strings.Builder) {
	sb.WriteString(" --log-file=")
	sb.WriteString(strings.TrimSuffix(viper.ConfigFileUsed(), ".json"))
	sb.WriteString("_" + time.Now().Format("20060102_15_04_05"))
}

func addExcludedPatterns(sb *strings.Builder) {
	for _, pattern := range viper.GetStringSlice("excludedPatterns") {
		sb.WriteString(" --exclude=\"")
		sb.WriteString(pattern)
		sb.WriteString("\"")
	}
}

func addUpdatedSourceDestination(sb *strings.Builder, src, dest string) {
	sb.WriteString(" ")
	sb.WriteString(src)
	sb.WriteString(" ")
	sb.WriteString(dest)
}

// addRawSourceDestination reads a source and a destination from the config file.
// It currently handles a single mapping for simplicity.
func addRawSourceDestination(sb *strings.Builder) {
	src, dest := exitOnInvalidSourceOrDestination()
	sb.WriteString(" ")
	sb.WriteString(src)
	sb.WriteString(" ")
	sb.WriteString(dest)
}
