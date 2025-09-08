package eject

import (
	"log"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

// extractVolumePath extracts the volume mount point from a destination path.
// For example, "/Volumes/SanDisk/macbook" becomes "/Volumes/SanDisk"
func extractVolumePath(dest string) string {
	if strings.HasPrefix(dest, "/Volumes/") {
		parts := strings.Split(dest, "/")
		if len(parts) >= 3 {
			// Reconstruct: "", "Volumes", "DiskName"
			return "/" + parts[1] + "/" + parts[2]
		}
	}
	return dest
}

func Eject() {
	dest := viper.GetString("destination")
	if dest == "" {
		log.Fatal("destination not set")
	}

	// Extract the volume mount point from the destination path
	volumePath := extractVolumePath(dest)
	
	log.Printf("Ejecting volume at '%s' (configured destination: '%s')", volumePath, dest)

	cmd := exec.Command("diskutil", "eject", volumePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to eject disk '%s': %v\nOutput: %s", volumePath, err, string(output))
	}

	log.Printf("Successfully ejected disk: %s", volumePath)
}
