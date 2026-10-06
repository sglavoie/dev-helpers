package inputs

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmationKeysPreserveDefaultsAndExplicitDeclines(t *testing.T) {
	for _, first := range []string{"Yes", "No"} {
		second := "Yes"
		if first == "Yes" {
			second = "No"
		}
		for _, tc := range []struct {
			key  tea.KeyMsg
			want string
		}{
			{tea.KeyMsg{Type: tea.KeyEnter}, first},
			{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}, "Yes"},
			{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}, "Yes"},
			{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, "No"},
			{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")}, "No"},
			{tea.KeyMsg{Type: tea.KeyEsc}, "No"},
			{tea.KeyMsg{Type: tea.KeyCtrlC}, "No"},
		} {
			m := model{list: list.New([]list.Item{item(first), item(second)}, itemDelegate{}, 40, 8)}
			updated, cmd := m.Update(tc.key)
			if updated.(model).choice != tc.want || cmd == nil {
				t.Fatalf("default %s, key %s: choice %q, want %q", first, tc.key, updated.(model).choice, tc.want)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("answer did not close prompt")
			}
			if updated.(model).View() != "" {
				t.Fatal("answered prompt was not cleared")
			}
		}
	}
}
