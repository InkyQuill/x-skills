package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/builtin"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
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

	if m.modal != nil {
		t.Fatalf("modal = %#v, want closed after successful migrate", m.modal)
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

func TestActiveMigrateModalKeepsFooterVisibleAndScrollsTargets(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	for i := 0; i < 12; i++ {
		makeSkill(t, cfg.MustActiveRoot("project", "agents"), fmt.Sprintf("skill-%02d", i), "Local.")
	}
	m := New(cfg)
	m.width = 100
	m.height = 18
	for _, group := range m.active {
		m.selected[ViewActive][group.ID] = true
	}

	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("migrate modal did not open")
	}
	view := plain(m.modal.View(m.width, m.height, m))
	if got := strings.Count(view, "\n") + 1; got > m.height {
		t.Fatalf("migrate modal height = %d, want <= %d:\n%s", got, m.height, view)
	}
	for _, want := range []string{"Migrate active skills", "Targets (12)", "skill-00", "[ Apply ]", "Cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("migrate modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "skill-11") {
		t.Fatalf("migrate modal should not render every target before scrolling:\n%s", view)
	}
	for _, unexpected := range []string{"Plan", "Compare active content", "If identical", "If different"} {
		if strings.Contains(view, unexpected) {
			t.Fatalf("migrate modal should not explain migration internals %q:\n%s", unexpected, view)
		}
	}

	for i := 0; i < 20; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	view = plain(m.modal.View(m.width, m.height, m))
	if got := strings.Count(view, "\n") + 1; got > m.height {
		t.Fatalf("scrolled migrate modal height = %d, want <= %d:\n%s", got, m.height, view)
	}
	for _, want := range []string{"skill-11", "[ Apply ]", "Cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("scrolled migrate modal missing %q:\n%s", want, view)
		}
	}
}

func TestActiveMigrateContinuesBatchAfterConflictResolution(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "alpha-skill", "Alpha.")
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "bravo-skill", "Active.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "bravo-skill", "Archived.")
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "charlie-skill", "Charlie.")
	m := New(cfg)
	m.width = 120
	m.height = 40
	for _, group := range m.active {
		m.selected[ViewActive][group.ID] = true
	}

	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("expected conflict modal")
	}
	if view := plain(m.modal.View(120, 40, m)); !strings.Contains(view, "Archive conflict: bravo-skill") {
		t.Fatalf("expected bravo conflict modal:\n%s", view)
	}

	updated, _ = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("modal = %#v, want closed after continuing batch", m.modal)
	}
	if m.status != "migrated 3 locations" {
		t.Fatalf("status = %q, want migrated 3 locations", m.status)
	}
	for _, name := range []string{"alpha-skill", "charlie-skill"} {
		activePath := filepath.Join(cfg.MustActiveRoot("project", "agents"), name)
		archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), name)
		if _, err := os.Stat(archivePath); err != nil {
			t.Fatalf("%s archive missing after batch continuation: %v", name, err)
		}
		resolved, err := filepath.EvalSymlinks(activePath)
		if err != nil {
			t.Fatalf("%s active link missing after batch continuation: %v", name, err)
		}
		if resolved != archivePath {
			t.Fatalf("%s active resolved to %q, want %q", name, resolved, archivePath)
		}
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "bravo-skill"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Archived." {
		t.Fatalf("bravo archive description = %q, want Archived.", info.Description)
	}
}

