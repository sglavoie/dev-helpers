package cmd

import (
	"fmt"
	"strconv"

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
