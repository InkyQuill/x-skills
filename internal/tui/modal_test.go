package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
)

func TestModalConsumesBackgroundKeys(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.modal = newResultModal("Done", []string{"ok"})

	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active while modal is open", m.view)
	}
	if m.modal == nil {
		t.Fatal("modal closed unexpectedly")
	}
}

func TestEscClosesModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.modal = newResultModal("Done", []string{"ok"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("modal = %#v, want nil", m.modal)
	}
}

func TestModalRendersOverShellWithoutRemovingFooter(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 100
	m.height = 30
	m.modal = newResultModal("Migration Results", []string{"2 succeeded"})

	view := plain(m.View())
	if !strings.Contains(view, "Migration Results") {
		t.Fatalf("view missing modal:\n%s", view)
	}
	if !strings.Contains(view, "^R refresh") {
		t.Fatalf("view missing footer shortcuts:\n%s", view)
	}
	if got := strings.Count(view, "\n") + 1; got != m.height {
		t.Fatalf("view height = %d, want %d:\n%s", got, m.height, view)
	}
}

func TestEnterOpensActiveDetailModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"Detail: zen-of-go", "Canonical name", "Active members", "Debug"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail modal missing %q:\n%s", want, view)
		}
	}
}

func TestEnterOpensRepoDetailModal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	active := makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), "zen-of-go"), active); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"Detail: zen-of-go (Repo)", "Archive path", cfg.ArchiveSkillsRoot(), "Description", "Go style.", "Usages", ".Ag"} {
		if !strings.Contains(view, want) {
			t.Fatalf("repo detail modal missing %q:\n%s", want, view)
		}
	}
}

func TestEnterOpensDoctorDetailModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(root, "zen-of-go")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), brokenPath); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"Detail: zen-of-go (Doctor)", "Issue kind", "broken-symlink", "Affected path", brokenPath, "Reason", "Safe fix"} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor detail modal missing %q:\n%s", want, view)
		}
	}
}

func TestQuestionMarkOpensHelpModalWithGlobalKeys(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("?"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Help", "A", "R", "D", "I", "^R", ".Ag", "~Cd"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help modal missing %q:\n%s", want, view)
		}
	}
}

func TestChoiceAndConfirmModalsHighlightSelectedControls(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	choice := newChoiceModal("Choose", nil, []string{"One", "Two"}, 0, func(*Model, int) {})
	choiceView := plain(choice.View(100, 30, m))
	if !strings.Contains(choiceView, "› One") {
		t.Fatalf("choice modal missing selected option:\n%s", choiceView)
	}
	if colorAvailableForTest() && !selectedBackgroundConfigured() {
		t.Fatal("choice modal selected background style is not configured")
	}

	confirm := newConfirmModal("Confirm", nil, false, func(*Model) {})
	confirmView := plain(confirm.View(100, 30, m))
	if !strings.Contains(confirmView, "[ Apply ]") {
		t.Fatalf("confirm modal missing selected button:\n%s", confirmView)
	}
	if colorAvailableForTest() && !selectedBackgroundConfigured() {
		t.Fatal("confirm modal selected background style is not configured")
	}
}

func TestPreviewModalTogglesRawAndRendered(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("p"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	rendered := m.modal.View(100, 30, m)
	if !strings.Contains(rendered, "rendered with Glamour") {
		t.Fatalf("preview missing rendered marker:\n%s", rendered)
	}

	updated, _ = m.Update(keyRunes("r"))
	m = mustModel(t, updated)
	raw := m.modal.View(100, 30, m)
	if !strings.Contains(raw, "raw SKILL.md") {
		t.Fatalf("preview missing raw marker:\n%s", raw)
	}
}

func TestPreviewModalScrollsRawContent(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "long-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: long-skill\n---\n# Title\nline one\nline two\nline three\nline four\nline five\nline six\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("long-skill", skill)

	updated, _ := m.Update(keyRunes("r"))
	m = mustModel(t, updated)
	before := plain(m.modal.View(100, 16, m))
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	after := plain(m.modal.View(100, 16, m))
	if before == after {
		t.Fatalf("down key did not scroll preview:\n%s", after)
	}
	if strings.Contains(after, "---\nname: long-skill") {
		t.Fatalf("preview did not advance past first raw lines:\n%s", after)
	}
}

func TestConflictModalShowsFileListAndDiff(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "zen-of-go", "Active.")
	makeSkill(t, archive, "zen-of-go", "Archived.")
	diff, err := buildDirectoryDiff(filepath.Join(active, "zen-of-go"), filepath.Join(archive, "zen-of-go"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(string) {})

	view := plain(m.modal.View(120, 40, m))
	for _, want := range []string{"Archive conflict: zen-of-go", "Decision applies to the whole skill directory", "Legend:", "Archive", "Incoming active", "Files", "SKILL.md", "Archive   description: Archived.", "Incoming  description: Active.", "k keep archive", "l save active"} {
		if !strings.Contains(view, want) {
			t.Fatalf("conflict modal missing %q:\n%s", want, view)
		}
	}
}

func TestConflictModalScrollsDiffBody(t *testing.T) {
	var lines []string
	lines = append(lines, "--- archive", "+++ active")
	for i := 0; i < 40; i++ {
		lines = append(lines, " line "+string(rune('A'+i%26)))
	}
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: strings.Join(lines, "\n")}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(string) {})
	m.width = 120
	m.height = 20

	before := plain(m.modal.View(120, 20, m))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	after := plain(m.modal.View(120, 20, m))
	if before == after {
		t.Fatalf("down key did not scroll diff body:\n%s", after)
	}
	if !strings.Contains(after, "lines 2-") {
		t.Fatalf("scroll position not updated:\n%s", after)
	}
}

func TestConflictModalAppliesKeepArchiveKey(t *testing.T) {
	called := ""
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(resolution string) {
		called = resolution
	})

	updated, _ := m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if called != actions.ConflictResolutionKeepArchive {
		t.Fatalf("called = %q, want keep archive", called)
	}
	if m.modal != nil {
		t.Fatal("modal still open after resolution")
	}
}
