package printer

import (
	"fmt"
	"time"
)

// RelativeTime reads history's local wall-clock timestamps. Older rows contain
// no timezone, so their original timezone cannot be recovered after travel.
func RelativeTime(timestamp string) string {
	return relativeTime(timestamp, time.Now())
}

func relativeTime(timestamp string, now time.Time) string {
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", timestamp, now.Location())
	if err != nil {
		return "unknown age"
	}
	elapsed := now.Sub(parsed)
	future := elapsed < 0
	if future {
		elapsed = -elapsed
	}
	if elapsed < time.Minute {
		return "just now"
	}
	value, unit := int(elapsed/time.Minute), "minute"
	if elapsed >= 24*time.Hour {
		value, unit = int(elapsed/(24*time.Hour)), "day"
	} else if elapsed >= time.Hour {
		value, unit = int(elapsed/time.Hour), "hour"
	}
	if value != 1 {
		unit += "s"
	}
	if future {
		return fmt.Sprintf("in %d %s", value, unit)
	}
	return fmt.Sprintf("%d %s ago", value, unit)
}

// TimestampWithAge preserves the exact recorded timestamp alongside its age.
func TimestampWithAge(timestamp string) string {
	return fmt.Sprintf("%s (%s)", timestamp, RelativeTime(timestamp))
}
