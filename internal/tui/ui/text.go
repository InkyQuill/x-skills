package ui

import (
	"strings"

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

// PadRightANSI pads value to width using terminal display cells rather than
// bytes or runes. Values wider than width are returned unchanged.
func PadRightANSI(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

// RenderWithBackground renders value with style, adding background unless it
// is lipgloss.NoColor, which callers use to mean "keep the terminal default".
func RenderWithBackground(style lipgloss.Style, background lipgloss.TerminalColor, value string) string {
	if _, noColor := background.(lipgloss.NoColor); noColor {
		return style.Render(value)
	}
	return style.Background(background).Render(value)
}
