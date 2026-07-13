package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestFilterNarrowsActiveRowsAndExcludesFullPaths(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	makeSkill(t, cfg.MustActiveRoot("project", "claude"), "prompt-master", "Prompts.")
	m := newLoadedModel(t, cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zen"))
	m = mustModel(t, updated)

	view := m.View()
	if !strings.Contains(view, "zen-of-go") {
		t.Fatalf("filtered view missing zen-of-go:\n%s", view)
	}
	if strings.Contains(view, "prompt-master") {
		t.Fatalf("filtered view still contains prompt-master:\n%s", view)
	}
	if strings.Contains(view, cfg.ProjectRoot) {
		t.Fatalf("filtered row leaked absolute path:\n%s", view)
	}
}

func TestFilterClearsOnViewSwitchAfterFilterAccept(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := newLoadedModel(t, cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zen"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	if m.filter.Active {
		t.Fatal("filter mode still active after view switch")
	}
	if m.filter.Query != "" {
		t.Fatalf("filter query = %q, want empty", m.filter.Query)
	}
}

func TestFilterSupportsBackspaceEditing(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zeta-skill", "Zeta.")
	m := newLoadedModel(t, cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zetx"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = mustModel(t, updated)

	if m.filter.Query != "zet" {
		t.Fatalf("filter query = %q, want zet", m.filter.Query)
	}
	view := plain(m.View())
	if !strings.Contains(view, "zeta-skill") || strings.Contains(view, "zen-of-go") {
		t.Fatalf("filter did not apply edited query:\n%s", view)
	}
}

func TestSelectionClearsOnViewSwitch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := newLoadedModel(t, cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if len(m.selected[ViewActive]) == 0 {
		t.Fatal("selection was not set")
	}
	updated, _ = m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if len(m.selected[ViewActive]) != 0 || len(m.selected[ViewRepo]) != 0 || len(m.selected[ViewDoctor]) != 0 {
		t.Fatalf("selected = %#v, want cleared", m.selected)
	}
}

func TestClearSelectionKeyClearsCurrentSelection(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := newLoadedModel(t, cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if len(m.selected[ViewActive]) == 0 {
		t.Fatal("selection was not set")
	}

	updated, _ = m.Update(keyRunes("c"))
	m = mustModel(t, updated)
	if len(m.selected[ViewActive]) != 0 {
		t.Fatalf("selected = %#v, want cleared", m.selected)
	}
}

func TestDoctorSpaceDoesNotToggleSelection(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(root, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	m := newLoadedModel(t, cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if len(m.selected[ViewDoctor]) != 0 {
		t.Fatalf("doctor selection = %#v, want empty", m.selected)
	}
	view := plain(m.View())
	if strings.Contains(view, "› ◇") || strings.Contains(view, "› ◆") {
		t.Fatalf("doctor view should not render selection checkbox:\n%s", view)
	}
	if strings.Contains(view, "c clear") {
		t.Fatalf("doctor footer should not advertise clear selection:\n%s", view)
	}
}

func TestDoctorClearSelectionKeyDoesNothing(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(root, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	m := newLoadedModel(t, cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("c"))
	m = mustModel(t, updated)
	if m.status == "selection cleared" {
		t.Fatalf("doctor clear key set selection status")
	}
	if ids := m.selectedIDsForView(); ids != nil {
		t.Fatalf("doctor selected IDs = %#v, want nil", ids)
	}
}

func TestFilterCursorAndActionsUseFilteredActiveRows(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "alpha-skill", "Alpha.")
	makeSkill(t, cfg.MustActiveRoot("project", "claude"), "target-skill", "Target.")
	m := newLoadedModel(t, cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("target"))
	m = mustModel(t, updated)

	view := plain(m.View())
	if !strings.Contains(view, "› ◇ ◇ target-skill") {
		t.Fatalf("filtered cursor is not drawn on target row:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("expected detail modal")
	}
	detail := m.modal.View(100, 30, m)
	if !strings.Contains(detail, "Detail: target-skill") {
		t.Fatalf("detail opened wrong skill:\n%s", detail)
	}
}

func TestFilterCursorAndActionsUseFilteredRepoRows(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "target-skill", "Target.")
	m := newLoadedModel(t, cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("target"))
	m = mustModel(t, updated)

	view := m.View()
	if !strings.Contains(view, "› ◇ target-skill") {
		t.Fatalf("filtered repo cursor is not drawn on target row:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if !m.selected[ViewRepo][repoID("target-skill")] {
		t.Fatalf("selected = %#v, want target-skill selected", m.selected)
	}
}

func TestFilterInputDoesNotInterceptUppercaseTabLetters(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "React-skill", "React.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	if m.view != ViewActive {
		t.Fatalf("view = %q, want active while typing uppercase R in filter", m.view)
	}
	if m.filter.Query != "R" {
		t.Fatalf("filter query = %q, want R", m.filter.Query)
	}
}
