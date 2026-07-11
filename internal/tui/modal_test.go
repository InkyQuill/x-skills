package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func TestConstrainedModalKeepsFooterVisibleWithLongBody(t *testing.T) {
	body := make([]string, 30)
	for i := range body {
		body[i] = "body line"
	}
	view := plain(renderConstrainedModal(80, 12, constrainedModalOptions{
		Title:     "Long modal",
		Body:      body,
		Footer:    []string{"footer commands"},
		Scroll:    20,
		UseScroll: true,
	}))

	lines := strings.Split(view, "\n")
	if len(lines) > 12 {
		t.Fatalf("modal height = %d, want <= 12:\n%s", len(lines), view)
	}
	if !strings.Contains(view, "Long modal") || !strings.Contains(view, "footer commands") {
		t.Fatalf("constrained modal lost title/footer:\n%s", view)
	}
	if !strings.Contains(view, "↑ more") || !strings.Contains(view, "↓ more") {
		t.Fatalf("constrained modal missing scroll indicators:\n%s", view)
	}
}

func TestVisibleModalBodyClampsNegativeScroll(t *testing.T) {
	got := visibleModalBody([]string{"one", "two", "three"}, 2, 0, -1, true)
	if len(got) != 2 {
		t.Fatalf("visibleModalBody() returned %d lines, want 2", len(got))
	}
}

func TestScrollableModalMovesWithJK(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i)
	}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.width = 80
	m.height = 12
	m.modal = newResultModal("Long result", lines)

	for i := 0; i < 10; i++ {
		updated, _ := m.Update(keyRunes("j"))
		m = mustModel(t, updated)
	}
	view := plain(m.View())
	if !strings.Contains(view, "line-11") || strings.Contains(view, "line-00") {
		t.Fatalf("result modal did not scroll with j:\n%s", view)
	}

	updated, _ := m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	view = plain(m.View())
	if !strings.Contains(view, "line-10") {
		t.Fatalf("result modal did not scroll with k:\n%s", view)
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
	for _, want := range []string{
		"Help",
		"A",
		"R",
		"D",
		"switch to Install view",
		"Install: / search",
		"Install: i install and use",
		"Install: a archive only",
		"^R",
		"toggle Active/Repo row selection",
		"clear Active/Repo selection",
		"↓ more",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("help modal missing %q:\n%s", want, view)
		}
	}
	for i := 0; i < 20; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	view = m.modal.View(100, 30, m)
	for _, want := range []string{".Ag", "~Cd"} {
		if !strings.Contains(view, want) {
			t.Fatalf("scrolled help modal missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"reserved for " + "Install view", "toggle row selection", "clear selection"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("help modal contains unscoped selection label %q:\n%s", unwanted, view)
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

func TestPreviewModalRenderedHidesFrontmatterAndShowsBody(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "focused-preview")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"---",
		"name: focused-preview",
		"description: Metadata description should stay out of rendered preview.",
		"---",
		"# Focused Preview",
		"",
		"Use this skill when installing quality checks.",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("focused-preview", skill)

	view := plain(m.modal.View(100, 30, m))
	for _, unexpected := range []string{"name: focused-preview", "description:"} {
		if strings.Contains(view, unexpected) {
			t.Fatalf("rendered preview should hide frontmatter %q:\n%s", unexpected, view)
		}
	}
	for _, want := range []string{"Metadata description", "Focused Preview", "installing quality checks"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rendered preview missing body content %q:\n%s", want, view)
		}
	}
}

func TestPreviewModalRenderedUsesFoldedBlockDescription(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "folded-description")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"---",
		"name: folded-description",
		"description: >",
		"  Folded description line",
		"  continues with useful detail.",
		"---",
		"# Folded Body",
		"",
		"Body content remains visible.",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("folded-description", skill)

	view := plain(m.modal.View(120, 30, m))
	for _, want := range []string{"Folded description line", "continues with useful detail", "Folded Body", "Body content remains visible"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rendered preview missing folded description content %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "description:") || strings.Contains(view, " > ") {
		t.Fatalf("rendered preview leaked raw folded frontmatter:\n%s", view)
	}
}

