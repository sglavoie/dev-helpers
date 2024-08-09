package buildcmd

import (
	"github.com/spf13/viper"
	"log"
	"os"
)

func exitOnInvalidSourceOrDestination() (string, string) {
	src := viper.GetString("source")
	if src == "" {
		log.Fatal("source not set")
	}
	dest := viper.GetString("destination")
	if dest == "" {
		log.Fatal("destination not set")
	}

	srcIsDir, err := isDirectory(src)
	if err != nil {
		log.Fatal(err)
	}
	if !srcIsDir {
		log.Fatal("source is not a directory")
	}
	destIsDir, err := isDirectory(dest)
	if err != nil {
		log.Fatal(err)
	}
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
