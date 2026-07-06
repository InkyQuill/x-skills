package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/InkyQuill/x-skills/internal/actions"
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

func renderRows(m Model, width, maxRows int) string {
	return renderListPanel(m, width, maxRows)
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
		}
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor >= 0 && m.cursor < len(skills) {
			skill := skills[m.cursor]
			lines = append(lines, "◇ "+skill.Name, "description", skill.Description, "usages", "  "+strings.Join(m.repoUsage[skill.Name], " "))
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
		chips := chipStyle.Render(strings.Join(group.Chips, " "))
		status := renderStatusChip(m, group.Status)
		count := ""
		if len(group.Members) > 1 {
			count = " " + mutedStyle.Render(fmt.Sprintf("%s%d", m.symbols.CountPrefix, len(group.Members)))
		}
		text := fmt.Sprintf("%s %s %s %s%s  %s", prefix, group.Name, chips, status, count, activeDetail(group))
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}

func renderRepoRows(m Model, width int) []string {
	var rows []string
	for i, skill := range m.visibleRepoSkills() {
		id := repoID(skill.Name)
		prefix := rowPrefix(m, i, id)
		usages := strings.Join(m.repoUsage[skill.Name], " ")
		text := fmt.Sprintf("%s %s %s %s", prefix, skill.Name, mutedStyle.Render(skill.Description), chipStyle.Render(usages))
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}

func renderDoctorRows(m Model, width int) []string {
	var rows []string
	for i, issue := range m.issues {
		id := issueID(issue)
		prefix := rowPrefix(m, i, id)
		text := fmt.Sprintf("%s %s %s %s  %s %s", prefix, dangerStyle.Render(m.symbols.Broken), issue.Kind, chipStyle.Render(issue.Location), issue.Name, issue.Reason)
		rows = append(rows, truncate(text, width-6))
	}
	return rows
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

func activeDetail(group ActiveGroup) string {
	if group.Status == actions.StatusBroken {
		return dangerStyle.Render(group.Reason)
	}
	if len(group.Members) > 1 {
		return mutedStyle.Render(fmt.Sprintf("%d linked locations", len(group.Members)))
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
		lines = append(lines, mutedStyle.Render("enter accept   esc clear/exit"))
	}
	lines = append(lines, mutedStyle.Render("enter details  / filter  p preview  m migrate  u unlink  c clear  ^R refresh  ? help  q quit"))
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
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
