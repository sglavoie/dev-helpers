package config

import "github.com/spf13/viper"

func setDefaultValues() {
	viper.Set("confirmExec", true)
	viper.Set("ejectOnExit", false)
	viper.Set("showProgress", true)
	viper.Set("editor", "")
	for _, backupType := range []string{"daily", "weekly", "monthly"} {
		prefix := "profiles.default.rsync." + backupType + "."
		viper.Set(prefix+"archive", true)
		viper.Set(prefix+"hardLinks", true)
		viper.Set(prefix+"delete", false)
		viper.Set(prefix+"ignoreErrors", false)
		viper.Set(prefix+"deleteExcluded", false)
		patterns := []string{}
		if backupType == "daily" {
			patterns = []string{".DS_Store", "*.tmp"}
		}
		viper.Set(prefix+"excludedPatterns", patterns)
	}
	setMirror()
}

func setMirror() {
	viper.Set(MirrorKey+".source", DefaultMirrorSource)
	viper.Set(MirrorKey+".destination", DefaultMirrorDestination)
}
