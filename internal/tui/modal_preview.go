package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"gopkg.in/yaml.v3"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type previewModal struct {
	title    string
	path     string
	raw      string
	content  string
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
	p.refreshContent()
	return p
}

func (p previewModal) Title() string {
	return p.title
}

func (p *previewModal) refreshContent() {
	if p.rendered {
		rendered, err := glamour.Render(renderedPreviewMarkdown(p.raw), "dark")
		if err == nil {
			p.content = strings.TrimRight(rendered, "\n")
			p.viewport.SetContent(p.content)
			return
		}
	}
	p.content = strings.TrimRight(p.raw, "\n")
	p.viewport.SetContent(p.content)
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
	skillFile := filepath.Base(filepath.Dir(p.path)) + "/" + filepath.Base(p.path)
	detail := tuiui.TruncateANSI(fmt.Sprintf("%s  |  %s", skillFile, mode), bodyWidth)
	lines := []string{
		accentStyle.Render(tuiui.TruncateANSI(p.title, bodyWidth)),
		mutedStyle.Render(detail),
		strings.Repeat("-", bodyWidth),
		p.viewport.View(),
		"",
		tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "r", Label: "raw/rendered"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		}),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func renderedPreviewMarkdown(value string) string {
	frontmatter, body, ok := splitYAMLFrontmatter(value)
	if !ok {
		return value
	}
	body = strings.TrimLeft(body, "\r\n")
	description := frontmatterDescription(frontmatter)
	if description == "" || strings.Contains(body, description) {
		return body
	}
	if strings.TrimSpace(body) == "" {
		return description + "\n"
	}
	return description + "\n\n" + body
}

func splitYAMLFrontmatter(value string) (string, string, bool) {
	if !strings.HasPrefix(value, "---\n") && !strings.HasPrefix(value, "---\r\n") {
		return "", value, false
	}
	lines := strings.SplitAfter(value, "\n")
	if len(lines) < 3 {
		return "", value, false
	}
	for i := 1; i < len(lines); i++ {
		if isYAMLDocumentDelimiter(lines[i]) {
			return strings.Join(lines[1:i], ""), linesAfter(lines, i+1), true
		}
	}
	return "", value, false
}

func frontmatterDescription(frontmatter string) string {
	var metadata struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.Description)
}

func isYAMLDocumentDelimiter(line string) bool {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	line = strings.TrimRight(line, " \t")
	return line == "---" || line == "..."
}

func linesAfter(lines []string, index int) string {
	if index >= len(lines) {
		return ""
	}
	return strings.Join(lines[index:], "")
}

func (p previewModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if msg.String() == "r" {
		p.rendered = !p.rendered
		p.refreshContent()
		p.viewport.GotoTop()
		m.modal = p
		return false, nil
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	m.modal = p
	return false, cmd
}
