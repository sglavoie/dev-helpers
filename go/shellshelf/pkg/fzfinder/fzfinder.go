package fzfinder

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	fzf "github.com/koki-develop/go-fzf"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/template"
)

var ErrCancelled = errors.New("selection cancelled")

type commandItem struct {
	command models.Command
	decoded string
}

func sortedItems(cmds models.Commands) []commandItem {
	items := make([]commandItem, 0, len(cmds))
	for _, cmd := range cmds {
		decoded, err := commands.Decode(cmd.Command)
		if err != nil {
			decoded = cmd.Command
		}
		items = append(items, commandItem{command: cmd, decoded: decoded})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].command.Name < items[j].command.Name
	})
	return items
}

func formatItem(item commandItem) string {
	parts := []string{fmt.Sprintf("[%s] %s", item.command.Id, item.command.Name)}
	if item.command.Description != "" {
		parts = append(parts, fmt.Sprintf("— %s", item.command.Description))
	}
	if len(item.command.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("[%s]", strings.Join(item.command.Tags, ", ")))
	}
	if names := template.ParamNames(item.decoded); len(names) > 0 {
		parts = append(parts, fmt.Sprintf("(params: %s)", strings.Join(names, ", ")))
	}
	return strings.Join(parts, "  ")
}

// SelectCommand shows a single-select fuzzy finder over all commands.
// Returns the selected command or an error if cancelled/empty.
func SelectCommand(cmds models.Commands) (models.Command, error) {
	if len(cmds) == 0 {
		return models.Command{}, errors.New("no commands available")
	}

	items := sortedItems(cmds)

	f, err := fzf.New(
		fzf.WithLimit(1),
		fzf.WithNoLimit(false),
		fzf.WithInputPlaceholder("Search commands..."),
	)
	if err != nil {
		return models.Command{}, fmt.Errorf("failed to create fuzzy finder: %w", err)
	}

	indices, err := f.Find(items, func(i int) string {
		return formatItem(items[i]) + " " + items[i].decoded
	})
	if err != nil {
		return models.Command{}, ErrCancelled
	}
	if len(indices) == 0 {
		return models.Command{}, ErrCancelled
	}

	return items[indices[0]].command, nil
}

// SelectAction shows a fuzzy finder to pick an action from the given list.
// Returns the selected action string or an error if cancelled.
func SelectAction(actions []string) (string, error) {
	if len(actions) == 0 {
		return "", errors.New("no actions available")
	}

	f, err := fzf.New(
		fzf.WithLimit(1),
		fzf.WithNoLimit(false),
		fzf.WithInputPlaceholder("Pick an action..."),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create fuzzy finder: %w", err)
	}

	indices, err := f.Find(actions, func(i int) string {
		return actions[i]
	})
	if err != nil {
		return "", ErrCancelled
	}
	if len(indices) == 0 {
		return "", ErrCancelled
	}

	return actions[indices[0]], nil
}

// SelectCommands shows a multi-select fuzzy finder.
// Returns the selected commands or an error if cancelled/empty.
func SelectCommands(cmds models.Commands) ([]models.Command, error) {
	if len(cmds) == 0 {
		return nil, errors.New("no commands available")
	}

	items := sortedItems(cmds)

	f, err := fzf.New(
		fzf.WithNoLimit(true),
		fzf.WithInputPlaceholder("Search commands (tab to select)..."),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create fuzzy finder: %w", err)
	}

	indices, err := f.Find(items, func(i int) string {
		return formatItem(items[i]) + " " + items[i].decoded
	})
	if err != nil {
		return nil, ErrCancelled
	}
	if len(indices) == 0 {
		return nil, ErrCancelled
	}

	selected := make([]models.Command, len(indices))
	for i, idx := range indices {
		selected[i] = items[idx].command
	}
	return selected, nil
}
