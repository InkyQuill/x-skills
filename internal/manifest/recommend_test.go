package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestRecommendMovesSourcedArchiveSkillsToRecommended(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "github-skill", remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub, Owner: "InkyQuill", Repo: "skills", SkillPath: "skills/github-skill", Ref: "main",
		Compatibility: &remote.CompatibilityProfile{Agents: []string{"codex", "claude"}},
	})
	writeRecommendationArchive(t, cfg, "git-skill", remote.SourceMetadata{
		SourceType: remote.SourceTypeGit, CloneURL: "https://git.example/skills.git", SkillPath: "packs/git-skill", Commit: "abc123",
		Compatibility: &remote.CompatibilityProfile{Agnostic: true},
	})
	if err := WriteLocal(cfg.ProjectRoot, Manifest{Version: 1, Skills: []Skill{
		{Name: "github-skill", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA},
		{Name: "keep-local", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintB},
	}}); err != nil {
		t.Fatal(err)
	}

	if err := Recommend(cfg, []string{"git-skill", "github-skill"}); err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	got, err := LoadRecommended(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	want := Manifest{Version: 1, Skills: []Skill{
		{Name: "git-skill", Source: Source{Type: SourceGit, Repository: "https://git.example/skills.git", Path: "packs/git-skill", Ref: "abc123"}, Compatibility: &remote.CompatibilityProfile{Agnostic: true}},
		{Name: "github-skill", Source: Source{Type: SourceGitHub, Repository: "InkyQuill/skills", Path: "skills/github-skill", Ref: "main"}, Compatibility: &remote.CompatibilityProfile{Agents: []string{"claude", "codex"}}},
	}}
	if len(got.Skills) != len(want.Skills) {
		t.Fatalf("recommended = %#v, want %#v", got, want)
	}
	for i := range want.Skills {
		if got.Skills[i].Name != want.Skills[i].Name || got.Skills[i].Source != want.Skills[i].Source ||
			got.Skills[i].Compatibility.Agnostic != want.Skills[i].Compatibility.Agnostic ||
			!slices.Equal(got.Skills[i].Compatibility.Agents, want.Skills[i].Compatibility.Agents) {
			t.Fatalf("recommended[%d] = %#v, want %#v", i, got.Skills[i], want.Skills[i])
		}
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Skills) != 1 || local.Skills[0].Name != "keep-local" {
		t.Fatalf("local skills = %#v, want only keep-local", local.Skills)
	}
}

func TestRecommendRejectsArchiveOnlyAndPlansWholeBatchBeforeWriting(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "sourced", remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/sourced"})
	writeRecommendationArchive(t, cfg, "archive-only", remote.SourceMetadata{})
	want := Manifest{Version: 1, Skills: []Skill{{Name: "existing", Source: Source{Type: SourceGit, Repository: "https://example/repo.git", Path: "skills/existing"}}}}
	if err := WriteRecommended(cfg.ProjectRoot, want); err != nil {
		t.Fatal(err)
	}

	err := Recommend(cfg, []string{"sourced", "archive-only"})
	if err == nil || !strings.Contains(err.Error(), "reproducible Git or GitHub source metadata") {
		t.Fatalf("Recommend() error = %v", err)
	}
	got, loadErr := LoadRecommended(cfg.ProjectRoot)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recommended changed on preflight failure: got %#v, want %#v", got, want)
	}
}

func TestRecommendRejectsInvalidSkillNameBeforeArchiveLookup(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	err := Recommend(cfg, []string{"../escape"})
	if err == nil || !strings.Contains(err.Error(), "invalid skill name") {
		t.Fatalf("Recommend() error = %v, want name validation", err)
	}
}

