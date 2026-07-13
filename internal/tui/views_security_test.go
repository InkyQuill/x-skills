package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderOverlaySanitizesLayerControlSequences(t *testing.T) {
	layer := "safe \x1b[7mstyled\x1b[0m\n\x1b[2Jclear\x1b[10;10H\n\x1b]8;;https://evil.test\x07link\x1b]8;;\x07"
	got := renderOverlay(strings.Repeat(strings.Repeat(".", 40)+"\n", 9)+strings.Repeat(".", 40), layer, 40, 10)
	for _, control := range []string{"\x1b]", "\x07", "\x1b[2J", "\x1b[10;10H"} {
		if strings.Contains(got, control) {
			t.Fatalf("renderOverlay() retained terminal control %q: %q", control, got)
		}
	}
	if !strings.Contains(got, "\x1b[7mstyled\x1b[0m") {
		t.Fatalf("renderOverlay() lost SGR styling: %q", got)
	}
	if !strings.Contains(got, "clear") || !strings.Contains(got, "link") {
		t.Fatalf("renderOverlay() lost layer text: %q", got)
	}
}

func TestSelectableRowSanitizesControlSequencesInEveryState(t *testing.T) {
	malicious := "\x1b[31mred\x1b[0m\x1b]8;;https://evil.test\x07link\x1b]8;;\x07\x1b[2Jclear"
	segments := []rowSegment{
		{text: malicious},
		{render: func(lipgloss.TerminalColor) string { return malicious }},
	}
	for _, state := range []struct {
		name              string
		focused, selected bool
	}{
		{name: "unselected"},
		{name: "focused", focused: true},
		{name: "selected", selected: true},
	} {
		t.Run(state.name, func(t *testing.T) {
			got := selectableRow(segments, state.focused, state.selected, 40)
			if !strings.Contains(got, "\x1b[31mred\x1b[0m") {
				t.Fatalf("selectableRow() lost SGR styling: %q", got)
			}
			for _, control := range []string{"\x1b]", "\x07", "\x1b[2J"} {
				if strings.Contains(got, control) {
					t.Fatalf("selectableRow() retained terminal control %q: %q", control, got)
				}
			}
		})
	}
}
