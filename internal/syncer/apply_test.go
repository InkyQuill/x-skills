package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/manifest"
)

func TestApplyMigratesPreservesLinksAndReconcilesManifest(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	destination := filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")
	makeApplySkillAt(t, destination, "destination copy")
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-from-agents")
	fp := applyFingerprint(t, source)
	plan := Plan{
		Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Links:      []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkNormalize, ArchivePath: archive, DestinationPath: destination}},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: destination, DestinationStatus: actions.StatusUnmanaged,
			Resolution: ConflictResolution{DestinationPath: destination, PreserveAs: "review-from-agents", Action: ConflictReplace}}},
	}

	var progress []Progress
	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) { progress = append(progress, update) }})
	if len(result.Succeeded) != 1 || len(result.Failed) != 0 || result.Cancelled {
		t.Fatalf("Apply result = %#v; plan error = %v", result, result.PlanError)
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
	fp := applyFingerprint(t, archive)
	first := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "review")
	secondRoot := filepath.Join(cfg.ProjectRoot, ".codex", "skills")
	second := filepath.Join(secondRoot, "review")
	makeApplySkillAt(t, first, "old destination")
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-from-agents")
	plan := Plan{
		Links: []Change{
			{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkNormalize, ArchivePath: archive, DestinationPath: first},
			{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate, ArchivePath: archive, DestinationPath: second},
		},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: first, DestinationStatus: actions.StatusUnmanaged,
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
	firstFP := applyFingerprint(t, firstArchive)
	secondFP := applyFingerprint(t, secondArchive)
	first := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "alpha")
	second := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "beta")
	plan := Plan{Links: []Change{
		{CandidateID: "alpha:" + firstFP, Name: "alpha", Fingerprint: firstFP, Action: LinkCreate, ArchivePath: firstArchive, DestinationPath: first},
		{CandidateID: "beta:" + secondFP, Name: "beta", Fingerprint: secondFP, Action: LinkCreate, ArchivePath: secondArchive, DestinationPath: second},
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
	source := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "portable")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, source); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "portable")
	fp := applyFingerprint(t, external)
	result := Apply(context.Background(), cfg, Plan{Migrations: []Change{{
		CandidateID: "portable:" + fp, Name: "portable", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive,
	}}})
	if len(result.Succeeded) != 1 || len(result.Failed) != 0 {
		t.Fatalf("Apply result = %#v; plan error = %v", result, result.PlanError)
	}
	info, err := os.Lstat(archive)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("archive is not a materialized directory: %v, %v", info, err)
	}
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "durable")
}

func TestApplyRejectsTamperedPlanBeforeAnyMutation(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "safe", "safe")
	fp := applyFingerprint(t, source)
	outside := filepath.Join(t.TempDir(), "outside")
	makeApplySkillAt(t, outside, "outside")
	plan := Plan{Migrations: []Change{{CandidateID: "safe:" + fp, Name: "safe", Fingerprint: fp,
		Action: "migrate", SourcePath: source, ArchivePath: outside}}}

	result := Apply(context.Background(), cfg, plan)
	if result.PlanError == nil {
		t.Fatalf("Apply accepted tampered archive path: %#v", result)
	}
	assertApplyFile(t, filepath.Join(source, "content.txt"), "safe")
	assertApplyFile(t, filepath.Join(outside, "content.txt"), "outside")
}

func TestApplyRejectsSourceAndDestinationDriftBeforeAnyMutation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		drift func(*testing.T, string)
	}{
		{name: "source", drift: func(t *testing.T, path string) { makeApplySkillAt(t, path, "changed") }},
		{name: "destination", drift: func(t *testing.T, path string) { makeApplySkillAt(t, path, "appeared") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := applyConfig(t)
			source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
			fp := applyFingerprint(t, source)
			archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
			destination := filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")
			plan := Plan{Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
				Links: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate, ArchivePath: archive, DestinationPath: destination}}}
			if test.name == "source" {
				test.drift(t, source)
			} else {
				test.drift(t, destination)
			}
			result := Apply(context.Background(), cfg, plan)
			if result.PlanError == nil {
				t.Fatalf("Apply accepted %s drift: %#v", test.name, result)
			}
			if _, err := os.Lstat(archive); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("archive mutated: %v", err)
			}
		})
	}
}

