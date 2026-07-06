package tui

import (
	"os"
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

func TestActiveUnlinkGroupsManagedBrokenAndUnmanaged(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "managed", "Managed.")
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(root, "managed")); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, root, "unmanaged", "Unmanaged.")
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.selected = map[string]bool{}
	for _, group := range m.active {
		m.selected[group.ID] = true
	}
	updated, _ := m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("unlink modal is nil")
	}
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Unlink active skills", "Managed links", "Broken links", "Unmanaged directories", "Migrate to repo, then unlink active copies"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlink modal missing %q:\n%s", want, view)
		}
	}
}

func TestRepoLinkModalShowsDestinationAndCreatesLink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("repo link modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Link repo skill", "scope", "project", "target", ".Ag", "Will create"} {
		if !strings.Contains(view, want) {
			t.Fatalf("link modal missing %q:\n%s", want, view)
		}
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot("project", "agents"), "zen-of-go")); err != nil {
		t.Fatalf("link was not created: %v", err)
	}
}

func TestRepoLinkModalCanChangeDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if _, err := os.Lstat(filepath.Join(cfg.GlobalCodexRoot, "zen-of-go")); err != nil {
		t.Fatalf("global codex link was not created: %v", err)
	}
}

func TestRepoUnlinkUsageChooserDefaultsAllUsagesSelected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	projectRoot := cfg.MustActiveRoot("project", "agents")
	globalRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(projectRoot, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(globalRoot, "zen-of-go")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Unlink usages: zen-of-go", "■ .Ag", "■ ~Cl", "Unlink selected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("usage chooser missing %q:\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(projectRoot, "zen-of-go")); !os.IsNotExist(err) {
		t.Fatalf("project usage still exists or unexpected error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(globalRoot, "zen-of-go")); !os.IsNotExist(err) {
		t.Fatalf("global usage still exists or unexpected error: %v", err)
	}
}

func TestRepoDeleteWithUsagesShowsScopeLimit(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(root, "zen-of-go")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Delete archive: zen-of-go", "Visible usages", "Only current project roots and global roots are known", "Unlink visible usages, then delete archive"} {
		if !strings.Contains(view, want) {
			t.Fatalf("delete modal missing %q:\n%s", want, view)
		}
	}
}

func TestDoctorFixModalShowsIssueCountsAndApplies(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "zen-of-go")
	if err := os.Symlink(filepath.Join(home, "missing"), broken); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("f"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("doctor fix modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Confirm", "Apply", "Doctor fixes", "broken symlink"} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor fix modal missing %q:\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(broken); !os.IsNotExist(err) {
		t.Fatalf("broken symlink still exists or unexpected error: %v", err)
	}
}
