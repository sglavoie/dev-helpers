package template

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// paramRegex matches {{name}} and {{name:default}} placeholders.
var paramRegex = regexp.MustCompile(`\{\{(\w+)(?::([^}]*))?\}\}`)

// Param represents a template parameter extracted from a command string.
type Param struct {
	Name       string
	Default    string
	HasDefault bool
	Position   int
}

// HasParams returns true if the command contains any template placeholders.
func HasParams(command string) bool {
	return paramRegex.MatchString(command)
}

// Parse extracts ordered, unique parameters from a command string.
// Parameters appear in the order of their first occurrence.
func Parse(command string) []Param {
	matches := paramRegex.FindAllStringSubmatchIndex(command, -1)
	seen := make(map[string]bool)
	var params []Param
	pos := 0

	for _, match := range matches {
		name := command[match[2]:match[3]]
		if seen[name] {
			continue
		}
		seen[name] = true

		p := Param{
			Name:     name,
			Position: pos,
		}
		if match[4] != -1 {
			p.Default = command[match[4]:match[5]]
			p.HasDefault = true
		}
		params = append(params, p)
		pos++
	}
	return params
}

// Render replaces all template placeholders with the provided values.
func Render(command string, values map[string]string) string {
	return paramRegex.ReplaceAllStringFunc(command, func(match string) string {
		sub := paramRegex.FindStringSubmatch(match)
		if val, ok := values[sub[1]]; ok {
			return val
		}
		return match
	})
}

// ParamNames returns just the parameter names from a command string.
func ParamNames(command string) []string {
	params := Parse(command)
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	return names
}

// PromptForParams interactively prompts the user for each parameter value.
func PromptForParams(params []Param) (map[string]string, error) {
	reader := bufio.NewReader(os.Stdin)
	values := make(map[string]string, len(params))

	for _, p := range params {
		if p.HasDefault {
			fmt.Printf("  %s [default: %s]: ", p.Name, p.Default)
		} else {
			fmt.Printf("  %s: ", p.Name)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading input for %s: %w", p.Name, err)
		}
		input = strings.TrimSpace(input)

		if input == "" && p.HasDefault {
			values[p.Name] = p.Default
		} else if input == "" && !p.HasDefault {
			return nil, fmt.Errorf("parameter %q is required", p.Name)
		} else {
			values[p.Name] = input
		}
	}

	return values, nil
}

// ResolveFromArgs maps positional arguments to parameters in order.
func ResolveFromArgs(params []Param, args []string) (map[string]string, error) {
	if len(args) != len(params) {
		return nil, fmt.Errorf(
			"expected %d argument(s) (%s), got %d",
			len(params),
			paramNameList(params),
			len(args),
		)
	}

	values := make(map[string]string, len(params))
	for i, p := range params {
		values[p.Name] = args[i]
	}
	return values, nil
}

func paramNameList(params []Param) string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
