package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/InkyQuill/x-skills/internal/pathidentity"
)

func TestApplyMigratesPreservesLinksAndReconcilesManifest(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	destination := filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")
	makeApplySkillAt(t, destination, "destination copy")
	destinationFP := applyFingerprint(t, destination)
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-from-agents")
	fp := applyFingerprint(t, source)
	plan := Plan{
		Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Links:      []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkNormalize, ArchivePath: archive, DestinationPath: destination, DestinationFingerprint: destinationFP}},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: destination, DestinationStatus: actions.StatusUnmanaged,
			DestinationFingerprint: destinationFP, Resolution: ConflictResolution{DestinationPath: destination, PreserveAs: "review-from-agents", Action: ConflictReplace}}},
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

func TestApplyArchiveConflictUsesSharedRenameForVisibleAliases(t *testing.T) {
	cfg := applyConfig(t)
	existing := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "existing")
	globalRoot := cfg.MustActiveRoot(config.ScopeGlobal, config.TargetAgents)
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(globalRoot, "friendly")
	if err := os.Symlink(existing, alias); err != nil {
		t.Fatal(err)
	}
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "incoming")
	fp := applyFingerprint(t, source)
	existingFP := applyFingerprint(t, existing)
	plan := Plan{
		Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: existing}},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: existing, DestinationStatus: actions.StatusManaged,
			DestinationFingerprint: existingFP, ManagedTarget: existing, Resolution: ConflictResolution{DestinationPath: existing, PreserveAs: "review-preserved", Action: ConflictReplace}}},
	}
	result := Apply(context.Background(), cfg, plan)
	if len(result.Failed) != 0 || result.PlanError != nil {
		t.Fatalf("result = %#v", result)
	}
	assertApplyLink(t, alias, filepath.Join(cfg.ArchiveSkillsRoot(), "review-preserved"))
	assertApplyFile(t, filepath.Join(existing, "content.txt"), "incoming")
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
	firstFP := applyFingerprint(t, first)
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-from-agents")
	plan := Plan{
		Links: []Change{
			{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkNormalize, ArchivePath: archive, DestinationPath: first, DestinationFingerprint: firstFP},
			{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate, ArchivePath: archive, DestinationPath: second},
		},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: first, DestinationStatus: actions.StatusUnmanaged,
			DestinationFingerprint: firstFP, Resolution: ConflictResolution{DestinationPath: first, PreserveAs: "review-from-agents", Action: ConflictReplace}}},
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
	if _, err := os.Lstat(second); err == nil || (!errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR)) {
		t.Fatalf("failed destination unexpectedly exists or returned unexpected error: %v", err)
	}
}

func TestApplyDoesNotReconcileAfterPreMutationFailure(t *testing.T) {
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	fp := applyFingerprint(t, source)
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
	if err := os.Mkdir(filepath.Join(cfg.ProjectRoot, manifest.LocalFilename), 0o755); err != nil {
		t.Fatal(err)
	}
	fs := defaultApplyFilesystem()
	fs.afterStage = func(string) error { return errors.New("injected staging rejection") }
	result := Apply(context.Background(), cfg, Plan{Migrations: []Change{{
		CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive,
	}}}, ApplyOptions{filesystem: fs})
	if len(result.Failed) != 1 || result.ManifestError != nil {
		t.Fatalf("Apply result = %#v, want failure without reconciliation", result)
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
		makeApplySkillAt(t, second, "appeared after validation")
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
	assertApplyFile(t, filepath.Join(second, "content.txt"), "appeared after validation")
}

func TestApplyReportsArchivePublicationWhenSourceRemovalFails(t *testing.T) {
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	fp := applyFingerprint(t, source)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "old archive")
	archiveConflictFP := applyFingerprint(t, archive)
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-old")
	plan := Plan{Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: archive, DestinationStatus: actions.StatusManaged,
			DestinationFingerprint: archiveConflictFP, ManagedTarget: archive, Resolution: ConflictResolution{DestinationPath: archive, PreserveAs: "review-old", Action: ConflictReplace}}}}
	fs := defaultApplyFilesystem()
	fs.removeAll = func(path string) error {
		if path == source {
			return errors.New("injected source removal failure")
		}
		return os.RemoveAll(path)
	}
	result := Apply(context.Background(), cfg, plan, ApplyOptions{filesystem: fs})
	if len(result.Failed) != 1 || !result.Failed[0].ArchiveChanged || result.Failed[0].SourceRemoved {
		t.Fatalf("Apply source-removal result = %#v", result)
	}
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "selected")
	assertApplyFile(t, filepath.Join(preserved, "content.txt"), "old archive")
	if _, err := os.Lstat(source); err != nil {
		t.Fatalf("partially removed source is not reported in filesystem: %v", err)
	}
}

