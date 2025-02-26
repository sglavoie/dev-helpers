package config

import "github.com/spf13/viper"

func setDefaultValues() {
	viper.Set("confirmExec", true)
	viper.Set("ejectOnExit", false)
	setDaily()
	setWeekly()
}

func setDaily() {
	viper.Set("rsync.daily.archive", true)

	// do not lose recent work
	viper.Set("rsync.daily.delete", false)
	viper.Set("rsync.daily.ignoreErrors", true)

	viper.Set("rsync.daily.deleteExcluded", true)
	viper.Set("rsync.daily.hardLinks", true)
	viper.Set("rsync.daily.pruneEmptyDirs", true)
	viper.Set("rsync.daily.verbose", false)

	viper.Set("rsync.daily.excludedPatterns", []string{})
}

func setWeekly() {
	viper.Set("rsync.weekly.archive", true)

	// keep only latest snapshot from daily
	viper.Set("rsync.weekly.delete", true) // !
	viper.Set("rsync.weekly.ignoreErrors", true)

	viper.Set("rsync.weekly.deleteExcluded", true)
	viper.Set("rsync.weekly.hardLinks", true)
	viper.Set("rsync.weekly.pruneEmptyDirs", true)
	viper.Set("rsync.weekly.verbose", false)

	viper.Set("rsync.weekly.excludedPatterns", []string{})
}
