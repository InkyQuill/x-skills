package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFooterLineRendersAndMutesToolHints(t *testing.T) {
	got := FooterLine(
		true,
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		[]Shortcut{
			{ASCII: "enter", Label: "apply"},
			{ASCII: "esc", Label: "cancel"},
		},
	)
	if got != "enter apply  esc cancel" {
		t.Fatalf("FooterLine() = %q", got)
	}
}
