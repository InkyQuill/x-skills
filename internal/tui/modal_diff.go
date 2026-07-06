package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
)

type conflictDiffModal struct {
	name     string
	diff     directoryDiff
	selected int
	apply    func(string)
}

func newConflictDiffModal(name string, diff directoryDiff, apply func(string)) modal {
	return conflictDiffModal{name: name, diff: diff, apply: apply}
}

func (c conflictDiffModal) Title() string {
	return "Archive conflict: " + c.name
}

func (c conflictDiffModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render("Archive conflict: " + c.name),
		"Decision applies to the whole skill directory.",
		"",
		"Files                         | full file diff",
		strings.Repeat("-", 72),
	}
	for i, file := range c.diff.Files {
		cursor := " "
		if i == c.selected {
			cursor = m.symbols.Cursor
		}
		lines = append(lines, fmt.Sprintf("%s %s %-26s | %s", cursor, diffMarker(file.Kind), file.Path, firstDiffLine(file.Text)))
	}
	if len(c.diff.Files) > 0 {
		lines = append(lines, "", c.diff.Files[c.selected].Text)
	}
	lines = append(lines, "", mutedStyle.Render("up/down scroll   tab focus   k keep archive   l save active   esc cancel   q close"))
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (c conflictDiffModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up":
		if c.selected > 0 {
			c.selected--
		}
		m.modal = c
	case "down":
		if c.selected+1 < len(c.diff.Files) {
			c.selected++
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

func firstDiffLine(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}
