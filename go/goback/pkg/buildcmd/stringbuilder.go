package buildcmd

import (
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
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

// CommandString is a shell-safe representation for display and copy/paste only.
// Execution uses args directly and never asks a shell to interpret configuration.
func (r *builder) CommandString() string {
	quoted := make([]string, len(r.args))
	for i, arg := range r.args {
		if arg != "" && strings.IndexFunc(arg, func(c rune) bool {
			return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_@%+=:,./-", c))
		}) == -1 {
			quoted[i] = arg
		} else {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
	}
	return strings.Join(quoted, " ")
}

func (r *builder) appendBooleanFlags() {
	r.args = append(r.args, r.getFlags()...)
}

func (r *builder) appendIncludedPatterns() {
	patterns := r.getIncludePatterns()
	for _, pattern := range patterns {
		r.args = append(r.args, "--include="+pattern)
	}
	r.hasIncludePatterns = len(patterns) > 0
}

func (r *builder) appendExcludedPatterns() {
	for _, pattern := range r.mergedExcludePatterns() {
		r.args = append(r.args, "--exclude="+pattern)
	}
	if r.hasIncludePatterns {
		r.args = append(r.args, "--exclude=*")
	}
}

// mergedExcludePatterns returns the exclude patterns for this backup type.
// For weekly and monthly backups, the daily patterns are merged in so that
// items excluded after the fact in the daily config are also pruned from
// derived backups.
func (r *builder) mergedExcludePatterns() []string {
	cfgPrefix := r.builderSettingsPrefix()
	patterns := viper.GetStringSlice(cfgPrefix + "excludedPatterns")

	switch r.builderType.(type) {
	case models.Weekly, models.Monthly:
		dailyPrefix := config.ActiveProfilePrefix() + "rsync.daily."
		dailyPatterns := viper.GetStringSlice(dailyPrefix + "excludedPatterns")
		patterns = MergeUnique(patterns, dailyPatterns)
	}

	return patterns
}

// MergeUnique appends items from extra into base, skipping duplicates.
func MergeUnique(base, extra []string) []string {
	seen := make(map[string]struct{}, len(base))
	for _, p := range base {
		seen[p] = struct{}{}
	}
	for _, p := range extra {
		if _, ok := seen[p]; !ok {
			base = append(base, p)
			seen[p] = struct{}{}
		}
	}
	return base
}

func (r *builder) appendSrcDest() {
	r.args = append(r.args, "--", r.updatedSrc, r.updatedDestDir)
}
