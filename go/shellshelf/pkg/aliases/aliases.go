package aliases

import (
	"errors"
	"fmt"
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
	"github.com/spf13/viper"
)

func Add(args []string) {
	as, err := Load()
	if err != nil {
		fmt.Println(err)
		as = map[string]string{}
	}

	cmds, err := commands.Load()
	if err != nil {
		fmt.Println(err)
		cmds = map[string]models.Command{}
	}

	a := models.Alias{
		CommandID: args[0],
		Name:      args[1],
	}
	as, err = add(as, a, cmds)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = Save(as)
	if err != nil {
		fmt.Println("Error saving aliases: %v", err)
		return
	}

	fmt.Println("Alias shelved successfully!")
}

func ClearAliases(cmd *cobra.Command) {
	as, err := Load()
	if err != nil {
		fmt.Println(err)
		as = map[string]string{}
	}

	if len(as) == 0 {
		clihelpers.FatalExit("No aliases to clear!")
	}

	f, err := clihelpers.GetFlagBool(cmd, "force")
	if err != nil {
		clihelpers.FatalExit("Error getting flag: %v", err)
	}

	if !f {
		confirmRemovalAlias(as)
	}

	err = Save(map[string]string{})
	if err != nil {
		clihelpers.FatalExit("Error saving aliases: %v", err)
	}

	fmt.Println("Aliases cleared successfully!")
}

func FindAlias(args []string) {
	as, err := Load()
	if err != nil {
		fmt.Println(err)
		as = map[string]string{}
	}

	matches := inAliasFields(as, args)
	if len(matches) == 0 {
		fmt.Println("No matches found")
		return
	}

	slices.Sort(matches)
	matches = slicingutils.UniqueEntries(matches)
	PrintMatches(as, matches)
}

func HandleAllFlagReturns(cmd *cobra.Command, flagsPassed int, args []string) bool {
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
		as, err := Load()
		if err != nil {
			fmt.Println(err)
			as = map[string]string{}
		}

		if len(as) == 0 {
			fmt.Println("No aliases found!")
			return true
		}

		var matches []string
		for k := range as {
			matches = append(matches, k)
		}

		slices.Sort(matches)
		PrintMatches(as, matches)

		if len(args) > 0 {
			fmt.Println("Search terms ignored")
		}
		return true
	}
	return false
}

func Load() (map[string]string, error) {
	if !viper.IsSet("aliases") {
		return nil, errors.New("'aliases' key not found in config")
	}

	var aliases map[string]string
	err := viper.UnmarshalKey("aliases", &aliases)
	if err != nil {
		return nil, err
	}

	return aliases, nil
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

func PrintMatches(as map[string]string, matches []string) {
	decoded, err := commands.LoadDecoded()
	if err != nil {
		return
	}

	clihelpers.PrintLineSeparator()
	for _, match := range matches {
		cmdId := as[match]
		fmt.Printf("%v\n", match)
		find.PrintMatch(decoded, cmdId)
		clihelpers.PrintLineSeparator()
	}
}

func Remove(cmd *cobra.Command, args []string) {
	as, err := Load()
	if err != nil {
		fmt.Println(err)
		as = map[string]string{}
	}

	ids, err := cmd.Flags().GetStringSlice("id")
	if err != nil {
		fmt.Println("Error getting id flag:", err)
		return
	}

	// Check if the --id flag is used
	if len(ids) > 0 {
		// Handle removal by IDs
		runLogicRemoveAliasByID(as, ids)
	} else if len(args) > 0 {
		runLogicRemoveAliasByName(as, args)
	} else {
		err := cmd.Help()
		if err != nil {
			fmt.Println("Error printing help:", err)
			return
		}
	}
}

func Save(aliases map[string]string) error {
	viper.Set("aliases", aliases)
	return viper.WriteConfig()
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

func deleteById(as map[string]string, args []string) int {
	c := 0
	for _, arg := range args {
		for k, v := range as {
			if v == arg {
				c++
				delete(as, k)
			}
		}
	}
	return c
}

func deleteByName(as map[string]string, args []string) {
	for _, arg := range args {
		delete(as, arg)
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

func runLogicRemoveAliasByID(as map[string]string, args []string) {
	areIDsValidElseExit(args)
	c := deleteById(as, args)

	err := Save(as)
	if err != nil {
		clihelpers.FatalExit("Error saving aliases: %v", err)
	}

	if c == 0 {
		fmt.Println("No aliases removed!")
		return
	}

	if c == 1 {
		fmt.Println("Alias removed successfully!")
		return
	}

	fmt.Println(c, "aliases removed successfully!")
}

func runLogicRemoveAliasByName(as map[string]string, args []string) {
	namesExistElseExit(as, args)
	deleteByName(as, args)

	err := Save(as)
	if err != nil {
		clihelpers.FatalExit("Error saving aliases: %v", err)
	}

	n := len(args)

	if n == 1 {
		fmt.Println("Alias removed successfully!")
		return
	}

	fmt.Println(n, "aliases removed successfully!")
}
