package printer

import (
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchBackspaceRemovesWholeCharacter(t *testing.T) {
	m := model{searchMode: true, searchInput: "café🙂"}
	for _, want := range []string{"café", "caf"} {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = next.(model)
		if m.searchInput != want || !utf8.ValidString(m.searchInput) {
			t.Fatalf("search = %q, want %q", m.searchInput, want)
		}
	}
}

func TestCtrlCQuitsDuringSearch(t *testing.T) {
	_, command := (model{searchMode: true}).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command == nil {
		t.Fatal("no quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl+C did not quit")
	}
}
