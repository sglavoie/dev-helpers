package buildcmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/viper"
)

func sourceAndDestination() (src, dest string) {
	prefix := config.ActiveProfilePrefix()
	return viper.GetString(prefix + "source"), viper.GetString(prefix + "destination")
}

func validateSourceAndDestination() (string, string, error) {
	return validatePaths(false)
}

func validateDerivedSourceAndDestination() (string, string, error) {
	return validatePaths(true)
}

func configuredPaths(derived bool) (string, string, error) {
	prefix := config.ActiveProfilePrefix()
	src := viper.GetString(prefix + "source")
	if src == "" && !derived {
		return "", "", fmt.Errorf("source not set for profile %q", config.ActiveProfileName)
	}
	dest := viper.GetString(prefix + "destination")
	if dest == "" {
		return "", "", fmt.Errorf("destination not set for profile %q", config.ActiveProfileName)
	}

	if derived {
		src = dest + "/daily/"
	}
	return src, dest, nil
}

func validatePaths(derived bool) (string, string, error) {
	src, dest, err := configuredPaths(derived)
	if err != nil {
		return "", "", err
	}
	srcIsDir, err := isDirectory(src)
	if err != nil {
		return "", "", err
	}
	if !srcIsDir {
		return "", "", fmt.Errorf("source is not a directory: %s", src)
	}

	destIsDir, err := isDirectory(dest)
	if err != nil {
		return "", "", err
	}
	if !destIsDir {
		return "", "", fmt.Errorf("destination is not a directory: %s", dest)
	}

	return src, dest, nil
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
