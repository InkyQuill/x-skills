package tui

import (
	"strings"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type textModal struct {
	title string
	label string
	input textinput.Model
	apply func(*Model, string) tea.Cmd
}

func newTextModal(title, label, value string, apply func(*Model, string) tea.Cmd) modal {
	input := textinput.New()
	input.SetValue(value)
	input.Focus()
	return textModal{title: title, label: label, input: input, apply: apply}
}

func (t textModal) Title() string {
	return t.title
}

func (t textModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render(t.title),
		"",
		t.label,
		t.input.View(),
		"",
		mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "apply"},
			{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
		})),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (t textModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "enter":
		return false, t.apply(m, strings.TrimSpace(t.input.Value()))
	}

	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	m.modal = t
	return false, cmd
}
