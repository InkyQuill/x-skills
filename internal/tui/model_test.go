package tui

import (
	"fmt"
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

func mustModel(t *testing.T, updated tea.Model) Model {
	t.Helper()
	m, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T, want tui.Model", updated)
	}
	return m
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func keyCtrlR() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlR}
}

func TestModelSwitchesViews(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo", m.view)
	}

	updated, _ = m.Update(keyRunes("D"))
	m = mustModel(t, updated)
	if m.view != ViewDoctor {
		t.Fatalf("view = %q, want doctor", m.view)
	}
}

func TestModelSwitchesViewsWithUppercaseGlobalTabs(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo", m.view)
	}

	updated, _ = m.Update(keyRunes("D"))
	m = mustModel(t, updated)
	if m.view != ViewDoctor {
		t.Fatalf("view = %q, want doctor", m.view)
	}

	updated, _ = m.Update(keyRunes("A"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active", m.view)
	}
}

func TestLowercaseTabKeysDoNotSwitchViews(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("r"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active because lowercase r is not a tab key", m.view)
	}

	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active because lowercase d is not a tab key", m.view)
	}
}

func TestCtrlRRefreshesWithoutTakingRepoKey(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.status = "old status"

	updated, _ := m.Update(keyCtrlR())
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active after refresh", m.view)
	}
	if m.status != "refreshed" {
		t.Fatalf("status = %q, want refreshed", m.status)
	}
}

func TestASCIIOptionUsesASCIISymbols(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")
	m := New(cfg, Options{ASCII: true})
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	view := m.View()
	if strings.Contains(view, "◆") || strings.Contains(view, "□") || strings.Contains(view, "■") {
		t.Fatalf("view contains unicode symbols in ASCII mode:\n%s", view)
	}
	if !strings.Contains(view, "* x-skills") {
		t.Fatalf("view missing ASCII product mark:\n%s", view)
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
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	for i := 0; i < 9; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}

	view := m.View()
	if !strings.Contains(view, "› □ skill-09") {
		t.Fatalf("view does not show selected last row:\n%s", view)
	}
	if strings.Contains(view, "skill-00") {
		t.Fatalf("view did not scroll away from first row:\n%s", view)
	}
}

func TestViewHeightDoesNotExceedWindowHeight(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	for i := 0; i < 12; i++ {
		makeSkill(t, cfg.ArchiveSkillsRoot(), fmt.Sprintf("skill-%02d", i), "Repo.")
	}
	m := New(cfg)
	m.width = 100
	m.height = 14
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	if got := strings.Count(m.View(), "\n") + 1; got != m.height {
		t.Fatalf("view height = %d, want %d:\n%s", got, m.height, m.View())
	}
}

func TestFooterShortcutsStayVisibleWithStatusAndModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 80
	m.height = 14
	m.status = "installed opentui-react"
	m.modal = newResultModal("Install Results", []string{"installed opentui-react"})

	view := m.View()
	if !strings.Contains(view, "installed opentui-react") {
		t.Fatalf("view missing status:\n%s", view)
	}
	if !strings.Contains(view, "enter details  / filter  p preview  m migrate  u unlink  c clear  ^R refresh") {
		t.Fatalf("view missing footer shortcuts:\n%s", view)
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
	if len(m.active[0].Chips) != 2 {
		t.Fatalf("chips = %#v, want 2", m.active[0].Chips)
	}
	if strings.Contains(m.View(), "sha:") {
		t.Fatalf("view leaked internal fingerprint:\n%s", m.View())
	}
}
