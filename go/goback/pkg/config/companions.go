package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// Companion is a program run alongside the daily backup of a profile.
type Companion struct {
	ID         string
	Name       string
	Command    []string
	DryRunArgs []string
}

// Argv returns the argument vector to execute, with the configured dry-run
// arguments appended when dryRun is set.
func (c Companion) Argv(dryRun bool) []string {
	argv := make([]string, 0, len(c.Command)+len(c.DryRunArgs))
	argv = append(argv, c.Command...)
	if dryRun {
		argv = append(argv, c.DryRunArgs...)
	}
	return argv
}

const companionsKey = "dailyCompanions"

// The id becomes the "companion/<id>" backup type in history, so it may not
// contain a slash or whitespace.
var companionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var companionKeys = map[string]string{
	"id":         "id",
	"name":       "name",
	"command":    "command",
	"dryrunargs": "dryRunArgs",
}

// DailyCompanions returns the validated companions of the active profile.
func DailyCompanions() ([]Companion, error) {
	return ProfileCompanions(ActiveProfileName)
}

// ProfileCompanions returns the validated companions of the named profile.
func ProfileCompanions(profile string) ([]Companion, error) {
	if profile == "" {
		return nil, nil
	}

	key := "profiles." + profile + "." + companionsKey
	raw := viper.Get(key)
	if raw == nil {
		return nil, nil
	}

	entries, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a list of companion objects", key)
	}

	companions := make([]Companion, 0, len(entries))
	seen := make(map[string]int, len(entries))
	for i, entry := range entries {
		where := fmt.Sprintf("%s[%d]", key, i)
		c, err := parseCompanion(entry, where)
		if err != nil {
			return nil, err
		}
		if first, dup := seen[c.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate companion id %q, already used by %s[%d]", where, c.ID, key, first)
		}
		seen[c.ID] = i
		companions = append(companions, c)
	}
	return companions, nil
}

func parseCompanion(entry any, where string) (Companion, error) {
	fields, err := companionFields(entry, where)
	if err != nil {
		return Companion{}, err
	}

	id, err := companionID(fields["id"], where)
	if err != nil {
		return Companion{}, err
	}

	command, err := companionArgs(fields["command"], where, "command")
	if err != nil {
		return Companion{}, err
	}
	if len(command) == 0 {
		return Companion{}, fmt.Errorf("%s: command is required and must list the program followed by each of its arguments", where)
	}
	if err := rejectShellString(command[0], where); err != nil {
		return Companion{}, err
	}

	dryRunArgs, err := companionArgs(fields["dryrunargs"], where, "dryRunArgs")
	if err != nil {
		return Companion{}, err
	}

	name := id
	if raw, ok := fields["name"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return Companion{}, fmt.Errorf("%s: name must be a string", where)
		}
		if strings.TrimSpace(s) == "" {
			return Companion{}, fmt.Errorf("%s: name must not be empty", where)
		}
		name = s
	}

	return Companion{ID: id, Name: name, Command: command, DryRunArgs: dryRunArgs}, nil
}

func companionFields(entry any, where string) (map[string]any, error) {
	raw, ok := entry.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object with id, name, command and dryRunArgs", where)
	}

	fields := make(map[string]any, len(raw))
	for k, v := range raw {
		lower := strings.ToLower(k)
		if _, known := companionKeys[lower]; !known {
			return nil, fmt.Errorf("%s: unknown key %q, allowed keys are id, name, command and dryRunArgs", where, k)
		}
		fields[lower] = v
	}
	return fields, nil
}

func companionID(raw any, where string) (string, error) {
	if raw == nil {
		return "", fmt.Errorf("%s: id is required", where)
	}
	id, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: id must be a string", where)
	}
	if !companionIDPattern.MatchString(id) {
		return "", fmt.Errorf("%s: id %q must start with a letter or digit and may only contain letters, digits, dots, dashes and underscores", where, id)
	}
	return id, nil
}

func companionArgs(raw any, where, field string) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case string:
		return nil, fmt.Errorf("%s: %s must be a list of arguments, not the string %q; companions are executed without a shell", where, field, v)
	case []string:
		return companionArgStrings(anySlice(v), where, field)
	case []any:
		return companionArgStrings(v, where, field)
	default:
		return nil, fmt.Errorf("%s: %s must be a list of arguments", where, field)
	}
}

func anySlice(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

func companionArgStrings(values []any, where, field string) ([]string, error) {
	args := make([]string, 0, len(values))
	for i, value := range values {
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s: %s[%d] must be a string", where, field, i)
		}
		if s == "" {
			return nil, fmt.Errorf("%s: %s[%d] must not be empty", where, field, i)
		}
		args = append(args, s)
	}
	return args, nil
}

func rejectShellString(program, where string) error {
	if strings.ContainsAny(program, " \t\n|&;<>()$`\\\"'*?") {
		return fmt.Errorf("%s: command[0] %q looks like a shell string; companions are executed without a shell, so give the program and each argument as separate list items", where, program)
	}
	return nil
}
