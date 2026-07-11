package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/remote"
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
		tabLabel(m.view == ViewInstall, "I", "Install"),
		tabLabel(false, "s", "Restore"),
		tabLabel(false, "S", "Sync"),
	}
	title := titleStyle.Render(m.pulseDiamond()+" x-skills") + "  " + strings.Join(tabs, " ")
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
	case ViewInstall:
		return renderInstallPanel(m, width, rowCount)
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
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}

	var sections []inspectorSection
	switch m.view {
	case ViewActive:
		sections = activeInspectorSections(m)
	case ViewRepo:
		sections = repoInspectorSections(m)
	case ViewDoctor:
		sections = doctorInspectorSections(m)
	case ViewInstall:
		sections = installInspectorSections(m)
	}
	content := renderInspectorDocument("Inspector", sections, contentWidth, contentHeight)
	return panelStyle.Width(width - 2).Render(content)
}

func activeInspectorSections(m Model) []inspectorSection {
	groups := m.visibleActiveGroups()
	if m.cursor < 0 || m.cursor >= len(groups) {
		return nil
	}

	group := groups[m.cursor]
	rows := []inspectorRow{
		{Key: "Aliases", Value: strings.Join(group.Aliases, ", ")},
		{
			Key: "Repo/Status",
			Render: func(width int) string {
				return truncate(renderStatusChip(m, group.Status), width)
			},
		},
		{Key: "Description", Value: group.Description, Block: true},
	}
	if len(group.Chips) > 0 {
		rows = append(rows, inspectorRow{
			Key: "Locations",
			Render: func(width int) string {
				return truncate(renderRootChips(m.symbols, group.Chips, lipgloss.NoColor{}), width)
			},
		})
	}
	if group.Status == actions.StatusBroken {
		rows = append(rows, inspectorRow{Key: "Reason", Value: group.Reason})
	}

	return []inspectorSection{{
		Title: group.Name,
		Rows:  rows,
	}}
}

func repoInspectorSections(m Model) []inspectorSection {
	skills := m.visibleRepoSkills()
	if m.cursor < 0 || m.cursor >= len(skills) {
		return nil
	}

	skill := skills[m.cursor]
	usages := m.repoUsage[skill.Name]
	rows := []inspectorRow{
		{Key: "Description", Value: skill.Description, Block: true},
		{
			Key: "Usages",
			Render: func(width int) string {
				return truncate(renderRootChips(m.symbols, usages, lipgloss.NoColor{}), width)
			},
		},
	}

	return []inspectorSection{{
		Title: skill.Name,
		Rows:  rows,
	}}
}

func doctorInspectorSections(m Model) []inspectorSection {
	if m.cursor < 0 || m.cursor >= len(m.issues) {
		return nil
	}

	issue := m.issues[m.cursor]
	rows := []inspectorRow{
		{Key: "Path", Value: issue.Path},
		{Key: "Reason", Value: issue.Reason},
		{Key: "Fix", Value: issue.SafeFix},
	}

	return []inspectorSection{{
		Title: string(issue.Kind),
		Rows:  rows,
	}}
}

func installInspectorSections(m Model) []inspectorSection {
	if m.cursor < 0 || m.cursor >= len(m.install.Results) {
		return nil
	}

	row := m.install.Results[m.cursor]
	result := row.Result
	overviewRows := []inspectorRow{}
	overviewRows = appendTextInspectorRow(overviewRows, "Description", result.Description)
	overviewRows = appendRichTextInspectorRow(overviewRows, "Source", result.Source(), installSourceStyle)
	if result.Installs > 0 {
		overviewRows = appendRichTextInspectorRow(overviewRows, "Installs", strconv.Itoa(result.Installs), installCountStyle)
	}

	stateRows := []inspectorRow{}
	stateRows = appendInstallArchiveInspectorRow(stateRows, row.ArchiveState)
	stateRows = appendTextInspectorRow(stateRows, "Check error", row.ArchiveCheckError)
	stateRows = appendInstallAuditInspectorRow(stateRows, row.AuditPill)

	repoRows := []inspectorRow{}
	repoRows = appendTextInspectorRow(repoRows, "Owner", result.Owner)
	repoRows = appendTextInspectorRow(repoRows, "Repo", result.Repo)
	repoRows = appendTextInspectorRow(repoRows, "Path", result.Path)

	sections := []inspectorSection{{
		Title: result.Name,
		Rows:  overviewRows,
	}}
	sections = appendInspectorSection(sections, "State", stateRows)
	sections = appendInspectorSection(sections, "Repository", repoRows)
	sections = appendInspectorSection(sections, "Actions", []inspectorRow{
		{Key: "Preview", Value: "enter preview"},
		{Key: "Install", Value: "i install & use"},
		{Key: "Archive", Value: "a archive only"},
	})
	return sections
}

