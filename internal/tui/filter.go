package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type filterState struct {
	Active bool
	Query  string
}

func (f filterState) matches(values ...string) bool {
	query := strings.TrimSpace(strings.ToLower(f.Query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (f *filterState) update(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		f.Active = false
		f.Query = ""
		return true
	case "enter":
		f.Active = false
		return true
	case "backspace":
		if len(f.Query) > 0 {
			f.Query = f.Query[:len(f.Query)-1]
		}
		return true
	}
	if len(msg.Runes) > 0 {
		f.Query += string(msg.Runes)
		return true
	}
	return true
}
