package buildcmd

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func (r *builder) validateBeforeRun() {
	if _, err := os.Stat(r.updatedDestDir); os.IsNotExist(err) {
		if err := os.MkdirAll(r.updatedDestDir, 0755); err != nil {
			log.Fatalf("failed to create destination directory %s: %v", r.updatedDestDir, err)
		}
	}

	if _, err := os.Stat(r.updatedSrc); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("source directory %s does not exist", r.updatedSrc))
	}

	if _, err := os.ReadDir(r.updatedSrc); err != nil {
		log.Fatal(fmt.Sprintf("source directory %s is empty", r.updatedSrc))
	}

	if r.updatedSrc == r.updatedDestDir {
		log.Fatal(fmt.Sprintf("source and destination are the same: %s", r.updatedSrc))
	}

	if strings.HasPrefix(r.updatedDestDir, r.updatedSrc) {
		log.Fatal(fmt.Sprintf("source directory %s is a parent of destination directory %s", r.updatedSrc, r.updatedDestDir))
	}
}
