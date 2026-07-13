package tui

import (
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestActiveDetailShowsDifferingDeclaredNameAsMetadata(t *testing.T) {
	modal := activeDetailModal(ActiveGroup{
		Identity:     "composition-patterns",
		DeclaredName: "vercel-composition-patterns",
		Status:       actions.StatusUnmanaged,
	}, symbolsFor(Options{}))
	view := plain(modal.View(100, 30, Model{}))
	if !strings.Contains(view, "Declared name: vercel-composition-patterns") {
		t.Fatalf("detail missing declared name metadata:\n%s", view)
	}
}

func TestInspectorRendersKeyValueHierarchy(t *testing.T) {
	got := renderInspectorDocument("Inspector", []inspectorSection{{
		Title: "next-best-practices",
		Rows: []inspectorRow{
			{Key: "Source", Value: "vercel-labs/skills"},
			{Key: "Status", Value: "update available"},
			{Key: "Audit", Value: "warn"},
		},
	}}, 80, 10)

	view := plain(got)
	for _, want := range []string{
		"Inspector",
		"next-best-practices",
		"Source      vercel-labs/skills",
		"Status      update available",
		"Audit       warn",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("inspector missing %q:\n%s", want, view)
		}
	}

	if colorAvailableForTest() {
		_, keyNoColor := inspectorKeyStyle.GetForeground().(lipgloss.NoColor)
		_, valueNoColor := inspectorValueStyle.GetForeground().(lipgloss.NoColor)
		if keyNoColor || valueNoColor {
			t.Fatal("inspector key/value foreground styles are not configured")
		}
		if inspectorKeyStyle.GetForeground() == inspectorValueStyle.GetForeground() {
			t.Fatal("inspector key/value styles must be configured separately")
		}
	}
}

func TestInspectorRendersRichValue(t *testing.T) {
	got := renderInspectorDocument("Inspector", []inspectorSection{{
		Title: "next-best-practices",
		Rows: []inspectorRow{{
			Key: "Usages",
			Render: func(width int) string {
				return tuiui.TruncateANSI(
					installSourceStyle.Render("vercel-labs/skills")+" "+
						installCountStyle.Render("812 installs"),
					width,
				)
			},
		}},
	}}, 80, 10)

	view := plain(got)
	if !strings.Contains(view, "Usages      vercel-labs/skills 812 installs") {
		t.Fatalf("rich inspector value missing:\n%s", view)
	}
	if colorAvailableForTest() {
		_, sourceNoColor := installSourceStyle.GetForeground().(lipgloss.NoColor)
		_, countNoColor := installCountStyle.GetForeground().(lipgloss.NoColor)
		if sourceNoColor || countNoColor {
			t.Fatal("install rich value styles are not configured")
		}
	}
}

func TestInspectorRendersBlockValueWithoutTruncatingDescription(t *testing.T) {
	description := "Use when adding, altering, or removing schema objects in Goose migrations with enough detail to wrap."
	got := renderInspectorDocument("Inspector", []inspectorSection{{
		Title: "goose-migration",
		Rows: []inspectorRow{
			{Key: "Source", Value: "local"},
			{Key: "Description", Value: description, Block: true},
		},
	}}, 42, 10)

	view := plain(got)
	if !strings.Contains(view, "Source      local") {
		t.Fatalf("inline key/value row changed:\n%s", view)
	}
	if !strings.Contains(view, "Description\nUse when adding, altering, or removing\nschema objects in Goose migrations with\nenough detail to wrap.") {
		t.Fatalf("block description did not render fully wrapped:\n%s", view)
	}
	if strings.Contains(view, "...") {
		t.Fatalf("block description was truncated:\n%s", view)
	}
	assertRawLinesWithinWidth(t, got, 42)
}

func TestBlockInspectorValueDoesNotPadUnequalLines(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	style := inspectorValueStyle
	inspectorValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	t.Cleanup(func() { inspectorValueStyle = style })

	got := blockInspectorValue("a longer line\nshort")(20)
	lines := strings.Split(got, "\n")
	want := []string{"a longer line", "short"}

	if len(lines) != len(want) {
		t.Fatalf("line count = %d, want %d: %q", len(lines), len(want), plain(got))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "\x1b[") {
			t.Errorf("line %d does not preserve its own ANSI style: %q", i, line)
		}
		if strings.HasSuffix(line, " ") {
			t.Errorf("line %d has trailing style padding: %q", i, plain(line))
		}
		if content := plain(line); content != want[i] {
			t.Errorf("line %d content = %q, want %q", i, content, want[i])
		}
		if width := lipgloss.Width(line); width != lipgloss.Width(want[i]) {
			t.Errorf("line %d ANSI width = %d, want %d", i, width, lipgloss.Width(want[i]))
		}
	}
}

