package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m FieldEditorModel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("📝 Edit Entry Fields"))
	b.WriteString("\n")

	// Mode selector - show all modes with current one highlighted
	b.WriteString(m.renderModeSelector())
	b.WriteString("\n")

	// Entry info
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginBottom(1)

	status := "Stopped"
	if m.entry.Active {
		status = "Running"
	}

	b.WriteString(infoStyle.Render(fmt.Sprintf("Entry ID: %d | Status: %s", m.entry.ShortID, status)))
	b.WriteString("\n\n")

	// Fields in a 2-column layout
	leftColumn := []string{}
	rightColumn := []string{}

	for i, field := range m.fields {
		fieldView := m.renderField(field, i == m.focused)
		if i%2 == 0 {
			leftColumn = append(leftColumn, fieldView)
		} else {
			rightColumn = append(rightColumn, fieldView)
		}
	}

	// Balance columns
	for len(leftColumn) < len(rightColumn) {
		leftColumn = append(leftColumn, "")
	}
	for len(rightColumn) < len(leftColumn) {
		rightColumn = append(rightColumn, "")
	}

	// Join columns
	leftContent := strings.Join(leftColumn, "\n\n")
	rightContent := strings.Join(rightColumn, "\n\n")

	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(50).Render(leftContent),
		lipgloss.NewStyle().Width(50).MarginLeft(4).Render(rightContent),
	)

	b.WriteString(columns)
	b.WriteString("\n\n")

	// Instructions
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	b.WriteString(helpStyle.Render("Tab/↑↓: Navigate • Enter: Save Changes • Esc: Cancel"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Shift+Tab: Switch Mode"))

	// Show error if any
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginTop(1)
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.err.Error())))
	}

	return b.String()
}

func (m FieldEditorModel) renderField(field fieldModel, focused bool) string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		MarginTop(1)

	if focused {
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("62"))
	} else {
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("241"))
	}

	b.WriteString(labelStyle.Render(field.displayName))
	b.WriteString("\n")
	b.WriteString(descStyle.Render(field.description))
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(field.input.View()))

	return b.String()
}

func (m FieldEditorModel) renderModeSelector() string {
	// Define all available modes
	allModes := []InputMode{
		ModeDurationStartTime,
		ModeStartEndTime,
		ModeDurationEndTime,
	}

	// Styles for mode display
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Bold(true).
		Padding(0, 1)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Bold(true)

	var modeItems []string
	for _, mode := range allModes {
		if mode == m.inputMode {
			modeItems = append(modeItems, selectedStyle.Render(mode.String()))
		} else {
			modeItems = append(modeItems, unselectedStyle.Render(mode.String()))
		}
	}

	// Join modes with separators
	modeDisplay := strings.Join(modeItems, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(" | "))

	return headerStyle.Render("Modes: ") + modeDisplay
}
