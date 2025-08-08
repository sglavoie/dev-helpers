package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectorSearchFunctionality(t *testing.T) {
	// Create test items
	items := []SelectorItem{
		{ID: "1", DisplayText: "meeting with client", Data: "data1"},
		{ID: "2", DisplayText: "coding session", Data: "data2"},
		{ID: "3", DisplayText: "team meeting", Data: "data3"},
		{ID: "4", DisplayText: "documentation review", Data: "data4"},
		{ID: "5", DisplayText: "client presentation", Data: "data5"},
	}

	model := NewSelectorModel("Test Selector", items)

	t.Log("=== TESTING INITIAL STATE ===")
	if len(model.filteredItems) != len(model.items) {
		t.Errorf("Initial filtered items should match all items: got %d, expected %d", len(model.filteredItems), len(model.items))
	}
	if model.searchMode {
		t.Error("Search mode should be false initially")
	}

	t.Log("=== TESTING ENTERING SEARCH MODE ===")
	// Press 'f' to enter search mode
	fKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	updatedModel, _ := model.Update(fKeyMsg)
	model = updatedModel.(SelectorModel)

	if !model.searchMode {
		t.Error("Search mode should be true after pressing 'f'")
	}
	if !model.searchInput.Focused() {
		t.Error("Search input should be focused after entering search mode")
	}

	t.Log("=== TESTING SEARCH FILTERING ===")
	// Type "meeting" to search
	searchChars := []rune{'m', 'e', 'e', 't', 'i', 'n', 'g'}
	for _, char := range searchChars {
		charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
		updatedModel, _ := model.Update(charMsg)
		model = updatedModel.(SelectorModel)
	}

	// Check that filtering worked
	if len(model.filteredItems) != 2 {
		t.Errorf("Expected 2 filtered items for 'meeting', got %d", len(model.filteredItems))
		for i, item := range model.filteredItems {
			t.Logf("Filtered item %d: %s", i, item.DisplayText)
		}
	}

	// Check that the right items are filtered
	expectedItems := []string{"meeting with client", "team meeting"}
	if len(model.filteredItems) == 2 {
		for i, expectedText := range expectedItems {
			if model.filteredItems[i].DisplayText != expectedText {
				t.Errorf("Expected filtered item %d to be '%s', got '%s'", i, expectedText, model.filteredItems[i].DisplayText)
			}
		}
	}

	t.Log("=== TESTING SEARCH REFINEMENT ===")
	// Add " with" to narrow down search to only "meeting with client"
	additionalChars := []rune{' ', 'w', 'i', 't', 'h'}
	for _, char := range additionalChars {
		charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
		updatedModel, _ := model.Update(charMsg)
		model = updatedModel.(SelectorModel)
	}

	// Should now have only 1 result
	if len(model.filteredItems) != 1 {
		t.Errorf("Expected 1 filtered item for 'meeting with', got %d", len(model.filteredItems))
		for i, item := range model.filteredItems {
			t.Logf("Remaining filtered item %d: %s", i, item.DisplayText)
		}
	}
	if len(model.filteredItems) == 1 && model.filteredItems[0].DisplayText != "meeting with client" {
		t.Errorf("Expected filtered item to be 'meeting with client', got '%s'", model.filteredItems[0].DisplayText)
	}

	t.Log("=== TESTING SEARCH CLEAR ===")
	// Press Esc to exit search mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = model.Update(escMsg)
	model = updatedModel.(SelectorModel)

	if model.searchMode {
		t.Error("Search mode should be false after pressing Esc")
	}
	if model.searchInput.Focused() {
		t.Error("Search input should not be focused after exiting search mode")
	}
	if len(model.filteredItems) != len(model.items) {
		t.Errorf("All items should be shown after exiting search: got %d, expected %d", len(model.filteredItems), len(model.items))
	}
	if model.searchInput.Value() != "" {
		t.Errorf("Search input should be cleared after exiting search mode, got '%s'", model.searchInput.Value())
	}

	t.Log("=== TESTING NO RESULTS ===")
	// Enter search mode again and search for something that doesn't exist
	fKeyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	updatedModel, _ = model.Update(fKeyMsg)
	model = updatedModel.(SelectorModel)

	noMatchChars := []rune{'x', 'y', 'z'}
	for _, char := range noMatchChars {
		charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
		updatedModel, _ := model.Update(charMsg)
		model = updatedModel.(SelectorModel)
	}

	if len(model.filteredItems) != 0 {
		t.Errorf("Expected 0 filtered items for 'xyz', got %d", len(model.filteredItems))
	}
	// Note: table cursor handling for empty results is managed by the table component itself
}