func TestInspectorPadsUnicodeKeysByDisplayWidth(t *testing.T) {
	const width = 20

	combiningKey := "Cafe\u0301"
	if lipgloss.Width(combiningKey) == len([]rune(combiningKey)) {
		t.Fatal("test setup must use a key whose display width differs from rune count")
	}

	got := renderInspectorDocument("Inspector", []inspectorSection{{
		Title: "unicode",
		Rows: []inspectorRow{
			{Key: "キー", Value: "値値値値値値"},
			{
				Key: combiningKey,
				Render: func(width int) string {
					return installSourceStyle.Render(tuiui.TruncateANSI("styled値値値", width))
				},
			},
		},
	}}, width, 10)

	assertRawLinesWithinWidth(t, got, width)

	lines := strings.Split(plain(got), "\n")
	if len(lines) != 4 {
		t.Fatalf("line count = %d, want 4:\n%s", len(lines), plain(got))
	}
	for _, line := range lines[2:] {
		keyColumn := ansi.Cut(line, 0, inspectorKeyWidth)
		if gotWidth := lipgloss.Width(keyColumn); gotWidth != inspectorKeyWidth {
			t.Fatalf("key column width = %d, want %d for %q:\n%s", gotWidth, inspectorKeyWidth, line, plain(got))
		}
	}
}

func TestInspectorTruncatesToWidth(t *testing.T) {
	const width = 18

	title := "Inspector title that is much too long"
	section := "section heading that is much too long"
	key := "ExtremelyLongKeyName"
	value := "plain value that is much too long"
	richValue := "rich value that is much too long"

	got := renderInspectorDocument(title, []inspectorSection{{
		Title: section,
		Rows: []inspectorRow{
			{Key: key, Value: value},
			{
				Key: "Rich",
				Render: func(width int) string {
					return installSourceStyle.Render(tuiui.TruncateANSI(richValue, width))
				},
			},
		},
	}}, width, 10)

	lines := strings.Split(plain(got), "\n")
	for _, line := range lines {
		if gotWidth := lipgloss.Width(line); gotWidth > width {
			t.Fatalf("line width = %d, want <= %d for %q:\n%s", gotWidth, width, line, plain(got))
		}
	}

	if len(lines) != 4 {
		t.Fatalf("line count = %d, want 4:\n%s", len(lines), plain(got))
	}
	for i, line := range lines {
		if !strings.Contains(line, "...") {
			t.Fatalf("line %d was not truncated: %q\n%s", i, line, plain(got))
		}
	}

	view := plain(got)
	for _, full := range []string{title, section, key, value, richValue} {
		if strings.Contains(view, full) {
			t.Fatalf("overlong content %q was not truncated:\n%s", full, view)
		}
	}
}

func TestInspectorTruncatesToHeight(t *testing.T) {
	got := renderInspectorDocument("Inspector", []inspectorSection{{
		Title: "next-best-practices",
		Rows: []inspectorRow{
			{Key: "Source", Value: "vercel-labs/skills"},
			{Key: "Status", Value: "update available"},
			{Key: "Audit", Value: "warn"},
		},
	}}, 80, 3)

	lines := strings.Split(plain(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3:\n%s", len(lines), got)
	}
	if strings.Contains(plain(got), "Status") || strings.Contains(plain(got), "Audit") {
		t.Fatalf("inspector exceeded requested height:\n%s", plain(got))
	}
}

func assertRawLinesWithinWidth(t *testing.T, raw string, width int) {
	t.Helper()
	for i, line := range strings.Split(plain(raw), "\n") {
		if gotWidth := lipgloss.Width(line); gotWidth > width {
			t.Fatalf("line %d width = %d, want <= %d for %q:\n%s", i, gotWidth, width, line, plain(raw))
		}
	}
}
