package buildcmd

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

func (r *builder) CommandString() string {
	return r.sb.String()
}

// addRawSourceDestination reads a source and a destination from the config file.
// It currently handles a single mapping for simplicity.
func (r *builder) addRawSourceDestination() {
	src, dest := mustExitOnInvalidSourceOrDestination()
	r.sb.WriteString(" ")
	r.sb.WriteString(src)
	r.sb.WriteString(" ")
	r.sb.WriteString(dest)
}

func (r *builder) appendBooleanFlags() {
	cfgPrefix := r.builderSettingsPrefix()
	if viper.GetBool(cfgPrefix + "archive") {
		r.sb.WriteString(" --archive")
	}
	if viper.GetBool(cfgPrefix + "delete") {
		r.sb.WriteString(" --delete")
	}
	if viper.GetBool(cfgPrefix + "deleteExcluded") {
		r.sb.WriteString(" --delete-excluded")
	}
	if viper.GetBool(cfgPrefix + "dryRun") {
		r.sb.WriteString(" --dry-run")
	}
	if viper.GetBool(cfgPrefix + "force") {
		r.sb.WriteString(" --force")
	}
	if viper.GetBool(cfgPrefix + "hardLinks") {
		r.sb.WriteString(" --hard-links")
	}
	if viper.GetBool(cfgPrefix + "ignoreErrors") {
		r.sb.WriteString(" --ignore-errors")
	}
	if viper.GetBool(cfgPrefix + "pruneEmptyDirs") {
		r.sb.WriteString(" --prune-empty-dirs")
	}
	if viper.GetBool(cfgPrefix + "verbose") {
		r.sb.WriteString(" --verbose")
	}

	if viper.GetBool("showProgress") {
		r.sb.WriteString(" --progress")
	}
}

func (r *builder) appendExcludedPatterns() {
	cfgPrefix := r.builderSettingsPrefix()
	for _, pattern := range viper.GetStringSlice(cfgPrefix + "excludedPatterns") {
		r.sb.WriteString(" --exclude=\"")
		r.sb.WriteString(pattern)
		r.sb.WriteString("\"")
	}
}

func (r *builder) appendLogFile() {
	r.sb.WriteString(" --log-file=")
	r.sb.WriteString(strings.TrimSuffix(viper.ConfigFileUsed(), ".json"))
	r.sb.WriteString("_")
	r.sb.WriteString(r.builderType.String())
	r.sb.WriteString("_")
	r.sb.WriteString(time.Now().Format("20060102_15_04_05"))
}

func (r *builder) appendSrcDest() {
	r.sb.WriteString(" ")
	r.sb.WriteString(r.updatedSrc)
	r.sb.WriteString(" ")
	r.sb.WriteString(r.updatedDestDir)
}