func TestActiveMigrateContinuesBatchAfterUseActiveConflictResolution(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "alpha-skill", "Active alpha.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Archived alpha.")
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "bravo-skill", "Active bravo.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "bravo-skill", "Archived bravo.")
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "charlie-skill", "Charlie.")
	m := New(cfg)
	m.width = 120
	m.height = 40
	for _, group := range m.active {
		m.selected[ViewActive][group.ID] = true
	}

	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("expected first conflict modal")
	}
	if view := plain(m.modal.View(120, 40, m)); !strings.Contains(view, "Archive conflict: alpha-skill") {
		t.Fatalf("expected alpha conflict modal:\n%s", view)
	}

	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("expected second conflict modal after choosing active for first conflict")
	}
	if view := plain(m.modal.View(120, 40, m)); !strings.Contains(view, "Archive conflict: bravo-skill") {
		t.Fatalf("expected bravo conflict modal after continuing queue:\n%s", view)
	}

	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("modal = %#v, want closed after completing queue", m.modal)
	}
	if m.status != "migrated 3 locations" {
		t.Fatalf("status = %q, want migrated 3 locations", m.status)
	}
	for _, tc := range []struct {
		name string
		want string
	}{
		{name: "alpha-skill", want: "Active alpha."},
		{name: "bravo-skill", want: "Active bravo."},
		{name: "charlie-skill", want: "Charlie."},
	} {
		info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), tc.name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Description != tc.want {
			t.Fatalf("%s archive description = %q, want %q", tc.name, info.Description, tc.want)
		}
	}
}

func TestActiveMigrateConflictResolutionReloadsActiveList(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "alpha-skill", "Active alpha.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Archived alpha.")
	m := New(cfg)
	m.width = 120
	m.height = 40
	for _, group := range m.active {
		m.selected[ViewActive][group.ID] = true
	}

	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("expected conflict modal")
	}

	updated, reloadCmd := m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if reloadCmd == nil {
		t.Fatal("post-mutation reload command = nil, want asynchronous reload")
	}
	updated, _ = m.Update(reloadCmd())
	m = mustModel(t, updated)

	if len(m.active) != 1 {
		t.Fatalf("active groups = %d, want 1", len(m.active))
	}
	group := m.active[0]
	if group.Name != "alpha-skill" || len(group.Members) != 1 {
		t.Fatalf("active group = %#v", group)
	}
	member := group.Members[0]
	if member.Status != actions.StatusManaged {
		t.Fatalf("active status = %q, want managed", member.Status)
	}
	if member.Description != "Active alpha." {
		t.Fatalf("active description = %q, want reloaded active description", member.Description)
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
	m.selected = map[ViewName]map[string]bool{
		ViewActive: {},
		ViewRepo:   {},
		ViewDoctor: {},
	}
	for _, group := range m.active {
		m.selected[ViewActive][group.ID] = true
	}
	updated, _ := m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("unlink modal is nil")
	}
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Unlink usages:", "◆ .Ag", "Unlink selected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlink modal missing %q:\n%s", want, view)
		}
	}
}

func TestActiveUnlinkManagedOnlyAsksForLocationsNotCopy(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "autofix", "Autofix.")
	agentsRoot := cfg.MustActiveRoot("global", "agents")
	claudeRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(agentsRoot, "autofix")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(claudeRoot, "autofix")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("unlink modal is nil")
	}
	view := m.modal.View(120, 35, m)
	for _, want := range []string{"Unlink usages: autofix", "~Ag", "~Cl", "Unlink selected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlink locations modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Migrate to repo") || strings.Contains(view, "Delete active copies") {
		t.Fatalf("managed-only unlink asked to copy/delete active directories:\n%s", view)
	}
}

