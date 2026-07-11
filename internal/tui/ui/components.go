package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type PillProps struct {
	Color      lipgloss.TerminalColor
	Background lipgloss.TerminalColor
	Text       string
	TextColor  lipgloss.TerminalColor
}

func Pill(left, right string, props PillProps) string {
	edge := lipgloss.NewStyle().Foreground(props.Color).Background(props.Background)
	content := lipgloss.NewStyle().Bold(true).Foreground(props.TextColor).Background(props.Color)
	return edge.Render(left) + content.Render(props.Text) + edge.Render(right)
}

type Shortcut struct {
	ASCII   string
	Unicode string
	Label   string
}

func (s Shortcut) Key(ascii bool) string {
	if !ascii && s.Unicode != "" {
		return s.Unicode
	}
	return s.ASCII
}

func RenderShortcut(ascii bool, keyStyle lipgloss.Style, command Shortcut) string {
	return keyStyle.Render(command.Key(ascii)) + " " + command.Label
}

func ToolHints(ascii bool, keyStyle lipgloss.Style, commands []Shortcut) string {
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		parts = append(parts, RenderShortcut(ascii, keyStyle, command))
	}
	return strings.Join(parts, "  ")
}

func FooterLine(ascii bool, keyStyle, mutedStyle lipgloss.Style, shortcuts []Shortcut) string {
	return mutedStyle.Render(ToolHints(ascii, keyStyle, shortcuts))
}