func TestSelectorNavigationInSearchMode(t *testing.T) {
	// Create test items
	items := []SelectorItem{
		{ID: "1", DisplayText: "apple", Data: "data1"},
		{ID: "2", DisplayText: "application", Data: "data2"},
		{ID: "3", DisplayText: "apply", Data: "data3"},
		{ID: "4", DisplayText: "banana", Data: "data4"},
	}

	model := NewSelectorModel("Test Selector", items)

	// Enter search mode and search for "app"
	fKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	updatedModel, _ := model.Update(fKeyMsg)
	model = updatedModel.(SelectorModel)

	searchChars := []rune{'a', 'p', 'p'}
	for _, char := range searchChars {
		charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
		updatedModel, _ := model.Update(charMsg)
		model = updatedModel.(SelectorModel)
	}

	// Should have 3 matches: apple, application, apply
	if len(model.filteredItems) != 3 {
		t.Errorf("Expected 3 filtered items for 'app', got %d", len(model.filteredItems))
	}

	// Test navigation in search mode
	t.Log("=== TESTING NAVIGATION IN SEARCH MODE ===")
	if model.table.Cursor() != 0 {
		t.Errorf("Initial selection should be 0, got %d", model.table.Cursor())
	}

	// Navigate down
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = model.Update(downMsg)
	model = updatedModel.(SelectorModel)
	if model.table.Cursor() != 1 {
		t.Errorf("Selection should be 1 after down, got %d", model.table.Cursor())
	}

	// Navigate down again
	updatedModel, _ = model.Update(downMsg)
	model = updatedModel.(SelectorModel)
	if model.table.Cursor() != 2 {
		t.Errorf("Selection should be 2 after second down, got %d", model.table.Cursor())
	}

	// Navigate up
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = model.Update(upMsg)
	model = updatedModel.(SelectorModel)
	if model.table.Cursor() != 1 {
		t.Errorf("Selection should be 1 after up, got %d", model.table.Cursor())
	}

	// Test selection while in search mode
	t.Log("=== TESTING SELECTION IN SEARCH MODE ===")
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(SelectorModel)

	if !model.done {
		t.Error("Model should be done after pressing Enter in search mode")
	}
	if model.selectedItem == nil {
		t.Error("Selected item should not be nil")
	} else if model.selectedItem.DisplayText != "application" {
		t.Errorf("Expected selected item to be 'application', got '%s'", model.selectedItem.DisplayText)
	}
}

func TestFilterItemsFunction(t *testing.T) {
	// Test the filterItems function directly
	items := []SelectorItem{
		{ID: "1", DisplayText: "Meeting with Client", Data: "data1"},
		{ID: "2", DisplayText: "Code Review Session", Data: "data2"},
		{ID: "3", DisplayText: "Client Presentation", Data: "data3"},
		{ID: "4", DisplayText: "Team Meeting", Data: "data4"},
	}

	model := NewSelectorModel("Test", items)

	t.Log("=== TESTING CASE INSENSITIVE FILTERING ===")
	model.filterItems("CLIENT")
	if len(model.filteredItems) != 2 {
		t.Errorf("Expected 2 items for 'CLIENT', got %d", len(model.filteredItems))
	}

	model.filterItems("meeting")
	if len(model.filteredItems) != 2 {
		t.Errorf("Expected 2 items for 'meeting', got %d", len(model.filteredItems))
	}

	model.filterItems("code")
	if len(model.filteredItems) != 1 {
		t.Errorf("Expected 1 item for 'code', got %d", len(model.filteredItems))
	}

	model.filterItems("")
	if len(model.filteredItems) != len(model.items) {
		t.Errorf("Expected all items for empty query, got %d", len(model.filteredItems))
	}
}
