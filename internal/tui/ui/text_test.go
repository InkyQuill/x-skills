package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestTruncateANSI(t *testing.T) {
	styled := "\x1b[38;5;196mstyled value\x1b[0m"

	for _, tt := range []struct {
		name  string
		value string
		width int
		want  string
	}{
		{name: "zero width", value: "value", width: 0, want: ""},
		{name: "negative width", value: "value", width: -3, want: ""},
		{name: "fits", value: "value", width: 10, want: "value"},
		{name: "exact width", value: "value", width: 5, want: "value"},
		{name: "over width", value: "value here", width: 8, want: "value..."},
		{name: "wide runes fit", value: "値値値", width: 6, want: "値値値"},
		{name: "wide runes over width", value: "値値値値", width: 6, want: "値..."},
		{name: "ansi preserved when fits", value: styled, width: 12, want: styled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateANSI(tt.value, tt.width); got != tt.want {
				t.Fatalf("TruncateANSI(%q, %d) = %q, want %q", tt.value, tt.width, got, tt.want)
			}
		})
	}
}

func TestTruncateANSIKeepsEscapeSequencesOutOfWidth(t *testing.T) {
	styled := "\x1b[38;5;196mstyled value\x1b[0m"
	got := TruncateANSI(styled, 9)
	if plain := ansi.Strip(got); plain != "styled..." {
		t.Fatalf("TruncateANSI() plain text = %q, want %q", plain, "styled...")
	}
	if lipgloss.Width(got) != 9 {
		t.Fatalf("TruncateANSI() display width = %d, want 9", lipgloss.Width(got))
	}
}

func TestSanitizeANSI(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "plain text unchanged", value: "plain text\tvalue\n", want: "plain text\tvalue\n"},
		{name: "sgr preserved", value: "\x1b[7;38;5;196mselected\x1b[0m", want: "\x1b[7;38;5;196mselected\x1b[0m"},
		{name: "osc hyperlink dropped", value: "a\x1b]8;;https://evil.test\x07link\x1b]8;;\x07b", want: "alinkb"},
		{name: "osc st terminator dropped", value: "a\x1b]0;title\x1b\\b", want: "ab"},
		{name: "cursor movement dropped", value: "a\x1b[10;10Hb", want: "ab"},
		{name: "screen clear dropped", value: "a\x1b[2Jb", want: "ab"},
		{name: "private csi dropped", value: "a\x1b[?25lb", want: "ab"},
		{name: "sgr with intermediates dropped", value: "a\x1b[1 mb", want: "ab"},
		{name: "dcs dropped", value: "a\x1bPq payload\x1b\\b", want: "ab"},
		{name: "bel and del dropped", value: "a\x07b\x7fc", want: "abc"},
		{name: "lone trailing escape dropped", value: "a\x1b", want: "a"},
		{name: "unterminated csi dropped", value: "a\x1b[31", want: "a"},
		{name: "unterminated osc dropped", value: "a\x1b]8;;https://evil.test", want: "a"},
		{name: "wide runes preserved", value: "値\x1b[31m値\x1b[0m", want: "値\x1b[31m値\x1b[0m"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeANSI(tt.value); got != tt.want {
				t.Fatalf("SanitizeANSI(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestPadRightANSIUsesDisplayWidth(t *testing.T) {
	got := PadRightANSI("値", 4)
	if got != "値  " || lipgloss.Width(got) != 4 {
		t.Fatalf("PadRightANSI() = %q width %d, want display width 4", got, lipgloss.Width(got))
	}
}

func TestRenderWithBackground(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(profile)
	})

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	t.Run("NoColor renders without background", func(t *testing.T) {
		got := RenderWithBackground(style, lipgloss.NoColor{}, "value")
		if want := style.Render("value"); got != want {
			t.Fatalf("RenderWithBackground() = %q, want %q", got, want)
		}
	})

	t.Run("background applied", func(t *testing.T) {
		got := RenderWithBackground(style, lipgloss.Color("236"), "value")
		if want := style.Background(lipgloss.Color("236")).Render("value"); got != want {
			t.Fatalf("RenderWithBackground() = %q, want %q", got, want)
		}
		if got == style.Render("value") {
			t.Fatal("RenderWithBackground() ignored the background color")
		}
	})
}
