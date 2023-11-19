package aliases

import (
	"errors"
	"fmt"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/find"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/slicingutils"
	"github.com/spf13/cobra"
)

func Add(args []string, cfg *models.Config) {
	a := models.Alias{
		CommandID: args[0],
		Name:      args[1],
	}

	var err error
	cfg.Aliases, err = add(cfg.Aliases, a, cfg.Commands)
	if err != nil {
		fmt.Println(err)
		return
	}

	config.SaveAliases(cfg)
}

func ClearAliases(cmd *cobra.Command, cfg *models.Config) {
	if len(cfg.Aliases) == 0 {
		clihelpers.FatalExit("No aliases to clear!")
	}

	f, err := clihelpers.GetFlagBool(cmd, "force")
	if err != nil {
		clihelpers.FatalExit("Error getting flag: %v", err)
	}

	if !f {
		confirmRemovalAlias(cfg.Aliases)
	}

	cfg.Aliases = map[string]string{}
	config.SaveAliases(cfg)
}

func FindAlias(args []string, cfg *models.Config) {
	matches := inAliasFields(cfg.Aliases, args)
	if len(matches) == 0 {
		fmt.Println("No matches found")
		return
	}

	slices.Sort(matches)
	matches = slicingutils.UniqueEntries(matches)
	PrintMatches(cfg, matches)
}

func HandleAllFlagReturns(cmd *cobra.Command, flagsPassed int, args []string, cfg *models.Config) bool {
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		fmt.Println("Error:", err)
		return true
	}
	if all && flagsPassed > 1 {
		fmt.Println("You cannot specify more than one flag when using --all")
		return true
	}
	if all {
		if len(cfg.Aliases) == 0 {
			fmt.Println("No aliases found!")
			return true
		}

		var matches []string
		for k := range cfg.Aliases {
			matches = append(matches, k)
		}

		slices.Sort(matches)
		PrintMatches(cfg, matches)

		if len(args) > 0 {
			fmt.Println("Search terms ignored")
		}
		return true
	}
	return false
}

func PreRunAdd(args []string) error {
	if len(args) != 2 {
		return errors.New("requires exactly two arguments")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("first argument must be an integer, got: '%v'", args[0])
	}
	if id < 1 {
		return errors.New("first argument must be 1 or greater")
	}

	if !isValid(args[1]) {
		return fmt.Errorf("invalid alias specified: %s", args[1])
	}
	return nil
}

func PrintMatches(cfg *models.Config, matches []string) {
	decoded, err := commands.LoadDecoded(cfg.Commands)
	if err != nil {
		return
	}

	clihelpers.PrintLineSeparator()
	for _, match := range matches {
		cmdId := cfg.Aliases[match]
		fmt.Printf("%v\n", match)
		find.PrintMatch(decoded, cmdId)
		clihelpers.PrintLineSeparator()
	}
}

func Remove(cmd *cobra.Command, args []string, cfg *models.Config) {
	ids, err := cmd.Flags().GetStringSlice("id")
	if err != nil {
		fmt.Println("Error getting id flag:", err)
		return
	}

	// Check if the --id flag is used
	if len(ids) > 0 {
		runLogicRemoveAliasByID(cfg, ids)
	} else if len(args) > 0 {
		runLogicRemoveAliasByName(cfg, args)
	} else {
		err := cmd.Help()
		if err != nil {
			fmt.Println("Error printing help:", err)
			return
		}
	}
}

func add(aliases map[string]string, alias models.Alias, cmds map[string]models.Command) (map[string]string, error) {
	if _, exists := aliases[alias.Name]; exists {
		return aliases, fmt.Errorf("alias '%s' already exists", alias.Name)
	}

	if _, exists := cmds[alias.CommandID]; !exists {
		return aliases, fmt.Errorf("command ID '%s' does not exist", alias.CommandID)
	}

	aliases[alias.Name] = alias.CommandID

	return aliases, nil
}

func areIDsValidElseExit(args []string) {
	for _, arg := range args {
		_, err := strconv.Atoi(arg)
		if err != nil {
			clihelpers.FatalExit("Error parsing ID: %v", err)
		}
	}
}

func confirmRemovalAlias(as map[string]string) {
	fmt.Println("Are you sure you want to remove the following aliases?")
	for k := range as {
		fmt.Printf("%v\n", k)
	}

	_, err := clihelpers.ReadUserConfirmation()
	if err != nil {
		return
	}
}

func deleteById(cfg *models.Config, args []string) int {
	c := 0
	for _, arg := range args {
		for k, v := range cfg.Aliases {
			if v == arg {
				c++
				delete(cfg.Aliases, k)
			}
		}
	}
	return c
}

func deleteByName(cfg *models.Config, args []string) {
	for _, arg := range args {
		delete(cfg.Aliases, arg)
	}
}

func get(aliases map[string]string, name string) (string, error) {
	if _, ok := aliases[name]; !ok {
		return "", fmt.Errorf("alias '%v' not found", name)
	}

	return aliases[name], nil
}

func inAliasFields(as map[string]string, args []string) []string {
	var matches []string
	for _, arg := range args {
		for k := range as {
			if strings.Contains(k, arg) {
				matches = append(matches, k)
			}
		}
	}
	return matches
}

func isValid(alias string) bool {
	// This pattern allows for alphanumeric characters, dashes, and underscores
	re := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return re.MatchString(alias)
}

func namesExistElseExit(as map[string]string, args []string) {
	for _, arg := range args {
		_, err := get(as, arg)
		if err != nil {
			clihelpers.FatalExit(err.Error())
		}
	}
}

func runLogicRemoveAliasByID(cfg *models.Config, args []string) {
	areIDsValidElseExit(args)
	c := deleteById(cfg, args)
	config.SaveAliases(cfg)

	if c == 0 {
		fmt.Println("No aliases removed!")
	}
}

func runLogicRemoveAliasByName(cfg *models.Config, args []string) {
	namesExistElseExit(cfg.Aliases, args)
	deleteByName(cfg, args)
	config.SaveAliases(cfg)
}
