package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	tabStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	activeTab     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	cursorBg      = lipgloss.NewStyle().Background(lipgloss.Color("60"))
	selectedBg    = lipgloss.NewStyle().Background(lipgloss.Color("238"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	kbdStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("236"))
	chipStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("60")).Padding(0, 1)
	projectChip   = lipgloss.NewStyle().Background(lipgloss.Color("24"))
	globalChip    = lipgloss.NewStyle().Background(lipgloss.Color("95"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	accentStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	dangerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	archiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	incomingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	diffMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	managedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	unmanaged     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func init() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		titleStyle = titleStyle.UnsetForeground().UnsetBackground()
		tabStyle = tabStyle.UnsetForeground().UnsetBackground()
		activeTab = activeTab.UnsetForeground().UnsetBackground()
		panelStyle = panelStyle.UnsetBorderForeground()
		cursorStyle = cursorStyle.UnsetForeground().UnsetBackground()
		selectedStyle = selectedStyle.UnsetForeground().UnsetBackground()
		cursorBg = cursorBg.UnsetBackground()
		selectedBg = selectedBg.UnsetBackground()
		mutedStyle = mutedStyle.UnsetForeground().UnsetBackground()
		kbdStyle = kbdStyle.UnsetForeground().UnsetBackground()
		chipStyle = chipStyle.UnsetForeground().UnsetBackground()
		projectChip = projectChip.UnsetForeground().UnsetBackground()
		globalChip = globalChip.UnsetForeground().UnsetBackground()
		okStyle = okStyle.UnsetForeground().UnsetBackground()
		accentStyle = accentStyle.UnsetForeground().UnsetBackground()
		dangerStyle = dangerStyle.UnsetForeground().UnsetBackground()
		archiveStyle = archiveStyle.UnsetForeground().UnsetBackground()
		incomingStyle = incomingStyle.UnsetForeground().UnsetBackground()
		diffMetaStyle = diffMetaStyle.UnsetForeground().UnsetBackground()
		managedStyle = managedStyle.UnsetForeground().UnsetBackground()
		unmanaged = unmanaged.UnsetForeground().UnsetBackground()
	}
}

func tabLabel(active bool, key, label string) string {
	text := key + ":" + label
	if active {
		return activeTab.Render(text)
	}
	return tabStyle.Render(text)
}

func modalStyle(width, height int) lipgloss.Style {
	modalWidth := width - 8
	if modalWidth > 88 {
		modalWidth = 88
	}
	if modalWidth < 40 {
		modalWidth = width - 2
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("110")).
		Padding(0, 1).
		Width(modalWidth)
}

func renderStatusChip(m Model, status string) string {
	switch status {
	case "managed":
		return managedStyle.Render(m.symbols.Managed + " managed")
	case "unmanaged":
		return unmanaged.Render(m.symbols.Unmanaged + " unmanaged")
	case "broken":
		return dangerStyle.Render(m.symbols.Broken + " broken")
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
