package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestFilterNarrowsActiveRowsAndExcludesFullPaths(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	makeSkill(t, cfg.MustActiveRoot("project", "claude"), "prompt-master", "Prompts.")
	m := New(cfg)

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

func TestFilterClearsOnViewSwitch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zen"))
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

func TestSelectionClearsOnViewSwitch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if len(m.selected) == 0 {
		t.Fatal("selection was not set")
	}
	updated, _ = m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if len(m.selected) != 0 {
		t.Fatalf("selected = %#v, want cleared", m.selected)
	}
}
