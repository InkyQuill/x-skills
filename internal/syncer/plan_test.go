package syncer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestPreflightPlansManagedReuseAndDestinationClassificationsWithoutMutation(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	archive := makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
	candidate.Fingerprint = mustPlanFingerprint(t, archive)
	candidate.Occurrences[0].Status = actions.StatusManaged
	candidate.Occurrences[0].Path = filepath.Join(cfg.ProjectRoot, ".codex", "skills", "review")
	mustPlanSymlink(t, archive, candidate.Occurrences[0].Path)

	managed := filepath.Join(destinations[0].Path, "review")
	mustPlanSymlink(t, archive, managed)
	matchingUnmanaged := makePlanSkill(t, destinations[1].Path, "review", "selected")
	before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)

	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Migrations) != 0 {
		t.Fatalf("Migrations = %#v, want managed archive reuse", plan.Migrations)
	}
	if len(plan.Links) != 1 || plan.Links[0].DestinationPath != matchingUnmanaged || plan.Links[0].Action != LinkNormalize {
		t.Fatalf("Links = %#v, want one normalization", plan.Links)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].DestinationPath != managed || plan.Skipped[0].Reason != SkipAlreadyManaged {
		t.Fatalf("Skipped = %#v, want already-managed destination", plan.Skipped)
	}
	assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
}

func TestPreflightPlansUnmanagedMigrationAndMissingLink(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)

	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Migrations) != 1 || plan.Migrations[0].SourcePath != source || plan.Migrations[0].ArchivePath != filepath.Join(cfg.ArchiveSkillsRoot(), "review") {
		t.Fatalf("Migrations = %#v", plan.Migrations)
	}
	if len(plan.Links) != 1 || plan.Links[0].Action != LinkCreate {
		t.Fatalf("Links = %#v, want create", plan.Links)
	}
	assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
}

func TestPreflightReportsManagedAndUnmanagedDestinationConflicts(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	makePlanSkill(t, destinations[0].Path, "review", "unmanaged conflict")
	otherArchive := makePlanSkill(t, cfg.ArchiveSkillsRoot(), "other", "managed conflict")
	mustPlanSymlink(t, otherArchive, filepath.Join(destinations[1].Path, "review"))

	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 2 {
		t.Fatalf("Conflicts = %#v", plan.Conflicts)
	}
	if plan.Conflicts[0].DestinationStatus != actions.StatusUnmanaged || plan.Conflicts[1].DestinationStatus != actions.StatusManaged {
		t.Fatalf("conflict statuses = %q, %q", plan.Conflicts[0].DestinationStatus, plan.Conflicts[1].DestinationStatus)
	}
	if plan.Conflicts[0].SuggestedPreserveAs != "review-from-agents" || plan.Conflicts[1].SuggestedPreserveAs != "review-from-claude" {
		t.Fatalf("suggestions = %q, %q", plan.Conflicts[0].SuggestedPreserveAs, plan.Conflicts[1].SuggestedPreserveAs)
	}
}

func TestPreflightAppliesExplicitConflictResolutionsAndValidatesUniqueNames(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	makePlanSkill(t, destinations[0].Path, "review", "first conflict")
	makePlanSkill(t, destinations[1].Path, "review", "second conflict")
	makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review-from-agents", "occupied")

	resolutions := []ConflictResolution{
		{DestinationPath: filepath.Join(destinations[0].Path, "review"), Action: ConflictReplace, PreserveAs: "review-preserved"},
		{DestinationPath: filepath.Join(destinations[1].Path, "review"), Action: ConflictKeep},
	}
	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, resolutions)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Resolution.Action != ConflictReplace || plan.Conflicts[0].Resolution.PreserveAs != "review-preserved" {
		t.Fatalf("Conflicts = %#v", plan.Conflicts)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Reason != SkipKeptDestination {
		t.Fatalf("Skipped = %#v", plan.Skipped)
	}

	resolutions[0].PreserveAs = "../escape"
	if _, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, resolutions); err == nil {
		t.Fatal("Preflight accepted unsafe preserve name")
	}
	resolutions[0].PreserveAs = "same-name"
	resolutions[1] = ConflictResolution{DestinationPath: filepath.Join(destinations[1].Path, "review"), Action: ConflictReplace, PreserveAs: "same-name"}
	if _, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, resolutions); err == nil {
		t.Fatal("Preflight accepted duplicate preserve names")
	}
}

