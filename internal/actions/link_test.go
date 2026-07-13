package actions

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/pathidentity"
)

// assertSamePath verifies two paths identify the same filesystem location.
func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	ok, err := pathidentity.EquivalentE(got, want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("path = %q, want same location as %q", got, want)
	}
}

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
	if result.Status != "linked" {
		t.Fatalf("Status = %q, want linked", result.Status)
	}
	resolved, err := filepath.EvalSymlinks(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	assertSamePath(t, resolved, source)
}

func TestLinkAlreadyLinkedAbsoluteTargetIsNoOp(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	destination := filepath.Join(cfg.MustActiveRoot("project", "codex"), "typescript-expert")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archivePath, destination); err != nil {
		t.Fatal(err)
	}

	before, err := os.Lstat(destination)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "already_linked" {
		t.Fatalf("Status = %q, want already_linked", result.Status)
	}
	after, err := os.Lstat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("mod time changed from %v to %v", before.ModTime(), after.ModTime())
	}
	gotTarget, err := os.Readlink(destination)
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget != archivePath {
		t.Fatalf("target = %q, want unchanged %q", gotTarget, archivePath)
	}
}

func TestLinkAlreadyLinkedRelativeTargetIsNoOp(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	destination := filepath.Join(cfg.MustActiveRoot("project", "codex"), "typescript-expert")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	relativeTarget, err := filepath.Rel(filepath.Dir(destination), archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relativeTarget, destination); err != nil {
		t.Fatal(err)
	}

	result, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "already_linked" {
		t.Fatalf("Status = %q, want already_linked", result.Status)
	}
	gotTarget, err := os.Readlink(destination)
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget != relativeTarget {
		t.Fatalf("target = %q, want unchanged %q", gotTarget, relativeTarget)
	}
}

func TestLinkAlreadyLinkedPlatformEquivalentTargetIsNoOp(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	destination := filepath.Join(cfg.MustActiveRoot("project", "codex"), "typescript-expert")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	aliasRoot := filepath.Join(t.TempDir(), "archive-alias")
	if err := os.Symlink(cfg.ArchiveSkillsRoot(), aliasRoot); err != nil {
		t.Fatal(err)
	}
	equivalentTarget := filepath.Join(aliasRoot, "typescript-expert")
	if err := os.Symlink(equivalentTarget, destination); err != nil {
		t.Fatal(err)
	}

	result, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "already_linked" {
		t.Fatalf("Status = %q, want already_linked", result.Status)
	}
}

func TestLinkRepoSkillUsesConfiguredCustomRoot(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")

	result, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "opencode"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != filepath.Join(project, ".opencode", "skills", "typescript-expert") {
		t.Fatalf("Path = %q", result.Path)
	}
	resolved, err := filepath.EvalSymlinks(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	assertSamePath(t, resolved, source)
}

func TestLinkFailsWhenDestinationExists(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	makeSkill(t, cfg.MustActiveRoot("project", "codex"), "typescript-expert", "Existing.")

	_, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err == nil {
		t.Fatal("expected destination exists error")
	}
}

func TestLinkExistingDestinationsAreNotReplaced(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, destination string)
	}{
		{
			name: "wrong target link",
			setup: func(t *testing.T, destination string) {
				t.Helper()
				wrongTarget := makeSkill(t, filepath.Join(t.TempDir(), "archive"), "other", "Other.")
				if err := os.Symlink(wrongTarget, destination); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "regular file",
			setup: func(t *testing.T, destination string) {
				t.Helper()
				if err := os.WriteFile(destination, []byte("keep me"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "directory",
			setup: func(t *testing.T, destination string) {
				t.Helper()
				if err := os.Mkdir(destination, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(destination, "marker"), []byte("keep me"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			cfg := config.Default(project, home)
			makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
			destination := filepath.Join(cfg.MustActiveRoot("project", "codex"), "typescript-expert")
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			tt.setup(t, destination)
			before, err := os.Lstat(destination)
			if err != nil {
				t.Fatal(err)
			}
			beforeTarget := ""
			if before.Mode()&os.ModeSymlink != 0 {
				beforeTarget, err = os.Readlink(destination)
				if err != nil {
					t.Fatal(err)
				}
			}

			_, err = Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
			if err == nil || !strings.Contains(err.Error(), "destination exists") {
				t.Fatalf("error = %v, want destination exists", err)
			}
			after, statErr := os.Lstat(destination)
			if statErr != nil {
				t.Fatal(statErr)
			}
			if before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("destination metadata changed: before=%v after=%v", before, after)
			}
			if beforeTarget != "" {
				afterTarget, readErr := os.Readlink(destination)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if afterTarget != beforeTarget {
					t.Fatalf("target = %q, want unchanged %q", afterTarget, beforeTarget)
				}
			}
		})
	}
}

func TestLinkExistingLinkInspectionFailureDoesNotReplaceDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permissions do not provide a portable unreadable-directory fixture")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission inspection failure cannot be reproduced as root")
	}
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	destination := filepath.Join(cfg.MustActiveRoot("project", "codex"), "typescript-expert")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(locked, "skill")
	if err := os.Symlink(target, destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o700); err != nil {
			t.Errorf("restore locked directory permissions: %v", err)
		}
	})

	_, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err == nil {
		t.Fatal("expected link inspection error")
	}
	if !strings.Contains(err.Error(), "inspect destination") || strings.Contains(err.Error(), "destination exists") {
		t.Fatalf("error = %q, want link inspection failure", err)
	}
	gotTarget, readErr := os.Readlink(destination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if gotTarget != target {
		t.Fatalf("target = %q, want unchanged %q", gotTarget, target)
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

func TestLinkRejectsInvalidScopeAndTarget(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	_, err := Link(cfg, LinkRequest{Name: "missing", Scope: "workspace", Target: "codex"})
	if err == nil || !strings.Contains(err.Error(), `unknown scope "workspace"`) {
		t.Fatalf("invalid scope error = %v", err)
	}

	_, err = Link(cfg, LinkRequest{Name: "missing", Scope: "project", Target: "cursor"})
	if err == nil || !strings.Contains(err.Error(), `unknown target "cursor"`) {
		t.Fatalf("invalid target error = %v", err)
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
			if _, statErr := os.Stat(cfg.MustActiveRoot("project", "codex")); !os.IsNotExist(statErr) {
				t.Fatalf("active root stat error = %v, want not exist", statErr)
			}
		})
	}
}
