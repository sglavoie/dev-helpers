package config

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
)

func Print() {
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return
	}
	b, _ := json.MarshalIndent(viper.AllSettings(), "", "  ")
	fmt.Print(string(b) + "\n")
}
