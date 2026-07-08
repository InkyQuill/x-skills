package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type choiceModal struct {
	title   string
	lines   []string
	choices []string
	index   int
	apply   func(*Model, int) tea.Cmd
}

func newChoiceModal(title string, lines, choices []string, defaultIndex int, apply func(*Model, int)) modal {
	return newChoiceModalWithCommand(title, lines, choices, defaultIndex, func(m *Model, choice int) tea.Cmd {
		apply(m, choice)
		return nil
	})
}

func newChoiceModalWithCommand(title string, lines, choices []string, defaultIndex int, apply func(*Model, int) tea.Cmd) modal {
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
		line := prefix + choice
		if i == c.index {
			line = selectedBg.Render(line)
		}
		body = append(body, line)
	}
	body = append(body, "", mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
		{ASCII: "up/down", Unicode: "↑/↓", Label: "choose"},
		{ASCII: "enter", Unicode: "↵", Label: "apply"},
		{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
		{ASCII: "q", Label: "close"},
	})))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (c choiceModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	switch msg.String() {
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
		return false, c.apply(m, c.index)
	}
	return false, nil
}