func TestApplyRejectsUnresolvedAndMalformedCancellationPlans(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
	fp := applyFingerprint(t, archive)
	destination := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "different")
	conflict := Conflict{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: destination, DestinationStatus: actions.StatusUnmanaged,
		Resolution: ConflictResolution{DestinationPath: destination}}
	for _, plan := range []Plan{
		{Conflicts: []Conflict{conflict}},
		{Cancelled: true, Links: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate,
			ArchivePath: archive, DestinationPath: filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")}}},
	} {
		if result := Apply(context.Background(), cfg, plan); result.PlanError == nil {
			t.Fatalf("Apply accepted invalid plan: %#v", plan)
		}
		assertApplyFile(t, filepath.Join(destination, "content.txt"), "different")
	}
}

func TestApplyRejectsArchivedSourceDriftBeforeLinking(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
	fp := applyFingerprint(t, archive)
	destination := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "review")
	plan := Plan{Links: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp,
		Action: LinkCreate, ArchivePath: archive, DestinationPath: destination}}}
	makeApplySkillAt(t, archive, "changed")
	result := Apply(context.Background(), cfg, plan)
	if result.PlanError == nil {
		t.Fatalf("Apply accepted drifted archive: %#v", result)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination mutated: %v", err)
	}
}

func TestApplyReportsRetainedMigrationAfterLateLinkRollback(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	fp := applyFingerprint(t, source)
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
	first := filepath.Join(cfg.ProjectRoot, ".claude", "skills", "review")
	secondRoot := filepath.Join(cfg.ProjectRoot, ".codex", "skills")
	second := filepath.Join(secondRoot, "review")
	plan := Plan{Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Links: []Change{
			{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate, ArchivePath: archive, DestinationPath: first},
			{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate, ArchivePath: archive, DestinationPath: second},
		}}
	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) {
		if update.Action != LinkCreate {
			return
		}
		if err := os.MkdirAll(filepath.Dir(secondRoot), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(secondRoot, []byte("block"), 0o644); err != nil {
			t.Fatal(err)
		}
	}})
	if len(result.Failed) != 1 || !result.Failed[0].ArchiveChanged || !result.Failed[0].SourceRemoved || !result.Failed[0].LinksRolledBack {
		t.Fatalf("Apply did not report coherent partial success: %#v", result)
	}
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "selected")
	if _, err := os.Lstat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source remains: %v", err)
	}
	if _, err := os.Lstat(first); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("first link was not rolled back: %v", err)
	}
}

func TestApplyReportsArchivePublicationWhenSourceRemovalFails(t *testing.T) {
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	fp := applyFingerprint(t, source)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "old archive")
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-old")
	plan := Plan{Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: archive, DestinationStatus: actions.StatusManaged,
			Resolution: ConflictResolution{DestinationPath: archive, PreserveAs: "review-old", Action: ConflictReplace}}}}
	parent := filepath.Dir(source)
	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) {
		if update.Action == ConflictReplace {
			if err := os.Chmod(parent, 0o555); err != nil {
				t.Fatal(err)
			}
		}
	}})
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 1 || !result.Failed[0].ArchiveChanged || result.Failed[0].SourceRemoved {
		t.Fatalf("Apply source-removal result = %#v", result)
	}
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "selected")
	assertApplyFile(t, filepath.Join(preserved, "content.txt"), "old archive")
	if _, err := os.Lstat(source); err != nil {
		t.Fatalf("partially removed source is not reported in filesystem: %v", err)
	}
}

func TestApplyReportsBackupCleanupFailureWithoutBreakingLink(t *testing.T) {
	cfg := applyConfig(t)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
	fp := applyFingerprint(t, archive)
	destination := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	plan := Plan{Links: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkNormalize,
		ArchivePath: archive, DestinationPath: destination}}}
	var backup string
	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) {
		matches, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".x-skills-backup-*"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("backup lookup = %v, %v", matches, err)
		}
		backup = matches[0]
		if err := os.Chmod(backup, 0); err != nil {
			t.Fatal(err)
		}
	}})
	if backup != "" {
		_ = os.Chmod(backup, 0o755)
		_ = os.RemoveAll(backup)
	}
	if len(result.Failed) != 1 || result.Failed[0].Err == nil || result.Failed[0].LinksRolledBack {
		t.Fatalf("Apply cleanup result = %#v", result)
	}
	assertApplyLink(t, destination, archive)
}

func applyFingerprint(t *testing.T, path string) string {
	t.Helper()
	fp, err := fingerprint.Directory(path)
	if err != nil {
		t.Fatal(err)
	}
	return fp
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
