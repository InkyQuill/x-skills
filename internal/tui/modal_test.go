package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
}
