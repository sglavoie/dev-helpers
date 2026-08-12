package mirror

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// itemizeWidth is the fixed width of the rsync change summary produced by
// "%i". The path always starts one space later, which is why a filename
// containing spaces, tabs, or quotes needs no unescaping to be located.
const itemizeWidth = 11

const (
	deletingPrefix   = "*deleting"
	transferredLabel = "Total transferred file size:"
)

// itemizePattern matches a change summary: an update type, a file type, and
// nine attribute characters. A created item has all nine set to "+".
var itemizePattern = regexp.MustCompile(`^[<>ch.][fdLDS][a-zA-Z.+? ]{9}$`)

// Deletion is a destination entry the mirror would remove. The path is
// relative to the destination and is reported exactly as rsync escaped it.
type Deletion struct {
	Path  string
	IsDir bool
}

// Changes is the complete plan of a mirror run.
type Changes struct {
	Created int
	Updated int
	Deleted int

	// TransferBytes is the raw size of the data rsync would transfer.
	TransferBytes uint64

	Deletions []Deletion
}

// Empty reports whether the destination already matches the source.
func (c Changes) Empty() bool {
	return c.Created == 0 && c.Updated == 0 && c.Deleted == 0
}

// ParseChanges reads the itemization and statistics of an rsync run. Lines it
// does not recognize are ignored, including the "created directory" notice
// rsync prints for a destination leaf a dry run does not actually create.
func ParseChanges(output string) (Changes, error) {
	var changes Changes
	transferredFound := false

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")

		if bytesValue, ok := parseTransferred(line); ok {
			changes.TransferBytes = bytesValue
			transferredFound = true
			continue
		}

		summary, path, ok := splitItemized(line)
		if !ok {
			continue
		}
		switch {
		case summary == deletingPrefix:
			changes.Deleted++
			changes.Deletions = append(changes.Deletions, Deletion{Path: path, IsDir: strings.HasSuffix(path, "/")})
		case strings.HasSuffix(summary, "+++++++++"):
			changes.Created++
		default:
			changes.Updated++
		}
	}

	if !transferredFound {
		return Changes{}, fmt.Errorf("rsync reported no %q line, so the size of the transfer is unknown", transferredLabel)
	}
	return changes, nil
}

// splitItemized separates a change summary from the path that follows it. The
// summary is returned with its padding removed so it can be compared, while
// the path keeps every character rsync emitted.
func splitItemized(line string) (string, string, bool) {
	if len(line) <= itemizeWidth+1 || line[itemizeWidth] != ' ' {
		return "", "", false
	}
	summary := line[:itemizeWidth]
	path := line[itemizeWidth+1:]

	if strings.HasPrefix(summary, deletingPrefix) && strings.TrimSpace(summary) == deletingPrefix {
		return deletingPrefix, path, true
	}
	if !itemizePattern.MatchString(summary) {
		return "", "", false
	}
	return summary, path, true
}

// parseTransferred reads the byte total rsync would send. The number is
// printed with thousands separators.
func parseTransferred(line string) (uint64, bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(line), transferredLabel)
	if !found {
		return 0, false
	}
	digits := strings.ReplaceAll(strings.TrimSuffix(strings.TrimSpace(rest), " bytes"), ",", "")
	value, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
