package buildcmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func sourceAndDestination() (src, dest string) {
	return viper.GetString("source"), viper.GetString("destination")
}

func mustExitOnInvalidSourceOrDestination() (string, string) {
	src := viper.GetString("source")
	if src == "" {
		log.Fatal("source not set")
	}
	dest := viper.GetString("destination")
	if dest == "" {
		log.Fatal("destination not set")
	}

	srcIsDir, err := isDirectory(src)
	if !srcIsDir {
		log.Fatal("source is not a directory")
	}

	destIsDir, err := isDirectory(dest)
	cobra.CheckErr(err)
	if !destIsDir {
		log.Fatal("destination is not a directory")
	}

	return src, dest
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