func TestActiveUnlinkManagedGroupRemovesEachSelectedLocation(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "autofix", "Autofix.")
	agentsPath := filepath.Join(cfg.MustActiveRoot("global", "agents"), "autofix")
	claudePath := filepath.Join(cfg.MustActiveRoot("global", "claude"), "autofix")
	if err := os.MkdirAll(filepath.Dir(agentsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(claudePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, agentsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, claudePath); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if _, err := os.Lstat(agentsPath); !os.IsNotExist(err) {
		t.Fatalf("agents link still exists or unexpected error: %v", err)
	}
	if _, err := os.Lstat(claudePath); !os.IsNotExist(err) {
		t.Fatalf("claude link still exists or unexpected error: %v", err)
	}
	if m.modal != nil {
		t.Fatalf("modal = %#v, want closed after successful managed unlink", m.modal)
	}
	if m.status != "unlinked 2 locations" {
		t.Fatalf("status = %q, want unlinked 2 locations", m.status)
	}
}

func TestActiveUnlinkUnmanagedAliasChoosesLocationThenArchivesSelectedLink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	agentsPath := makeSkill(t, cfg.MustActiveRoot("global", "agents"), "code-review", "Review.")
	claudePath := filepath.Join(cfg.MustActiveRoot("global", "claude"), "code-review")
	if err := os.MkdirAll(filepath.Dir(claudePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(agentsPath, claudePath); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("unlink modal is nil")
	}
	view := m.modal.View(120, 35, m)
	for _, want := range []string{"Unlink usages: code-review", "~Ag", "~Cl", "Unlink selected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlink locations modal missing %q:\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("copy/delete choice modal is nil")
	}
	if !strings.Contains(m.modal.View(120, 35, m), "Copy selected unmanaged skills to repo") {
		t.Fatalf("expected copy/delete choice modal:\n%s", m.modal.View(120, 35, m))
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	archived := filepath.Join(cfg.ArchiveSkillsRoot(), "code-review")
	if _, err := os.Stat(archived); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(agentsPath); err != nil {
		t.Fatalf("agents source should remain: %v", err)
	}
	if _, err := os.Lstat(claudePath); !os.IsNotExist(err) {
		t.Fatalf("claude link still exists or unexpected error: %v", err)
	}
	if m.modal != nil {
		t.Fatalf("modal = %#v, want closed after successful unlink", m.modal)
	}
	if m.status == "" {
		t.Fatal("status is empty after successful unlink")
	}
}

func TestActiveUnlinkUnmanagedAliasConflictOpensDiffAndContinues(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, filepath.Join(home, "external"), "code-review", "Review.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "code-review", "Archived.")
	activePath := filepath.Join(cfg.MustActiveRoot("project", "codex"), "code-review")
	if err := os.MkdirAll(filepath.Dir(activePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, activePath); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.modal == nil {
		t.Fatal("conflict modal is nil")
	}
	view := m.modal.View(120, 40, m)
	if !strings.Contains(view, "Archive conflict: code-review") {
		t.Fatalf("expected archive conflict diff modal:\n%s", view)
	}

	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	info, err := skills.Read(archived)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Review." {
		t.Fatalf("archive description = %q, want Review.", info.Description)
	}
	if _, err := os.Lstat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active link still exists or unexpected error: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("external source should remain: %v", err)
	}
	if m.modal != nil {
		t.Fatalf("modal = %#v, want closed after resolved unlink", m.modal)
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
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"Link repo skill", "Destination", ".Ag", "project:agents", "Will create"} {
		if !strings.Contains(view, want) {
			t.Fatalf("link modal missing %q:\n%s", want, view)
		}
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot("project", "agents"), "zen-of-go")); err != nil {
		t.Fatalf("link was not created: %v", err)
	}
	view = plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "✓ zen-of-go linked") {
		t.Fatalf("link result should report first successful apply, not a second failure:\n%s", view)
	}
	if strings.Contains(view, "already exists") {
		t.Fatalf("link result reports duplicate second apply:\n%s", view)
	}
}

