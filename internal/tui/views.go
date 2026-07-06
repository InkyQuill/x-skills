package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
)

type ActiveGroup struct {
	ID          string
	Name        string
	Status      string
	Description string
	Locations   []string
	Members     []actions.ActiveSkill
	Reason      string
}

func groupActiveSkills(skills []actions.ActiveSkill) []ActiveGroup {
	groups := map[string]*ActiveGroup{}
	var order []string

	for _, skill := range skills {
		key := activeGroupKey(skill)
		group, ok := groups[key]
		if !ok {
			group = &ActiveGroup{
				ID:          "active:" + key,
				Name:        skill.Name,
				Status:      skill.Status,
				Description: skill.Description,
				Reason:      skill.Reason,
			}
			groups[key] = group
			order = append(order, key)
		}
		group.Members = append(group.Members, skill)
		group.Locations = appendUnique(group.Locations, skill.Root.Label)
		if group.Description == "" {
			group.Description = skill.Description
		}
		if group.Reason == "" {
			group.Reason = skill.Reason
		}
		group.Status = mergedStatus(group.Status, skill.Status)
	}

	result := make([]ActiveGroup, 0, len(order))
	for _, key := range order {
		sort.Strings(groups[key].Locations)
		result = append(result, *groups[key])
	}
	return result
}

func activeGroupKey(skill actions.ActiveSkill) string {
	if skill.Status == actions.StatusBroken {
		return "broken:" + skill.Path
	}

	target := skill.Path
	if resolved, err := filepath.EvalSymlinks(skill.Path); err == nil {
		target = resolved
	}
	if fp, err := fingerprint.Directory(target); err == nil {
		return "sha:" + fp
	}
	return "path:" + target
}

func mergedStatus(current, next string) string {
	if current == actions.StatusBroken || next == actions.StatusBroken {
		return actions.StatusBroken
	}
	if current == actions.StatusUnmanaged || next == actions.StatusUnmanaged {
		return actions.StatusUnmanaged
	}
	return actions.StatusManaged
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

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
		tabLabel(m.view == ViewActive, "a", "active"),
		tabLabel(m.view == ViewRepo, "r", "repo"),
		tabLabel(m.view == ViewDoctor, "d", "doctor"),
	}
	title := titleStyle.Render("x-skills") + "  " + strings.Join(tabs, " ")
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
		locations := chipStyle.Render(strings.Join(group.Locations, ", "))
		status := renderStatusChip(group.Status)
		text := fmt.Sprintf("%s %s %s %s  %s", prefix, group.Name, locations, status, activeDetail(group))
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}

func renderRepoRows(m Model, width int) []string {
	var rows []string
	for i, skill := range m.repo {
		id := repoID(skill.Name)
		prefix := rowPrefix(m, i, id)
		text := fmt.Sprintf("%s %s %s", prefix, skill.Name, mutedStyle.Render(skill.Description))
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}

func renderDoctorRows(m Model, width int) []string {
	var rows []string
	for i, issue := range m.issues {
		id := issueID(issue)
		prefix := rowPrefix(m, i, id)
		text := fmt.Sprintf("%s %s %s %s  %s", prefix, issue.Name, chipStyle.Render(issue.Location), dangerStyle.Render(issue.Kind), issue.Reason)
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}

func rowPrefix(m Model, index int, id string) string {
	cursor := " "
	if index == m.cursor {
		cursor = ">"
	}
	selected := " "
	if m.selected[id] {
		selected = "x"
	}
	return cursorStyle.Render(cursor) + selectedStyle.Render("["+selected+"]")
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
	lines = append(lines, mutedStyle.Render("space select  i install  m migrate  u unlink  f fix  q quit"))
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
}
