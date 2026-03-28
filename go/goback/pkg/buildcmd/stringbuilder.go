package buildcmd

import (
	"github.com/spf13/viper"
)

// booleanFlags maps config keys to rsync flag strings for flags that map
// directly without CLI override logic.
var booleanFlags = []struct {
	configKey string
	flag      string
}{
	{"archive", "--archive"},
	{"delete", "--delete"},
	{"deleteExcluded", "--delete-excluded"},
	{"force", "--force"},
	{"hardLinks", "--hard-links"},
	{"ignoreErrors", "--ignore-errors"},
	{"pruneEmptyDirs", "--prune-empty-dirs"},
}

func isDryRun() bool {
	return viper.GetBool("cliDryRun")
}

func (r *builder) CommandString() string {
	return r.sb.String()
}

func (r *builder) appendBooleanFlags() {
	cfgPrefix := r.builderSettingsPrefix()

	for _, f := range booleanFlags {
		if viper.GetBool(cfgPrefix + f.configKey) {
			r.sb.WriteString(" " + f.flag)
		}
	}

	if viper.GetBool(cfgPrefix+"dryRun") || isDryRun() {
		r.sb.WriteString(" --dry-run")
	}
	if viper.GetBool(cfgPrefix+"verbose") || viper.GetBool("cliVerbose") {
		r.sb.WriteString(" --verbose")
	}

	quiet := viper.GetBool("cliQuiet")
	if viper.GetBool("showProgress") && !quiet {
		r.sb.WriteString(" --progress")
	}

	verbose := viper.GetBool(cfgPrefix+"verbose") || viper.GetBool("cliVerbose")
	if !quiet && (verbose || isDryRun()) {
		r.sb.WriteString(" --stats")
	}
}

func (r *builder) appendIncludedPatterns() {
	cfgPrefix := r.builderSettingsPrefix()
	patterns := viper.GetStringSlice(cfgPrefix + "includedPatterns")
	for _, pattern := range patterns {
		r.sb.WriteString(" --include=\"")
		r.sb.WriteString(pattern)
		r.sb.WriteString("\"")
	}
	r.hasIncludePatterns = len(patterns) > 0
}

func (r *builder) appendExcludedPatterns() {
	cfgPrefix := r.builderSettingsPrefix()
	for _, pattern := range viper.GetStringSlice(cfgPrefix + "excludedPatterns") {
		r.sb.WriteString(" --exclude=\"")
		r.sb.WriteString(pattern)
		r.sb.WriteString("\"")
	}
	// When include patterns are used, add a final --exclude="*" to exclude everything else
	if r.hasIncludePatterns {
		r.sb.WriteString(" --exclude=\"*\"")
	}
}

func (r *builder) appendSrcDest() {
	r.sb.WriteString(" '")
	r.sb.WriteString(r.updatedSrc)
	r.sb.WriteString("' '")
	r.sb.WriteString(r.updatedDestDir)
	r.sb.WriteString("'")
}
