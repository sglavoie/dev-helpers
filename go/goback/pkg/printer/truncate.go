package printer

import (
	"regexp"
	"strings"
)

// TruncateExecTimeToNearest takes a string, split it on the fractional part,
// and truncates the fractional part to the nearest precision, without
// touching units in the suffix of the string.
func TruncateExecTimeToNearest(s string, precision int) string {
	parts := strings.Split(s, ".")

	if len(parts) == 1 {
		return s
	}

	if len(parts[1]) <= precision {
		return s
	}

	// Get digits in the fractional part
	re := regexp.MustCompile(`\d`)
	digits := re.FindAllString(parts[1], -1)

	// Get letters in the fractional part (unit suffix to keep)
	re = regexp.MustCompile(`[a-zA-Z]`)
	letters := re.FindAllString(parts[1], -1)

	truncated := strings.Join(digits[:precision], "")
	sb := &strings.Builder{}
	sb.WriteString(parts[0])
	sb.WriteString(".")
	sb.WriteString(truncated)
	sb.WriteString(" ")
	sb.WriteString(strings.Join(letters, ""))
	return sb.String()
}
