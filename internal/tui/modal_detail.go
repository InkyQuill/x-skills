package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/doctor"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type detailModal struct {
	title  string
	lines  []string
	scroll scrollState
}

func newDetailModal(title string, lines []string) modal {
	return detailModal{title: title, lines: lines}
}

func doctorDetailModal(issue doctor.Issue) modal {
	lines := []string{
		"Issue kind",
		"  " + issue.Kind,
		"Affected path",
		"  " + issue.Path,
		"Reason",
		"  " + issue.Reason,
		"Safe fix",
		"  " + issue.SafeFix,
	}
	return newDetailModal("Detail: "+issue.Name+" (Doctor)", lines)
}

func (d detailModal) Title() string {
	return d.title
}

func (d detailModal) View(width, height int, m Model) string {
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: d.title,
		Body:  d.lines,
		Footer: []string{tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		})},
		Scroll:    int(d.scroll),
		UseScroll: true,
	})
}

func (d detailModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if d.scroll.Handle(msg, len(d.lines), constrainedModalBodyHeight(m.height, 1)) {
		m.modal = d
	}
	return false, nil
}
