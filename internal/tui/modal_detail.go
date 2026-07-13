package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type detailModal struct {
	title  string
	lines  []string
	scroll scrollState
}

func newDetailModal(title string, lines []string) modal {
	return detailModal{title: title, lines: lines}
}

func activeDetailModal(group ActiveGroup, symbols symbols) modal {
	lines := []string{
		"Canonical name: " + group.Identity,
		"Status: " + group.Status,
		"Aliases: " + strings.Join(group.Aliases, ", "),
		"",
		"Active members",
	}
	for _, member := range group.Members {
		lines = append(lines, fmt.Sprintf("  %s  %s", renderRootChip(symbols, rootLabel(member.Root), lipgloss.NoColor{}), member.Path))
	}
	lines = append(lines, "", "Debug", "  fingerprint: "+group.Fingerprint)
	if group.DeclaredName != "" && group.DeclaredName != group.Identity {
		lines = append(lines, "Declared name: "+group.DeclaredName)
	}
	return newDetailModal("Detail: "+group.Identity+" (Active)", lines)
}

func repoDetailModal(skill repo.Skill, usages []string, symbols symbols) modal {
	usageText := "none"
	if len(usages) > 0 {
		usageText = renderRootChips(symbols, usages, lipgloss.NoColor{})
	}
	lines := []string{
		"Archive path",
		"  " + skill.Path,
		"Description",
		"  " + skill.Description,
		"Usages",
		"  " + usageText,
	}
	if skill.DeclaredName != "" && skill.DeclaredName != skill.Identity {
		lines = append(lines, "Declared name: "+skill.DeclaredName)
	}
	return newDetailModal("Detail: "+skill.Identity+" (Repo)", lines)
}

func doctorDetailModal(issue doctor.Issue) modal {
	lines := []string{
		"Issue kind",
		"  " + issue.Kind,
		"Affected path",
		"  " + issue.Path,
		"Reason",
		"  " + issue.Reason,
		"Safe fix",
		"  " + issue.SafeFix,
	}
	return newDetailModal("Detail: "+issue.Name+" (Doctor)", lines)
}

func (d detailModal) Title() string {
	return d.title
}

func (d detailModal) View(width, height int, m Model) string {
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: d.title,
		Body:  d.lines,
		Footer: []string{tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		})},
		Scroll:    int(d.scroll),
		UseScroll: true,
	})
}

func (d detailModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if d.scroll.Handle(msg, len(d.lines), constrainedModalBodyHeight(m.height, 1)) {
		m.modal = d
	}
	return false, nil
}
