package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type previewModal struct {
	title    string
	path     string
	raw      string
	rendered bool
}

func newPreviewModal(title, skillPath string) modal {
	rawBytes, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	raw := ""
	if err != nil {
		raw = "read SKILL.md: " + err.Error()
	} else {
		raw = string(rawBytes)
	}
	return previewModal{title: title, path: filepath.Join(skillPath, "SKILL.md"), raw: raw, rendered: true}
}

func (p previewModal) Title() string {
	return p.title
}

func (p previewModal) View(width, height int, m Model) string {
	mode := "rendered with Glamour"
	bodyText := p.raw
	if p.rendered {
		if rendered, err := glamour.Render(p.raw, "dark"); err == nil {
			bodyText = rendered
		}
	} else {
		mode = "raw SKILL.md"
	}
	lines := []string{
		accentStyle.Render(p.title),
		p.path + "       " + mode,
		strings.Repeat("-", 60),
		bodyText,
		"",
		mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "r", Label: "raw/rendered"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		})),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (p previewModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if msg.String() == "r" {
		p.rendered = !p.rendered
		m.modal = p
	}
	return false, nil
}
