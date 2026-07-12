package syncer

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	bindPlanCandidate(&candidate)
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
	bindPlanCandidate(&candidate)
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
	bindPlanCandidate(&candidate)
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
	bindPlanCandidate(&candidate)
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
	bindPlanCandidate(&candidate)
	makePlanSkill(t, destinations[0].Path, "review", "conflict")
	makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review-from-agents", "occupied")
	makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review-from-agents-2", "occupied")
	before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)

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
	assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
}

func TestPreflightRejectsAmbiguousVariantsAndUnusedResolutions(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	bindPlanCandidate(&candidate)
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
	bindPlanCandidate(&candidate)
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

func TestPreflightRevalidatesCandidateIdentityAndOccurrences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*NameGroup)
	}{
		{name: "group name differs", mutate: func(group *NameGroup) { group.Name = "other" }},
		{name: "forged ID", mutate: func(group *NameGroup) { group.Variants[0].ID = "review:forged" }},
		{name: "occurrence name differs", mutate: func(group *NameGroup) { group.Variants[0].Occurrences[0].Name = "other" }},
		{name: "occurrence content drifted", mutate: func(group *NameGroup) {
			_ = os.WriteFile(filepath.Join(group.Variants[0].Occurrences[0].Path, "SKILL.md"), []byte("changed"), 0o644)
		}},
		{name: "managed occurrence outside archive", mutate: func(group *NameGroup) { group.Variants[0].Occurrences[0].Status = actions.StatusManaged }},
		{name: "occurrence outside declared Skills Folder", mutate: func(group *NameGroup) {
			group.Variants[0].Occurrences[0].Root.Path = filepath.Dir(filepath.Dir(group.Variants[0].Occurrences[0].Root.Path))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, candidate, destinations := planFixture(t, "review")
			source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
			candidate.Occurrences[0].Path = source
			candidate.Fingerprint = mustPlanFingerprint(t, source)
			bindPlanCandidate(&candidate)
			group := NameGroup{Name: "review", Variants: []Candidate{candidate}}
			tt.mutate(&group)
			if _, err := Preflight(cfg, []NameGroup{group}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil); err == nil {
				t.Fatal("Preflight accepted stale or inconsistent candidate data")
			}
		})
	}
}

func TestPreflightRejectsDuplicateGroupsAndCandidateIDs(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	bindPlanCandidate(&candidate)
	group := NameGroup{Name: "review", Variants: []Candidate{candidate}}

	if _, err := Preflight(cfg, []NameGroup{group, group}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil); err == nil {
		t.Fatal("Preflight accepted duplicate name groups and candidate IDs")
	}
	otherGroup := NameGroup{Name: "other", Variants: []Candidate{{ID: candidate.ID, Name: "other", Fingerprint: candidate.Fingerprint, Occurrences: candidate.Occurrences}}}
	if _, err := Preflight(cfg, []NameGroup{group, otherGroup}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil); err == nil {
		t.Fatal("Preflight accepted a candidate ID reused across groups")
	}
}

func TestPreflightRequiresExplicitResolutionForDivergentArchive(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	bindPlanCandidate(&candidate)
	archive := makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review", "different archive")
	destination := filepath.Join(destinations[0].Path, "review")
	mustPlanSymlink(t, archive, destination)
	before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)

	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Migrations) != 0 || len(plan.Skipped) != 0 || len(plan.Conflicts) != 1 || plan.Conflicts[0].DestinationPath != archive {
		t.Fatalf("divergent archive plan = %#v", plan)
	}
	if plan.Conflicts[0].SuggestedPreserveAs != "review-from-archive" {
		t.Fatalf("archive suggestion = %q", plan.Conflicts[0].SuggestedPreserveAs)
	}
	assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)

	resolution := []ConflictResolution{{DestinationPath: archive, Action: ConflictReplace, PreserveAs: "review-preserved"}}
	plan, err = Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Migrations) != 1 || len(plan.Conflicts) != 1 || plan.Conflicts[0].Resolution.Action != ConflictReplace {
		t.Fatalf("resolved archive plan = %#v", plan)
	}
	if len(plan.Skipped) != 0 {
		t.Fatalf("changed archive was incorrectly treated as managed no-op: %#v", plan.Skipped)
	}
	assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
}