func TestPreflightCancelStopsPlanningAndSuggestionsAreUnique(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	makePlanSkill(t, destinations[0].Path, "review", "conflict")
	makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review-from-agents", "occupied")
	makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review-from-agents-2", "occupied")

	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Conflicts[0].SuggestedPreserveAs; got != "review-from-agents-3" {
		t.Fatalf("suggestion = %q", got)
	}

	resolution := []ConflictResolution{{DestinationPath: filepath.Join(destinations[0].Path, "review"), Action: ConflictCancel}}
	plan, err = Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Cancelled || len(plan.Migrations) != 0 || len(plan.Links) != 0 {
		t.Fatalf("cancelled plan = %#v", plan)
	}
}

func TestPreflightRejectsAmbiguousVariantsAndUnusedResolutions(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	other := candidate
	other.ID = "review:other"
	other.Fingerprint = "other"
	group := NameGroup{Name: "review", Variants: []Candidate{candidate, other}}

	if _, err := Preflight(cfg, []NameGroup{group}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID, other.ID}}, nil); err == nil {
		t.Fatal("Preflight accepted two selected variants for one name")
	}

	resolution := []ConflictResolution{{DestinationPath: filepath.Join(destinations[0].Path, "missing"), Action: ConflictKeep}}
	if _, err := Preflight(cfg, []NameGroup{group}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, resolution); err == nil {
		t.Fatal("Preflight accepted a resolution that does not match a conflict")
	}
}

func TestPreflightRejectsPreserveNameReservedByPlannedArchive(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	makePlanSkill(t, destinations[0].Path, "review", "conflict")
	resolution := []ConflictResolution{{
		DestinationPath: filepath.Join(destinations[0].Path, "review"),
		Action:          ConflictReplace,
		PreserveAs:      "review",
	}}

	if _, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, resolution); err == nil {
		t.Fatal("Preflight accepted preserve name reserved by its planned archive")
	}
}

func planFixture(t *testing.T, name string) (config.Config, Candidate, []roots.ActiveRoot) {
	t.Helper()
	base := t.TempDir()
	cfg := config.Default(filepath.Join(base, "project"), filepath.Join(base, "home"))
	destinations := []roots.ActiveRoot{
		{Scope: config.ScopeProject, Target: config.TargetAgents, Path: filepath.Join(cfg.ProjectRoot, ".agents", "skills"), Label: ".Ag"},
		{Scope: config.ScopeProject, Target: config.TargetClaude, Path: filepath.Join(cfg.ProjectRoot, ".claude", "skills"), Label: ".Cl"},
	}
	sourcePath := filepath.Join(cfg.ProjectRoot, ".codex", "skills", name)
	return cfg, Candidate{ID: name + ":candidate", Name: name, Occurrences: []actions.ActiveSkill{{Name: name, Path: sourcePath, Status: actions.StatusUnmanaged}}}, destinations
}

func makePlanSkill(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustPlanSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func mustPlanFingerprint(t *testing.T, path string) string {
	t.Helper()
	fp, err := fingerprint.Directory(path)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func snapshotPlanTree(t *testing.T, paths ...string) map[string]os.FileInfo {
	t.Helper()
	snapshot := map[string]os.FileInfo{}
	for _, root := range paths {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				snapshot[path] = info
			}
			return nil
		})
	}
	return snapshot
}

func assertPlanSnapshot(t *testing.T, before map[string]os.FileInfo, paths ...string) {
	t.Helper()
	after := snapshotPlanTree(t, paths...)
	if len(after) != len(before) {
		t.Fatalf("filesystem entry count changed: before %d, after %d", len(before), len(after))
	}
	for path, beforeInfo := range before {
		afterInfo, ok := after[path]
		if !ok || beforeInfo.Mode() != afterInfo.Mode() || beforeInfo.Size() != afterInfo.Size() {
			t.Fatalf("filesystem changed at %s", path)
		}
	}
}
