package config

import "github.com/spf13/viper"

func setDefaultValues() {
	viper.Set("rsyncFlags.archive", true)
	viper.Set("rsyncFlags.delete", true)
	viper.Set("rsyncFlags.deleteExcluded", true)
	viper.Set("rsyncFlags.dryRun", true)
	viper.Set("rsyncFlags.force", true)
	viper.Set("rsyncFlags.hardLinks", true)
	viper.Set("rsyncFlags.ignoreErrors", true)
	viper.Set("rsyncFlags.pruneEmptyDirs", true)
	viper.Set("rsyncFlags.verbose", false)
	viper.Set("excludedPatterns", []string{})
	viper.Set("srcDest", map[string]string{})
}
