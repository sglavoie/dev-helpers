package mirror

import (
	"fmt"
	"strings"
)

var byteUnits = []string{"KiB", "MiB", "GiB", "TiB", "PiB"}

// HumanBytes renders a raw byte count for a human. Raw counts stay available
// separately, so nothing downstream compares rounded values.
func HumanBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	unit := byteUnits[0]
	for _, next := range byteUnits {
		unit = next
		value /= 1024
		if value < 1024 {
			break
		}
	}
	return fmt.Sprintf("%.1f %s", value, unit)
}

func nonEmptyLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
