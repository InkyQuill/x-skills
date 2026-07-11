package tui

import tea "github.com/charmbracelet/bubbletea"

const (
	keyActive     = "A"
	keyRepo       = "R"
	keyDoctor     = "D"
	keyInstall    = "I"
	keyHelp       = "?"
	keyRepoRename = "n"
)

func repoActionKeys() map[string]string {
	return map[string]string{
		"delete":    "d",
		"link":      "l",
		"recommend": "r",
		"rename":    keyRepoRename,
		"unlink":    "u",
	}
}

func isRefreshKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyCtrlR || msg.String() == "ctrl+r"
}
