package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestFooterLineRendersAndMutesToolHints(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(profile)
	})

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	shortcuts := []Shortcut{
		{ASCII: "enter", Unicode: "↵", Label: "apply"},
		{ASCII: "esc", Label: "cancel"},
	}

	for _, tt := range []struct {
		name  string
		ascii bool
		plain string
		key   string
	}{
		{name: "ASCII", ascii: true, plain: "enter apply  esc cancel", key: "enter"},
		{name: "Unicode with ASCII fallback", ascii: false, plain: "↵ apply  esc cancel", key: "↵"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := FooterLine(tt.ascii, keyStyle, mutedStyle, shortcuts)
			if plain := ansi.Strip(got); plain != tt.plain {
				t.Fatalf("FooterLine() plain text = %q, want %q", plain, tt.plain)
			}
			if !strings.Contains(got, "\x1b[38;5;196m"+tt.key) {
				t.Fatalf("FooterLine() does not apply key style: %q", got)
			}
			if !strings.Contains(got, "\x1b[38;5;244m") {
				t.Fatalf("FooterLine() does not apply muted style: %q", got)
			}
		})
	}
}
