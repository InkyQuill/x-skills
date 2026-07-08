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
	title string
	lines []string
}

func newDetailModal(title string, lines []string) modal {
	return detailModal{title: title, lines: lines}
}

func activeDetailModal(group ActiveGroup, symbols symbols) modal {
	lines := []string{
		"Canonical name: " + group.Name,
		"Status: " + group.Status,
		"Aliases: " + strings.Join(group.Aliases, ", "),
		"",
		"Active members",
	}
	for _, member := range group.Members {
		lines = append(lines, fmt.Sprintf("  %s  %s", renderRootChip(symbols, rootChip(member.Root.Scope, member.Root.Target), lipgloss.NoColor{}), member.Path))
	}
	lines = append(lines, "", "Debug", "  fingerprint: "+group.Fingerprint)
	return newDetailModal("Detail: "+group.Name+" (Active)", lines)
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
	return newDetailModal("Detail: "+skill.Name+" (Repo)", lines)
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
	body := append([]string{accentStyle.Render(d.title), ""}, d.lines...)
	body = append(body, "", mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
		{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
		{ASCII: "esc", Unicode: "Esc", Label: "close"},
		{ASCII: "q", Label: "close"},
	})))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (d detailModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	return closeOnEscapeOrQuit(msg), nil
}
