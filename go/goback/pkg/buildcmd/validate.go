package buildcmd

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func (r *builder) validateBeforeRun() {
	if _, err := os.Stat(r.updatedDestDir); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("r.updatedDestination directory %s does not exist", r.updatedDestDir))
	}

	if _, err := os.Stat(r.updatedSrc); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("source directory %s does not exist", r.updatedSrc))
	}

	if _, err := os.ReadDir(r.updatedSrc); err != nil {
		log.Fatal(fmt.Sprintf("source directory %s is empty", r.updatedSrc))
	}

	if r.updatedSrc == r.updatedDestDir {
		log.Fatal(fmt.Sprintf("source and r.updatedDestination are the same: %s", r.updatedSrc))
	}

	if strings.HasPrefix(r.updatedDestDir, r.updatedSrc) {
		log.Fatal(fmt.Sprintf("source directory %s is a parent of r.updatedDestination directory %s", r.updatedSrc, r.updatedDestDir))
	}
}
