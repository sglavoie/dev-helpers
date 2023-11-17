package find

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/slicingutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func HandleAllExceptFlagReturns(cmd *cobra.Command, flagsPassed int, decoded map[string]models.Command, args []string) bool {
	allExcept, err := cmd.Flags().GetBool("exclude")
	if err != nil {
		fmt.Println("Error:", err)
		return true
	}
	if allExcept && flagsPassed > 1 {
		fmt.Println("You cannot specify more than one flag when using --all-except")
		return true
	}
	if allExcept {
		if args != nil && len(args) == 0 {
			fmt.Println("Search term(s) required")
			return true
		}
		printMatchesExcept(decoded, args)
		return true
	}
	return false
}

func HandleAllFlagReturns(cmd *cobra.Command, flagsPassed int, decoded map[string]models.Command, args []string) bool {
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		fmt.Println("Error:", err)
		return true
	}
	if all && flagsPassed > 1 {
		if flagsPassed > 2 {
			fmt.Println("You cannot specify more than one flag when using --all except for --editor")
			return true
		}

		editor, err := cmd.Flags().GetBool("editor")
		if err != nil {
			fmt.Println("Error:", err)
			return true
		}
		if !editor {
			fmt.Println("Only --editor can be used with --all")
			return true
		}
		return false
	}
	if all {
		printAll(decoded)
		if len(args) > 0 {
			fmt.Println("Search terms ignored")
		}
		return true
	}
	return false
}

func HandleFindInAllCommands(cmd *cobra.Command, decoded map[string]models.Command, args []string) {
	if len(args) == 0 {
		err := cmd.Help()
		if err != nil {
			return
		}
		fmt.Println("You must specify at least one search term")
		return
	}
	matches := inAllCommandFields(decoded, args)
	if len(matches) == 0 {
		fmt.Println("No matches found")
		return
	}

	PrintMatches(decoded, matches)
}

func InFlagsPassed(cmd *cobra.Command, decoded map[string]models.Command) []string {
	var matches []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		searchFunc := makeSearchFunc(f.Name)
		value, _ := cmd.Flags().GetStringSlice(f.Name) // Works for both string and stringSlice flags
		matches = append(matches, searchFunc(decoded, value)...)
	})

	return slicingutils.UniqueEntries(matches)
}

func PrintMatch(cmds map[string]models.Command, id string) {
	decodedCmd := cmds[id]
	if decodedCmd.Description == "" {
		fmt.Printf("[%v] %v\n", id, decodedCmd.Name)
	} else {
		fmt.Printf("[%v] %v - %v\n", id, decodedCmd.Name, decodedCmd.Description)
	}
	if len(decodedCmd.Tags) > 0 {
		fmt.Printf("Tags: %v\n", decodedCmd.Tags)
	}
	fmt.Printf("%v\n", decodedCmd.Command)
}

func PrintMatches(cmds map[string]models.Command, matches []string) {
	clihelpers.PrintLineSeparator()
	for _, id := range matches {
		PrintMatch(cmds, id)
		clihelpers.PrintLineSeparator()
	}
}

func ShowMatchesInEditor(cmds map[string]models.Command, matches []string) error {
	details := getMatchesString(cmds, matches)

	tmpfile, err := os.CreateTemp("", "commands")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			fmt.Printf("failed to remove temp file: %v", err)
		}
	}(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(details)); err != nil {
		err := tmpfile.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %v", err)
	}

	commands.OpenFileWithEditor(tmpfile.Name())

	return nil
}

