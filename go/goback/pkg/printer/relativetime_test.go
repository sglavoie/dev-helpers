package printer

import (
	"testing"
	"time"
)

func TestRelativeTimeUsesLocalHistoryTime(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.FixedZone("local", -6*3600))
	for _, tc := range []struct{ timestamp, want string }{
		{"2026-10-06 12:00:00", "just now"},
		{"2026-10-06 11:59:00", "1 minute ago"},
		{"2026-10-06 10:00:00", "2 hours ago"},
		{"2026-10-05 12:00:00", "1 day ago"},
		{"2026-10-04 12:00:00", "2 days ago"},
		{"2026-10-06 13:00:00", "in 1 hour"},
		{"invalid", "unknown age"},
	} {
		if got := relativeTime(tc.timestamp, now); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.timestamp, got, tc.want)
		}
	}
}
