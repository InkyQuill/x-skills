package tui

import (
	"fmt"

	"github.com/InkyQuill/x-skills/internal/roots"
	tea "github.com/charmbracelet/bubbletea"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type helpModal struct {
	scroll int
}

func newHelpModal() modal {
	return helpModal{}
}

func (h helpModal) Title() string {
	return "Help"
}

func (h helpModal) View(width, height int, m Model) string {
	lines := []string{
		"Keyboard Shortcuts",
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "A", Label: "switch to Active view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "R", Label: "switch to Repo view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "D", Label: "switch to Doctor view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "I", Label: "switch to Install view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "enter", Unicode: "↵", Label: "view row details"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "/", Label: "enter local filter mode"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "/", Label: "Install: / search"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "o", Label: "Install: edit owner filter"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "i", Label: "Install: i install and use"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "a", Label: "Install: a archive only"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "space", Label: "toggle Active/Repo row selection (Install too)"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "c", Label: "clear Active/Repo selection (Install too)"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "p", Label: "preview SKILL.md"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "l", Label: "link repo skill"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "r", Label: "Repo: toggle project recommendation"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "u", Label: "unlink active/repo usages"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "d", Label: "delete repo skill"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "m", Label: "migrate active skill"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "f", Label: "fix doctor issues"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "s", Label: "restore project skills"}) + " · " +
			helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "S", Label: "sync Skills Folders"}),
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
	}
	lines = append(lines, helpRootLines(m)...)
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: "Help",
		Body:  lines,
		Footer: []string{mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		}))},
		Scroll:    h.scroll,
		UseScroll: true,
	})
}

func helpRootLines(m Model) []string {
	activeRoots := roots.ActiveRoots(m.cfg, roots.Filter{})
	lines := []string{"Root Chip Legend"}
	if len(activeRoots) == 0 {
		return append(lines, "  no active roots configured")
	}
	for _, root := range activeRoots {
		lines = append(lines, fmt.Sprintf("  %s  %s:%s", rootLabel(root), root.Scope, root.Target))
	}
	return lines
}

func helpCommand(ascii bool, command tuiui.Shortcut) string {
	return tuiui.RenderShortcut(ascii, kbdStyle, command)
}

func (h helpModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if delta := modalMoveDelta(msg); delta != 0 {
		h.scroll += delta
		if h.scroll < 0 {
			h.scroll = 0
		}
		m.modal = h
	}
	return false, nil
}
