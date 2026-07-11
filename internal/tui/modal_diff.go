package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

const (
	minConflictDiffWidth  = 72
	minConflictDiffHeight = 18
)

type conflictDiffModal struct {
	name          string
	diff          directoryDiff
	selected      int
	scroll        int
	incomingLabel string
	apply         func(*Model, string) tea.Cmd
}

func newConflictDiffModal(name string, diff directoryDiff, apply func(string)) modal {
	return newConflictDiffModalWithIncomingLabel(name, diff, "Incoming active", apply)
}

func newConflictDiffModalWithIncomingLabel(name string, diff directoryDiff, incomingLabel string, apply func(string)) modal {
	return newConflictDiffModalWithModelApply(name, diff, incomingLabel, func(_ *Model, resolution string) {
		apply(resolution)
	})
}

func newConflictDiffModalWithModelApply(name string, diff directoryDiff, incomingLabel string, apply func(*Model, string)) modal {
	return newConflictDiffModalWithModelCommandApply(name, diff, incomingLabel, func(m *Model, resolution string) tea.Cmd {
		apply(m, resolution)
		return nil
	})
}

func newConflictDiffModalWithModelCommandApply(name string, diff directoryDiff, incomingLabel string, apply func(*Model, string) tea.Cmd) modal {
	return conflictDiffModal{name: name, diff: diff, incomingLabel: incomingLabel, apply: apply}
}

func conflictDiffTooSmall(width, height int) bool {
	return width < minConflictDiffWidth || height < minConflictDiffHeight
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
	)
	if conflictDiffTooSmall(width, height) {
		lines := []string{
			accentStyle.Render("Archive conflict: " + c.name),
			"",
			"Terminal too small to review this diff.",
			fmt.Sprintf("Please resize to at least %dx%d.", minConflictDiffWidth, minConflictDiffHeight),
			"",
			tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
				{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
				{ASCII: "q", Label: "cancel"},
			}),
		}
		return modalStyle(width, height).Render(strings.Join(lines, "\n"))
	}

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
	bodyHeight := conflictDiffBodyHeight(height)
	acceptLabel := "save active"
	if c.incomingLabel == "Incoming remote" {
		acceptLabel = "use incoming"
	}
	footer := tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
		{ASCII: "left/right", Unicode: "←/→", Label: "file"},
		{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
		{ASCII: "k", Label: "keep archive"},
		{ASCII: "l", Label: acceptLabel},
		{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
	})
	lines := []string{
		accentStyle.Render("Archive conflict: " + c.name),
		"Decision applies to the whole skill directory.",
		diffLegend(c.incomingLabel),
		"",
		fmt.Sprintf("%-*s │ %s", fileWidth, "Files", "Diff review"),
		fmt.Sprintf("%s─┼─%s", strings.Repeat("─", fileWidth), strings.Repeat("─", diffWidth)),
	}
	diffLines := []string{mutedStyle.Render("No differences.")}
	if len(c.diff.Files) > 0 {
		diffLines = coloredDiffLines(c.diff.Files[c.selected].Text, c.incomingLabel)
	}
	c.scroll = tuiui.ClampScroll(c.scroll, len(diffLines), bodyHeight)
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
	if conflictDiffTooSmall(m.width, m.height) {
		return false, nil
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
		m.modal = nil
		return false, c.apply(m, actions.ConflictResolutionKeepArchive)
	case "l":
		m.modal = nil
		return false, c.apply(m, actions.ConflictResolutionUseActive)
	}
	c.scroll = tuiui.ClampScroll(c.scroll, c.diffLineCount(), conflictDiffBodyHeight(m.height))
	if m.modal != nil {
		m.modal = c
	}
	return false, nil
}

func conflictDiffBodyHeight(height int) int {
	const (
		verticalChrome = 12
		minimum        = 4
	)
	bodyHeight := height - verticalChrome
	if bodyHeight < minimum {
		return minimum
	}
	return bodyHeight
}

func (c conflictDiffModal) diffLineCount() int {
	if len(c.diff.Files) == 0 {
		return 1
	}
	return len(coloredDiffLines(c.diff.Files[c.selected].Text, c.incomingLabel))
}

func diffLegend(incomingLabel string) string {
	return "Legend: " + archiveStyle.Render("Archive") + "  " + incomingStyle.Render(incomingLabel)
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

func coloredDiffLines(text string, incomingLabel string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, colorDiffLine(line, incomingLabel))
	}
	return lines
}

func colorDiffLine(line string, incomingLabel string) string {
	switch {
	case strings.HasPrefix(line, "--- archive"):
		return diffMetaStyle.Render("Archive block")
	case strings.HasPrefix(line, "+++ active"):
		return diffMetaStyle.Render(incomingLabel + " block")
	case strings.HasPrefix(line, "-"):
		return archiveStyle.Render("Archive   " + strings.TrimPrefix(line, "-"))
	case strings.HasPrefix(line, "+"):
		return incomingStyle.Render("Incoming  " + strings.TrimPrefix(line, "+"))
	default:
		return mutedStyle.Render("          " + strings.TrimPrefix(line, " "))
	}
}
