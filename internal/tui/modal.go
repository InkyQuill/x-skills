package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type modal interface {
	Title() string
	View(width, height int, m Model) string
	Update(msg tea.KeyMsg, m *Model) (close bool, cmd tea.Cmd)
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

func clampModalIndex(index int, count int) int {
	if count <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
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

	header := []string{accentStyle.Render(truncate(opts.Title, contentWidth))}
	footer := truncateModalLines(opts.Footer, contentWidth)
	footerHeight := len(footer)
	if footerHeight > 0 {
		footerHeight++
	}
	bodyHeight := maxContentHeight - len(header) - footerHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	body := visibleModalBody(truncateModalLines(opts.Body, contentWidth), bodyHeight, opts.Focus, opts.Scroll, opts.UseScroll)
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
		modalWidth = width - 2
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
		maxHeight = height - 2
	}
	if maxHeight < 1 {
		return 1
	}
	return maxHeight
}

func truncateModalLines(lines []string, width int) []string {
	truncated := make([]string, 0, len(lines))
	for _, line := range lines {
		truncated = append(truncated, truncate(line, width))
	}
	return truncated
}

func visibleModalBody(lines []string, height int, focus int, scroll int, useScroll bool) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	start := scroll
	if !useScroll {
		if focus < 0 {
			focus = 0
		}
		if focus >= len(lines) {
			focus = len(lines) - 1
		}
		start = focus - height/2
		if start < 0 {
			start = 0
		}
		if start+height > len(lines) {
			start = len(lines) - height
		}
	} else {
		maxStart := len(lines) - height
		if start > maxStart {
			start = maxStart
		}
	}
	body := append([]string(nil), lines[start:start+height]...)
	if start > 0 {
		body[0] = mutedStyle.Render("↑ more")
	}
	if start+height < len(lines) {
		body[len(body)-1] = mutedStyle.Render("↓ more")
	}
	return body
}
