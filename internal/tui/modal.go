package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type modal interface {
	Title() string
	View(width, height int, m Model) string
	Update(msg tea.KeyMsg, m *Model) (shouldClose bool, cmd tea.Cmd)
}

func closeOnEscapeOrQuit(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "q":
		return true
	default:
		return false
	}
}

func modalMoveDelta(msg tea.KeyMsg) int {
	switch msg.String() {
	case "up", "k":
		return -1
	case "down", "j":
		return 1
	case "pgup", "ctrl+u":
		return -5
	case "pgdown", "ctrl+d":
		return 5
	default:
		return 0
	}
}

type scrollState int

func (s *scrollState) Handle(msg tea.KeyMsg, bodyHeight, viewportHeight int) bool {
	delta := modalMoveDelta(msg)
	if delta == 0 {
		return false
	}
	*s = scrollState(tuiui.ClampScroll(int(*s)+delta, bodyHeight, viewportHeight))
	return true
}

type constrainedModalOptions struct {
	Title     string
	Body      []string
	Footer    []string
	Focus     int
	Scroll    int
	UseScroll bool
}

func renderConstrainedModal(width, height int, opts constrainedModalOptions) string {
	contentWidth := modalContentWidth(width)
	maxContentHeight := modalContentHeight(height)

	header := []string{accentStyle.Render(tuiui.TruncateANSI(ansi.Strip(opts.Title), contentWidth))}
	footer := truncateModalLines(opts.Footer, contentWidth)
	footerHeight := len(footer)
	if footerHeight > 0 {
		footerHeight++
	}
	bodyHeight := maxContentHeight - len(header) - footerHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	safeBody := make([]string, len(opts.Body))
	for i, line := range opts.Body {
		safeBody[i] = ansi.Strip(line)
	}
	body := presentModalBody(truncateModalLines(safeBody, contentWidth), bodyHeight, opts.Focus, opts.Scroll, opts.UseScroll)
	lines := make([]string, 0, len(header)+len(body)+footerHeight)
	lines = append(lines, header...)
	lines = append(lines, body...)
	if len(footer) > 0 {
		lines = append(lines, "")
		lines = append(lines, footer...)
	}

	for len(lines) > maxContentHeight {
		lines = lines[:maxContentHeight]
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func modalContentWidth(width int) int {
	modalWidth := width - 8
	if modalWidth > 88 {
		modalWidth = 88
	}
	if modalWidth < 40 {
		modalWidth = min(40, width-2)
	}
	contentWidth := modalWidth - 2
	if contentWidth < 1 {
		return 1
	}
	return contentWidth
}

func modalContentHeight(height int) int {
	maxHeight := height - 4
	if maxHeight < 4 {
		maxHeight = min(4, height-2)
	}
	if maxHeight < 1 {
		return 1
	}
	return maxHeight
}

func constrainedModalBodyHeight(height, footerLines int) int {
	footerHeight := footerLines
	if footerHeight > 0 {
		footerHeight++
	}
	bodyHeight := modalContentHeight(height) - 1 - footerHeight
	if bodyHeight < 1 {
		return 1
	}
	return bodyHeight
}

func truncateModalLines(lines []string, width int) []string {
	truncated := make([]string, 0, len(lines))
	for _, line := range lines {
		truncated = append(truncated, tuiui.TruncateANSI(line, width))
	}
	return truncated
}

func presentModalBody(lines []string, height int, focus int, scroll int, useScroll bool) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	start := scroll
	if !useScroll {
		focus = tuiui.ClampIndex(focus, len(lines))
		start = focus - height/2
	}
	start = tuiui.ClampScroll(start, len(lines), height)
	body := tuiui.VisibleLines(lines, start, height)
	if start > 0 {
		body[0] = mutedStyle.Render("↑ more")
	}
	if start+height < len(lines) {
		body[len(body)-1] = mutedStyle.Render("↓ more")
	}
	return body
}
