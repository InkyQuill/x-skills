package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestPlanRestoreBlocksFullRemovalsWhenDesiredSkillUnavailable(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(root.Path, "extra"), "extra")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "missing", Source: Source{Type: SourceArchive}, Fingerprint: "sha256:missing"}}}); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Unavailable) != 1 || plan.Unavailable[0].Name != "missing" {
		t.Fatalf("unavailable = %#v, want missing", plan.Unavailable)
	}
	if len(plan.Removals) != 0 {
		t.Fatalf("removals = %#v, want blocked", plan.Removals)
	}
	if !plan.RemovalsBlocked {
		t.Fatal("RemovalsBlocked = false, want true")
	}
}

func TestPlanRestoreScopesAdditionsAndFullRemovalsToExplicitProjectRoots(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	agents := restoreRoot(t, cfg, config.TargetAgents)
	codex := restoreRoot(t, cfg, config.TargetCodex)
	global := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal})[0]
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	makeRestoreSkill(t, filepath.Join(agents.Path, "extra"), "extra")
	makeRestoreSkill(t, filepath.Join(codex.Path, "untouched"), "untouched")
	makeRestoreSkill(t, filepath.Join(global.Path, "global-untouched"), "global-untouched")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}}}}); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{agents}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Additions) != 1 || plan.Additions[0].Path != filepath.Join(agents.Path, "wanted") {
		t.Fatalf("additions = %#v", plan.Additions)
	}
	if len(plan.Removals) != 1 || plan.Removals[0].Path != filepath.Join(agents.Path, "extra") || plan.Removals[0].Kind != ChangeMigrate {
		t.Fatalf("removals = %#v, want unmanaged migration in explicit root", plan.Removals)
	}
}

func TestApplyRestoreAddsLinksAndMigratesUnmanagedExtrasWithoutDeletingArchives(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	makeRestoreSkill(t, filepath.Join(root.Path, "extra"), "extra")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyRestore(context.Background(), cfg, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Additions) != 1 || len(result.Removals) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "extra")); err != nil {
		t.Fatalf("migrated archive missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root.Path, "extra")); !os.IsNotExist(err) {
		t.Fatalf("extra remains in destination: %v", err)
	}
	if target, err := filepath.EvalSymlinks(filepath.Join(root.Path, "wanted")); err != nil || target != filepath.Join(cfg.ArchiveSkillsRoot(), "wanted") {
		t.Fatalf("wanted link = %q, %v", target, err)
	}
}

func restoreRoot(t *testing.T, cfg config.Config, target string) roots.ActiveRoot {
	t.Helper()
	for _, root := range roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject, Target: target}) {
		return root
	}
	t.Fatal("project root not found")
	return roots.ActiveRoot{}
}

func makeRestoreSkill(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: restore test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
