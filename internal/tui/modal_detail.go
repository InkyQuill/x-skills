package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type detailModal struct {
	title string
	lines []string
}

func newDetailModal(title string, lines []string) modal {
	return detailModal{title: title, lines: lines}
}

func activeDetailModal(group ActiveGroup) modal {
	lines := []string{
		"Canonical name: " + group.Name,
		"Status: " + group.Status,
		"Aliases: " + strings.Join(group.Aliases, ", "),
		"",
		"Active members",
	}
	for _, member := range group.Members {
		lines = append(lines, fmt.Sprintf("  %s  %s", rootChip(member.Root.Scope, member.Root.Target), member.Path))
	}
	lines = append(lines, "", "Debug", "  fingerprint: "+group.Fingerprint)
	return newDetailModal("Detail: "+group.Name+" (Active)", lines)
}

func (d detailModal) Title() string {
	return d.title
}

func (d detailModal) View(width, height int, m Model) string {
	body := append([]string{accentStyle.Render(d.title), ""}, d.lines...)
	body = append(body, "", mutedStyle.Render("up/down scroll   esc close   q close"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (d detailModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	return closeOnEscapeOrQuit(msg), nil
}
