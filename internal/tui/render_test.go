package tui

import (
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	tea "github.com/charmbracelet/bubbletea"
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

func TestActiveViewRendersConfiguredRootLabel(t *testing.T) {
	cfg := customRootConfig(t)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, "opencode"), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 34

	view := plain(m.View())
	if !strings.Contains(view, ".Oc") {
		t.Fatalf("active view missing configured label:\n%s", view)
	}
	if strings.Contains(view, ".Ag") {
		t.Fatalf("active view should not show disabled built-in label:\n%s", view)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("detail modal is nil")
	}
	detail := plain(m.modal.View(120, 40, m))
	if !strings.Contains(detail, ".Oc") {
		t.Fatalf("active detail missing configured label:\n%s", detail)
	}
}

func TestHelpModalRendersConfiguredRootLabels(t *testing.T) {
	cfg := customRootConfig(t)
	m := New(cfg)
	view := plain(newHelpModal().View(120, 40, m))
	if !strings.Contains(view, ".Oc") || !strings.Contains(view, "project:opencode") {
		t.Fatalf("help modal missing configured root:\n%s", view)
	}
	if strings.Contains(view, ".Ag  project agents") || strings.Contains(view, ".Cl  project claude") {
		t.Fatalf("help modal shows stale built-in root inventory:\n%s", view)
	}
}

func TestStatusRowsDistinguishableWithoutColor(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	statuses := []string{actions.StatusManaged, actions.StatusUnmanaged, actions.StatusBroken}

	for _, mode := range []struct {
		name  string
		ascii bool
	}{
		{name: "unicode", ascii: false},
		{name: "ASCII", ascii: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			rows := make(map[string]string, len(statuses))
			for _, status := range statuses {
				m := New(cfg, Options{ASCII: mode.ascii})
				m.width = 120
				m.height = 30
				m.active = []ActiveGroup{{
					ID:          "active:same-skill",
					Name:        "same-skill",
					Status:      status,
					Description: "Same description.",
					Chips:       []string{".Ag"},
				}}
				rendered := renderActiveRows(m, 80)
				if len(rendered) != 1 {
					t.Fatalf("renderActiveRows() returned %d rows, want 1", len(rendered))
				}
				rows[status] = plain(rendered[0])
			}
			for i, left := range statuses {
				for _, right := range statuses[i+1:] {
					if rows[left] == rows[right] {
						t.Fatalf("%s and %s rows are identical without color:\n%q", left, right, rows[left])
					}
				}
			}
		})
	}
}

func TestActiveInspectorShowsBrokenReason(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 120
	m.height = 30
	m.active = []ActiveGroup{
		{
			ID:     "active:broken-skill",
			Name:   "broken-skill",
			Status: actions.StatusBroken,
			Reason: "symlink target missing",
		},
	}

	view := plain(m.View())
	for _, want := range []string{"Inspector", "broken-skill", "Reason", "symlink target missing"} {
		if !strings.Contains(view, want) {
			t.Fatalf("active inspector missing %q:\n%s", want, view)
		}
	}
}

func TestActiveInspectorUsesKeyValueRows(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 120
	m.height = 30
	m.active = []ActiveGroup{
		{
			ID:          "active:zen-of-go",
			Name:        "zen-of-go",
			Aliases:     []string{"go", "pro"},
			Status:      actions.StatusManaged,
			Description: "Go style.",
			Chips:       []string{".Ag", "~Cl"},
		},
	}

	view := plain(m.View())
	for _, want := range []string{
		"Inspector",
		"zen-of-go",
		"Aliases",
		"go, pro",
		"Repo/Status",
		"managed",
		"Description",
		"Go style.",
		"Locations",
		".Ag",
		"~Cl",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("active inspector missing %q:\n%s", want, view)
		}
	}
}

func TestRepoInspectorUsesKeyValueRowsAndUsageChips(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 30
	m.repoUsage["zen-of-go"] = []string{".Ag", "~Cl"}
	m.setView(ViewRepo)

	view := plain(m.View())
	for _, want := range []string{
		"Inspector",
		"zen-of-go",
		"Description",
		"Go style.",
		"Usages",
		".Ag",
		"~Cl",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("repo inspector missing %q:\n%s", want, view)
		}
	}

	raw := m.View()
	for _, want := range []string{".Ag", "~Cl"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("repo inspector missing rich usage chip %q:\n%s", want, raw)
		}
	}
}

func TestDoctorInspectorUsesKeyValueRows(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 120
	m.height = 30
	m.issues = []doctor.Issue{
		{
			Kind:     doctor.KindBrokenSymlink,
			Name:     "zen-of-go",
			Location: ".Ag",
			Path:     "/tmp/zen",
			Reason:   "missing target",
			SafeFix:  "unlink stale",
		},
	}
	m.setView(ViewDoctor)

	view := plain(m.View())
	for _, want := range []string{
		"Inspector",
		string(doctor.KindBrokenSymlink),
		"Path",
		"/tmp/zen",
		"Reason",
		"missing target",
		"Fix",
		"unlink stale",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor inspector missing %q:\n%s", want, view)
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
	want := "↵ details  / filter  p preview  l link  r recommend  u unlink  d delete  c clear  ^R refresh"
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
