package config

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"os"
)

func Print() {
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return
	}
	b, _ := json.MarshalIndent(viper.AllSettings(), "", "  ")
	fmt.Print(string(b) + "\n")
}

func PrintRaw() {
	file, err := os.Open(viper.ConfigFileUsed())
	if err != nil {
		cobra.CheckErr(err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			cobra.CheckErr(err)
		}
	}(file)

	_, err = io.Copy(os.Stdout, file)
}
