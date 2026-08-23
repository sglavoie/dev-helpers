package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// entryTimeLayout is the format every time field of the editor is read and
// written in.
const entryTimeLayout = "2006-01-02 15:04:05"

// formatEntryTime renders a time the way the editor's time fields expect it.
func formatEntryTime(t time.Time) string {
	return t.Format(entryTimeLayout)
}

// parseEntryTime reads a time field, interpreting it in the local time zone.
func parseEntryTime(value string) (time.Time, error) {
	return time.ParseInLocation(entryTimeLayout, value, time.Local)
}

// parseTagList splits a comma-separated tags field into its non-empty tags.
func parseTagList(tagsStr string) []string {
	cleanTags := []string{}
	for _, tag := range strings.Split(tagsStr, ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}
	return cleanTags
}

// setCompletedInterval turns the entry into a finished interval spanning the
// given times.
func setCompletedInterval(entry *models.Entry, startTime, endTime time.Time, duration int) {
	entry.StartTime = startTime
	entry.EndTime = &endTime
	entry.Duration = duration
	entry.Active = false
}

// setActiveInterval turns the entry back into a running one, which is what an
// empty end time means.
func setActiveInterval(entry *models.Entry, startTime time.Time) {
	entry.StartTime = startTime
	entry.EndTime = nil
	entry.Duration = 0
	entry.Active = true
}

// keywordField and the builders below describe the editor's fields; the same
// field appears in several modes, with only its description differing.
func keywordField(entry *models.Entry) fieldModel {
	return fieldModel{
		name:        "keyword",
		displayName: "Keyword",
		description: "Primary categorization for this entry",
		fieldType:   fieldTypeString,
		value:       entry.Keyword,
	}
}

func tagsField(entry *models.Entry) fieldModel {
	return fieldModel{
		name:        "tags",
		displayName: "Tags",
		description: "Comma-separated tags for secondary categorization",
		fieldType:   fieldTypeTags,
		value:       strings.Join(entry.Tags, ", "),
	}
}

func durationField(entry *models.Entry, description string) fieldModel {
	return fieldModel{
		name:        "duration",
		displayName: "Duration",
		description: description,
		fieldType:   fieldTypeDuration,
		value:       formatDurationHMS(entry.GetCurrentDuration()),
	}
}

func startTimeField(entry *models.Entry) fieldModel {
	return fieldModel{
		name:        "start_time",
		displayName: "Start Time",
		description: "When tracking started (format: " + entryTimeLayout + ")",
		fieldType:   fieldTypeTime,
		value:       formatEntryTime(entry.StartTime),
	}
}

// endTimeField falls back to the given text when the entry has no end time,
// which is how an active entry and a brand new one are told apart.
func endTimeField(entry *models.Entry, description, fallback string) fieldModel {
	value := fallback
	if entry.EndTime != nil {
		value = formatEntryTime(*entry.EndTime)
	}

	return fieldModel{
		name:        "end_time",
		displayName: "End Time",
		description: description,
		fieldType:   fieldTypeTime,
		value:       value,
	}
}

func formatDurationHMS(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func parseDurationHMS(duration string) (int, error) {
	parts := strings.Split(duration, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("duration must be in HH:MM:SS format")
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 {
		return 0, fmt.Errorf("invalid hours")
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("invalid minutes (0-59)")
	}

	seconds, err := strconv.Atoi(parts[2])
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, fmt.Errorf("invalid seconds (0-59)")
	}

	return hours*3600 + minutes*60 + seconds, nil
}