func TestApplyPublicationFailureRestoresArchiveAliases(t *testing.T) {
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	fp := applyFingerprint(t, source)
	archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "old archive")
	archiveFP := applyFingerprint(t, archive)
	usage := filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")
	if err := os.MkdirAll(filepath.Dir(usage), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archive, usage); err != nil {
		t.Fatal(err)
	}
	preserved := filepath.Join(cfg.ArchiveSkillsRoot(), "review-old")
	plan := Plan{
		Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: "migrate", SourcePath: source, ArchivePath: archive}},
		Conflicts: []Conflict{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, DestinationPath: archive,
			DestinationStatus: actions.StatusManaged, DestinationFingerprint: archiveFP, ManagedTarget: archive,
			Resolution: ConflictResolution{DestinationPath: archive, PreserveAs: "review-old", Action: ConflictReplace}}},
	}
	fs := defaultApplyFilesystem()
	fs.rename = func(oldPath, newPath string) error {
		if newPath == archive && strings.Contains(filepath.Base(oldPath), ".x-skills-stage") {
			return errors.New("injected publication failure")
		}
		return os.Rename(oldPath, newPath)
	}
	result := Apply(context.Background(), cfg, plan, ApplyOptions{filesystem: fs})
	if len(result.Failed) != 1 || !strings.Contains(result.Failed[0].Err.Error(), "injected publication failure") {
		t.Fatalf("Apply result = %#v", result)
	}
	assertApplyFile(t, filepath.Join(archive, "content.txt"), "old archive")
	if _, err := os.Lstat(preserved); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preserved alias remains after rollback: %v", err)
	}
	assertApplyLink(t, usage, archive)
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
	}, filesystem: applyFilesystem{removeAll: func(path string) error {
		if backup != "" && path == backup {
			return errors.New("injected backup cleanup failure")
		}
		return os.RemoveAll(path)
	},
	}})
	if backup != "" {
		_ = os.RemoveAll(backup)
	}
	if len(result.Failed) != 1 || result.Failed[0].Err == nil || result.Failed[0].LinksRolledBack {
		t.Fatalf("Apply cleanup result = %#v", result)
	}
	assertApplyLink(t, destination, archive)
}

func TestApplyRevalidatesLaterSkillAfterProgressCallbackDrift(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	alphaArchive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "alpha", "alpha")
	betaArchive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "beta", "beta")
	alphaFP := applyFingerprint(t, alphaArchive)
	betaFP := applyFingerprint(t, betaArchive)
	alphaDestination := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "alpha")
	betaDestination := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "beta")
	plan := Plan{Links: []Change{
		{CandidateID: "alpha:" + alphaFP, Name: "alpha", Fingerprint: alphaFP, Action: LinkCreate, ArchivePath: alphaArchive, DestinationPath: alphaDestination},
		{CandidateID: "beta:" + betaFP, Name: "beta", Fingerprint: betaFP, Action: LinkCreate, ArchivePath: betaArchive, DestinationPath: betaDestination},
	}}
	result := Apply(context.Background(), cfg, plan, ApplyOptions{Progress: func(update Progress) {
		if update.Skill == "alpha" {
			makeApplySkillAt(t, betaDestination, "late beta")
		}
	}})
	if len(result.Succeeded) != 1 || result.Succeeded[0].Name != "alpha" || len(result.Failed) != 1 || result.Failed[0].Name != "beta" {
		t.Fatalf("Apply later drift result = %#v", result)
	}
	assertApplyFile(t, filepath.Join(betaDestination, "content.txt"), "late beta")
}

