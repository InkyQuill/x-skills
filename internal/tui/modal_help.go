package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
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
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "A", Label: "switch to Active view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "R", Label: "switch to Repo view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "D", Label: "switch to Doctor view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "I", Label: "Install (design in progress, not yet available)"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "enter", Unicode: "↵", Label: "view row details"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "/", Label: "enter local filter mode"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "space", Label: "toggle Active/Repo row selection"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "c", Label: "clear Active/Repo selection"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "p", Label: "preview SKILL.md"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "l", Label: "link repo skill"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "u", Label: "unlink active/repo usages"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "d", Label: "delete repo skill"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "m", Label: "migrate active skill"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "f", Label: "fix doctor issues"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "^R", Label: "rescan filesystem"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "?", Label: "show this help screen"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "q", Label: "quit application"}),
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

func helpCommand(ascii bool, command tuiui.Shortcut) string {
	return tuiui.RenderShortcut(ascii, kbdStyle, command)
}

func (h helpModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	return closeOnEscapeOrQuit(msg), nil
}