func TestPreviewModalRenderedKeepsIndentedDelimiterInBlockDescription(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "block-description")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"---",
		"name: block-description",
		"description: |",
		"  Intro line before a YAML-looking marker.",
		"  ---",
		"  Still part of the description.",
		"---",
		"# Block Body",
		"",
		"Body content follows the real delimiter.",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("block-description", skill)

	view := plain(m.modal.View(120, 30, m))
	for _, want := range []string{"Intro line before", "Still part of the description", "Block Body", "Body content follows"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rendered preview missing block description content %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "description:") {
		t.Fatalf("rendered preview leaked raw block frontmatter:\n%s", view)
	}
}

func TestPreviewModalRawShowsFrontmatter(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "raw-preview")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: raw-preview\ndescription: Raw metadata remains visible.\n---\n# Raw Preview\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("raw-preview", skill)

	updated, _ := m.Update(keyRunes("r"))
	m = mustModel(t, updated)
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"raw SKILL.md", "name: raw-preview", "Raw metadata remains visible"} {
		if !strings.Contains(view, want) {
			t.Fatalf("raw preview missing %q:\n%s", want, view)
		}
	}
}

func TestPreviewModalCompactsLongPathHeader(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	longParent := filepath.Join(root, "deeply", "nested", "path", "that", "should", "not", "be", "shown")
	skillName := "extremely-long-preview-skill-name-that-still-has-readable-body"
	skill := filepath.Join(longParent, skillName)
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: compact-header\n---\n# Body Starts Here\nReadable install details stay visible.\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("Preview: "+strings.Repeat("long-title-", 12), skill)

	const width = 72
	view := plain(m.modal.View(width, 16, m))
	if strings.Contains(view, root) || strings.Contains(view, "deeply/nested/path") {
		t.Fatalf("preview header should not show full path:\n%s", view)
	}
	if !strings.Contains(view, "Body Starts Here") || !strings.Contains(view, "Readable install details") {
		t.Fatalf("long header consumed preview body:\n%s", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if gotWidth := lipgloss.Width(line); gotWidth > width {
			t.Fatalf("line %d width = %d, want <= %d for %q:\n%s", i, gotWidth, width, line, view)
		}
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

func TestPreviewModalScrollStateStaysBounded(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "short-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: short-skill\n---\n# Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("short-skill", skill)

	for i := 0; i < 100; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	before := plain(m.modal.View(100, 16, m))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	after := plain(m.modal.View(100, 16, m))
	if before != after {
		t.Fatalf("preview should stay stable after scrolling past end:\nbefore:\n%s\nafter:\n%s", before, after)
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

func TestConflictModalSupportsIncomingRemoteLabel(t *testing.T) {
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "--- archive\n+++ active\n-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModalWithIncomingLabel("zen-of-go", diff, "Incoming remote", func(string) {})

	view := plain(m.modal.View(120, 40, m))
	for _, want := range []string{"Legend:", "Archive", "Incoming remote"} {
		if !strings.Contains(view, want) {
			t.Fatalf("conflict modal missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "l use incoming") {
		t.Fatalf("remote conflict modal should say use incoming:\n%s", view)
	}
	if strings.Contains(view, "Incoming active") {
		t.Fatalf("remote conflict modal should not say Incoming active:\n%s", view)
	}
	if strings.Contains(view, "save active") {
		t.Fatalf("remote conflict modal should not say save active:\n%s", view)
	}
}

func TestConflictModalPromptsResizeWhenTooSmall(t *testing.T) {
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(string) {})

	view := plain(m.modal.View(50, 12, m))
	for _, want := range []string{"Archive conflict: zen-of-go", "Terminal too small", "resize", "Esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("small diff modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "SKILL.md") {
		t.Fatalf("small diff modal should not squeeze diff content:\n%s", view)
	}
}

func TestConflictModalIgnoresResolutionKeysWhenTooSmall(t *testing.T) {
	calls := []string{}
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.width = 50
	m.height = 12
	m.modal = newConflictDiffModal("zen-of-go", diff, func(resolution string) {
		calls = append(calls, resolution)
	})

	updated, _ := m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if len(calls) != 0 {
		t.Fatalf("apply called after k: %v", calls)
	}
	if m.modal == nil {
		t.Fatal("modal closed after k")
	}

	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if len(calls) != 0 {
		t.Fatalf("apply called after l: %v", calls)
	}
	if m.modal == nil {
		t.Fatal("modal closed after l")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("modal did not close after Esc")
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
	m.width = 120
	m.height = 40
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
