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

// SanitizeANSI removes terminal control sequences from value while keeping
// SGR styling (colors and text attributes). OSC, DCS, and non-SGR CSI
// sequences — hyperlinks, cursor movement, screen clearing — are dropped, as
// are raw control bytes other than newline and tab.
func SanitizeANSI(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); {
		b := value[i]
		if b == 0x1b {
			i += consumeEscape(value[i:], &out)
			continue
		}
		if b == '\n' || b == '\t' || (b >= 0x20 && b != 0x7f) {
			out.WriteByte(b)
		}
		i++
	}
	return out.String()
}

// consumeEscape inspects an escape sequence at the start of s, writes it to
// out only when it is a plain SGR sequence, and reports the bytes consumed.
func consumeEscape(s string, out *strings.Builder) int {
	if len(s) == 1 {
		return 1
	}
	switch s[1] {
	case '[':
		j := 2
		for j < len(s) && s[j] >= 0x30 && s[j] <= 0x3f {
			j++
		}
		for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2f {
			j++
		}
		if j >= len(s) {
			return len(s)
		}
		if s[j] == 'm' && isSGRParams(s[2:j]) {
			out.WriteString(s[:j+1])
		}
		return j + 1
	case ']', 'P', 'X', '^', '_':
		for j := 2; j < len(s); j++ {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
		}
		return len(s)
	default:
		return 2
	}
}

func isSGRParams(params string) bool {
	for i := 0; i < len(params); i++ {
		b := params[i]
		if (b < '0' || b > '9') && b != ';' && b != ':' {
			return false
		}
	}
	return true
}

// RenderWithBackground renders value with style, adding background unless it
// is lipgloss.NoColor, which callers use to mean "keep the terminal default".
func RenderWithBackground(style lipgloss.Style, background lipgloss.TerminalColor, value string) string {
	if _, noColor := background.(lipgloss.NoColor); noColor {
		return style.Render(value)
	}
	return style.Background(background).Render(value)
}