// inAllCommandFields searches for keywords within the specified fields of all commands.
// Returns a slice of unique command IDs that match **all** the keywords.
func inAllCommandFields(cmds map[string]models.Command, keywords []string) []string {
	matches := make(map[string]struct{}) // Set to keep track of unique command IDs
	numKeywords := len(keywords)

	for id, cmd := range cmds {
		numMatches := 0
		for _, keyword := range keywords {
			if searchCommandFields(cmd, keyword) {
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

func getMatchesString(cmds map[string]models.Command, matches []string) string {
	var details string
	for _, id := range matches {
		command := cmds[id]
		details += clihelpers.GetLineSeparator() + "\n"
		details += fmt.Sprintf("[%s] %s\n", id, command.Name)
		if command.Description != "" {
			details += fmt.Sprintf("Description: %s\n", command.Description)
		}
		if len(command.Tags) > 0 {
			details += fmt.Sprintf("Tags: %v\n", command.Tags)
		}
		details += fmt.Sprintf("Command: %s\n", command.Command)
	}
	return details
}

// makeSearchFunc creates a function to search commands based on a given field and keywords.
func makeSearchFunc(field string) func(map[string]models.Command, []string) []string {
	return func(cmds map[string]models.Command, keywords []string) []string {
		matches := make(map[string]struct{}) // Set to keep track of unique command IDs
		numKeywords := len(keywords)

		for id, cmd := range cmds {
			numMatches := 0
			var fieldValue string
			switch field {
			case "command":
				fieldValue = cmd.Command
			case "description":
				fieldValue = cmd.Description
			case "name":
				fieldValue = cmd.Name
			case "tags":
				for _, tag := range cmd.Tags {
					for _, keyword := range keywords {
						if strings.Contains(strings.ToLower(tag), strings.ToLower(keyword)) {
							numMatches++
						}
					}
				}
				if numMatches > 0 {
					matches[id] = struct{}{}
				}
				continue // Skip the rest of the loop since tags are already handled.
			}

			// Check if the fieldValue contains all keywords (non-slice fields)
			for _, keyword := range keywords {
				if strings.Contains(strings.ToLower(fieldValue), strings.ToLower(keyword)) {
					numMatches++
				}
			}

			if numMatches == numKeywords {
				matches[id] = struct{}{}
			}
		}

		// Convert the set of matches to a slice and sort it.
		var matchSlice []string
		for id := range matches {
			matchSlice = append(matchSlice, id)
		}
		slices.Sort(matchSlice)
		return matchSlice
	}
}

func printAll(cmds map[string]models.Command) {
	clihelpers.PrintLineSeparator()

	// Extract keys and convert them to integers for sorting
	var keys []int
	for k := range cmds {
		if id, err := strconv.Atoi(k); err == nil {
			keys = append(keys, id)
		}
	}

	// Sort the keys
	sort.Ints(keys)

	for _, id := range keys {
		cmd := cmds[strconv.Itoa(id)]
		if cmd.Description == "" {
			fmt.Printf("[%v] %v\n", id, cmd.Name)
		} else {
			fmt.Printf("[%v] %v - %v\n", id, cmd.Name, cmd.Description)
		}
		if len(cmd.Tags) > 0 {
			fmt.Printf("Tags: %v\n", cmd.Tags)
		}
		fmt.Printf("%v\n", cmd.Command)
		clihelpers.PrintLineSeparator()
	}
}

func printMatchesExcept(decoded map[string]models.Command, args []string) {
	// for each argument, check if it matches any command fields
	var matches []string
	for _, arg := range args {
		arg := []string{arg}
		match := inAllCommandFields(decoded, arg)
		matches = append(matches, match...)
	}

	matches = slicingutils.UniqueEntries(matches)

	// Keep only the command IDs that don't match the search terms
	var reverse []string
	for id := range decoded {
		if !slices.Contains(matches, id) {
			reverse = append(reverse, id)
		}
	}

	slices.Sort(reverse)
	if len(reverse) == 0 {
		fmt.Println("No matches found")
		return
	}
	PrintMatches(decoded, reverse)
}

// searchCommandFields searches for a keyword within the specified fields of a command.
// Returns true if the keyword is found.
func searchCommandFields(cmd models.Command, keyword string) bool {
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
