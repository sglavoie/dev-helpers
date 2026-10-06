package config

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/editor"
	"github.com/spf13/viper"
)

func Edit() {
	// Read the editor preference when possible, but malformed configuration
	// must remain editable using the environment's editor.
	_ = viper.ReadInConfig()
	editor.OpenFileWithEditor(viper.ConfigFileUsed())
}
