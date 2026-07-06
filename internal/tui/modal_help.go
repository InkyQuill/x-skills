package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type helpModal struct{}

func newHelpModal() modal {
	return helpModal{}
}

func (h helpModal) Title() string {
	return "Help"
}

func (h helpModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render("Help"),
		"Keyboard Shortcuts",
		"  A        switch to Active view",
		"  R        switch to Repo view",
		"  D        switch to Doctor view",
		"  I        reserved for Install view",
		"  enter    view row details",
		"  /        enter local filter mode",
		"  space    toggle row selection",
		"  c        clear selection",
		"  p        preview SKILL.md",
		"  ^R       rescan filesystem",
		"  ?        show this help screen",
		"  q        quit application",
		"",
		"Symbol Legend",
		"  " + m.symbols.Cursor + "  cursor position",
		"  " + m.symbols.Unchecked + "  unselected item",
		"  " + m.symbols.Checked + "  selected item",
		"  " + m.symbols.CountPrefix + "N group count badge",
		"",
		"Root Chip Legend",
		"  .Ag  project agents",
		"  .Cl  project claude",
		"  .Cd  project codex",
		"  ~Ag  global agents",
		"  ~Cl  global claude",
		"  ~Cd  global codex",
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (h helpModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	return closeOnEscapeOrQuit(msg), nil
}
