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

func viewForKey(msg tea.KeyMsg) (ViewName, bool) {
	switch msg.String() {
	case keyActive:
		return ViewActive, true
	case keyRepo:
		return ViewRepo, true
	case keyDoctor:
		return ViewDoctor, true
	case keyInstall:
		return ViewInstall, true
	default:
		return "", false
	}
}

func listCursorDelta(msg tea.KeyMsg) int {
	switch msg.String() {
	case "up", "k":
		return -1
	case "down", "j":
		return 1
	default:
		return 0
	}
}
