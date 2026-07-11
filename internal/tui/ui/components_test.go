package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestJoinPills(t *testing.T) {
	styled := "\x1b[38;5;24m~scope\x1b[0m"

	for _, tt := range []struct {
		name      string
		pills     []string
		separator string
		want      string
	}{
		{name: "empty", pills: nil, separator: " ", want: ""},
		{name: "single", pills: []string{styled}, separator: " ", want: styled},
		{name: "multiple", pills: []string{styled, "plain "}, separator: " ", want: styled + " plain "},
		{name: "custom separator", pills: []string{"a", "b"}, separator: " | ", want: "a | b"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinPills(tt.pills, tt.separator); got != tt.want {
				t.Fatalf("JoinPills(%q, %q) = %q, want %q", tt.pills, tt.separator, got, tt.want)
			}
		})
	}
}

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
