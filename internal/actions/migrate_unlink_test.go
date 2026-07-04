package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestMigrateMovesDirectoryToRepoAndLinksBack(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.ActiveRoot("project", "codex"), "next-best-practices", "Next.")

	result, err := Migrate(cfg, MigrateRequest{Name: "next-best-practices", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(cfg.ArchiveSkillsRoot(), "next-best-practices")
	if result.Path != archived {
		t.Fatalf("Path = %q, want %q", result.Path, archived)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
}

func TestMigrateRequiresConfirmationBeforeMoving(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.ActiveRoot("project", "codex"), "local-only", "Local.")

	_, err := Migrate(cfg, MigrateRequest{Name: "local-only", Scope: "project", Target: "codex"})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "local-only")); !os.IsNotExist(err) {
		t.Fatalf("archive stat error = %v, want not exist", err)
	}
}

func TestUnlinkManagedRemovesSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")
	root := cfg.ActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "opentui-react")
	if err := os.Symlink(source, active); err != nil {
		t.Fatal(err)
	}

	_, err := Unlink(cfg, UnlinkRequest{Name: "opentui-react", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}

func TestUnlinkUnmanagedMigratesToRepoWithoutActiveCopy(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.ActiveRoot("project", "codex"), "local-only", "Local.")

	result, err := Unlink(cfg, UnlinkRequest{Name: "local-only", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(cfg.ArchiveSkillsRoot(), "local-only")
	if result.Path != archived {
		t.Fatalf("Path = %q, want %q", result.Path, archived)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
}

func TestUnlinkUnmanagedDeleteRemovesActiveDirectory(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.ActiveRoot("project", "codex"), "local-only", "Local.")

	_, err := Unlink(cfg, UnlinkRequest{Name: "local-only", Scope: "project", Target: "codex", DeleteUnmanaged: true, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
}
