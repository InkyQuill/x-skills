package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
)

func makeSkill(t *testing.T, root, name, description string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestModelSwitchesViews(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo", m.view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if m.view != ViewDoctor {
		t.Fatalf("view = %q, want doctor", m.view)
	}
}

func TestWizardPreviewIncludesDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")

	m := New(cfg)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = updated.(Model)

	if !m.wizard.Open {
		t.Fatal("wizard is not open")
	}
	if m.wizard.Preview == "" {
		t.Fatal("wizard preview is empty")
	}
	if !strings.Contains(m.wizard.Preview, "./.agents") {
		t.Fatalf("preview missing default destination: %q", m.wizard.Preview)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = updated.(Model)
	if !strings.Contains(m.wizard.Preview, "~/.codex") {
		t.Fatalf("preview missing updated destination: %q", m.wizard.Preview)
	}
}

func TestWizardConsumesBackgroundKeys(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")

	m := New(cfg)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = updated.(Model)
	if !m.wizard.Open {
		t.Fatal("wizard is not open")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo while wizard is open", m.view)
	}
}

func TestActiveGroupsMergeByFingerprint(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "shared-skill", "Shared.")

	projectRoot := cfg.ActiveRoot("project", "agents")
	globalRoot := cfg.ActiveRoot("global", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(projectRoot, "shared-skill")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(globalRoot, "renamed-skill")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	if len(m.active) != 1 {
		t.Fatalf("active groups = %d, want 1", len(m.active))
	}
	if len(m.active[0].Locations) != 2 {
		t.Fatalf("locations = %#v, want 2", m.active[0].Locations)
	}
	if strings.Contains(m.View(), "sha:") {
		t.Fatalf("view leaked internal fingerprint:\n%s", m.View())
	}
}
