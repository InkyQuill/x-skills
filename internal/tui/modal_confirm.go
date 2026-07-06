package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type confirmModal struct {
	title       string
	lines       []string
	destructive bool
	choice      int
	apply       func(*Model)
}

func newConfirmModal(title string, lines []string, destructive bool, apply func(*Model)) modal {
	choice := 0
	if destructive {
		choice = 1
	}
	return confirmModal{title: title, lines: lines, destructive: destructive, choice: choice, apply: apply}
}

func (c confirmModal) Title() string {
	return c.title
}

func (c confirmModal) View(width, height int, m Model) string {
	buttons := "[ Apply ]   Cancel"
	if c.choice == 1 {
		buttons = "Apply   [ Cancel ]"
	}
	body := append([]string{accentStyle.Render(c.title), ""}, c.lines...)
	body = append(body, "", buttons, mutedStyle.Render("left/right choose   enter apply   y/n select   esc cancel"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
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
