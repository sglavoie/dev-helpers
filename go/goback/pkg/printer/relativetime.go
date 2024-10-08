package printer

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// RelativeTime converts a string like `2024-09-07 10:38:26` to
// a relative time expressed in days, e.g. `2 days ago` or `today`.
func RelativeTime(t string) string {
	parsedTime, err := time.Parse("2006-01-02 15:04:05", t)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("invalid time format: %v", err))
	}

	duration := time.Since(parsedTime)

	switch {
	case duration.Hours() >= 24:
		days := int(duration.Hours() / 24)
		plural := "s"
		if days == 1 {
			plural = ""
		}
		return fmt.Sprintf("%d day%s ago", days, plural)
	default:
		return fmt.Sprintf("today")
	}
}
