package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TruncateANSI shortens value to the given display width, appending "..."
// when content is cut. ANSI escape sequences do not count toward the width.
func TruncateANSI(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "...")
}

// RenderWithBackground renders value with style, adding background unless it
// is lipgloss.NoColor, which callers use to mean "keep the terminal default".
func RenderWithBackground(style lipgloss.Style, background lipgloss.TerminalColor, value string) string {
	if _, noColor := background.(lipgloss.NoColor); noColor {
		return style.Render(value)
	}
	return style.Background(background).Render(value)
}
