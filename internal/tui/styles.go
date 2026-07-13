package tui

import (
	"os"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	versionStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	updateStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	tabStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	activeTab           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	panelStyle          = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	cursorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	selectedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	cursorBg            = lipgloss.NewStyle().Background(lipgloss.Color("60"))
	selectedBg          = lipgloss.NewStyle().Background(lipgloss.Color("238"))
	mutedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	kbdStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("236"))
	projectChip         = lipgloss.NewStyle().Background(lipgloss.Color("24"))
	globalChip          = lipgloss.NewStyle().Background(lipgloss.Color("95"))
	chipText            = lipgloss.NewStyle().Foreground(lipgloss.Color("230"))
	okStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	accentStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	inspectorTitleStyle = accentStyle
	inspectorKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	inspectorValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	installSourceStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	installCountStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	dangerStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	archiveStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	incomingStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	diffMetaStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	managedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	unmanaged           = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func init() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		titleStyle = titleStyle.UnsetForeground().UnsetBackground()
		versionStyle = versionStyle.UnsetForeground().UnsetBackground()
		updateStyle = updateStyle.UnsetForeground().UnsetBackground()
		tabStyle = tabStyle.UnsetForeground().UnsetBackground()
		activeTab = activeTab.UnsetForeground().UnsetBackground()
		panelStyle = panelStyle.UnsetBorderForeground()
		cursorStyle = cursorStyle.UnsetForeground().UnsetBackground()
		selectedStyle = selectedStyle.UnsetForeground().UnsetBackground()
		cursorBg = cursorBg.UnsetBackground()
		selectedBg = selectedBg.UnsetBackground()
		mutedStyle = mutedStyle.UnsetForeground().UnsetBackground()
		kbdStyle = kbdStyle.UnsetForeground().UnsetBackground()
		projectChip = projectChip.UnsetForeground().UnsetBackground()
		globalChip = globalChip.UnsetForeground().UnsetBackground()
		chipText = chipText.UnsetForeground().UnsetBackground()
		okStyle = okStyle.UnsetForeground().UnsetBackground()
		accentStyle = accentStyle.UnsetForeground().UnsetBackground()
		inspectorTitleStyle = inspectorTitleStyle.UnsetForeground().UnsetBackground()
		inspectorKeyStyle = inspectorKeyStyle.UnsetForeground().UnsetBackground()
		inspectorValueStyle = inspectorValueStyle.UnsetForeground().UnsetBackground()
		installSourceStyle = installSourceStyle.UnsetForeground().UnsetBackground()
		installCountStyle = installCountStyle.UnsetForeground().UnsetBackground()
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
	modalWidth := modalContentWidth(width) + 2
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("110")).
		Padding(0, 1).
		MaxHeight(max(height-2, 1)).
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

func renderStatusDotWithBackground(m Model, status string, background lipgloss.TerminalColor) string {
	switch status {
	case "managed":
		return tuiui.RenderWithBackground(managedStyle, background, m.symbols.Managed)
	case "unmanaged":
		return tuiui.RenderWithBackground(unmanaged, background, m.symbols.Unmanaged)
	case "broken":
		return tuiui.RenderWithBackground(dangerStyle, background, m.symbols.Broken)
	default:
		return tuiui.RenderWithBackground(mutedStyle, background, m.symbols.Unmanaged)
	}
}
