package actions

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLinkFailsWhenRepoSkillMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)

	_, err := Link(cfg, LinkRequest{Name: "missing", Scope: "project", Target: "codex"})
	if err == nil {
		t.Fatal("expected repo skill not found error")
	}
	if !strings.Contains(err.Error(), "repo skill") {
		t.Fatalf("error = %q, want repo skill", err)
	}
}

func TestLinkRejectsPathLikeSkillNames(t *testing.T) {
	for _, name := range []string{"", "../outside", "/absolute", ".", "..", "nested/name", `nested\name`} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			cfg := config.Default(project, home)

			_, err := Link(cfg, LinkRequest{Name: name, Scope: "project", Target: "codex"})
			if err == nil {
				t.Fatal("expected invalid skill name error")
			}
			if !strings.Contains(err.Error(), "invalid skill name") {
				t.Fatalf("error = %q, want invalid skill name", err)
			}
			if _, statErr := os.Stat(cfg.ActiveRoot("project", "codex")); !os.IsNotExist(statErr) {
				t.Fatalf("active root stat error = %v, want not exist", statErr)
			}
		})
	}
}
