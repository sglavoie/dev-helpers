package config

import (
	"encoding/json"
	"fmt"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Print() {
	errRead := viper.ReadInConfig()
	cobra.CheckErr(errRead)
	err := viper.Unmarshal(&cfg)
	cobra.CheckErr(err)
	b, _ := json.MarshalIndent(viper.AllSettings(), "", "  ")
	printer.Pager(string(b)+"\n", viper.ConfigFileUsed())
}

func PrintRaw() {
	file, err := os.Open(viper.ConfigFileUsed())
	cobra.CheckErr(err)
	defer func(file *os.File) {
		errClose := file.Close()
		cobra.CheckErr(errClose)
	}(file)

	_, err = io.Copy(os.Stdout, file)
	cobra.CheckErr(err)
	addNewlineAtTheEndIfMissing(file)
}

func addNewlineAtTheEndIfMissing(file *os.File) {
	_, err := file.Seek(-1, 2)
	cobra.CheckErr(err)
	lastByte := make([]byte, 1)
	_, err = file.Read(lastByte)
	cobra.CheckErr(err)
	if lastByte[0] != '\n' {
		fmt.Println()
	}
}
