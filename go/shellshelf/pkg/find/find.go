package find

import (
	"slices"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
)

// SearchCommandFields searches for a keyword within the specified fields of a command.
// Returns true if the keyword is found.
func SearchCommandFields(cmd models.Command, keyword string) bool {
	keywordLower := strings.ToLower(keyword)

	fieldsToSearch := []string{
		cmd.Name,
		cmd.Description,
		cmd.Command,
	}

	for _, field := range fieldsToSearch {
		if strings.Contains(strings.ToLower(field), keywordLower) {
			return true
		}
	}

	for _, tag := range cmd.Tags {
		if strings.Contains(strings.ToLower(tag), keywordLower) {
			return true
		}
	}

	return false
}

// InCommandFields searches for keywords within the specified fields of all commands.
// Returns a slice of unique command IDs that match **all** the keywords.
func InCommandFields(cmds map[string]models.Command, keywords []string) []string {
	matches := make(map[string]struct{}) // Set to keep track of unique command IDs
	numKeywords := len(keywords)

	for id, cmd := range cmds {
		numMatches := 0
		for _, keyword := range keywords {
			if SearchCommandFields(cmd, keyword) {
				numMatches++
			}
		}

		if numMatches == numKeywords {
			matches[id] = struct{}{}
		}
	}

	// Convert the set of matches to a slice
	var matchSlice []string
	for id := range matches {
		matchSlice = append(matchSlice, id)
	}

	slices.Sort(matchSlice)
	return matchSlice
}
