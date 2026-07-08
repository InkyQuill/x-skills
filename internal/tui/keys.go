package tui

import tea "github.com/charmbracelet/bubbletea"

const (
	keyActive  = "A"
	keyRepo    = "R"
	keyDoctor  = "D"
	keyInstall = "I"
	keyHelp    = "?"
)

func isRefreshKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyCtrlR || msg.String() == "ctrl+r"
}
