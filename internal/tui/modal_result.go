package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type resultModal struct {
	title string
	lines []string
}

func newResultModal(title string, lines []string) modal {
	return resultModal{title: title, lines: lines}
}

func (r resultModal) Title() string {
	return r.title
}

func (r resultModal) View(width, height int, m Model) string {
	body := append([]string{accentStyle.Render(r.title), ""}, r.lines...)
	body = append(body, "", mutedStyle.Render("enter close   esc close   q close"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (r resultModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		return true, nil
	default:
		return false, nil
	}
}
