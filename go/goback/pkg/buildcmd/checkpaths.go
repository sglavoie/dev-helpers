package buildcmd

import (
	"log"
	"os"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func sourceAndDestination() (src, dest string) {
	prefix := config.ActiveProfilePrefix()
	return viper.GetString(prefix + "source"), viper.GetString(prefix + "destination")
}

func mustExitOnInvalidSourceOrDestination() (string, string) {
	prefix := config.ActiveProfilePrefix()
	src := viper.GetString(prefix + "source")
	if src == "" {
		log.Fatalf("source not set for profile %q", config.ActiveProfileName)
	}
	dest := viper.GetString(prefix + "destination")
	if dest == "" {
		log.Fatalf("destination not set for profile %q", config.ActiveProfileName)
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
