package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type choiceModal struct {
	title   string
	lines   []string
	choices []string
	index   int
	apply   func(*Model, int)
}

func newChoiceModal(title string, lines, choices []string, defaultIndex int, apply func(*Model, int)) modal {
	return choiceModal{title: title, lines: lines, choices: choices, index: defaultIndex, apply: apply}
}

func (c choiceModal) Title() string {
	return c.title
}

func (c choiceModal) View(width, height int, m Model) string {
	body := append([]string{accentStyle.Render(c.title), ""}, c.lines...)
	body = append(body, "")
	for i, choice := range c.choices {
		prefix := "  "
		if i == c.index {
			prefix = m.symbols.Cursor + " "
		}
		body = append(body, prefix+choice)
	}
	body = append(body, "", mutedStyle.Render("up/down choose   enter apply   esc cancel   q close"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (c choiceModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up":
		if c.index > 0 {
			c.index--
		}
		m.modal = c
	case "down":
		if c.index+1 < len(c.choices) {
			c.index++
		}
		m.modal = c
	case "enter":
		c.apply(m, c.index)
	}
	if msg.Type == tea.KeyEnter {
		c.apply(m, c.index)
	}
	return false, nil
}
