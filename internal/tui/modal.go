package tui

import tea "github.com/charmbracelet/bubbletea"

type modal interface {
	Title() string
	View(width, height int, m Model) string
	Update(msg tea.KeyMsg, m *Model) (close bool, cmd tea.Cmd)
}

func closeOnEscapeOrQuit(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "q":
		return true
	default:
		return false
	}
}
