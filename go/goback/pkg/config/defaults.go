package config

import "github.com/spf13/viper"

func setDefaultValues() {
	viper.Set("confirmExec", true)
	viper.Set("ejectOnExit", false)
	viper.Set("showProgress", false)
	viper.Set("editor", "")
	setDaily()
	setWeekly()
}

func setDaily() {
	prefix := "profiles.default.rsync.daily."
	viper.Set(prefix+"archive", true)
	viper.Set(prefix+"delete", false)
	viper.Set(prefix+"ignoreErrors", true)
	viper.Set(prefix+"deleteExcluded", true)
	viper.Set(prefix+"hardLinks", true)
	viper.Set(prefix+"pruneEmptyDirs", true)
	viper.Set(prefix+"verbose", false)
	viper.Set(prefix+"excludedPatterns", []string{})
}

func setWeekly() {
	prefix := "profiles.default.rsync.weekly."
	viper.Set(prefix+"archive", true)
	viper.Set(prefix+"delete", true)
	viper.Set(prefix+"ignoreErrors", true)
	viper.Set(prefix+"deleteExcluded", true)
	viper.Set(prefix+"hardLinks", true)
	viper.Set(prefix+"pruneEmptyDirs", true)
	viper.Set(prefix+"verbose", false)
	viper.Set(prefix+"excludedPatterns", []string{})
}
