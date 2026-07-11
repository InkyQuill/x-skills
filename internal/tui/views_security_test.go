package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSelectableRowSanitizesControlSequencesInEveryState(t *testing.T) {
	malicious := "safe\x1b[31mCSI\x1b]8;;https://evil.test\x07OSC\x1b]8;;\x07"
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
			if strings.ContainsAny(got, "\x1b\x07") {
				t.Fatalf("selectableRow() retained terminal controls: %q", got)
			}
		})
	}
}
