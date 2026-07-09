package actions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func TestMigrateMovesDirectoryToRepoAndLinksBack(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "next-best-practices", "Next.")

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
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "local-only", "Local.")

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

func TestMigrateRejectsInvalidScopeAndTarget(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	_, err := Migrate(cfg, MigrateRequest{Name: "local-only", Scope: "workspace", Target: "codex", Confirmed: true})
	if err == nil || !strings.Contains(err.Error(), `unknown scope "workspace"`) {
		t.Fatalf("invalid scope error = %v", err)
	}

	_, err = Migrate(cfg, MigrateRequest{Name: "local-only", Scope: "project", Target: "cursor", Confirmed: true})
	if err == nil || !strings.Contains(err.Error(), `unknown target "cursor"`) {
		t.Fatalf("invalid target error = %v", err)
	}
}

func TestMigrateRelinksWhenArchiveDestinationHasSameFingerprint(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "local-only", "Local.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "local-only", "Archived.")
	if err := os.WriteFile(filepath.Join(archived, "SKILL.md"), []byte("---\nname: local-only\ndescription: Local.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Migrate(cfg, MigrateRequest{Name: "local-only", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ResultRelinked {
		t.Fatalf("Status = %q, want %q", result.Status, ResultRelinked)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
}

func TestMigrateReturnsConflictWhenArchiveDestinationDiffers(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "local-only", "Local.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "local-only", "Archived.")

	_, err := Migrate(cfg, MigrateRequest{Name: "local-only", Scope: "project", Target: "codex", Confirmed: true})
	if err == nil {
		t.Fatal("expected archive conflict error")
	}
	var conflict *ArchiveConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %[1]v, want ArchiveConflictError", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateConflictUseActiveReplacesArchive(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "local-only", "Local.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "local-only", "Archived.")

	_, err := Migrate(cfg, MigrateRequest{
		Name:               "local-only",
		Scope:              "project",
		Target:             "codex",
		Confirmed:          true,
		ConflictResolution: ConflictResolutionUseActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := skills.Read(archived)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Local." {
		t.Fatalf("Description = %q, want Local.", info.Description)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
}

func TestUnlinkManagedRemovesSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")
	root := cfg.MustActiveRoot("project", "codex")
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

func TestUnlinkBrokenSymlinkRemovesSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "missing-target")
	if err := os.Symlink(filepath.Join(home, "missing"), active); err != nil {
		t.Fatal(err)
	}

	_, err := Unlink(cfg, UnlinkRequest{Name: "missing-target", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
}

func TestUnlinkRejectsInvalidScopeAndTarget(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	_, err := Unlink(cfg, UnlinkRequest{Name: "local-only", Scope: "workspace", Target: "codex", Confirmed: true})
	if err == nil || !strings.Contains(err.Error(), `unknown scope "workspace"`) {
		t.Fatalf("invalid scope error = %v", err)
	}

	_, err = Unlink(cfg, UnlinkRequest{Name: "local-only", Scope: "project", Target: "cursor", Confirmed: true})
	if err == nil || !strings.Contains(err.Error(), `unknown target "cursor"`) {
		t.Fatalf("invalid target error = %v", err)
	}
}

func TestUnlinkUnmanagedExternalSymlinkWithoutDeleteArchivesTarget(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, filepath.Join(home, "external"), "external-only", "External.")
	root := cfg.MustActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "external-only")
	if err := os.Symlink(source, active); err != nil {
		t.Fatal(err)
	}

	result, err := Unlink(cfg, UnlinkRequest{Name: "external-only", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ResultMigratedUnlinked {
		t.Fatalf("Status = %q, want %q", result.Status, ResultMigratedUnlinked)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "external-only")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active link still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}

func TestUnlinkUnmanagedExternalSymlinkPreservesMatchingArchive(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, filepath.Join(home, "external"), "external-only", "External.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "external-only", "External.")
	root := cfg.MustActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "external-only")
	if err := os.Symlink(source, active); err != nil {
		t.Fatal(err)
	}

	result, err := Unlink(cfg, UnlinkRequest{Name: "external-only", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ResultMigratedUnlinked {
		t.Fatalf("Status = %q, want %q", result.Status, ResultMigratedUnlinked)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archive was not preserved: %v", err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active link still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("external source was touched: %v", err)
	}
}

func TestUnlinkUnmanagedExternalSymlinkConflictCanUseActive(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, filepath.Join(home, "external"), "external-only", "External.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "external-only", "Archived.")
	root := cfg.MustActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "external-only")
	if err := os.Symlink(source, active); err != nil {
		t.Fatal(err)
	}

	_, err := Unlink(cfg, UnlinkRequest{Name: "external-only", Scope: "project", Target: "codex", Confirmed: true})
	var conflict *ArchiveConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %[1]v, want ArchiveConflictError", err)
	}
	if conflict.Name != "external-only" {
		t.Fatalf("conflict name = %q, want external-only", conflict.Name)
	}
	if _, err := os.Lstat(active); err != nil {
		t.Fatalf("active link should remain until conflict is resolved: %v", err)
	}

	result, err := Unlink(cfg, UnlinkRequest{
		Name:               "external-only",
		Scope:              "project",
		Target:             "codex",
		Confirmed:          true,
		ConflictResolution: ConflictResolutionUseActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ResultMigratedUnlinked {
		t.Fatalf("Status = %q, want %q", result.Status, ResultMigratedUnlinked)
	}
	info, err := skills.Read(archived)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "External." {
		t.Fatalf("archive description = %q, want External.", info.Description)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active link still exists or unexpected err: %v", err)
	}
}

func TestUnlinkUnmanagedSymlinkArchivesResolvedSkillAndRemovesOnlyLink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.MustActiveRoot("global", "agents"), "code-review", "Review.")
	claudeRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(claudeRoot, "code-review")
	if err := os.Symlink(source, alias); err != nil {
		t.Fatal(err)
	}

	result, err := Unlink(cfg, UnlinkRequest{Name: "code-review", Scope: "global", Target: "claude", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(cfg.ArchiveSkillsRoot(), "code-review")
	if result.Path != archived {
		t.Fatalf("Path = %q, want %q", result.Path, archived)
	}
	if result.Status != ResultMigratedUnlinked {
		t.Fatalf("Status = %q, want %q", result.Status, ResultMigratedUnlinked)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(alias); !os.IsNotExist(err) {
		t.Fatalf("alias still exists or unexpected err: %v", err)
	}
}

func TestUnlinkUnmanagedExternalSymlinkWithDeleteRemovesOnlySymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, filepath.Join(home, "external"), "external-only", "External.")
	root := cfg.MustActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "external-only")
	if err := os.Symlink(source, active); err != nil {
		t.Fatal(err)
	}

	result, err := Unlink(cfg, UnlinkRequest{Name: "external-only", Scope: "project", Target: "codex", DeleteUnmanaged: true, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "removed unmanaged symlink" {
		t.Fatalf("Status = %q, want removed unmanaged symlink", result.Status)
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
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "local-only", "Local.")

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
	active := makeSkill(t, cfg.MustActiveRoot("project", "codex"), "local-only", "Local.")

	_, err := Unlink(cfg, UnlinkRequest{Name: "local-only", Scope: "project", Target: "codex", DeleteUnmanaged: true, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
}
