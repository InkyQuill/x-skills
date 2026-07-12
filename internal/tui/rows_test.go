package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestActiveGroupRowsShowRootChipsAliasesAndCount(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	projectRoot := cfg.MustActiveRoot("project", "agents")
	globalRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(projectRoot, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(globalRoot, "go-style")); err != nil {
		t.Fatal(err)
	}

	groups := groupActiveSkills([]actions.ActiveSkill{
		{Name: "zen-of-go", Path: filepath.Join(projectRoot, "zen-of-go"), Root: roots.ActiveRoot{Scope: "project", Target: "agents", Label: ".agents", Path: projectRoot}, Status: actions.StatusManaged, Description: "Go style."},
		{Name: "zen-of-go", Path: filepath.Join(globalRoot, "go-style"), Root: roots.ActiveRoot{Scope: "global", Target: "claude", Label: "~/.claude", Path: globalRoot}, Status: actions.StatusManaged, Description: "Go style."},
	})

	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if groups[0].Name != "zen-of-go" {
		t.Fatalf("Name = %q, want zen-of-go", groups[0].Name)
	}
	if !containsString(groups[0].Aliases, "go-style") {
		t.Fatalf("Aliases = %#v, want go-style", groups[0].Aliases)
	}
	if !containsString(groups[0].Chips, ".agents") || !containsString(groups[0].Chips, "~/.claude") {
		t.Fatalf("Chips = %#v, want .agents and ~/.claude", groups[0].Chips)
	}
}

func TestRenderActiveRowsUseSpecSymbols(t *testing.T) {
	m := Model{
		symbols: symbolsFor(Options{}),
		view:    ViewActive,
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {},
			ViewDoctor: {},
		},
		active: []ActiveGroup{{
			ID:          "active:one",
			Name:        "zen-of-go",
			Status:      actions.StatusUnmanaged,
			Description: "Go style.",
			Chips:       []string{".Ag", "~Cl"},
			Members:     []actions.ActiveSkill{{Path: "/a"}, {Path: "/b"}},
		}},
	}

	rows := renderActiveRows(m, 100)
	got := strings.Join(rows, "\n")
	for _, want := range []string{"› ◇ ◇ zen-of-go", ".Ag", "~Cl", "Go style."} {
		if !strings.Contains(got, want) {
			t.Fatalf("row missing %q:\n%s", want, got)
		}
	}
	for _, unexpected := range []string{"◆ unmanaged", "unmanaged", "×2"} {
		if strings.Contains(got, unexpected) {
			t.Fatalf("row should not contain %q:\n%s", unexpected, got)
		}
	}
	if strings.Contains(got, "2 linked locations") {
		t.Fatalf("row should show description instead of location placeholder:\n%s", got)
	}
}

func TestStatusRenderersUseExactSymbolsIndependentlyOfRowState(t *testing.T) {
	tests := []struct {
		name       string
		ascii      bool
		status     string
		wantChip   string
		wantMarker string
	}{
		{name: "unicode managed", status: actions.StatusManaged, wantChip: "✓ managed", wantMarker: "✓"},
		{name: "unicode unmanaged", status: actions.StatusUnmanaged, wantChip: "◇ unmanaged", wantMarker: "◇"},
		{name: "unicode broken", status: actions.StatusBroken, wantChip: "× broken", wantMarker: "×"},
		{name: "ASCII managed", ascii: true, status: actions.StatusManaged, wantChip: "+ managed", wantMarker: "+"},
		{name: "ASCII unmanaged", ascii: true, status: actions.StatusUnmanaged, wantChip: "? unmanaged", wantMarker: "?"},
		{name: "ASCII broken", ascii: true, status: actions.StatusBroken, wantChip: "x broken", wantMarker: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, cursor := range []int{0, 1} {
				for _, selected := range []bool{false, true} {
					m := Model{
						symbols: symbolsFor(Options{ASCII: tt.ascii}),
						cursor:  cursor,
						selected: map[ViewName]map[string]bool{
							ViewActive: {"active:one": selected},
						},
					}
					if got := plain(renderStatusChip(m, tt.status)); got != tt.wantChip {
						t.Errorf("renderStatusChip(cursor=%d, selected=%t) = %q, want %q", cursor, selected, got, tt.wantChip)
					}
					if got := plain(renderStatusDotWithBackground(m, tt.status, lipgloss.NoColor{})); got != tt.wantMarker {
						t.Errorf("renderStatusDotWithBackground(cursor=%d, selected=%t) = %q, want %q", cursor, selected, got, tt.wantMarker)
					}
				}
			}
		})
	}
}