func TestPreflightValidatesDestinationSetBeforePlanning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(t *testing.T, cfg config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot
	}{
		{name: "duplicate", mutate: func(_ *testing.T, _ config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot {
			return []roots.ActiveRoot{destinations[0], destinations[0]}
		}},
		{name: "canonical alias", mutate: func(t *testing.T, cfg config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot {
			alias := filepath.Join(filepath.Dir(cfg.ProjectRoot), "agents-alias")
			if err := os.MkdirAll(destinations[0].Path, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(destinations[0].Path, alias); err != nil {
				t.Fatal(err)
			}
			aliased := destinations[0]
			aliased.Path = alias
			return []roots.ActiveRoot{destinations[0], aliased}
		}},
		{name: "outside configured project roots", mutate: func(t *testing.T, cfg config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot {
			outside := destinations[0]
			outside.Path = filepath.Join(cfg.HomeDir, "outside")
			return []roots.ActiveRoot{outside}
		}},
		{name: "root is file", mutate: func(t *testing.T, _ config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot {
			if err := os.MkdirAll(filepath.Dir(destinations[0].Path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destinations[0].Path, []byte("file"), 0o644); err != nil {
				t.Fatal(err)
			}
			return destinations[:1]
		}},
		{name: "existing ancestor is file", mutate: func(t *testing.T, cfg config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot {
			if err := os.MkdirAll(cfg.ProjectRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cfg.ProjectRoot, ".agents"), []byte("file"), 0o644); err != nil {
				t.Fatal(err)
			}
			return destinations[:1]
		}},
		{name: "unwritable root", mutate: func(t *testing.T, _ config.Config, destinations []roots.ActiveRoot) []roots.ActiveRoot {
			if err := os.MkdirAll(destinations[0].Path, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(destinations[0].Path, 0o555); err != nil {
				t.Fatal(err)
			}
			return destinations[:1]
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, candidate, destinations := planFixture(t, "review")
			source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
			candidate.Occurrences[0].Path = source
			candidate.Fingerprint = mustPlanFingerprint(t, source)
			bindPlanCandidate(&candidate)
			destinations = tt.mutate(t, cfg, destinations)
			if _, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, nil); err == nil {
				t.Fatal("Preflight accepted invalid destination set")
			}
		})
	}
}

func TestPreflightRejectsOverlappingConfiguredDestinations(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cfg := config.Default(filepath.Join(base, "project"), filepath.Join(base, "home"))
	if err := os.MkdirAll(filepath.Join(cfg.HomeDir, ".x-skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	configData := "version: 1\nactive_roots:\n" +
		"  - scope: project\n    target: agents\n    path: .skills\n" +
		"  - scope: project\n    target: claude\n    path: .skills/nested\n"
	if err := os.WriteFile(filepath.Join(cfg.HomeDir, ".x-skills", "config.yaml"), []byte(configData), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	destinations := roots.ActiveRoots(loaded, roots.Filter{Scope: config.ScopeProject})[:2]
	source := makePlanSkill(t, filepath.Join(loaded.ProjectRoot, ".codex", "skills"), "review", "selected")
	var sourceRoot roots.ActiveRoot
	for _, root := range roots.ActiveRoots(loaded, roots.Filter{Scope: config.ScopeProject}) {
		if root.Target == config.TargetCodex {
			sourceRoot = root
		}
	}
	candidate := Candidate{Name: "review", Occurrences: []actions.ActiveSkill{{Name: "review", Root: sourceRoot, Path: source, Status: actions.StatusUnmanaged}}}
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	bindPlanCandidate(&candidate)

	if _, err := Preflight(loaded, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations, Selection{CandidateIDs: []string{candidate.ID}}, nil); err == nil {
		t.Fatal("Preflight accepted overlapping destination roots")
	}
}

func TestPreflightValidatesVariantByNameStructure(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	bindPlanCandidate(&candidate)
	groups := []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}

	tests := []Selection{
		{VariantByName: map[string]string{"review": "review:unknown"}},
		{VariantByName: map[string]string{"other": candidate.ID}},
	}
	for _, selection := range tests {
		if _, err := Preflight(cfg, groups, destinations[:1], selection, nil); err == nil {
			t.Fatalf("Preflight accepted invalid variant selection %#v", selection)
		}
	}
}

func TestPreflightRejectsSelectedDestinationAsCandidateSourceIncludingAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		alias bool
	}{
		{name: "direct destination root"},
		{name: "aliased destination root", alias: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, _, destinations := planFixture(t, "review")
			source := makePlanSkill(t, destinations[0].Path, "review", "selected")
			sourceRoot := destinations[0]
			if tt.alias {
				alias := filepath.Join(filepath.Dir(cfg.ProjectRoot), "agents-source-alias")
				if err := os.Symlink(destinations[0].Path, alias); err != nil {
					t.Fatal(err)
				}
				sourceRoot.Path = alias
				source = filepath.Join(alias, "review")
			}
			candidate := Candidate{Name: "review", Occurrences: []actions.ActiveSkill{{
				Name: "review", Root: sourceRoot, Path: source, Status: actions.StatusUnmanaged,
			}}}
			candidate.Fingerprint = mustPlanFingerprint(t, source)
			bindPlanCandidate(&candidate)
			before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)

			plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil)
			if err == nil {
				t.Fatalf("Preflight accepted selected destination as source: %#v", plan)
			}
			assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
		})
	}
}

func TestPreflightRejectsArchiveStorageThatCannotPublishMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T, cfg config.Config)
	}{
		{name: "archive Skills Folder is file", setup: func(t *testing.T, cfg config.Config) {
			if err := os.MkdirAll(cfg.ArchiveRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfg.ArchiveSkillsRoot(), []byte("file"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "archive Skills Folder is unwritable", setup: func(t *testing.T, cfg config.Config) {
			if err := os.MkdirAll(cfg.ArchiveSkillsRoot(), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(cfg.ArchiveSkillsRoot(), 0o555); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "nearest archive ancestor is unwritable", setup: func(t *testing.T, cfg config.Config) {
			if err := os.MkdirAll(cfg.ArchiveRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(cfg.ArchiveRoot, 0o555); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, candidate, destinations := planFixture(t, "review")
			source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
			candidate.Occurrences[0].Path = source
			candidate.Fingerprint = mustPlanFingerprint(t, source)
			bindPlanCandidate(&candidate)
			tt.setup(t, cfg)
			before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)

			plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, nil)
			if err == nil {
				t.Fatalf("Preflight emitted plan for unusable archive storage: %#v", plan)
			}
			assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
		})
	}
}

func TestPreflightRejectsUnwritableArchiveStorageForPreservation(t *testing.T) {
	t.Parallel()

	cfg, candidate, destinations := planFixture(t, "review")
	source := makePlanSkill(t, filepath.Dir(candidate.Occurrences[0].Path), "review", "selected")
	candidate.Occurrences[0].Path = source
	candidate.Fingerprint = mustPlanFingerprint(t, source)
	bindPlanCandidate(&candidate)
	makePlanSkill(t, cfg.ArchiveSkillsRoot(), "review", "selected")
	makePlanSkill(t, destinations[0].Path, "review", "destination conflict")
	if err := os.Chmod(cfg.ArchiveSkillsRoot(), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(cfg.ArchiveSkillsRoot(), 0o755)
	})
	before := snapshotPlanTree(t, cfg.ProjectRoot, cfg.ArchiveRoot)
	resolution := []ConflictResolution{{
		DestinationPath: filepath.Join(destinations[0].Path, "review"),
		Action:          ConflictReplace,
		PreserveAs:      "review-from-agents",
	}}

	plan, err := Preflight(cfg, []NameGroup{{Name: "review", Variants: []Candidate{candidate}}}, destinations[:1], Selection{CandidateIDs: []string{candidate.ID}}, resolution)
	if err == nil {
		t.Fatalf("Preflight emitted preservation plan for unwritable archive storage: %#v", plan)
	}
	assertPlanSnapshot(t, before, cfg.ProjectRoot, cfg.ArchiveRoot)
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
	sourceRoot := roots.ActiveRoot{Scope: config.ScopeProject, Target: config.TargetCodex, Path: filepath.Dir(sourcePath), Label: ".Cd"}
	return cfg, Candidate{ID: name + ":candidate", Name: name, Occurrences: []actions.ActiveSkill{{Name: name, Root: sourceRoot, Path: sourcePath, Status: actions.StatusUnmanaged}}}, destinations
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

func bindPlanCandidate(candidate *Candidate) {
	candidate.ID = candidate.Name + ":" + candidate.Fingerprint
}

type planSnapshotEntry struct {
	mode    os.FileMode
	size    int64
	modTime int64
	content string
}

func snapshotPlanTree(t *testing.T, paths ...string) map[string]planSnapshotEntry {
	t.Helper()
	snapshot := map[string]planSnapshotEntry{}
	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) && path == root {
					return nil
				}
				return err
			}
			entry := planSnapshotEntry{mode: info.Mode(), size: info.Size(), modTime: info.ModTime().UnixNano()}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				target, readErr := os.Readlink(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				entry.content = "link:" + target
			case info.Mode().IsRegular():
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				entry.content = fmt.Sprintf("sha256:%x", sha256.Sum256(content))
			}
			snapshot[path] = entry
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func TestClassifyDestinationRejectsBrokenSymlink(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	path := filepath.Join(cfg.ProjectRoot, "broken")
	if err := os.Symlink(filepath.Join(cfg.ProjectRoot, "missing"), path); err != nil {
		t.Fatal(err)
	}
	_, err := classifyDestination(cfg, path, filepath.Join(cfg.ArchiveSkillsRoot(), "review"), "candidate", false, false)
	if err == nil || !strings.Contains(err.Error(), "resolve destination symlink") {
		t.Fatalf("error = %v, want actionable broken symlink error", err)
	}
}

func TestClassifyDestinationRejectsSymlinkToNonDirectory(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	file := filepath.Join(cfg.ProjectRoot, "regular-file")
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.ProjectRoot, "review")
	if err := os.Symlink(file, path); err != nil {
		t.Fatal(err)
	}
	_, err := classifyDestination(cfg, path, filepath.Join(cfg.ArchiveSkillsRoot(), "review"), "candidate", false, false)
	if err == nil || !strings.Contains(err.Error(), "symlink to non-directory") {
		t.Fatalf("error = %v, want symlink to non-directory rejection", err)
	}
}

func TestArchivePathMatchesFingerprintRejectsSymlinkedArchiveEntry(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	external := makePlanSkill(t, t.TempDir(), "external", "external")
	if err := os.MkdirAll(cfg.ArchiveSkillsRoot(), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "external")
	if err := os.Symlink(external, archive); err != nil {
		t.Fatal(err)
	}
	_, err := archivePathMatchesFingerprint(cfg, archive, mustPlanFingerprint(t, external))
	if err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("error = %v, want symlinked archive rejection", err)
	}
}

func assertPlanSnapshot(t *testing.T, before map[string]planSnapshotEntry, paths ...string) {
	t.Helper()
	after := snapshotPlanTree(t, paths...)
	if len(after) != len(before) {
		t.Fatalf("filesystem entry count changed: before %d, after %d", len(before), len(after))
	}
	for path, beforeEntry := range before {
		afterEntry, ok := after[path]
		if !ok || beforeEntry != afterEntry {
			t.Fatalf("filesystem changed at %s", path)
		}
	}
}
