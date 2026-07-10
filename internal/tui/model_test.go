package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

func customRootConfig(t *testing.T) config.Config {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`active_roots:
  - scope: project
    target: agents
    enabled: false
  - scope: project
    target: claude
    enabled: false
  - scope: project
    target: codex
    enabled: false
  - scope: global
    target: agents
    enabled: false
  - scope: global
    target: claude
    enabled: false
  - scope: global
    target: codex
    enabled: false
  - scope: project
    target: opencode
    path: .opencode/skills
    label: .Oc
`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func mustModel(t *testing.T, updated tea.Model) Model {
	t.Helper()
	m, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T, want tui.Model", updated)
	}
	return m
}

func plain(value string) string {
	return ansi.Strip(value)
}

func selectedBackgroundConfigured() bool {
	_, noColor := selectedBg.GetBackground().(lipgloss.NoColor)
	return !noColor
}

func cursorBackgroundConfigured() bool {
	_, noColor := cursorBg.GetBackground().(lipgloss.NoColor)
	return !noColor
}

func rowBackgroundsAreDistinct() bool {
	return cursorBg.GetBackground() != selectedBg.GetBackground()
}

func rootBadgeBackgroundsConfigured() bool {
	_, projectNoColor := projectChip.GetBackground().(lipgloss.NoColor)
	_, globalNoColor := globalChip.GetBackground().(lipgloss.NoColor)
	return !projectNoColor && !globalNoColor
}

func colorAvailableForTest() bool {
	_, disabled := os.LookupEnv("NO_COLOR")
	return !disabled
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

	view := plain(m.View())
	if strings.Contains(view, "◆") || strings.Contains(view, "◇") {
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

	view := plain(m.View())
	if !strings.Contains(view, "› ◇ skill-09") {
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

	view := plain(m.View())
	if !strings.Contains(view, "installed opentui-react") {
		t.Fatalf("view missing status:\n%s", view)
	}
	if !strings.Contains(view, "↵ details  / filter  p preview  m migrate  u unlink  c clear  ^R refresh") {
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

func TestActiveViewSortsSkillsAlphabeticallyByName(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zeta-skill", "Zeta.")
	makeSkill(t, cfg.MustActiveRoot("project", "claude"), "alpha-skill", "Alpha.")

	m := New(cfg)
	if len(m.active) < 2 {
		t.Fatalf("active groups = %#v, want at least 2", m.active)
	}
	if m.active[0].Name != "alpha-skill" || m.active[1].Name != "zeta-skill" {
		t.Fatalf("active order = %#v, want alphabetical by name", []string{m.active[0].Name, m.active[1].Name})
	}
}

func TestRepoViewSortsSkillsAlphabeticallyByName(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zeta-skill", "Zeta.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")

	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	skills := m.visibleRepoSkills()
	if len(skills) < 2 {
		t.Fatalf("repo skills = %#v, want at least 2", skills)
	}
	if skills[0].Name != "alpha-skill" || skills[1].Name != "zeta-skill" {
		t.Fatalf("repo order = %#v, want alphabetical by name", []string{skills[0].Name, skills[1].Name})
	}
}

func TestDoctorViewSortsIssuesAlphabeticallyByName(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	agentsRoot := cfg.MustActiveRoot("project", "agents")
	claudeRoot := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing-zeta"), filepath.Join(agentsRoot, "zeta-skill")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing-alpha"), filepath.Join(claudeRoot, "alpha-skill")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)
	if len(m.issues) < 2 {
		t.Fatalf("doctor issues = %#v, want at least 2", m.issues)
	}
	if m.issues[0].Name != "alpha-skill" || m.issues[1].Name != "zeta-skill" {
		t.Fatalf("doctor issue order = %#v, want alphabetical by name", []string{m.issues[0].Name, m.issues[1].Name})
	}
}