func appendTextInspectorRow(rows []inspectorRow, key, value string) []inspectorRow {
	if value == "" {
		return rows
	}
	return append(rows, inspectorRow{Key: key, Value: value, Block: key == "Description"})
}

func appendRichTextInspectorRow(rows []inspectorRow, key, value string, style lipgloss.Style) []inspectorRow {
	if value == "" {
		return rows
	}
	return append(rows, inspectorRow{
		Key: key,
		Render: func(width int) string {
			return style.Render(truncate(value, width))
		},
	})
}

func appendInspectorSection(sections []inspectorSection, title string, rows []inspectorRow) []inspectorSection {
	if len(rows) == 0 {
		return sections
	}
	return append(sections, inspectorSection{Title: title, Rows: rows})
}

func appendInstallArchiveInspectorRow(rows []inspectorRow, state string) []inspectorRow {
	if state == "" {
		return rows
	}
	return append(rows, inspectorRow{
		Key: "Archive",
		Render: func(width int) string {
			return truncate(renderInstallArchiveState(state), width)
		},
	})
}

func appendInstallAuditInspectorRow(rows []inspectorRow, audit string) []inspectorRow {
	if audit == "" {
		return rows
	}
	return append(rows, inspectorRow{
		Key: "Audit",
		Render: func(width int) string {
			return truncate(renderInstallAuditState(audit), width)
		},
	})
}

func renderInstallArchiveState(state string) string {
	return renderInstallArchiveStateWithBackground(state, lipgloss.NoColor{})
}

func renderInstallArchiveStateWithBackground(state string, background lipgloss.TerminalColor) string {
	switch state {
	case remote.ArchiveStateArchived:
		return renderWithOptionalBackground(okStyle, state, background)
	case remote.ArchiveStateUpdateAvailable:
		return renderWithOptionalBackground(incomingStyle, state, background)
	case remote.ArchiveStateNameConflict:
		return renderWithOptionalBackground(dangerStyle, state, background)
	default:
		return renderWithOptionalBackground(mutedStyle, state, background)
	}
}

func renderInstallAuditState(audit string) string {
	return renderInstallAuditStateWithBackground(audit, lipgloss.NoColor{})
}

