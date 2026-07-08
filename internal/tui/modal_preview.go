package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type previewModal struct {
	title    string
	path     string
	raw      string
	rendered bool
	viewport viewport.Model
}

func newPreviewModal(title, skillPath string) modal {
	rawBytes, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	raw := ""
	if err != nil {
		raw = "read SKILL.md: " + err.Error()
	} else {
		raw = string(rawBytes)
	}
	vp := viewport.New(0, 0)
	p := previewModal{title: title, path: filepath.Join(skillPath, "SKILL.md"), raw: raw, rendered: true, viewport: vp}
	p.viewport.SetContent(p.renderContent())
	return p
}

func (p previewModal) Title() string {
	return p.title
}

func (p previewModal) renderContent() string {
	if p.rendered {
		rendered, err := glamour.Render(p.raw, "dark")
		if err == nil {
			return strings.TrimRight(rendered, "\n")
		}
	}
	return strings.TrimRight(p.raw, "\n")
}

func (p previewModal) View(width, height int, m Model) string {
	mode := "rendered with Glamour"
	if !p.rendered {
		mode = "raw SKILL.md"
	}
	bodyHeight := height - 12
	if bodyHeight < 4 {
		bodyHeight = 4
	}
	bodyWidth := width - 12
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	p.viewport.Width = bodyWidth
	p.viewport.Height = bodyHeight
	p.viewport.SetContent(p.renderContent())
	lines := []string{
		accentStyle.Render(p.title),
		p.path + "       " + mode,
		strings.Repeat("-", 60),
		p.viewport.View(),
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
		p.viewport.SetContent(p.renderContent())
		p.viewport.GotoTop()
		m.modal = p
		return false, nil
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	m.modal = p
	return false, cmd
}
