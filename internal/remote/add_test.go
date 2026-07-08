package remote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func TestApplyArchiveOnlyCopiesSkillAndWritesMetadata(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	incoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")
	meta := SourceMetadata{
		SourceType:   SourceTypeGitHub,
		Owner:        "vercel-labs",
		Repo:         "skills",
		CloneURL:     "https://github.com/vercel-labs/skills.git",
		Commit:       "abc",
		SkillPath:    "skills/svelte-coder",
		UpstreamName: "svelte-coder",
	}

	result, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    meta,
		Conflict:    ConflictReplaceArchive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AddStatusArchived {
		t.Fatalf("status = %q", result.Status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Svelte help." {
		t.Fatalf("description = %q", info.Description)
	}
	if _, ok, err := ReadSourceMetadata(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); err != nil || !ok {
		t.Fatalf("source metadata missing: ok=%v err=%v", ok, err)
	}
}

func TestPlanArchiveDetectsNameConflictWithoutSourceIdentity(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Local archived.")
	incoming := writeIncomingSkill(t, "svelte-coder", "Remote.")
	meta := SourceMetadata{
		SourceType: SourceTypeGitHub,
		Owner:      "vercel-labs",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}
	plan, err := PlanArchive(cfg, incoming, "svelte-coder", meta)
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != ArchiveStateNameConflict {
		t.Fatalf("state = %q, want name conflict", plan.State)
	}
}

func writeIncomingSkill(t *testing.T, name, desc string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeArchivedSkillForRemoteTest(t *testing.T, cfg config.Config, name, desc string) string {
	t.Helper()
	dir := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
