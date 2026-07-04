package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	tabStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	activeTab     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	wizardStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("110")).Padding(0, 1)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	chipStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("60")).Padding(0, 1)
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	accentStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	dangerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	managedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	unmanaged     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func tabLabel(active bool, key, label string) string {
	text := key + ":" + label
	if active {
		return activeTab.Render(text)
	}
	return tabStyle.Render(text)
}

func renderStatusChip(status string) string {
	switch status {
	case "managed":
		return managedStyle.Render(status)
	case "unmanaged":
		return unmanaged.Render(status)
	case "broken":
		return dangerStyle.Render(status)
	default:
		return mutedStyle.Render(status)
	}
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "...")
}
