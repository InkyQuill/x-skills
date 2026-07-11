package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type resultModal struct {
	title  string
	lines  []string
	scroll scrollState
}

func newResultModal(title string, lines []string) modal {
	return resultModal{title: title, lines: lines}
}

func (r resultModal) Title() string {
	return r.title
}

func (r resultModal) View(width, height int, m Model) string {
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: r.title,
		Body:  r.lines,
		Footer: []string{tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "enter", Unicode: "↵", Label: "close"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		})},
		Scroll:    int(r.scroll),
		UseScroll: true,
	})
}

func (r resultModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		return true, nil
	default:
		if r.scroll.Handle(msg, len(r.lines), constrainedModalBodyHeight(m.height, 1)) {
			m.modal = r
			return false, nil
		}
		return false, nil
	}
}
