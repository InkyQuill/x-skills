package tui

import (
	"strings"

	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
	"github.com/charmbracelet/lipgloss"
)

const inspectorKeyWidth = 12

type inspectorValue func(width int) string

type inspectorRow struct {
	Key    string
	Value  string
	Render inspectorValue
	Block  bool
}

type inspectorSection struct {
	Title string
	Rows  []inspectorRow
}

func textInspectorValue(value string) inspectorValue {
	return func(width int) string {
		return inspectorValueStyle.Render(tuiui.TruncateANSI(value, width))
	}
}

func blockInspectorValue(value string) inspectorValue {
	return func(width int) string {
		lines := wrapInspectorText(value, width)
		// Render lines one at a time: a single multi-line Render pads every
		// line to the longest line's width, leaving trailing spaces.
		for i, line := range lines {
			lines[i] = inspectorValueStyle.Render(line)
		}
		return strings.Join(lines, "\n")
	}
}

// renderInspectorDocument renders into content-sized dimensions; callers pass
// the usable inner width and height, excluding any surrounding panel chrome.
func renderInspectorDocument(title string, sections []inspectorSection, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := []string{inspectorTitleStyle.Render(title)}
	for _, section := range sections {
		if section.Title != "" {
			lines = append(lines, accentStyle.Render(section.Title))
		}
		for _, row := range section.Rows {
			lines = append(lines, renderInspectorRow(row, width)...)
		}
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = tuiui.TruncateANSI(line, width)
	}
	return strings.Join(lines, "\n")
}

func renderInspectorRow(row inspectorRow, width int) []string {
	if width <= 0 {
		return nil
	}

	if row.Block {
		return renderInspectorBlockRow(row, width)
	}

	keyWidth := inspectorKeyWidth
	if keyWidth > width {
		keyWidth = width
	}
	valueWidth := width - keyWidth

	render := row.Render
	if render == nil {
		render = textInspectorValue(row.Value)
	}
	renderedValue := render(valueWidth)
	valueLines := strings.Split(renderedValue, "\n")
	if len(valueLines) == 0 {
		valueLines = []string{""}
	}

	lines := make([]string, 0, len(valueLines))
	for i, valueLine := range valueLines {
		key := ""
		if i == 0 {
			key = row.Key
		}
		key = padInspectorKey(key, keyWidth)
		valueLine = tuiui.TruncateANSI(valueLine, valueWidth)
		lines = append(lines, tuiui.TruncateANSI(inspectorKeyStyle.Render(key)+valueLine, width))
	}
	return lines
}

func renderInspectorBlockRow(row inspectorRow, width int) []string {
	render := row.Render
	if render == nil {
		render = blockInspectorValue(row.Value)
	}
	lines := []string{tuiui.TruncateANSI(inspectorKeyStyle.Render(row.Key), width)}
	for _, valueLine := range strings.Split(render(width), "\n") {
		lines = append(lines, tuiui.TruncateANSI(valueLine, width))
	}
	return lines
}

func wrapInspectorText(value string, width int) []string {
	if width <= 0 {
		return nil
	}
	var wrapped []string
	for _, rawLine := range strings.Split(value, "\n") {
		rawLine = strings.TrimSpace(rawLine)
		if rawLine == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for lipgloss.Width(rawLine) > width {
			cut := inspectorWrapCut(rawLine, width)
			wrapped = append(wrapped, strings.TrimSpace(rawLine[:cut]))
			rawLine = strings.TrimSpace(rawLine[cut:])
		}
		wrapped = append(wrapped, rawLine)
	}
	if len(wrapped) == 0 {
		return []string{""}
	}
	return wrapped
}

func inspectorWrapCut(value string, width int) int {
	cut := 0
	lastSpace := -1
	currentWidth := 0
	for i, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width {
			if lastSpace > 0 {
				return lastSpace
			}
			if cut > 0 {
				return cut
			}
			return i + len(string(r))
		}
		currentWidth += runeWidth
		cut = i + len(string(r))
		if r == ' ' || r == '\t' {
			lastSpace = cut
		}
	}
	return len(value)
}

func padInspectorKey(key string, width int) string {
	key = tuiui.TruncateANSI(key, width)
	padding := width - lipgloss.Width(key)
	if padding <= 0 {
		return key
	}
	return key + strings.Repeat(" ", padding)
}