func renderInstallAuditStateWithBackground(audit string, background lipgloss.TerminalColor) string {
	switch {
	case strings.Contains(audit, "risky"):
		return renderWithOptionalBackground(dangerStyle, audit, background)
	case strings.Contains(audit, "warn"):
		return renderWithOptionalBackground(archiveStyle, audit, background)
	case strings.Contains(audit, "safe"):
		return renderWithOptionalBackground(okStyle, audit, background)
	default:
		return renderWithOptionalBackground(inspectorValueStyle, audit, background)
	}
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
		rows = append(rows, selectableRow(
			[]rowSegment{
				{text: prefix + " "},
				{render: func(background lipgloss.TerminalColor) string {
					return renderStatusDotWithBackground(m, group.Status, background)
				}},
				{text: " " + group.Name + " "},
				{render: func(background lipgloss.TerminalColor) string {
					return renderRootChips(m.symbols, group.Chips, background)
				}},
				{text: " " + activeDetail(group)},
			},
			i == m.cursor,
			m.selected[ViewActive][group.ID],
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
			m.selected[ViewRepo][id],
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
				{text: prefix + " "},
				{render: func(background lipgloss.TerminalColor) string {
					return renderStatusDotWithBackground(m, actions.StatusBroken, background)
				}},
				{text: fmt.Sprintf(" %s ", issue.Kind)},
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

func installPanelTitle(m Model) string {
	query := m.install.Query
	if query == "" {
		return "Install: search"
	}
	if m.install.Owner != "" {
		return fmt.Sprintf("Install: search %q  owner: %s", query, m.install.Owner)
	}
	return fmt.Sprintf("Install: search %q", query)
}

func renderInstallPanel(m Model, width, rowCount int) string {
	rows := []string{accentStyle.Render("/ search: " + m.install.Query + "_")}
	resultRowCount := rowCount - len(rows)
	if resultRowCount > 0 {
		resultRows := renderInstallRows(m, width)
		if len(resultRows) == 0 {
			resultRows = []string{mutedStyle.Render(m.install.Message)}
		}
		if len(resultRows) > resultRowCount {
			start := visibleStart(m.cursor, len(resultRows), resultRowCount)
			resultRows = resultRows[start : start+resultRowCount]
		}
		rows = append(rows, resultRows...)
	}
	for len(rows) < rowCount {
		rows = append(rows, "")
	}
	return panelStyle.Width(width - 2).Render(installPanelTitle(m) + "\n" + strings.Join(rows, "\n"))
}

func renderInstallRows(m Model, width int) []string {
	if len(m.install.Results) == 0 {
		return nil
	}
	var rows []string
	for i, row := range m.install.Results {
		result := row.Result
		id := installID(result)
		segments := []rowSegment{
			{text: fmt.Sprintf("%s %s", rowPrefix(m, i, id), result.Name)},
		}
		if source := result.Source(); source != "" {
			segments = append(segments, rowSegment{
				text: "  ",
			}, rowSegment{
				render: func(background lipgloss.TerminalColor) string {
					return renderWithOptionalBackground(installSourceStyle, source, background)
				},
			})
		}
		if count := renderInstallCount(result.Installs); count != "" {
			segments = append(segments, rowSegment{
				text: "  ",
			}, rowSegment{
				render: func(background lipgloss.TerminalColor) string {
					return renderInstallCountWithBackground(result.Installs, background)
				},
			})
		}
		if state := renderInstallStatePill(row.ArchiveState); state != "" {
			segments = append(segments, rowSegment{
				text: "  ",
			}, rowSegment{
				render: func(background lipgloss.TerminalColor) string {
					return renderInstallStatePillWithBackground(row.ArchiveState, background)
				},
			})
		}
		if row.ArchiveCheckError != "" {
			segments = append(segments, rowSegment{
				text: "  ",
			}, rowSegment{
				render: func(background lipgloss.TerminalColor) string {
					return renderWithOptionalBackground(mutedStyle, "check failed", background)
				},
			})
		}
		if row.AuditPill != "" {
			audit := row.AuditPill
			segments = append(segments, rowSegment{
				text: "  ",
			}, rowSegment{
				render: func(background lipgloss.TerminalColor) string {
					return renderInstallAuditStateWithBackground(audit, background)
				},
			})
		}
		if result.Description != "" {
			description := result.Description
			segments = append(segments, rowSegment{
				text: "  ",
			}, rowSegment{
				render: func(background lipgloss.TerminalColor) string {
					return renderWithOptionalBackground(mutedStyle, description, background)
				},
			})
		}
		rows = append(rows, selectableRow(segments, i == m.cursor, m.selected[ViewInstall][id], width-6))
	}
	return rows
}

func installID(result remote.SearchResult) string {
	if result.ID != "" {
		return "install:" + result.ID
	}
	if result.Owner != "" || result.Repo != "" || result.Path != "" {
		return "install:" + installAuditKey(result)
	}
	return "install:" + result.Name
}

func renderInstallStatePill(state string) string {
	return renderInstallStatePillWithBackground(state, lipgloss.NoColor{})
}

func renderInstallStatePillWithBackground(state string, background lipgloss.TerminalColor) string {
	if state == "" {
		return ""
	}
	return renderInstallArchiveStateWithBackground(state, background)
}

func renderInstallCount(count int) string {
	return renderInstallCountWithBackground(count, lipgloss.NoColor{})
}

func renderInstallCountWithBackground(count int, background lipgloss.TerminalColor) string {
	if count <= 0 {
		return ""
	}
	return renderWithOptionalBackground(installCountStyle, fmt.Sprintf("%d installs", count), background)
}

func renderWithOptionalBackground(
	style lipgloss.Style,
	text string,
	background lipgloss.TerminalColor,
) string {
	if _, noColor := background.(lipgloss.NoColor); noColor {
		return style.Render(text)
	}
	return style.Background(background).Render(text)
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
			TextColor:  chipText.GetForeground(),
		})
	}
	return tuiui.Pill(symbols.BadgeLeft, symbols.BadgeRight, tuiui.PillProps{
		Color:      projectChip.GetBackground(),
		Background: background,
		Text:       chip,
		TextColor:  chipText.GetForeground(),
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
	if m.selected[m.view][id] {
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
			{ASCII: "r", Label: "recommend"},
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
	case ViewInstall:
		return renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "preview"},
			{ASCII: "space", Label: "select"},
			{ASCII: "/", Label: "search"},
			{ASCII: "o", Label: "owner"},
			{ASCII: "i", Label: "install & use"},
			{ASCII: "a", Label: "archive only"},
			{ASCII: "c", Label: "clear"},
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
		lines[row] = truncate(lines[row], width)
	}
	return strings.Join(lines, "\n")
}
