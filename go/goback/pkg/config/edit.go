package config

import (
	"github.com/spf13/viper"
	"goback/pkg/editor"
)

func Edit() {
	editor.OpenFileWithEditor(viper.ConfigFileUsed())
}
