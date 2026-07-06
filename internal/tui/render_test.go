package tui

import (
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestWideShellRendersListInspectorStatusAndFooter(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 34
	m.status = "relinked zen-of-go to existing archive"

	view := m.View()
	for _, want := range []string{"A:Active", "R:Repo", "D:Doctor", "Active skills", "Inspector", "zen-of-go", "relinked zen-of-go", "^R refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wide shell missing %q:\n%s", want, view)
		}
	}
}

func TestNarrowShellCollapsesInspector(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 80
	m.height = 24

	view := m.View()
	if strings.Contains(view, "Inspector") {
		t.Fatalf("narrow shell should not show inspector:\n%s", view)
	}
	if !strings.Contains(view, "Active skills") || !strings.Contains(view, "^R refresh") {
		t.Fatalf("narrow shell missing list/footer:\n%s", view)
	}
}
