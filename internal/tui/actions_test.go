package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func TestActiveMigrateSameSHAArchivesRelinkWithoutConflict(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Same.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Same.")

	m := New(cfg)
	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("migrate modal did not open")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.modal == nil || !strings.Contains(m.modal.View(100, 30, m), "Migration Results") {
		t.Fatalf("expected result modal, got %#v", m.modal)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
	if !strings.Contains(m.status, "relinked") {
		t.Fatalf("status = %q, want relinked", m.status)
	}
}

func TestActiveMigrateDivergentArchiveOpensConflictModal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Active.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Archived.")

	m := New(cfg)
	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := m.modal.View(120, 40, m)
	if !strings.Contains(view, "Archive conflict: zen-of-go") {
		t.Fatalf("expected conflict modal:\n%s", view)
	}

	updated, _ = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	info, err := skills.Read(archived)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Archived." {
		t.Fatalf("archive description = %q, want Archived.", info.Description)
	}
}
