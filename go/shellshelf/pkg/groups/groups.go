package groups

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/template"
)

// Add creates a new group with the given name and command IDs.
func Add(name string, commandIDs []string, stopOnError bool) error {
	cfg := config.Cfg

	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	if err := commands.AreAllCommandIDsValid(cfg.Commands, commandIDs); err != nil {
		return fmt.Errorf("invalid command IDs: %w", err)
	}

	if cfg.Groups == nil {
		cfg.Groups = make(models.Groups)
	}

	cfg.Groups[name] = models.Group{
		Name:        name,
		CommandIDs:  commandIDs,
		StopOnError: stopOnError,
	}

	config.SaveGroups(cfg)
	return nil
}

// Remove deletes a group by name.
func Remove(name string) error {
	cfg := config.Cfg

	if _, ok := cfg.Groups[name]; !ok {
		return fmt.Errorf("group '%s' not found", name)
	}

	delete(cfg.Groups, name)
	config.SaveGroups(cfg)
	return nil
}

// Get returns a group by name.
func Get(name string) (models.Group, error) {
	cfg := config.Cfg

	group, ok := cfg.Groups[name]
	if !ok {
		return models.Group{}, fmt.Errorf("group '%s' not found", name)
	}

	return group, nil
}

// List returns all groups sorted by name.
func List() []models.Group {
	cfg := config.Cfg

	groups := make([]models.Group, 0, len(cfg.Groups))
	for _, g := range cfg.Groups {
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}

// Run executes all commands in a group sequentially.
func Run(group models.Group) error {
	cfg := config.Cfg
	total := len(group.CommandIDs)

	for i, id := range group.CommandIDs {
		cmd, err := commands.GetById(cfg.Commands, id)
		if err != nil {
			if group.StopOnError {
				return fmt.Errorf("step %d/%d: %w", i+1, total, err)
			}
			fmt.Fprintf(os.Stderr, "[Step %d/%d] Error: %v\n", i+1, total, err)
			continue
		}

		decoded, err := commands.Decode(cmd.Command)
		if err != nil {
			if group.StopOnError {
				return fmt.Errorf("step %d/%d: failed to decode command '%s': %w", i+1, total, cmd.Name, err)
			}
			fmt.Fprintf(os.Stderr, "[Step %d/%d] Error decoding '%s': %v\n", i+1, total, cmd.Name, err)
			continue
		}

		fmt.Printf("[Step %d/%d: %s]\n", i+1, total, cmd.Name)

		// Resolve template parameters if present
		params := template.Parse(decoded)
		if len(params) > 0 {
			fmt.Println("Enter template parameters:")
			values, err := template.PromptForParams(params)
			if err != nil {
				if group.StopOnError {
					return fmt.Errorf("step %d/%d '%s': %w", i+1, total, cmd.Name, err)
				}
				fmt.Fprintf(os.Stderr, "[Step %d/%d] '%s' param error: %v\n", i+1, total, cmd.Name, err)
				continue
			}
			decoded = template.Render(decoded, values)
		}

		shellCmd := exec.Command("/bin/sh", "-c", decoded)
		shellCmd.Stdin = os.Stdin
		shellCmd.Stdout = os.Stdout
		shellCmd.Stderr = os.Stderr

		if err := shellCmd.Run(); err != nil {
			if group.StopOnError {
				return fmt.Errorf("step %d/%d '%s' failed: %w", i+1, total, cmd.Name, err)
			}
			fmt.Fprintf(os.Stderr, "[Step %d/%d] '%s' failed: %v\n", i+1, total, cmd.Name, err)
		}
	}

	return nil
}

// GroupsContainingCommand returns the names of groups that contain the given command ID.
func GroupsContainingCommand(commandID string) []string {
	cfg := config.Cfg
	var groupNames []string

	for _, g := range cfg.Groups {
		if slices.Contains(g.CommandIDs, commandID) {
			groupNames = append(groupNames, g.Name)
		}
	}

	sort.Strings(groupNames)
	return groupNames
}
