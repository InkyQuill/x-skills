package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type confirmModal struct {
	title  string
	lines  []string
	choice int
	apply  func(*Model)
}

func newConfirmModal(title string, lines []string, destructive bool, apply func(*Model)) modal {
	choice := 0
	if destructive {
		choice = 1
	}
	return confirmModal{title: title, lines: lines, choice: choice, apply: apply}
}

func (c confirmModal) Title() string {
	return c.title
}

func (c confirmModal) View(width, height int, m Model) string {
	apply := "[ Apply ]"
	cancel := "Cancel"
	if c.choice == 1 {
		apply = "Apply"
		cancel = "[ Cancel ]"
	}
	if c.choice == 0 {
		apply = selectedBg.Render(apply)
	} else {
		cancel = selectedBg.Render(cancel)
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: c.title,
		Body:  c.lines,
		Footer: []string{
			apply + "   " + cancel,
			mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
				{ASCII: "left/right", Unicode: "←/→", Label: "choose"},
				{ASCII: "enter", Unicode: "↵", Label: "apply"},
				{ASCII: "y/n", Label: "select"},
				{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
			})),
		},
	})
}

func (c confirmModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		if c.choice == 0 {
			c.apply(m)
			return false, nil
		}
		return true, nil
	}
	switch msg.String() {
	case "esc", "q", "n":
		return true, nil
	case "left", "right":
		if c.choice == 0 {
			c.choice = 1
		} else {
			c.choice = 0
		}
		m.modal = c
	case "y":
		c.apply(m)
	}
	return false, nil
}
