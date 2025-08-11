package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// ArgumentType represents the type of argument parsed
type ArgumentType int

const (
	ArgumentTypeKeyword ArgumentType = iota
	ArgumentTypeID
)

// ParsedArgument holds the result of parsing a keyword/ID argument
type ParsedArgument struct {
	Type    ArgumentType
	Keyword string
	ID      int
	Entry   *models.Entry // The entry found (if parsed as ID)
}

// ParseKeywordOrID parses an argument string as either a keyword or an ID (1-1000)
// If it's a valid ID in range, it tries to find the entry and returns it
// Otherwise, it treats it as a keyword
func ParseKeywordOrID(argument string, cfg *models.Config) (*ParsedArgument, error) {
	if argument == "" {
		return nil, fmt.Errorf("argument cannot be empty")
	}

	// Try to parse as an ID first
	if id, err := strconv.Atoi(argument); err == nil && id >= 1 && id <= 1000 {
		// Parse as ID
		entry := cfg.GetEntryByShortID(id)
		if entry == nil {
			return nil, fmt.Errorf("no entry found with short ID %d", id)
		}

		return &ParsedArgument{
			Type:    ArgumentTypeID,
			ID:      id,
			Entry:   entry,
			Keyword: entry.Keyword, // Also include keyword for convenience
		}, nil
	}

	// Parse as keyword
	return &ParsedArgument{
		Type:    ArgumentTypeKeyword,
		Keyword: argument,
	}, nil
}

// ParseDuration parses duration strings like "5m", "1h30m", "2h30m30s", "5", etc.
// If no unit is specified (e.g., "5"), it defaults to minutes.
// Supports formats: "5", "5m", "1h", "1h30", "1h30m", "2h30m30s"
// Spaces are automatically trimmed and ignored.
func ParseDuration(input string) (time.Duration, error) {
	if input == "" {
		return 0, fmt.Errorf("duration cannot be empty")
	}

	// Remove all spaces
	input = strings.ReplaceAll(input, " ", "")
	input = strings.ToLower(input)

	// If it's just a number (no units), assume minutes
	if matched, _ := regexp.MatchString(`^\d+$`, input); matched {
		minutes, err := strconv.Atoi(input)
		if err != nil {
			return 0, fmt.Errorf("invalid number format: %s", input)
		}
		return time.Duration(minutes) * time.Minute, nil
	}

	// Parse complex duration formats
	// Regex to match patterns like: 1h30m15s, 5m, 2h, 1h30, etc.
	// Note: Minutes without 'm' are only allowed when preceded by hours (e.g., 1h30)
	re := regexp.MustCompile(`^(?:(\d+)h(?:(\d+)m?)?)?(?:(\d+)m)?(?:(\d+)s)?$`)
	matches := re.FindStringSubmatch(input)
	
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s (supported formats: 5, 5m, 1h, 1h30, 1h30m, 2h30m30s)", input)
	}

	var duration time.Duration

	// Parse hours
	if matches[1] != "" {
		hours, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("invalid hours value: %s", matches[1])
		}
		duration += time.Duration(hours) * time.Hour
	}

	// Parse minutes after hours (e.g., "1h30")
	if matches[2] != "" {
		minutes, err := strconv.Atoi(matches[2])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes value: %s", matches[2])
		}
		duration += time.Duration(minutes) * time.Minute
	}

	// Parse standalone minutes (e.g., "30m")
	if matches[3] != "" {
		minutes, err := strconv.Atoi(matches[3])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes value: %s", matches[3])
		}
		duration += time.Duration(minutes) * time.Minute
	}

	// Parse seconds
	if matches[4] != "" {
		seconds, err := strconv.Atoi(matches[4])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds value: %s", matches[4])
		}
		duration += time.Duration(seconds) * time.Second
	}

	if duration == 0 {
		return 0, fmt.Errorf("duration cannot be zero")
	}

	return duration, nil
}

// SortEntriesByStartTimeDesc sorts entries by StartTime in descending order (most recent first)
// This is used by continue, delete, and set commands to show latest entries first
func SortEntriesByStartTimeDesc(entries []models.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartTime.After(entries[j].StartTime)
	})
}

// SortEntriesPtrsByStartTimeDesc sorts entry pointers by StartTime in descending order (most recent first)
// This is used when working with slices of entry pointers
func SortEntriesPtrsByStartTimeDesc(entries []*models.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartTime.After(entries[j].StartTime)
	})
}

// SortEntriesByShortIDAsc sorts entries by ShortID in ascending order (1, 2, 3...)
// This is used by the list command to show entries in ID order
func SortEntriesByShortIDAsc(entries []models.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ShortID < entries[j].ShortID
	})
}
