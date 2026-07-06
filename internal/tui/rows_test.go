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
	if !containsString(groups[0].Chips, ".Ag") || !containsString(groups[0].Chips, "~Cl") {
		t.Fatalf("Chips = %#v, want .Ag and ~Cl", groups[0].Chips)
	}
}

func TestRenderActiveRowsUseSpecSymbols(t *testing.T) {
	m := Model{
		symbols:  symbolsFor(Options{}),
		view:     ViewActive,
		selected: map[string]bool{},
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
	for _, want := range []string{"› □", "zen-of-go", ".Ag", "~Cl", "◆ unmanaged", "×2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("row missing %q:\n%s", want, got)
		}
	}
}

func TestRepoRowsShowUsageChipsAndSelectionMarkers(t *testing.T) {
	m := Model{
		symbols:  symbolsFor(Options{}),
		view:     ViewRepo,
		cursor:   0,
		selected: map[string]bool{"repo:zen-of-go": true},
		repo: []repo.Skill{{
			Name:        "zen-of-go",
			Description: "Go style guide",
		}},
		repoUsage: map[string][]string{"zen-of-go": {".Ag", "~Cl"}},
	}

	got := strings.Join(renderRepoRows(m, 100), "\n")
	for _, want := range []string{"› ■", "zen-of-go", "Go style guide", ".Ag", "~Cl"} {
		if !strings.Contains(got, want) {
			t.Fatalf("repo row missing %q:\n%s", want, got)
		}
	}
}

func TestDoctorRowsShowIssueReasonAndLocation(t *testing.T) {
	m := Model{
		symbols:  symbolsFor(Options{}),
		view:     ViewDoctor,
		selected: map[string]bool{},
		issues: []doctor.Issue{{
			Kind:     doctor.KindBrokenSymlink,
			Name:     "zen-of-go",
			Location: ".Ag",
			Reason:   "symlink target missing",
		}},
	}

	got := strings.Join(renderDoctorRows(m, 100), "\n")
	for _, want := range []string{"›", "▲", "broken-symlink", "zen-of-go", ".Ag", "symlink target missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor row missing %q:\n%s", want, got)
		}
	}
}
