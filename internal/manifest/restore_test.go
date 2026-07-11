package manifest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestPlanRestoreRejectsExistingArchiveWithWrongIdentity(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "wanted")
	makeRestoreSkill(t, archive, "wanted")
	if err := remote.WriteSourceMetadata(archive, remote.SourceMetadata{SourceType: remote.SourceTypeGit, CloneURL: "wrong", SkillPath: "wrong"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecommended(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceGit, Repository: "expected", Path: "skills/wanted", Ref: "main"}}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Unavailable) != 1 || !strings.Contains(plan.Unavailable[0].Reason, "identity") {
		t.Fatalf("unavailable = %#v", plan.Unavailable)
	}
}

func TestPlanRestoreRejectsArchiveFingerprintMismatch(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Unavailable) != 1 || !strings.Contains(plan.Unavailable[0].Reason, "fingerprint") {
		t.Fatalf("unavailable = %#v", plan.Unavailable)
	}
}

func TestPlanRestoreRejectsArchiveSourceWithoutFingerprint(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	contents := "version: 1\nskills:\n  - name: wanted\n    source: {type: archive}\n"
	if err := os.WriteFile(filepath.Join(project, LocalFilename), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}})
	if err == nil || !strings.Contains(err.Error(), "archive source requires a content fingerprint") {
		t.Fatalf("PlanRestore() error = %v, want missing fingerprint rejection", err)
	}
}

func TestPlanRestoreClassifiesBrokenExtraAsRemoval(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	if err := os.MkdirAll(root.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(project, "missing"), filepath.Join(root.Path, "broken")); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Removals) != 1 || plan.Removals[0].Kind != ChangeRemove {
		t.Fatalf("removals = %#v", plan.Removals)
	}
}

func TestRestorePlanCloseDiscardsStaging(t *testing.T) {
	plan := RestorePlan{checkoutRoot: t.TempDir()}
	path := plan.checkoutRoot
	if err := plan.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("staging remains: %v", err)
	}
}

func TestPlanRestoreExposesMigrationConflictWithoutMutation(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(root.Path, "extra"), "extra")
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "extra"), "extra")
	if err := os.WriteFile(filepath.Join(root.Path, "extra", "active"), []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := fingerprint.Directory(filepath.Join(root.Path, "extra"))
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Conflicts) != 1 || plan.Removals[0].ArchiveName != "" {
		t.Fatalf("plan = %#v", plan)
	}
	after, _ := fingerprint.Directory(filepath.Join(root.Path, "extra"))
	if before != after {
		t.Fatal("planning mutated unmanaged extra")
	}
}

func TestPlanRestoreNormalizesDivergentDesiredDestination(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	makeRestoreSkill(t, filepath.Join(root.Path, "wanted"), "wanted")
	if err := os.WriteFile(filepath.Join(root.Path, "wanted", "local"), []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "wanted")}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Normalizations) != 1 || len(plan.Additions) != 1 || len(plan.Conflicts) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanAndApplyRestoreFetchRemoteSkill(t *testing.T) {
	project, home, upstream := t.TempDir(), t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(upstream, "skills", "remote-skill"), "remote-skill")
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}, {"add", "."}, {"commit", "-m", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = upstream
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := WriteRecommended(project, Manifest{Version: 1, Skills: []Skill{{Name: "remote-skill", Source: Source{Type: SourceGit, Repository: upstream, Path: "skills/remote-skill"}}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Available) != 1 || !plan.Available[0].NeedsArchive {
		t.Fatalf("available = %#v", plan.Available)
	}
	if _, err := ApplyRestore(context.Background(), cfg, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "remote-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRestoreUsesResolvedPreserveName(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(root.Path, "extra"), "extra")
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "extra"), "extra")
	if err := os.WriteFile(filepath.Join(root.Path, "extra", "unique"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	plan.Removals[0].ArchiveName = plan.Conflicts[0].SuggestedName
	name := plan.Removals[0].ArchiveName
	if _, err := ApplyRestore(context.Background(), cfg, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), name, "unique")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "extra", "SKILL.md")); err != nil {
		t.Fatal("existing archive was mutated")
	}
}

func TestApplyRestoreKeepsSafeAdditionsWhenRemovalsBlocked(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "available"), "available")
	makeRestoreSkill(t, filepath.Join(root.Path, "extra"), "extra")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "available", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "available")}, {Name: "missing", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA}}}); err != nil {
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
	if len(result.Additions) != 1 || !result.RemovalsBlocked {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root.Path, "extra", "SKILL.md")); err != nil {
		t.Fatal("blocked extra was removed")
	}
}

func TestApplyRestoreReconcilesAfterPartialMutationFailure(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	agents := restoreRoot(t, cfg, config.TargetAgents)
	codex := restoreRoot(t, cfg, config.TargetCodex)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "wanted")}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{agents, codex}})
	if err != nil {
		t.Fatal(err)
	}
	// Create a late conflict after planning so one earlier addition succeeds first.
	makeRestoreSkill(t, plan.Additions[1].Path, "wanted")
	result, err := ApplyRestore(context.Background(), cfg, plan)
	if err == nil || len(result.Additions) != 1 {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	local, loadErr := LoadLocal(project)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(local.Skills) != 1 || local.Skills[0].Name != "wanted" {
		t.Fatalf("local = %#v", local)
	}
}

