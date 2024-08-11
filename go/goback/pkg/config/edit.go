package config

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/editor"
	"github.com/spf13/viper"
)

func Edit() {
	editor.OpenFileWithEditor(viper.ConfigFileUsed())
}