func TestRecommendRestoresRecommendedWhenLocalWriteFails(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "new-skill", remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/new-skill"})
	want := Manifest{Version: 1, Skills: []Skill{{Name: "existing", Source: Source{Type: SourceGit, Repository: "https://example/repo.git", Path: "skills/existing"}}}}
	if err := WriteRecommended(cfg.ProjectRoot, want); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cfg.ProjectRoot, LocalFilename), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Recommend(cfg, []string{"new-skill"}); err == nil {
		t.Fatal("Recommend() error = nil, want local write failure")
	}
	got, err := LoadRecommended(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recommended after rollback = %#v, want %#v", got, want)
	}
}

func TestUnrecommendMovesStillActiveProjectSkillToLocal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "shared", remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/shared"})
	if err := WriteRecommended(cfg.ProjectRoot, Manifest{Version: 1, Skills: []Skill{{Name: "shared", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/shared"}}}}); err != nil {
		t.Fatal(err)
	}
	projectSkill := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "shared")
	if err := os.MkdirAll(filepath.Dir(projectSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), "shared"), projectSkill); err != nil {
		t.Fatal(err)
	}

	if err := Unrecommend(cfg, []string{"shared"}); err != nil {
		t.Fatalf("Unrecommend() error = %v", err)
	}
	recommended, _ := LoadRecommended(cfg.ProjectRoot)
	if len(recommended.Skills) != 0 {
		t.Fatalf("recommended skills = %#v, want empty", recommended.Skills)
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Skills) != 1 || local.Skills[0].Name != "shared" || local.Skills[0].Source.Type != SourceGitHub {
		t.Fatalf("local skills = %#v, want active sourced shared skill", local.Skills)
	}
}

func TestUnrecommendReconcilesUnmanagedActiveIdentity(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "shared", remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/shared"})
	if err := WriteRecommended(cfg.ProjectRoot, Manifest{Version: 1, Skills: []Skill{{Name: "shared", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/shared"}}}}); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "shared")
	if err := os.MkdirAll(active, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "SKILL.md"), []byte("---\nname: shared\ndescription: divergent local copy\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Unrecommend(cfg, []string{"shared"}); err != nil {
		t.Fatalf("Unrecommend() error = %v", err)
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Skills) != 1 || local.Skills[0].Source.Type != SourceArchive || local.Skills[0].Fingerprint == "" {
		t.Fatalf("local skills = %#v, want observed archive identity with fingerprint", local.Skills)
	}
}

func TestUnrecommendRejectsDivergentActiveIdentitiesWithoutWriting(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "shared", remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/shared"})
	want := Manifest{Version: 1, Skills: []Skill{{Name: "shared", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/shared"}}}}
	if err := WriteRecommended(cfg.ProjectRoot, want); err != nil {
		t.Fatal(err)
	}
	for target, description := range map[string]string{config.TargetAgents: "first", config.TargetCodex: "second"} {
		active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, target), "shared")
		if err := os.MkdirAll(active, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(active, "SKILL.md"), []byte("---\nname: shared\ndescription: "+description+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := Unrecommend(cfg, []string{"shared"})
	if err == nil || !strings.Contains(err.Error(), "divergent identities") {
		t.Fatalf("Unrecommend() error = %v", err)
	}
	got, loadErr := LoadRecommended(cfg.ProjectRoot)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recommended changed on reconciliation failure: got %#v, want %#v", got, want)
	}
}

func TestUnrecommendWithoutActiveProjectSkillOnlyRemovesRecommended(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	writeRecommendationArchive(t, cfg, "shared", remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/shared"})
	if err := WriteRecommended(cfg.ProjectRoot, Manifest{Version: 1, Skills: []Skill{{Name: "shared", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/shared"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := Unrecommend(cfg, []string{"shared"}); err != nil {
		t.Fatal(err)
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Skills) != 0 {
		t.Fatalf("local skills = %#v, want empty", local.Skills)
	}
}

func writeRecommendationArchive(t *testing.T, cfg config.Config, name string, meta remote.SourceMetadata) {
	t.Helper()
	dir := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if meta.SourceType != "" {
		if err := remote.WriteSourceMetadata(dir, meta); err != nil {
			t.Fatal(err)
		}
	}
}