func TestApplyRestoreRejectsChangedPlannedPath(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "wanted")}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}})
	if err != nil {
		t.Fatal(err)
	}
	plan.Additions[0].Path = filepath.Join(project, "elsewhere", "wanted")
	if _, err := ApplyRestore(context.Background(), cfg, plan); err == nil {
		t.Fatal("ApplyRestore error = nil")
	}
	if _, err := os.Lstat(filepath.Join(root.Path, "wanted")); !os.IsNotExist(err) {
		t.Fatal("planned destination mutated")
	}
}

func TestApplyRestoreUnavailableBlocksDesiredNormalization(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "available"), "available")
	if err := os.MkdirAll(root.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root.Path, "available")
	if err := os.Symlink(filepath.Join(project, "gone"), broken); err != nil {
		t.Fatal(err)
	}
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "available", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "available")}, {Name: "missing", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA}}}); err != nil {
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
	if len(result.Normalizations) != 0 {
		t.Fatalf("normalizations = %#v", result.Normalizations)
	}
	if _, err := os.Lstat(broken); err != nil {
		t.Fatalf("broken desired occurrence was removed: %v", err)
	}
}

func TestApplyRestoreUnavailableIgnoresBlockedConflictAndAppliesUnrelatedAddition(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "divergent"), "divergent")
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "safe"), "safe")
	makeRestoreSkill(t, filepath.Join(root.Path, "divergent"), "divergent")
	if err := os.WriteFile(filepath.Join(root.Path, "divergent", "local"), []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{
		{Name: "divergent", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "divergent")},
		{Name: "missing", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA},
		{Name: "safe", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "safe")},
	}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || len(plan.Normalizations) != 1 || plan.Normalizations[0].ArchiveName != "" {
		t.Fatalf("plan = %#v", plan)
	}
	if !plan.RemovalsBlocked {
		t.Fatal("RemovalsBlocked = false")
	}
	result, err := ApplyRestore(context.Background(), cfg, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovalsBlocked || len(result.Additions) != 1 || result.Additions[0].Name != "safe" {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root.Path, "divergent", "local")); err != nil {
		t.Fatal("divergent active copy was mutated")
	}
	if target, err := filepath.EvalSymlinks(filepath.Join(root.Path, "safe")); err != nil || target != filepath.Join(cfg.ArchiveSkillsRoot(), "safe") {
		t.Fatalf("safe link = %q, %v", target, err)
	}
}

func TestPlanRestoreUsesManagedOccurrenceNameInsteadOfFrontmatterName(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "custom-name"), "upstream-name")
	if err := os.MkdirAll(root.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), "custom-name"), filepath.Join(root.Path, "custom-name")); err != nil {
		t.Fatal(err)
	}
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "custom-name", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "custom-name")}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Removals) != 0 || len(plan.Normalizations) != 0 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanAndApplyRestoreUseUnmanagedOccurrenceName(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(root.Path, "custom-name"), "upstream-name")
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removals) != 1 || plan.Removals[0].Name != "custom-name" {
		t.Fatalf("removals = %#v", plan.Removals)
	}
	if _, err := ApplyRestore(context.Background(), cfg, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "custom-name", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestPlanAndApplyRestoreUseManagedExtraOccurrenceName(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "custom-name"), "upstream-name")
	if err := os.MkdirAll(root.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root.Path, "custom-name")
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), "custom-name"), active); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{root}, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removals) != 1 || plan.Removals[0].Name != "custom-name" || plan.Removals[0].Kind != ChangeRemove {
		t.Fatalf("removals = %#v", plan.Removals)
	}
	if _, err := ApplyRestore(context.Background(), cfg, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active extra remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "custom-name", "SKILL.md")); err != nil {
		t.Fatal("archive removed")
	}
}

func TestApplyRestoreReportsSuccessfulNormalizationBeforeLaterFailure(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	agents := restoreRoot(t, cfg, config.TargetAgents)
	codex := restoreRoot(t, cfg, config.TargetCodex)
	makeRestoreSkill(t, filepath.Join(cfg.ArchiveSkillsRoot(), "wanted"), "wanted")
	makeRestoreSkill(t, filepath.Join(agents.Path, "wanted"), "wanted")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "wanted")}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestore(context.Background(), cfg, RestoreRequest{Destinations: []roots.ActiveRoot{agents, codex}})
	if err != nil {
		t.Fatal(err)
	}
	makeRestoreSkill(t, filepath.Join(codex.Path, "wanted"), "wanted")
	result, err := ApplyRestore(context.Background(), cfg, plan)
	if err == nil {
		t.Fatal("ApplyRestore error = nil")
	}
	if len(result.Normalizations) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPlanRestoreBlocksFullRemovalsWhenDesiredSkillUnavailable(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	root := restoreRoot(t, cfg, config.TargetAgents)
	makeRestoreSkill(t, filepath.Join(root.Path, "extra"), "extra")
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "missing", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA}}}); err != nil {
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
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "wanted")}}}); err != nil {
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
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "wanted", Source: Source{Type: SourceArchive}, Fingerprint: restoreArchiveFingerprint(t, cfg, "wanted")}}}); err != nil {
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

func restoreArchiveFingerprint(t *testing.T, cfg config.Config, name string) string {
	t.Helper()
	fp, err := fingerprint.Directory(filepath.Join(cfg.ArchiveSkillsRoot(), name))
	if err != nil {
		t.Fatal(err)
	}
	return fp
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
