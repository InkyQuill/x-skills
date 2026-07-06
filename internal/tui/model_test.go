package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
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

func mustModel(t *testing.T, updated tea.Model) Model {
	t.Helper()
	m, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T, want tui.Model", updated)
	}
	return m
}

func TestModelSwitchesViews(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = mustModel(t, updated)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo", m.view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = mustModel(t, updated)
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
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = mustModel(t, updated)

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
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = mustModel(t, updated)
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
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = mustModel(t, updated)
	if !m.wizard.Open {
		t.Fatal("wizard is not open")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = mustModel(t, updated)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo while wizard is open", m.view)
	}
}

func TestRowsScrollToKeepCursorVisible(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	for i := 0; i < 10; i++ {
		makeSkill(t, cfg.ArchiveSkillsRoot(), fmt.Sprintf("skill-%02d", i), "Repo.")
	}

	m := New(cfg)
	m.width = 80
	m.height = 10
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = mustModel(t, updated)
	for i := 0; i < 9; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}

	view := m.View()
	if !strings.Contains(view, ">[ ] skill-09") {
		t.Fatalf("view does not show selected last row:\n%s", view)
	}
	if strings.Contains(view, "skill-00") {
		t.Fatalf("view did not scroll away from first row:\n%s", view)
	}
}

func TestFooterShortcutsStayVisibleWithStatusAndWizard(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 80
	m.height = 14
	m.status = "installed opentui-react"
	m.wizard = Wizard{Open: true, Action: ActionInstall, Preview: "Install opentui-react"}

	view := m.View()
	if !strings.Contains(view, "installed opentui-react") {
		t.Fatalf("view missing status:\n%s", view)
	}
	if !strings.Contains(view, "space select  i install  m migrate  u unlink  f fix  q quit") {
		t.Fatalf("view missing footer shortcuts:\n%s", view)
	}
}

func TestMigrateWizardIgnoresManagedDuplicateLinks(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "shared-skill", "Shared.")
	agentsRoot := cfg.MustActiveRoot("project", "agents")
	claudeRoot := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(agentsRoot, "shared-skill")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(claudeRoot, "shared-skill")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.openWizard(ActionMigrate)
	if len(m.wizard.Active) != 0 {
		t.Fatalf("migrate wizard active = %#v, want none for managed links", m.wizard.Active)
	}
	if !strings.Contains(m.wizard.Preview, "No unmanaged active skill directories selected.") {
		t.Fatalf("preview = %q", m.wizard.Preview)
	}
}

func TestMigrateWizardPromptsForArchiveConflict(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Active.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Archived.")

	m := New(cfg)
	m.openWizard(ActionMigrate)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if !m.wizard.Open || m.wizard.Conflict == nil {
		t.Fatalf("wizard conflict = %#v, open=%v", m.wizard.Conflict, m.wizard.Open)
	}
	if !strings.Contains(m.wizard.Preview, "archive") || !strings.Contains(m.wizard.Preview, "active") {
		t.Fatalf("preview missing side-by-side labels:\n%s", m.wizard.Preview)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = mustModel(t, updated)
	if m.wizard.Open {
		t.Fatal("wizard still open after resolving conflict")
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
	info, err := skills.Read(archived)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Archived." {
		t.Fatalf("Description = %q, want Archived.", info.Description)
	}
}

func TestActiveGroupsMergeByFingerprint(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "shared-skill", "Shared.")

	projectRoot := cfg.MustActiveRoot("project", "agents")
	globalRoot := cfg.MustActiveRoot("global", "claude")
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
