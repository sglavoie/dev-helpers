package buildcmd

import (
	"fmt"
	"os"
	"strings"
)

func (r *builder) validateBeforeRun() error {
	if _, err := os.Stat(r.updatedDestDir); os.IsNotExist(err) {
		if err := os.MkdirAll(r.updatedDestDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", r.updatedDestDir, err)
		}
	}

	if _, err := os.Stat(r.updatedSrc); os.IsNotExist(err) {
		return fmt.Errorf("source directory %s does not exist", r.updatedSrc)
	}

	entries, err := os.ReadDir(r.updatedSrc)
	if err != nil {
		return fmt.Errorf("cannot read source directory %s: %w", r.updatedSrc, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("source directory %s is empty", r.updatedSrc)
	}

	if r.updatedSrc == r.updatedDestDir {
		return fmt.Errorf("source and destination are the same: %s", r.updatedSrc)
	}

	if strings.HasPrefix(r.updatedDestDir, r.updatedSrc) {
		return fmt.Errorf("source directory %s is a parent of destination directory %s", r.updatedSrc, r.updatedDestDir)
	}

	return nil
}
