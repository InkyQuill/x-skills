package tui

import (
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
)

func TestWideShellRendersListInspectorStatusAndFooter(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 34
	m.status = "relinked zen-of-go to existing archive"

	view := plain(m.View())
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

	view := plain(m.View())
	if strings.Contains(view, "Inspector") {
		t.Fatalf("narrow shell should not show inspector:\n%s", view)
	}
	if !strings.Contains(view, "Active skills") || !strings.Contains(view, "^R refresh") {
		t.Fatalf("narrow shell missing list/footer:\n%s", view)
	}
}

func TestRepoInspectorDoesNotStretchToBodyHeight(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 24
	m.setView(ViewRepo)

	view := plain(m.View())
	if !strings.Contains(view, "Repo skills") || !strings.Contains(view, "Inspector") || !strings.Contains(view, "^R refresh") {
		t.Fatalf("repo shell missing expected regions:\n%s", view)
	}
	if strings.Contains(view, "┘ └") {
		t.Fatalf("inspector appears stretched to list height; want inspector bottom above list bottom:\n%s", view)
	}
}

func TestRepoFooterShowsRepoActions(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 24
	m.setView(ViewRepo)

	view := plain(m.View())
	want := "↵ details  / filter  p preview  l link  u unlink  d delete  c clear  ^R refresh"
	if !strings.Contains(view, want) {
		t.Fatalf("repo footer missing repo actions:\n%s", view)
	}
}

func TestDoctorInspectorDoesNotStretchToBodyHeight(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 24
	m.issues = append(m.issues, doctor.Issue{Kind: doctor.KindBrokenSymlink, Name: "zen-of-go", Location: ".Ag", Reason: "missing"})
	m.setView(ViewDoctor)

	view := plain(m.View())
	if !strings.Contains(view, "Doctor issues") || !strings.Contains(view, "Inspector") || !strings.Contains(view, "^R refresh") {
		t.Fatalf("doctor shell missing expected regions:\n%s", view)
	}
	if strings.Contains(view, "┘ └") {
		t.Fatalf("inspector appears stretched to list height; want inspector bottom above list bottom:\n%s", view)
	}
}
