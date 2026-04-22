package printer

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	title string

	highlightStyle = lipgloss.NewStyle().Bold(true).Reverse(true)

	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
)

func Pager(s string, t string) {
	title = t
	p := tea.NewProgram(
		model{content: s},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run the pager:", err)
		os.Exit(1)
	}
}

type model struct {
	content  string
	lines    []string // original content split by newlines (set on init)
	ready    bool
	viewport viewport.Model

	// digit accumulation for N% jump (Session 2)
	digitBuf string

	// search state (Session 3)
	searchMode  bool
	searchInput string
	searchQuery string
	matchLines  []int // line indices (0-based) containing the query
	matchIndex  int   // current position in matchLines
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "enter":
				m.searchMode = false
				m.searchQuery = m.searchInput
				m.performSearch()
				return m, nil
			case "esc":
				m.searchMode = false
				m.searchInput = ""
				return m, nil
			case "backspace":
				if len(m.searchInput) > 0 {
					m.searchInput = m.searchInput[:len(m.searchInput)-1]
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 {
					m.searchInput += string(msg.Runes)
				}
				return m, nil
			}
		}
		switch k := msg.String(); k {
		case "ctrl+c", "q":
			m.digitBuf = ""
			return m, tea.Quit
		case "esc":
			m.digitBuf = ""
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.matchLines = nil
				m.viewport.SetContent(m.content)
				return m, nil
			}
			return m, tea.Quit
		case "g":
			m.digitBuf = ""
			m.viewport.GotoTop()
			return m, nil
		case "G":
			m.digitBuf = ""
			m.viewport.GotoBottom()
			return m, nil
		case "/":
			m.searchMode = true
			m.searchInput = ""
			m.digitBuf = ""
			return m, nil
		case "n":
			if len(m.matchLines) > 0 {
				m.matchIndex = (m.matchIndex + 1) % len(m.matchLines)
				m.viewport.SetYOffset(m.matchLines[m.matchIndex])
			}
			m.digitBuf = ""
			return m, nil
		case "N":
			if len(m.matchLines) > 0 {
				m.matchIndex = (m.matchIndex - 1 + len(m.matchLines)) % len(m.matchLines)
				m.viewport.SetYOffset(m.matchLines[m.matchIndex])
			}
			m.digitBuf = ""
			return m, nil
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.digitBuf += k
			return m, nil
		case "%":
			if m.digitBuf != "" {
				pct, _ := strconv.Atoi(m.digitBuf)
				if pct > 100 {
					pct = 100
				}
				total := m.viewport.TotalLineCount()
				target := total * pct / 100
				m.viewport.SetYOffset(target)
				m.digitBuf = ""
			}
			return m, nil
		default:
			m.digitBuf = ""
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.content)
			m.lines = strings.Split(m.content, "\n")
			m.viewport.KeyMap.PageDown.SetKeys("pgdown", " ", "f", "ctrl+f")
			m.viewport.KeyMap.PageUp.SetKeys("pgup", "b", "ctrl+b")
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m model) headerView() string {
	title := titleStyle.Render(title)
	line := strings.Repeat("─", maxPage(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	topLine := m.viewport.YOffset + 1
	bottomLine := min(m.viewport.YOffset+m.viewport.VisibleLineCount(), m.viewport.TotalLineCount())
	total := m.viewport.TotalLineCount()
	pct := m.viewport.ScrollPercent() * 100

	var infoText string
	if m.searchMode {
		infoText = fmt.Sprintf("/%s_", m.searchInput)
	} else if m.searchQuery != "" && len(m.matchLines) > 0 {
		infoText = fmt.Sprintf("Lines %d-%d of %d | [%d/%d] \"%s\" | %3.f%%",
			topLine, bottomLine, total,
			m.matchIndex+1, len(m.matchLines), m.searchQuery, pct)
	} else if m.searchQuery != "" {
		infoText = fmt.Sprintf("Lines %d-%d of %d | no matches for \"%s\" | %3.f%%",
			topLine, bottomLine, total, m.searchQuery, pct)
	} else if m.digitBuf != "" {
		infoText = fmt.Sprintf("Lines %d-%d of %d | %s…%%", topLine, bottomLine, total, m.digitBuf)
	} else {
		infoText = fmt.Sprintf("Lines %d-%d of %d | %3.f%%", topLine, bottomLine, total, pct)
	}
	info := infoStyle.Render(infoText)
	line := strings.Repeat("─", maxPage(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func (m *model) performSearch() {
	m.matchLines = nil
	m.matchIndex = 0

	if m.searchQuery == "" {
		m.viewport.SetContent(m.content)
		return
	}

	query := strings.ToLower(m.searchQuery)
	for i, line := range m.lines {
		if strings.Contains(strings.ToLower(line), query) {
			m.matchLines = append(m.matchLines, i)
		}
	}

	m.highlightContent()

	if len(m.matchLines) > 0 {
		m.viewport.SetYOffset(m.matchLines[0])
	}
}

func (m *model) highlightContent() {
	if m.searchQuery == "" {
		m.viewport.SetContent(m.content)
		return
	}

	matchSet := make(map[int]struct{}, len(m.matchLines))
	for _, idx := range m.matchLines {
		matchSet[idx] = struct{}{}
	}

	query := strings.ToLower(m.searchQuery)
	highlighted := make([]string, len(m.lines))
	for i, line := range m.lines {
		if _, ok := matchSet[i]; ok {
			highlighted[i] = highlightMatches(line, query)
		} else {
			highlighted[i] = line
		}
	}

	m.viewport.SetContent(strings.Join(highlighted, "\n"))
}

func highlightMatches(line, queryLower string) string {
	lower := strings.ToLower(line)
	var result strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], queryLower)
		if idx == -1 {
			result.WriteString(line[pos:])
			break
		}
		result.WriteString(line[pos : pos+idx])
		matchEnd := pos + idx + len(queryLower)
		result.WriteString(highlightStyle.Render(line[pos+idx : matchEnd]))
		pos = matchEnd
	}
	return result.String()
}

func maxPage(a, b int) int {
	if a > b {
		return a
	}
	return b
}
