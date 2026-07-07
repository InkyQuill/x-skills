package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
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
	body = append(body, "", mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
		{ASCII: "enter", Unicode: "↵", Label: "close"},
		{ASCII: "esc", Unicode: "Esc", Label: "close"},
		{ASCII: "q", Label: "close"},
	})))
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
