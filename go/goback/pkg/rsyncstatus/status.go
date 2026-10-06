// Package rsyncstatus explains rsync outcomes without interpreting companion
// exit codes, which belong to the companion programs themselves.
package rsyncstatus

// Explanation follows rsync's documented exit values:
// https://download.samba.org/pub/rsync/rsync.1.html#EXIT_VALUES
func Explanation(code int) string {
	switch code {
	case 1:
		return "invalid rsync options; check the command above"
	case 3:
		return "could not select input/output paths; check paths and permissions"
	case 10:
		return "socket I/O error; check the connection"
	case 11:
		return "file I/O error; check free space, drive health, and permissions"
	case 12:
		return "rsync protocol error; inspect rsync's error output"
	case 23:
		return "partial transfer due to errors; inspect rsync's error output"
	case 24:
		return "source files vanished during transfer; retry when the source is idle"
	case 30, 35:
		return "connection or transfer timed out; check the connection and retry"
	default:
		return ""
	}
}