func TestApplyReportsIncompleteRollbackForRemovalAndRestoreFailures(t *testing.T) {
	for _, test := range []struct {
		name          string
		existingFirst bool
		fault         func(string, string) error
	}{
		{name: "remove created link", fault: func(old, newPath string) error { return nil }},
		{name: "restore destination backup", existingFirst: true, fault: func(old, newPath string) error {
			if strings.HasPrefix(filepath.Base(old), ".x-skills-backup") {
				return errors.New("injected restore failure")
			}
			return os.Rename(old, newPath)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := applyConfig(t)
			archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
			fp := applyFingerprint(t, archive)
			first := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "review")
			second := filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")
			action := LinkCreate
			if test.existingFirst {
				makeApplySkillAt(t, first, "selected")
				action = LinkNormalize
			}
			plan := Plan{Links: []Change{
				{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: action, ArchivePath: archive, DestinationPath: first},
				{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp, Action: LinkCreate, ArchivePath: archive, DestinationPath: second},
			}}
			fs := defaultApplyFilesystem()
			if test.name == "remove created link" {
				fs.removeAll = func(path string) error {
					if path == first {
						return errors.New("injected remove failure")
					}
					return os.RemoveAll(path)
				}
			} else {
				fs.rename = test.fault
			}
			drifted := false
			result := Apply(context.Background(), cfg, plan, ApplyOptions{filesystem: fs, Progress: func(update Progress) {
				if !drifted {
					drifted = true
					makeApplySkillAt(t, second, "late")
				}
			}})
			if len(result.Failed) != 1 || result.Failed[0].LinksRolledBack || !result.Failed[0].LinksRollbackIncomplete {
				t.Fatalf("Apply rollback result = %#v", result)
			}
		})
	}
}

func TestApplyRejectsStagedMigrationFingerprintDriftBeforePublication(t *testing.T) {
	t.Parallel()
	cfg := applyConfig(t)
	source := makeApplySkill(t, filepath.Join(cfg.ProjectRoot, ".agents", "skills"), "review", "selected")
	fp := applyFingerprint(t, source)
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "review")
	plan := Plan{Migrations: []Change{{CandidateID: "review:" + fp, Name: "review", Fingerprint: fp,
		Action: "migrate", SourcePath: source, ArchivePath: archive}}}
	fs := defaultApplyFilesystem()
	fs.afterStage = func(staged string) error {
		return os.WriteFile(filepath.Join(staged, "content.txt"), []byte("injected drift"), 0o644)
	}
	result := Apply(context.Background(), cfg, plan, ApplyOptions{filesystem: fs})
	if len(result.Failed) != 1 || result.Failed[0].ArchiveChanged || result.Failed[0].SourceRemoved {
		t.Fatalf("Apply staged drift result = %#v", result)
	}
	assertApplyFile(t, filepath.Join(source, "content.txt"), "selected")
	if _, err := os.Lstat(archive); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive published: %v", err)
	}
}

