package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
)

func TestApplyMigratesPreservesLinksAndReconcilesManifest(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".source", "skills"), "review", "selected")
	destination := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "review")
	makeApplySkillAt(t, destination, "destination copy")
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-from-agents")
	plan := Plan{
		Migrations: []Change{{CandidateID: "review:id", Name: "review", Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Links:      []Change{{CandidateID: "review:id", Name: "review", Action: LinkNormalize, ArchivePath: archive, DestinationPath: destination}},
		Conflicts: []Conflict{{CandidateID: "review:id", Name: "review", DestinationPath: destination,
			Resolution: ConflictResolution{DestinationPath: destination, PreserveAs: "review-from-agents", Action: ConflictReplace}}},
	}

	var progress []Progress
	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) { progress = append(progress, update) }})
	if len(result.Succeeded) != 1 || len(result.Failed) != 0 || result.Cancelled {
		t.Fatalf("Apply result = %#v", result)
	}
	assertApplyLink(t, destination, archive)
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "selected")
	assertApplyFile(t, filepath.Join(preserved, "content.txt"), "destination copy")
	if _, err := os.Lstat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists: %v", err)
	}
	if len(progress) == 0 || progress[len(progress)-1].Completed != progress[len(progress)-1].Total {
		t.Fatalf("progress = %#v", progress)
	}
	local, err := manifest.LoadLocal(cfg.ProjectRoot)
	if err != nil || len(local.Skills) != 1 || local.Skills[0].Name != "review" {
		t.Fatalf("local manifest = %#v, %v", local, err)
	}
}

func TestApplyRollsBackLinksForFailedSkillButKeepsPreservedArchives(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
	first := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "review")
	secondRoot := filepath.Join(cfg.ProjectRoot, ".codex", "skills")
	second := filepath.Join(secondRoot, "review")
	makeApplySkillAt(t, first, "old destination")
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-from-agents")
	plan := Plan{
		Links: []Change{
			{CandidateID: "review:id", Name: "review", Action: LinkNormalize, ArchivePath: archive, DestinationPath: first},
			{CandidateID: "review:id", Name: "review", Action: LinkCreate, ArchivePath: archive, DestinationPath: second},
		},
		Conflicts: []Conflict{{CandidateID: "review:id", Name: "review", DestinationPath: first,
			Resolution: ConflictResolution{DestinationPath: first, PreserveAs: "review-from-agents", Action: ConflictReplace}}},
	}

	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) {
		if update.Skill == "review" && update.Action == LinkNormalize {
			if err := os.RemoveAll(secondRoot); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(secondRoot), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(secondRoot, []byte("block"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}})
	if len(result.Failed) != 1 || result.Failed[0].Err == nil || len(result.Succeeded) != 0 {
		t.Fatalf("Apply result = %#v", result)
	}
	assertApplyFile(t, filepath.Join(first, "content.txt"), "old destination")
	assertApplyFile(t, filepath.Join(preserved, "content.txt"), "old destination")
	if _, err := os.Lstat(second); !errors.Is(err, os.ErrNotExist) && err == nil {
		t.Fatalf("failed destination unexpectedly exists")
	}
}

func TestApplyCancellationStopsBeforeNextSkillAndReportsPartialSuccess(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	firstArchive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "alpha", "a")
	secondArchive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "beta", "b")
	first := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "alpha")
	second := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "beta")
	plan := Plan{Links: []Change{
		{CandidateID: "alpha:id", Name: "alpha", Action: LinkCreate, ArchivePath: firstArchive, DestinationPath: first},
		{CandidateID: "beta:id", Name: "beta", Action: LinkCreate, ArchivePath: secondArchive, DestinationPath: second},
	}}

	result := Apply(ctx, cfg, plan, ApplyOptions{Progress: func(update Progress) {
		if update.Skill == "alpha" {
			cancel()
		}
	}})
	if !result.Cancelled || len(result.Succeeded) != 1 || result.Succeeded[0].Name != "alpha" {
		t.Fatalf("Apply result = %#v", result)
	}
	assertApplyLink(t, first, firstArchive)
	if _, err := os.Lstat(second); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second skill was applied: %v", err)
	}
}

func TestApplyMaterializesUnmanagedSymlinkSource(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	external := makeApplySkill(t, filepath.Join(t.TempDir(), "external"), "portable", "durable")
	source := filepath.Join(cfg.ProjectRoot, ".source", "skills", "portable")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, source); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "portable")
	result := Apply(context.Background(), cfg, Plan{Migrations: []Change{{
		CandidateID: "portable:id", Name: "portable", Action: "migrate", SourcePath: source, ArchivePath: archive,
	}}})
	if len(result.Succeeded) != 1 || len(result.Failed) != 0 {
		t.Fatalf("Apply result = %#v", result)
	}
	info, err := os.Lstat(archive)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("archive is not a materialized directory: %v, %v", info, err)
	}
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "durable")
}

func applyConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := config.Default(filepath.Join(root, "project"), filepath.Join(root, "home"))
	for _, path := range []string{cfg.ProjectRoot, cfg.ArchiveSkillsRoot()} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return cfg
}

func makeApplySkill(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	makeApplySkillAt(t, path, content)
	return path
}

func makeApplySkillAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+filepath.Base(path)+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "content.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertApplyLink(t *testing.T, path, want string) {
	t.Helper()
	got, err := filepath.EvalSymlinks(path)
	if err != nil || got != want {
		t.Fatalf("link %q = %q, %v; want %q", path, got, err, want)
	}
}

func assertApplyFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("file %q = %q, %v; want %q", path, got, err, want)
	}
}
