package tui

import (
	"fmt"
	"strings"

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

	footerHeight := 2
	bodyHeight := height - 4 - footerHeight
	if m.wizard.Open {
		bodyHeight -= 5
	}
	if bodyHeight < 4 {
		bodyHeight = 4
	}

	parts := []string{
		renderHeader(m, width),
		renderRows(m, width, bodyHeight),
	}
	if m.wizard.Open {
		parts = append(parts, renderWizard(m, width))
	}
	parts = append(parts, renderStatus(m, width))
	return strings.Join(parts, "\n")
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
	var rows []string
	switch m.view {
	case ViewActive:
		rows = renderActiveRows(m, width)
	case ViewRepo:
		rows = renderRepoRows(m, width)
	case ViewDoctor:
		rows = renderDoctorRows(m, width)
	}
	if len(rows) == 0 {
		rows = []string{mutedStyle.Render("No items.")}
	}
	if len(rows) > maxRows {
		start := visibleStart(m.cursor, len(rows), maxRows)
		rows = rows[start : start+maxRows]
	}
	for len(rows) < maxRows {
		rows = append(rows, "")
	}
	return panelStyle.Width(width - 2).Render(strings.Join(rows, "\n"))
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
	for i, group := range m.active {
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
	for i, skill := range m.repo {
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

func renderWizard(m Model, width int) string {
	title := fmt.Sprintf("%s wizard", m.wizard.Action)
	lines := []string{
		accentStyle.Render(title),
		truncate(m.wizard.Preview, width-6),
		mutedStyle.Render("enter apply  esc cancel"),
	}
	return wizardStyle.Width(width - 2).Render(strings.Join(lines, "\n"))
}

func renderStatus(m Model, width int) string {
	var lines []string
	switch {
	case m.err != nil:
		lines = append(lines, dangerStyle.Render(m.err.Error()))
	case m.status != "":
		lines = append(lines, okStyle.Render(m.status))
	}
	lines = append(lines, mutedStyle.Render("enter details  / filter  p preview  m migrate  u unlink  ^R refresh  ? help  q quit"))
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
}
