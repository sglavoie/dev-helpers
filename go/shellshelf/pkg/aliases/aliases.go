package aliases

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
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
		fmt.Println("Error saving aliases:", err)
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
		clihelpers.FatalExit("Error getting flag:", err)
	}

	if !f {
		confirmRemovalAlias(as)
	}

	err = Save(map[string]string{})
	if err != nil {
		clihelpers.FatalExit("Error saving aliases:", err)
	}

	fmt.Println("Aliases cleared successfully!")
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
			clihelpers.FatalExit("Error parsing ID:", err)
		}
	}
}

func confirmRemovalAlias(as map[string]string) {
	fmt.Println("Are you sure you want to remove the following aliases?")
	for k := range as {
		fmt.Printf("%v\n", k)
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("y/N: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		clihelpers.FatalExit("Error reading input:", err)
	}

	input = strings.TrimSpace(input)

	if input != "y" && input != "Y" {
		clihelpers.FatalExit("Aborting")
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
		clihelpers.FatalExit("Error saving aliases:", err)
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
		clihelpers.FatalExit("Error saving aliases:", err)
	}

	n := len(args)

	if n == 1 {
		fmt.Println("Alias removed successfully!")
		return
	}

	fmt.Println(n, "aliases removed successfully!")
}