func TestRepoLinkModalShowsFocusedDestinationAndSelectedChoice(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)

	raw := m.modal.View(100, 30, m)
	if colorAvailableForTest() && !selectedBackgroundConfigured() {
		t.Fatal("selected choice background style is not configured")
	}
	view := plain(raw)
	if !strings.Contains(view, "› ● .Ag  project:agents") {
		t.Fatalf("link modal missing focused selected destination:\n%s", view)
	}
	if !strings.Contains(view, "  ○ .Cl  project:claude") {
		t.Fatalf("link modal missing configured destination list:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	raw = m.modal.View(100, 30, m)
	view = plain(raw)
	if !strings.Contains(view, "› ● .Cl  project:claude") {
		t.Fatalf("link modal missing focused destination after moving:\n%s", view)
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

	for i := 0; i < 5; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mustModel(t, updated)

	if _, err := os.Lstat(filepath.Join(cfg.GlobalCodexRoot, "zen-of-go")); err != nil {
		t.Fatalf("global codex link was not created: %v", err)
	}
}

func TestRepoLinkModalUsesConfiguredRoots(t *testing.T) {
	cfg := customRootConfig(t)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.setView(ViewRepo)

	updated, _ := m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("repo link modal is nil")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, ".Oc") || strings.Contains(view, ".Ag") {
		t.Fatalf("repo link modal should use configured custom root only:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(cfg.ProjectRoot, ".opencode", "skills", "zen-of-go")); err != nil {
		t.Fatalf("custom root link was not created: %v", err)
	}
}

func TestRepoLinkUsesSelectedRepoRowInsteadOfCursor(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "target-skill", "Target.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	view := plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "  target-skill") || strings.Contains(view, "  alpha-skill") {
		t.Fatalf("link modal should target selected repo row, not cursor:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot("project", "agents"), "target-skill")); err != nil {
		t.Fatalf("selected repo skill was not linked: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot("project", "agents"), "alpha-skill")); !os.IsNotExist(err) {
		t.Fatalf("cursor repo skill should not be linked, err=%v", err)
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
	raw := m.modal.View(110, 35, m)
	if colorAvailableForTest() && !selectedBackgroundConfigured() {
		t.Fatal("usage chooser selected row background style is not configured")
	}
	view := plain(raw)
	for _, want := range []string{"Unlink usages: zen-of-go", "◆ .Ag", "◆ ~Cl", "Unlink selected"} {
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

func TestRepoUsageModalConstrainsLongTargetList(t *testing.T) {
	targets := make([]repoUsageTarget, 0, 24)
	selected := map[int]bool{}
	for i := 0; i < 24; i++ {
		targets = append(targets, repoUsageTarget{
			Name:   fmt.Sprintf("skill-%02d", i),
			Chip:   ".Ag",
			Path:   fmt.Sprintf("/very/long/path/to/active/root/skill-%02d", i),
			Status: actions.StatusManaged,
		})
		selected[i] = true
	}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.width = 90
	m.height = 14
	m.modal = repoUsageModal{name: "zen-of-go", targets: targets, selected: selected, index: 18}

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 14 {
		t.Fatalf("view height = %d, want 14:\n%s", len(lines), view)
	}
	plainView := plain(view)
	for _, want := range []string{"Unlink usages: zen-of-go", "skill-18", "Unlink selected", "↑/↓ move"} {
		if !strings.Contains(plainView, want) {
			t.Fatalf("constrained usage modal missing %q:\n%s", want, plainView)
		}
	}
	if strings.Contains(plainView, "skill-00") {
		t.Fatalf("usage modal did not scroll long target list:\n%s", plainView)
	}

	for i := 0; i < 5; i++ {
		updated, _ := m.Update(keyRunes("j"))
		m = mustModel(t, updated)
	}
	plainView = plain(m.View())
	if !strings.Contains(plainView, "skill-23") {
		t.Fatalf("usage modal did not scroll with j key:\n%s", plainView)
	}
}

func TestRepoUnlinkUsesSelectedRepoRowInsteadOfCursor(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	alphaArchive := makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")
	targetArchive := makeSkill(t, cfg.ArchiveSkillsRoot(), "target-skill", "Target.")
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	alphaUsage := filepath.Join(root, "alpha-skill")
	targetUsage := filepath.Join(root, "target-skill")
	if err := os.Symlink(alphaArchive, alphaUsage); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetArchive, targetUsage); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	view := plain(m.modal.View(120, 35, m))
	if !strings.Contains(view, "Unlink usages: target-skill") || strings.Contains(view, "alpha-skill") {
		t.Fatalf("unlink modal should target selected repo row, not cursor:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(targetUsage); !os.IsNotExist(err) {
		t.Fatalf("selected repo usage still exists or unexpected error: %v", err)
	}
	if _, err := os.Lstat(alphaUsage); err != nil {
		t.Fatalf("cursor repo usage should remain: %v", err)
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

func TestRepoDeleteUsesSelectedRepoRowInsteadOfCursor(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	alphaArchive := makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")
	targetArchive := makeSkill(t, cfg.ArchiveSkillsRoot(), "target-skill", "Target.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	view := plain(m.modal.View(120, 35, m))
	if !strings.Contains(view, "Delete archive: target-skill") || strings.Contains(view, "Delete archive: alpha-skill") {
		t.Fatalf("delete modal should target selected repo row, not cursor:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Stat(targetArchive); !os.IsNotExist(err) {
		t.Fatalf("selected repo archive still exists or unexpected error: %v", err)
	}
	if _, err := os.Stat(alphaArchive); err != nil {
		t.Fatalf("cursor repo archive should remain: %v", err)
	}
}

func TestRepoDeleteMultiSelectedNoVisibleUsagesUsesPluralDirectCopy(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "target-skill", "Target.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	view := plain(m.modal.View(120, 35, m))
	for _, want := range []string{
		"Delete archives: 2 selected repo skills",
		"Selected archives",
		"alpha-skill",
		"target-skill",
		"No visible usages in the current working set.",
		"Delete archives",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("batch delete modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "This archive is used in the current working set.") {
		t.Fatalf("batch delete without usages should not show usage warning:\n%s", view)
	}
	if strings.Contains(view, "Unlink visible usages, then delete") {
		t.Fatalf("batch delete without usages should use direct delete action:\n%s", view)
	}
}

func TestRepoDeleteMultiSelectedMixedVisibleUsagesPluralizesWarning(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "alpha-skill", "Alpha.")
	targetArchive := makeSkill(t, cfg.ArchiveSkillsRoot(), "target-skill", "Target.")
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetArchive, filepath.Join(root, "target-skill")); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	view := plain(m.modal.View(120, 35, m))
	for _, want := range []string{
		"Delete archives: 2 selected repo skills",
		"Selected archives",
		"alpha-skill",
		"target-skill",
		"One or more selected archives are used in the current working set.",
		"Visible usages",
		"Unlink visible usages, then delete archives",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("mixed batch delete modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "This archive is used in the current working set.") {
		t.Fatalf("mixed batch delete should use plural usage warning:\n%s", view)
	}
}

func TestRepoDeleteSkipsArchiveDeletionWhenUnlinkFails(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.active = []ActiveGroup{{
		Name: "zen-of-go",
		Members: []actions.ActiveSkill{{
			Name:   "zen-of-go",
			Path:   filepath.Join(cfg.MustActiveRoot("project", "agents"), "zen-of-go"),
			Root:   roots.ActiveRoot{Scope: config.ScopeProject, Target: config.TargetAgents},
			Status: actions.StatusManaged,
		}},
	}}

	m.applyRepoDelete("zen-of-go")

	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archive should remain after unlink failure: %v", err)
	}
	if m.modal == nil {
		t.Fatal("delete result modal is nil")
	}
	view := plain(m.modal.View(120, 30, m))
	if !strings.Contains(view, "skipped because unlink failed") {
		t.Fatalf("delete result missing skip message:\n%s", view)
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
	for _, want := range []string{"Doctor fixes", "broken symlink", "Built-in skills", "~Ag", "Archive only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor fix modal missing %q:\n%s", want, view)
		}
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("doctor fix returned nil command")
	}
	updated, _ = m.Update(cmd())
	m = mustModel(t, updated)
	if _, err := os.Lstat(broken); !os.IsNotExist(err) {
		t.Fatalf("broken symlink still exists or unexpected error: %v", err)
	}
}

func TestDoctorBuiltInFixModalDefaultsToGlobalAgentsAndCanChooseArchiveOnly(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.reload()
	m.view = ViewDoctor
	m.openDoctorFixModal()

	view := plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "[x] ~Ag") || !strings.Contains(view, "[ ] Archive only") {
		t.Fatalf("unexpected defaults:\n%s", view)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	view = plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "[x] Archive only") {
		t.Fatalf("archive-only option not selected:\n%s", view)
	}
}

func TestDoctorBuiltInFixRunsInCommandAndAppliesGenerationSafeResult(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.reload()
	m.view = ViewDoctor
	m.openDoctorFixModal()
	catalog, _ := builtin.List()
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), catalog[0].Name)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("Enter returned nil command")
	}
	if _, err := os.Lstat(archive); !os.IsNotExist(err) {
		t.Fatalf("filesystem mutated before command execution: %v", err)
	}
	msg := cmd()
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("command did not archive built-in: %v", err)
	}

	m.doctorFixToken++
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal != nil || m.status == "" {
		t.Fatalf("stale result applied: modal=%T status=%q", m.modal, m.status)
	}
}
