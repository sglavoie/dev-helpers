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
	errRead := viper.ReadInConfig()
	if errRead != nil {
		cobra.CheckErr(errRead)
	}
	err := viper.Unmarshal(&cfg)
	if err != nil {
		cobra.CheckErr(err)
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
	if err != nil {
		cobra.CheckErr(err)
	}
	addNewlineAtTheEndIfMissing(file)
}

func addNewlineAtTheEndIfMissing(file *os.File) {
	_, err := file.Seek(-1, 2)
	if err != nil {
		cobra.CheckErr(err)
		return
	}
	lastByte := make([]byte, 1)
	_, err = file.Read(lastByte)
	if err != nil {
		cobra.CheckErr(err)
		return
	}
	if lastByte[0] != '\n' {
		fmt.Println()
	}
}
