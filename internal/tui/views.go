package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/InkyQuill/x-skills/internal/actions"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 32
	}

	status := renderStatus(m, width)
	statusHeight := lipgloss.Height(status)
	bodyHeight := height - 1 - statusHeight
	if bodyHeight < 4 {
		bodyHeight = 4
	}

	base := strings.Join([]string{
		renderHeader(m, width),
		renderBody(m, width, bodyHeight),
		status,
	}, "\n")
	if m.modal != nil {
		return renderOverlay(base, m.modal.View(width, height, m), width, height)
	}
	return normalizeViewHeight(base, width, height)
}

func renderHeader(m Model, width int) string {
	tabs := []string{
		tabLabel(m.view == ViewActive, "A", "Active"),
		tabLabel(m.view == ViewRepo, "R", "Repo"),
		tabLabel(m.view == ViewDoctor, "D", "Doctor"),
	}
	title := titleStyle.Render(m.symbols.ProductMark+" x-skills") + "  " + strings.Join(tabs, " ")
	return truncate(title, width)
}

func renderBody(m Model, width, height int) string {
	if width < 100 {
		return renderListPanel(m, width, height)
	}
	inspectorWidth := 32
	listWidth := width - inspectorWidth - 3
	left := renderListPanel(m, listWidth, height)
	right := renderInspector(m, inspectorWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func renderListPanel(m Model, width, maxRows int) string {
	rowCount := maxRows - 3
	if rowCount < 1 {
		rowCount = 1
	}
	var rows []string
	title := "Active skills"
	switch m.view {
	case ViewActive:
		rows = renderActiveRows(m, width)
	case ViewRepo:
		title = "Repo skills"
		rows = renderRepoRows(m, width)
	case ViewDoctor:
		title = "Doctor issues"
		rows = renderDoctorRows(m, width)
	}
	if len(rows) == 0 {
		rows = []string{mutedStyle.Render("No items.")}
	}
	if len(rows) > rowCount {
		start := visibleStart(m.cursor, len(rows), rowCount)
		rows = rows[start : start+rowCount]
	}
	for len(rows) < rowCount {
		rows = append(rows, "")
	}
	return panelStyle.Width(width - 2).Render(title + "\n" + strings.Join(rows, "\n"))
}

func renderInspector(m Model, width, height int) string {
	contentHeight := height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}
	lines := []string{"Inspector", ""}
	switch m.view {
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor >= 0 && m.cursor < len(groups) {
			group := groups[m.cursor]
			lines = append(lines, "◇ "+group.Name, "aliases", "  "+strings.Join(group.Aliases, ", "), "repo", "  "+group.Status)
			if group.Status == actions.StatusBroken && group.Reason != "" {
				lines = append(lines, "reason", "  "+group.Reason)
			}
		}
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor >= 0 && m.cursor < len(skills) {
			skill := skills[m.cursor]
			lines = append(lines, "◇ "+skill.Name, "description", skill.Description, "usages", "  "+renderRootChips(m.symbols, m.repoUsage[skill.Name], lipgloss.NoColor{}))
		}
	case ViewDoctor:
		if m.cursor >= 0 && m.cursor < len(m.issues) {
			issue := m.issues[m.cursor]
			lines = append(lines, "◇ "+issue.Kind, "path", issue.Path, "reason", issue.Reason, "fix", issue.SafeFix)
		}
	}
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	return panelStyle.Width(width - 2).Render(strings.Join(lines, "\n"))
}

func visibleStart(cursor, count, maxRows int) int {
	if count <= maxRows || maxRows <= 0 {
		return 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= count {
		cursor = count - 1
	}
	start := cursor - maxRows + 1
	if start < 0 {
		return 0
	}
	if start+maxRows > count {
		return count - maxRows
	}
	return start
}

func renderActiveRows(m Model, width int) []string {
	var rows []string
	for i, group := range m.visibleActiveGroups() {
		prefix := rowPrefix(m, i, group.ID)
		status := renderStatusChip(m, group.Status)
		count := ""
		if len(group.Members) > 1 {
			count = " " + mutedStyle.Render(fmt.Sprintf("%s%d", m.symbols.CountPrefix, len(group.Members)))
		}
		rows = append(rows, selectableRow(
			[]rowSegment{
				{text: fmt.Sprintf("%s %s ", prefix, group.Name)},
				{render: func(background lipgloss.TerminalColor) string {
					return renderRootChips(m.symbols, group.Chips, background)
				}},
				{text: fmt.Sprintf(" %s%s  %s", status, count, activeDetail(group))},
			},
			i == m.cursor,
			m.selected[group.ID],
			width-6,
		))
	}
	return rows
}

func renderRepoRows(m Model, width int) []string {
	var rows []string
	for i, skill := range m.visibleRepoSkills() {
		id := repoID(skill.Name)
		prefix := rowPrefix(m, i, id)
		rows = append(rows, selectableRow(
			[]rowSegment{
				{text: fmt.Sprintf("%s %s ", prefix, skill.Name)},
				{render: func(background lipgloss.TerminalColor) string {
					return renderRootChips(m.symbols, m.repoUsage[skill.Name], background)
				}},
				{text: " " + mutedStyle.Render(skill.Description)},
			},
			i == m.cursor,
			m.selected[id],
			width-6,
		))
	}
	return rows
}

func renderDoctorRows(m Model, width int) []string {
	var rows []string
	for i, issue := range m.issues {
		prefix := cursorPrefix(m, i)
		rows = append(rows, selectableRow(
			[]rowSegment{
				{text: fmt.Sprintf("%s %s %s ", prefix, dangerStyle.Render(m.symbols.Broken), issue.Kind)},
				{render: func(background lipgloss.TerminalColor) string {
					return renderRootChip(m.symbols, issue.Location, background)
				}},
				{text: fmt.Sprintf("  %s %s", issue.Name, issue.Reason)},
			},
			i == m.cursor,
			false,
			width-6,
		))
	}
	return rows
}

func renderRootChips(symbols symbols, chips []string, background lipgloss.TerminalColor) string {
	rendered := make([]string, 0, len(chips))
	for _, chip := range chips {
		rendered = append(rendered, renderRootChip(symbols, chip, background))
	}
	return strings.Join(rendered, " ")
}

func renderRootChip(symbols symbols, chip string, background lipgloss.TerminalColor) string {
	if strings.HasPrefix(chip, "~") {
		return tuiui.Pill(symbols.BadgeLeft, symbols.BadgeRight, tuiui.PillProps{
			Color:      globalChip.GetBackground(),
			Background: background,
			Text:       chip,
			TextColor:  lipgloss.Color("230"),
		})
	}
	return tuiui.Pill(symbols.BadgeLeft, symbols.BadgeRight, tuiui.PillProps{
		Color:      projectChip.GetBackground(),
		Background: background,
		Text:       chip,
		TextColor:  lipgloss.Color("230"),
	})
}

type rowSegment struct {
	text   string
	render func(background lipgloss.TerminalColor) string
}

func selectableRow(segments []rowSegment, focused bool, selected bool, width int) string {
	if !focused && !selected {
		return truncate(joinRowSegments(segments, lipgloss.NoColor{}), width)
	}

	rowStyle := selectedBg
	if focused {
		rowStyle = cursorBg
	}
	background := rowStyle.GetBackground()

	var rendered strings.Builder
	remaining := width
	for _, segment := range segments {
		if remaining <= 0 {
			break
		}
		text := segment.text
		if segment.render != nil {
			text = segment.render(background)
		} else {
			text = ansi.Strip(text)
		}
		if lipgloss.Width(text) > remaining {
			text = truncate(text, remaining)
		}
		if segment.render != nil {
			rendered.WriteString(text)
		} else {
			rendered.WriteString(rowStyle.Render(text))
		}
		remaining -= lipgloss.Width(text)
	}
	if remaining > 0 {
		rendered.WriteString(rowStyle.Render(strings.Repeat(" ", remaining)))
	}
	return rendered.String()
}

func joinRowSegments(segments []rowSegment, background lipgloss.TerminalColor) string {
	var row strings.Builder
	for _, segment := range segments {
		if segment.render != nil {
			row.WriteString(segment.render(background))
			continue
		}
		row.WriteString(segment.text)
	}
	return row.String()
}

func rowPrefix(m Model, index int, id string) string {
	cursor := " "
	if index == m.cursor {
		cursor = m.symbols.Cursor
	}
	selected := m.symbols.Unchecked
	if m.selected[id] {
		selected = m.symbols.Checked
	}
	return cursorStyle.Render(cursor) + " " + selectedStyle.Render(selected)
}

func cursorPrefix(m Model, index int) string {
	cursor := " "
	if index == m.cursor {
		cursor = m.symbols.Cursor
	}
	return cursorStyle.Render(cursor)
}

func activeDetail(group ActiveGroup) string {
	if group.Status == actions.StatusBroken {
		return dangerStyle.Render(group.Reason)
	}
	return mutedStyle.Render(group.Description)
}

func renderStatus(m Model, width int) string {
	var lines []string
	switch {
	case m.err != nil:
		lines = append(lines, dangerStyle.Render(m.err.Error()))
	case m.status != "":
		lines = append(lines, okStyle.Render(m.status))
	}
	if m.filter.Active {
		lines = append(lines, accentStyle.Render("/ filter: "+m.filter.Query+"_"))
		lines = append(lines, mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{{ASCII: "enter", Unicode: "↵", Label: "accept"}, {ASCII: "esc", Unicode: "Esc", Label: "clear/exit"}})))
	}
	lines = append(lines, mutedStyle.Render(commandPalette(m)))
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
}