func TestApplyRejectsReplacementIdentityDriftAtMutationBoundaries(t *testing.T) {
	for _, test := range []struct {
		name    string
		managed bool
		after   bool
	}{
		{name: "unmanaged before preservation"},
		{name: "unmanaged before replacement", after: true},
		{name: "managed before preservation", managed: true},
		{name: "managed before replacement", managed: true, after: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := applyConfig(t)
			archive := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
			selectedFP := applyFingerprint(t, archive)
			destination := filepath.Join(cfg.ProjectRoot, ".agents", "skills", "review")
			status := actions.StatusUnmanaged
			managedTarget := ""
			if test.managed {
				managedTarget = makeApplySkill(t, cfg.ArchiveSkillsRoot(), "old-review", "approved old")
				if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(managedTarget, destination); err != nil {
					t.Fatal(err)
				}
				status = actions.StatusManaged
			} else {
				makeApplySkillAt(t, destination, "approved old")
			}
			fingerprintPath := destination
			if test.managed {
				fingerprintPath = managedTarget
			}
			destinationFP := applyFingerprint(t, fingerprintPath)
			preserveName := "review-approved-old"
			change := Change{CandidateID: "review:" + selectedFP, Name: "review", Fingerprint: selectedFP, Action: LinkNormalize,
				ArchivePath: archive, DestinationPath: destination, DestinationFingerprint: destinationFP, ManagedTarget: managedTarget}
			conflict := Conflict{CandidateID: change.CandidateID, Name: change.Name, Fingerprint: change.Fingerprint,
				DestinationPath: destination, DestinationStatus: status, DestinationFingerprint: destinationFP, ManagedTarget: managedTarget,
				Resolution: ConflictResolution{DestinationPath: destination, PreserveAs: preserveName, Action: ConflictReplace}}
			fs := defaultApplyFilesystem()
			drift := func(string) error {
				if test.managed {
					other := makeApplySkill(t, cfg.ArchiveSkillsRoot(), "other-review", "new managed content")
					if err := os.Remove(destination); err != nil {
						return err
					}
					return os.Symlink(other, destination)
				}
				return os.WriteFile(filepath.Join(destination, "content.txt"), []byte("new unmanaged content"), 0o644)
			}
			if test.after {
				fs.afterPreserve = drift
			} else {
				fs.beforePreserve = drift
			}

			result := Apply(context.Background(), cfg, Plan{Links: []Change{change}, Conflicts: []Conflict{conflict}}, ApplyOptions{filesystem: fs})
			if len(result.Failed) != 1 || result.Failed[0].Err == nil {
				t.Fatalf("Apply drift result = %#v", result)
			}
			info, err := os.Lstat(destination)
			if err != nil || (test.managed && info.Mode()&os.ModeSymlink == 0) || (!test.managed && !info.IsDir()) {
				t.Fatalf("destination was replaced: %v, %v", info, err)
			}
		})
	}
}

func TestClassificationMatchesConflictComparesManagedTargetIdentity(t *testing.T) {
	t.Parallel()

	managedTarget := makeApplySkill(t, t.TempDir(), "approved", "approved")
	alias := filepath.Join(t.TempDir(), "approved-alias")
	if err := os.Symlink(managedTarget, alias); err != nil {
		t.Fatal(err)
	}
	fp := applyFingerprint(t, managedTarget)

	classification := destinationClassification{
		kind:          destinationDivergent,
		status:        actions.StatusManaged,
		fingerprint:   fp,
		managedTarget: managedTarget,
	}
	conflict := Conflict{
		DestinationStatus:      actions.StatusManaged,
		DestinationFingerprint: fp,
		ManagedTarget:          alias,
	}
	if !classificationMatchesConflict(classification, conflict) {
		t.Fatal("classificationMatchesConflict treated equivalent managed targets as drift")
	}
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
	if err != nil || !applySamePath(got, want) {
		t.Fatalf("link %q = %q, %v; want %q", path, got, err, want)
	}
}

// applySamePath compares symlink targets by filesystem identity so tests are
// insensitive to platform-specific raw path spelling.
func applySamePath(a, b string) bool {
	return pathidentity.Equivalent(a, b)
}

func assertApplyFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("file %q = %q, %v; want %q", path, got, err, want)
	}
}
