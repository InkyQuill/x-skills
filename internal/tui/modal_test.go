package tui

import (
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

	view := m.View()
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
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Detail: zen-of-go", "Canonical name", "Active members", "Debug"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail modal missing %q:\n%s", want, view)
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

	view := m.modal.View(120, 40, m)
	for _, want := range []string{"Archive conflict: zen-of-go", "Decision applies to the whole skill directory", "Files", "SKILL.md", "-description: Archived.", "+description: Active.", "k keep archive", "l save active"} {
		if !strings.Contains(view, want) {
			t.Fatalf("conflict modal missing %q:\n%s", want, view)
		}
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