func TestRepoRowsShowUsageChipsAndSelectionMarkers(t *testing.T) {
	m := Model{
		symbols: symbolsFor(Options{}),
		view:    ViewRepo,
		cursor:  0,
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {"repo:zen-of-go": true},
			ViewDoctor: {},
		},
		repo: []repo.Skill{{
			Name:        "zen-of-go",
			Description: "Go style guide",
		}},
		repoUsage: map[string][]string{"zen-of-go": {".Ag", "~Cl"}},
	}

	got := strings.Join(renderRepoRows(m, 100), "\n")
	if colorAvailableForTest() && (!selectedBackgroundConfigured() || !cursorBackgroundConfigured()) {
		t.Fatal("row background styles are not configured")
	}
	if colorAvailableForTest() && !rowBackgroundsAreDistinct() {
		t.Fatal("cursor and selected row background styles must be different")
	}
	if colorAvailableForTest() && !rootBadgeBackgroundsConfigured() {
		t.Fatal("root badge background styles are not configured")
	}
	plain := strings.TrimRight(ansi.Strip(got), " ")
	want := "› ◆ zen-of-go .Ag ~Cl Go style guide"
	if plain != want {
		t.Fatalf("repo row = %q, want %q", plain, want)
	}
}

func TestHighlightedRepoRowPreservesRootPills(t *testing.T) {
	m := Model{
		symbols: symbolsFor(Options{}),
		view:    ViewRepo,
		cursor:  0,
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {"repo:zen-of-go": true},
			ViewDoctor: {},
		},
		repo: []repo.Skill{{
			Name:        "zen-of-go",
			Description: "Go style guide",
		}},
		repoUsage: map[string][]string{"zen-of-go": {".Ag", "~Cl"}},
	}

	got := strings.Join(renderRepoRows(m, 100), "\n")
	if colorAvailableForTest() && !strings.Contains(got, "") {
		t.Fatalf("highlighted row lost root pill edge glyphs:\n%q", got)
	}
	if colorAvailableForTest() && !rootBadgeBackgroundsConfigured() {
		t.Fatal("root badge background styles are not configured")
	}
}

func TestRenderPillUsesRoundedCapsuleShape(t *testing.T) {
	symbols := symbolsFor(Options{})
	got := plain(tuiui.Pill(symbols.BadgeLeft, symbols.BadgeRight, tuiui.PillProps{
		Color:      projectChip.GetBackground(),
		Background: selectedBg.GetBackground(),
		Text:       ".Ag",
		TextColor:  lipgloss.Color("230"),
	}))
	want := ".Ag"
	if got != want {
		t.Fatalf("pill = %q, want %q", got, want)
	}
}

func TestRootChipCapsUnknownTargets(t *testing.T) {
	if got := rootChip("project", "opencode"); got != ".Op" {
		t.Fatalf("rootChip(project, opencode) = %q, want .Op", got)
	}
	if got := rootChip("global", "hermes"); got != "~He" {
		t.Fatalf("rootChip(global, hermes) = %q, want ~He", got)
	}
}

func TestDoctorRowsShowIssueReasonAndLocation(t *testing.T) {
	m := Model{
		symbols: symbolsFor(Options{}),
		view:    ViewDoctor,
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {},
			ViewDoctor: {},
		},
		issues: []doctor.Issue{{
			Kind:     doctor.KindBrokenSymlink,
			Name:     "zen-of-go",
			Location: ".Ag",
			Reason:   "symlink target missing",
		}},
	}

	got := strings.Join(renderDoctorRows(m, 100), "\n")
	for _, want := range []string{"›", "×", "broken-symlink", "zen-of-go", ".Ag", "symlink target missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor row missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, m.symbols.Unchecked) || strings.Contains(got, m.symbols.Checked) {
		t.Fatalf("doctor row should not render selection checkbox:\n%s", got)
	}
}
