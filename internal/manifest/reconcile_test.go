package manifest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestReconcileLocalUsesUnionOfProjectRootsAndExcludesGlobalAndRecommended(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeReconcileArchive(t, cfg, "alpha", nil)
	makeReconcileArchive(t, cfg, "recommended", nil)
	makeReconcileArchive(t, cfg, "global-only", nil)
	linkReconcileSkill(t, cfg, config.ScopeProject, config.TargetAgents, "alpha")
	linkReconcileSkill(t, cfg, config.ScopeProject, config.TargetCodex, "recommended")
	linkReconcileSkill(t, cfg, config.ScopeGlobal, config.TargetAgents, "global-only")
	if err := WriteRecommended(project, Manifest{Version: 1, Skills: []Skill{{Name: "recommended", Source: Source{Type: SourceGit, Repository: "git://example/repo", Path: "skills/recommended"}}}}); err != nil {
		t.Fatal(err)
	}

	result, err := ReconcileLocal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true")
	}
	got, err := LoadLocal(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 1 || got.Skills[0].Name != "alpha" {
		t.Fatalf("skills = %#v, want alpha only", got.Skills)
	}
}

func TestReconcileLocalRemovesPresentArchiveAfterLastProjectOccurrence(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeReconcileArchive(t, cfg, "gone", nil)
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "gone", Source: Source{Type: SourceArchive}, Fingerprint: "old"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileLocal(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLocal(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 0 {
		t.Fatalf("skills = %#v, want empty", got.Skills)
	}
}

func TestReconcileLocalRetainsUnavailableArchiveOnlyEntry(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	want := Skill{Name: "elsewhere", Source: Source{Type: SourceArchive}, Fingerprint: "sha256:missing"}
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{want}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileLocal(cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadLocal(project)
	if len(got.Skills) != 1 || !sameIdentity(got.Skills[0], want) {
		t.Fatalf("skills = %#v, want retained entry", got.Skills)
	}
}

func TestReconcileLocalRemovesUnavailableEntryOwnedByRecommendedManifest(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	if err := WriteLocal(project, Manifest{Version: 1, Skills: []Skill{{Name: "shared", Source: Source{Type: SourceArchive}, Fingerprint: "missing"}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecommended(project, Manifest{Version: 1, Skills: []Skill{{Name: "shared", Source: Source{Type: SourceGit, Repository: "git://example/repo", Path: "skills/shared"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileLocal(cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadLocal(project)
	if len(got.Skills) != 0 {
		t.Fatalf("skills = %#v, want recommended name excluded", got.Skills)
	}
}

func TestReconcileLocalRejectsDivergentSameNameOccurrencesWithoutWriting(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	want := Manifest{Version: 1, Skills: []Skill{{Name: "same", Source: Source{Type: SourceArchive}, Fingerprint: "previous"}}}
	if err := WriteLocal(project, want); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{config.TargetAgents, config.TargetCodex} {
		root, _ := cfg.ActiveRoot(config.ScopeProject, target)
		dir := filepath.Join(root, "same")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("---\nname: same\ndescription: " + target + "\n---\n")
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReconcileLocal(cfg); err == nil {
		t.Fatal("ReconcileLocal error = nil, want divergent identity conflict")
	}
	got, _ := LoadLocal(project)
	if !sameIdentity(got.Skills[0], want.Skills[0]) {
		t.Fatalf("manifest changed after conflict: %#v", got)
	}
}

func TestReconcileLocalAcceptsIdenticalSameNameOccurrences(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	for _, target := range []string{config.TargetAgents, config.TargetCodex} {
		root, _ := cfg.ActiveRoot(config.ScopeProject, target)
		dir := filepath.Join(root, "same")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: same\ndescription: same\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReconcileLocal(cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadLocal(project)
	if len(got.Skills) != 1 || got.Skills[0].Name != "same" {
		t.Fatalf("skills = %#v, want one same", got.Skills)
	}
}

func TestReconcileLocalDoesNotWriteUnchangedNormalizedManifest(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeReconcileArchive(t, cfg, "stable", nil)
	linkReconcileSkill(t, cfg, config.ScopeProject, config.TargetAgents, "stable")
	if _, err := ReconcileLocal(cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, LocalFilename)
	old := time.Unix(123, 0)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	result, err := ReconcileLocal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Fatal("Changed = true, want false")
	}
	info, _ := os.Stat(path)
	if !info.ModTime().Equal(old) {
		t.Fatalf("mtime = %v, want unchanged", info.ModTime())
	}
}

func makeReconcileArchive(t *testing.T, cfg config.Config, name string, meta *remote.SourceMetadata) {
	t.Helper()
	dir := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		if err := remote.WriteSourceMetadata(dir, *meta); err != nil {
			t.Fatal(err)
		}
	}
}

func linkReconcileSkill(t *testing.T, cfg config.Config, scope, target, name string) {
	t.Helper()
	root, err := cfg.ActiveRoot(scope, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), name), filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
}
