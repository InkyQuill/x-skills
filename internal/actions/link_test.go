package actions

import (
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestLinkRepoSkillCreatesSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")

	result, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != filepath.Join(project, ".codex", "skills", "typescript-expert") {
		t.Fatalf("Path = %q", result.Path)
	}
	resolved, err := filepath.EvalSymlinks(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != source {
		t.Fatalf("resolved = %q, want %q", resolved, source)
	}
}

func TestLinkFailsWhenDestinationExists(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	makeSkill(t, cfg.ActiveRoot("project", "codex"), "typescript-expert", "Existing.")

	_, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err == nil {
		t.Fatal("expected destination exists error")
	}
}
