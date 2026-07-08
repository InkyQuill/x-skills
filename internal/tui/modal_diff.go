package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type conflictDiffModal struct {
	name     string
	diff     directoryDiff
	selected int
	scroll   int
	apply    func(string)
}

func newConflictDiffModal(name string, diff directoryDiff, apply func(string)) modal {
	return conflictDiffModal{name: name, diff: diff, apply: apply}
}

func (c conflictDiffModal) Title() string {
	return "Archive conflict: " + c.name
}

func (c conflictDiffModal) View(width, height int, m Model) string {
	const (
		horizontalMargin   = 12
		narrowMargin       = 4
		maxInnerWidth      = 84
		minInnerWidth      = 36
		fileColumnWidth    = 24
		columnDividerWidth = 3
		minDiffWidth       = 20
		verticalChrome     = 12
		minBodyHeight      = 4
	)
	innerWidth := width - horizontalMargin
	if innerWidth > maxInnerWidth {
		innerWidth = maxInnerWidth
	}
	if innerWidth < minInnerWidth {
		innerWidth = width - narrowMargin
	}
	fileWidth := fileColumnWidth
	diffWidth := innerWidth - fileWidth - columnDividerWidth
	if diffWidth < minDiffWidth {
		diffWidth = minDiffWidth
	}
	bodyHeight := height - verticalChrome
	if bodyHeight < minBodyHeight {
		bodyHeight = minBodyHeight
	}
	footer := mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
		{ASCII: "left/right", Unicode: "←/→", Label: "file"},
		{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
		{ASCII: "k", Label: "keep archive"},
		{ASCII: "l", Label: "save active"},
		{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
	}))
	lines := []string{
		accentStyle.Render("Archive conflict: " + c.name),
		"Decision applies to the whole skill directory.",
		diffLegend(),
		"",
		fmt.Sprintf("%-*s │ %s", fileWidth, "Files", "Diff review"),
		fmt.Sprintf("%s─┼─%s", strings.Repeat("─", fileWidth), strings.Repeat("─", diffWidth)),
	}
	diffLines := []string{mutedStyle.Render("No differences.")}
	if len(c.diff.Files) > 0 {
		diffLines = coloredDiffLines(c.diff.Files[c.selected].Text)
	}
	c.scroll = clampScroll(c.scroll, len(diffLines), bodyHeight)
	for row := 0; row < bodyHeight; row++ {
		fileCell := ""
		if row < len(c.diff.Files) {
			file := c.diff.Files[row]
			fileCell = fmt.Sprintf("%s %s %s", fileCursor(m, row, c.selected), diffMarker(file.Kind), file.Path)
		}
		diffIndex := c.scroll + row
		diffCell := ""
		if diffIndex < len(diffLines) {
			diffCell = diffLines[diffIndex]
		}
		lines = append(lines, fmt.Sprintf("%-*s │ %s", fileWidth, truncate(fileCell, fileWidth), truncate(diffCell, diffWidth)))
	}
	if len(diffLines) > 0 {
		end := c.scroll + bodyHeight
		if end > len(diffLines) {
			end = len(diffLines)
		}
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%-*s │ lines %d-%d/%d", fileWidth, "file: left/right", c.scroll+1, end, len(diffLines))))
	}
	lines = append(lines, footer)
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func fileCursor(m Model, index, selected int) string {
	if index == selected {
		return m.symbols.Cursor
	}
	return " "
}

func (c conflictDiffModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	switch msg.String() {
	case "left":
		if c.selected > 0 {
			c.selected--
			c.scroll = 0
		}
		m.modal = c
	case "right":
		if c.selected+1 < len(c.diff.Files) {
			c.selected++
			c.scroll = 0
		}
		m.modal = c
	case "down":
		c.scroll++
		m.modal = c
	case "up":
		if c.scroll > 0 {
			c.scroll--
		}
		m.modal = c
	case "pgdown":
		c.scroll += 10
		m.modal = c
	case "pgup":
		c.scroll -= 10
		if c.scroll < 0 {
			c.scroll = 0
		}
		m.modal = c
	case "k":
		c.apply(actions.ConflictResolutionKeepArchive)
		return true, nil
	case "l":
		c.apply(actions.ConflictResolutionUseActive)
		return true, nil
	}
	return false, nil
}

func diffLegend() string {
	return "Legend: " + archiveStyle.Render("Archive") + "  " + incomingStyle.Render("Incoming active")
}

func diffMarker(kind string) string {
	switch kind {
	case "added":
		return "+"
	case "removed":
		return "-"
	case "binary":
		return "!"
	default:
		return "±"
	}
}

func coloredDiffLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, colorDiffLine(line))
	}
	return lines
}

func colorDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "--- archive"):
		return diffMetaStyle.Render("Archive block")
	case strings.HasPrefix(line, "+++ active"):
		return diffMetaStyle.Render("Incoming active block")
	case strings.HasPrefix(line, "-"):
		return archiveStyle.Render("Archive   " + strings.TrimPrefix(line, "-"))
	case strings.HasPrefix(line, "+"):
		return incomingStyle.Render("Incoming  " + strings.TrimPrefix(line, "+"))
	default:
		return mutedStyle.Render("          " + strings.TrimPrefix(line, " "))
	}
}

func clampScroll(scroll, count, height int) int {
	if scroll < 0 {
		return 0
	}
	maxScroll := count - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}