func commandPalette(m Model) string {
	switch m.view {
	case ViewRepo:
		return renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "details"},
			{ASCII: "/", Label: "filter"},
			{ASCII: "p", Label: "preview"},
			{ASCII: "l", Label: "link"},
			{ASCII: "u", Label: "unlink"},
			{ASCII: "d", Label: "delete"},
			{ASCII: "c", Label: "clear"},
			{ASCII: "^R", Label: "refresh"},
			{ASCII: "?", Label: "help"},
			{ASCII: "q", Label: "quit"},
		})
	case ViewDoctor:
		return renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "details"},
			{ASCII: "f", Label: "fix"},
			{ASCII: "^R", Label: "refresh"},
			{ASCII: "?", Label: "help"},
			{ASCII: "q", Label: "quit"},
		})
	default:
		return renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "details"},
			{ASCII: "/", Label: "filter"},
			{ASCII: "p", Label: "preview"},
			{ASCII: "m", Label: "migrate"},
			{ASCII: "u", Label: "unlink"},
			{ASCII: "c", Label: "clear"},
			{ASCII: "^R", Label: "refresh"},
			{ASCII: "?", Label: "help"},
			{ASCII: "q", Label: "quit"},
		})
	}
}

func renderCommandPalette(ascii bool, commands []tuiui.Shortcut) string {
	return tuiui.ToolHints(ascii, kbdStyle, commands)
}

func normalizeViewHeight(view string, width, height int) string {
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
}

func renderOverlay(base, layer string, width, height int) string {
	lines := strings.Split(normalizeViewHeight(base, width, height), "\n")
	layerLines := strings.Split(layer, "\n")
	layerHeight := len(layerLines)
	layerWidth := 0
	for _, line := range layerLines {
		if lineWidth := lipgloss.Width(line); lineWidth > layerWidth {
			layerWidth = lineWidth
		}
	}
	top := (height - layerHeight) / 2
	if top < 0 {
		top = 0
	}
	left := (width - layerWidth) / 2
	if left < 0 {
		left = 0
	}
	for i, line := range layerLines {
		row := top + i
		if row < 0 || row >= len(lines) {
			continue
		}
		right := width - left - lipgloss.Width(line)
		if right < 0 {
			right = 0
		}
		lines[row] = strings.Repeat(" ", left) + line + strings.Repeat(" ", right)
	}
	return strings.Join(lines, "\n")
}
