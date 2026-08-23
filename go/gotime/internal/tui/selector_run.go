package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// RunSelectorWithConfig runs the selector TUI with explicit column configuration
func RunSelectorWithConfig(title string, items []SelectorItem, config TableConfig) (*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model, err := runSelectorProgram(NewSelectorModelWithConfig(title, items, config), "selector")
	if err != nil {
		return nil, err
	}
	return model.GetSelectedItem(), nil
}

// RunMultiSelectorWithConfig runs the multi-selector TUI with explicit column configuration
func RunMultiSelectorWithConfig(title string, items []SelectorItem, config TableConfig) ([]*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model, err := runSelectorProgram(NewMultiSelectorModelWithConfig(title, items, config), "multi-selector")
	if err != nil {
		return nil, err
	}
	return model.GetSelectedItems(), nil
}

// RunSelector runs the selector TUI and returns the selected item
func RunSelector(title string, items []SelectorItem) (*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model, err := runSelectorProgram(NewSelectorModel(title, items), "selector")
	if err != nil {
		return nil, err
	}
	return model.GetSelectedItem(), nil
}

// RunMultiSelector runs the multi-selector TUI and returns the selected items
func RunMultiSelector(title string, items []SelectorItem) ([]*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model, err := runSelectorProgram(NewMultiSelectorModel(title, items), "multi-selector")
	if err != nil {
		return nil, err
	}
	return model.GetSelectedItems(), nil
}

// runSelectorProgram drives a selector model to completion and returns the
// finished model, turning both a failed program and a cancelled selection into
// an error. The kind is only used to name the TUI in the failure message.
func runSelectorProgram(model SelectorModel, kind string) (SelectorModel, error) {
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return SelectorModel{}, fmt.Errorf("failed to run %s TUI: %w", kind, err)
	}

	selectorModel := finalModel.(SelectorModel)
	if selectorModel.GetError() != nil {
		return SelectorModel{}, selectorModel.GetError()
	}

	if selectorModel.IsCancelled() {
		return SelectorModel{}, fmt.Errorf("cancelled")
	}

	return selectorModel, nil
}
