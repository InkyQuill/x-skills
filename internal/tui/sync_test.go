package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/syncer"
)

func TestSyncCandidateDefaultsAndBackNavigation(t *testing.T) {
	w := syncWorkbenchModal{stage: syncStageCandidates, groups: []syncer.NameGroup{
		{Name: "portable", Variants: []syncer.Candidate{{ID: "portable:a", Compatibility: compatibility.Assessment{State: compatibility.StateCompatible}}}},
		{Name: "native", Variants: []syncer.Candidate{{ID: "native:b", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}}}},
	}}
	w.setCandidateDefaults()
	if !w.selected["portable:a"] || w.selected["native:b"] {
		t.Fatalf("defaults = %#v", w.selected)
	}
	m := Model{modal: w, symbols: symbolsFor(defaultOptions())}
	w.Update(tea.KeyMsg{Type: tea.KeyEsc}, &m)
	got := m.modal.(syncWorkbenchModal)
	if got.stage != syncStageDestinations || !got.selected["portable:a"] {
		t.Fatalf("back state = %#v", got)
	}
}

func TestSyncCandidateDefaultAndRenderingUseChosenCompatibleVariant(t *testing.T) {
	group := syncer.NameGroup{Name: "mixed", Variants: []syncer.Candidate{
		{ID: "mixed:bad", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}},
		{ID: "mixed:good", Compatibility: compatibility.Assessment{State: compatibility.StateCompatible}},
	}}
	w := syncWorkbenchModal{stage: syncStageCandidates, groups: []syncer.NameGroup{group}}
	w.setCandidateDefaults()
	if !w.selected["mixed:good"] || w.selected["mixed:bad"] {
		t.Fatalf("selected = %#v", w.selected)
	}
	view := plain(w.View(80, 20, Model{symbols: symbolsFor(defaultOptions())}))
	if !strings.Contains(view, "[compatible]") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestSyncWorkbenchSmallTerminalRender(t *testing.T) {
	w := newSyncWorkbenchModal(config.Default(t.TempDir(), t.TempDir()))
	view := plain(w.View(36, 9, Model{symbols: symbolsFor(defaultOptions())}))
	if !strings.Contains(view, "Sync project skills") || len(strings.Split(view, "\n")) > 9 {
		t.Fatalf("small view:\n%s", view)
	}
}

func TestSyncMessagesAreGenerationGuarded(t *testing.T) {
	m := Model{syncToken: 4, modal: syncWorkbenchModal{token: 4}}
	updated, _ := m.Update(syncCandidatesMsg{token: 3, groups: []syncer.NameGroup{{Name: "stale"}}})
	got := updated.(Model).modal.(syncWorkbenchModal)
	if len(got.groups) != 0 {
		t.Fatal("stale candidates changed workbench")
	}
}

func TestSyncApplyEscapeCancelsAndInvalidatesGeneration(t *testing.T) {
	cancelled := false
	m := Model{syncToken: 7, syncInFlight: true, syncCancel: func() { cancelled = true }}
	w := syncWorkbenchModal{token: 7, stage: syncStageConfirmation, isApplying: true}
	m.modal = w
	closed, _ := w.Update(tea.KeyMsg{Type: tea.KeyEsc}, &m)
	if closed || !cancelled || m.syncToken != 7 || !strings.Contains(m.status, "cancelling") {
		t.Fatalf("closed=%v cancelled=%v token=%d", closed, cancelled, m.syncToken)
	}
}

func TestSyncPlanningErrorRestoresEditableStage(t *testing.T) {
	w := syncWorkbenchModal{token: 2, stage: syncStageVariants, isLoading: true}
	m := Model{syncToken: 2, syncInFlight: true, modal: w}
	m.applySyncPlan(syncPlanMsg{token: 2, err: errors.New("bad preserve name")})
	got := m.modal.(syncWorkbenchModal)
	if got.isLoading || got.stage != syncStageVariants {
		t.Fatalf("workbench after error = %#v", got)
	}
}

func TestSyncReplanClearsStaleConflictResolutions(t *testing.T) {
	w := syncWorkbenchModal{
		stage:    syncStageCandidates,
		groups:   []syncer.NameGroup{{Name: "skill", Variants: []syncer.Candidate{{ID: "skill:a"}}}},
		selected: map[string]bool{"skill:a": true}, variants: map[string]string{"skill": "skill:a"},
		conflictNames: map[string]string{"/old": "old-copy"}, plan: syncer.Plan{Conflicts: []syncer.Conflict{{DestinationPath: "/old"}}},
	}
	w.toggle(&Model{})
	if len(w.plan.Conflicts) != 0 {
		t.Fatalf("stale plan retained: %#v", w)
	}
	w.reconcileConflictNames(syncer.Plan{Conflicts: []syncer.Conflict{{DestinationPath: "/new", SuggestedPreserveAs: "new-copy"}}})
	if _, stale := w.conflictNames["/old"]; stale || w.conflictNames["/new"] != "new-copy" {
		t.Fatalf("reconciled names = %#v", w.conflictNames)
	}
	w.conflictNames["/new"] = "edited-copy"
	w.reconcileConflictNames(syncer.Plan{Conflicts: []syncer.Conflict{{DestinationPath: "/new", SuggestedPreserveAs: "new-suggestion"}}})
	if w.conflictNames["/new"] != "edited-copy" {
		t.Fatalf("matching edit lost: %#v", w.conflictNames)
	}
}

func TestSyncResultReportsManifestFailure(t *testing.T) {
	m := Model{syncToken: 1}
	m.applySyncResult(syncResultMsg{token: 1, result: syncer.Result{ManifestError: errors.New("write manifest")}})
	if !strings.Contains(m.status, "write manifest") || !strings.Contains(m.status, "failed") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestSyncConfirmationShowsExactChangesAndWarnings(t *testing.T) {
	w := syncWorkbenchModal{stage: syncStageConfirmation,
		groups:   []syncer.NameGroup{{Name: "legacy", Variants: []syncer.Candidate{{ID: "legacy:a", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}}}}},
		selected: map[string]bool{"legacy:a": true}, variants: map[string]string{"legacy": "legacy:a"},
		plan: syncer.Plan{Links: []syncer.Change{{Name: "legacy", DestinationPath: "/project/.agents/skills/legacy", Action: syncer.LinkCreate}}},
	}
	view := plain(w.View(100, 30, Model{symbols: symbolsFor(defaultOptions())}))
	for _, want := range []string{"legacy", "/project/.agents/skills/legacy", "incompatible"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestSyncWorkbenchEndToEndAppliesAndReconcilesManifest(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	source := filepath.Join(project, ".claude", "skills", "portable")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: portable\n---\nportable\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("S"))
	m = mustModel(t, updated)
	w := m.modal.(syncWorkbenchModal)
	w.destinations[0].checked = true
	m.modal = w
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("candidate command is nil")
	}
	updated, _ = m.Update(cmd())
	m = mustModel(t, updated)
	if m.modal.(syncWorkbenchModal).stage != syncStageCandidates {
		t.Fatal("did not reach candidates")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("plan command is nil")
	}
	updated, _ = m.Update(cmd())
	m = mustModel(t, updated)
	if m.modal.(syncWorkbenchModal).stage != syncStageConfirmation {
		t.Fatalf("stage = %v", m.modal.(syncWorkbenchModal).stage)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	for cmd != nil {
		updated, cmd = m.Update(cmd())
		m = mustModel(t, updated)
	}
	destination := filepath.Join(project, ".agents", "skills", "portable")
	if info, err := os.Lstat(destination); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("destination link: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".x-skills.local.yaml")); err != nil {
		t.Fatalf("local manifest: %v", err)
	}
}

func TestSyncLoadingPreventsDoubleSubmit(t *testing.T) {
	w := syncWorkbenchModal{stage: syncStageCandidates, isLoading: true}
	m := Model{modal: w}
	closed, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	if closed || cmd != nil {
		t.Fatalf("closed=%v cmd=%v", closed, cmd)
	}
}

func TestSyncStaleProgressDoesNotContinue(t *testing.T) {
	m := Model{syncToken: 3, status: "unchanged"}
	nextCalled := false
	cmd := m.applySyncProgress(syncProgressMsg{token: 2, next: func() tea.Msg { nextCalled = true; return nil }})
	if cmd != nil || nextCalled || m.status != "unchanged" {
		t.Fatal("stale progress was applied")
	}
}

func TestSyncContextAPIsHonorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.Default(t.TempDir(), t.TempDir())
	if _, err := syncer.DiscoverContext(ctx, cfg, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("discover error = %v", err)
	}
	if _, err := syncer.PreflightContext(ctx, cfg, nil, nil, syncer.Selection{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("preflight error = %v", err)
	}
}

func TestSyncMultipleGroupsAndConflictsKeepIndependentState(t *testing.T) {
	w := syncWorkbenchModal{
		stage: syncStageConflicts,
		groups: []syncer.NameGroup{
			{Name: "one", Variants: []syncer.Candidate{{ID: "one:a"}, {ID: "one:b"}}},
			{Name: "two", Variants: []syncer.Candidate{{ID: "two:c"}}},
		},
		selected: map[string]bool{"one:b": true, "two:c": true}, variants: map[string]string{"one": "one:b", "two": "two:c"},
		plan:          syncer.Plan{Conflicts: []syncer.Conflict{{Name: "one", DestinationPath: "/one"}, {Name: "two", DestinationPath: "/two"}}},
		conflictNames: map[string]string{"/one": "one-copy", "/two": "two-copy"}, index: 1,
	}
	selection := w.selection()
	if strings.Join(selection.CandidateIDs, ",") != "one:b,two:c" {
		t.Fatalf("selection = %#v", selection)
	}
	m := Model{modal: w}
	w.startConflictEdit(&m)
	editor := m.modal.(syncWorkbenchModal)
	editor.input.SetValue("two-preserved")
	editor.updateConflictInput(tea.KeyMsg{Type: tea.KeyEnter}, &m)
	got := m.modal.(syncWorkbenchModal)
	if got.conflictNames["/one"] != "one-copy" || got.conflictNames["/two"] != "two-preserved" {
		t.Fatalf("names = %#v", got.conflictNames)
	}
}

func TestSyncStalePlanAndResultAreDiscarded(t *testing.T) {
	m := Model{syncToken: 5, modal: syncWorkbenchModal{token: 5, stage: syncStageCandidates}, status: "current"}
	m.applySyncPlan(syncPlanMsg{token: 4, plan: syncer.Plan{Links: []syncer.Change{{Name: "stale"}}}})
	if m.modal.(syncWorkbenchModal).stage != syncStageCandidates {
		t.Fatal("stale plan applied")
	}
	m.applySyncResult(syncResultMsg{token: 4, result: syncer.Result{ManifestError: errors.New("stale")}})
	if m.status != "current" {
		t.Fatalf("stale result status = %q", m.status)
	}
}

func TestSyncCurrentProgressUpdatesStatusAndContinues(t *testing.T) {
	m := Model{syncToken: 2}
	next := func() tea.Msg { return nil }
	cmd := m.applySyncProgress(syncProgressMsg{token: 2, progress: syncer.Progress{Completed: 1, Total: 3, Skill: "one", Action: "link"}, next: next})
	if cmd == nil || !strings.Contains(m.status, "1/3") || !strings.Contains(m.status, "one") {
		t.Fatalf("status=%q cmd=%v", m.status, cmd)
	}
}

func TestSyncEarlierStageDoesNotSubmitStaleConflictResolutions(t *testing.T) {
	w := syncWorkbenchModal{stage: syncStageVariants, conflictNames: map[string]string{"/old": "old-copy"}}
	if got := w.resolutionsForPlan(); got != nil {
		t.Fatalf("resolutions = %#v", got)
	}
	w.stage = syncStageConflicts
	if got := w.resolutionsForPlan(); len(got) != 1 {
		t.Fatalf("conflict resolutions = %#v", got)
	}
}

func TestSyncDiscoveryErrorRestoresDestinationStage(t *testing.T) {
	w := syncWorkbenchModal{token: 2, stage: syncStageDestinations, isLoading: true}
	m := Model{syncToken: 2, syncInFlight: true, modal: w}
	m.applySyncCandidates(syncCandidatesMsg{token: 2, err: errors.New("scan denied")})
	got := m.modal.(syncWorkbenchModal)
	if got.isLoading || got.stage != syncStageDestinations || !strings.Contains(m.status, "scan denied") {
		t.Fatalf("modal=%#v status=%q", got, m.status)
	}
}

func TestSyncCancelledResultReportsPartialOutcome(t *testing.T) {
	m := Model{syncToken: 3}
	m.applySyncResult(syncResultMsg{token: 3, result: syncer.Result{Cancelled: true, Succeeded: []syncer.SkillResult{{Name: "one"}}, Failed: []syncer.SkillResult{{Name: "two"}}}})
	if !strings.Contains(m.status, "after 1 skills") || !strings.Contains(m.status, "1 failed") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestSyncResultReportsReloadErrorWithSuccessfulCounts(t *testing.T) {
	m := Model{syncToken: 3}
	m.applySyncResult(syncResultMsg{token: 3, result: syncer.Result{Succeeded: []syncer.SkillResult{{Name: "one"}}}, reloadErr: errors.New("reload failed")})
	if !strings.Contains(m.status, "synced 1") || !strings.Contains(m.status, "reload failed") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestSyncMultipleDivergentGroupsNavigateIndependently(t *testing.T) {
	w := syncWorkbenchModal{stage: syncStageVariants, groups: []syncer.NameGroup{
		{Name: "one", Variants: []syncer.Candidate{{ID: "one:a"}, {ID: "one:b"}}},
		{Name: "two", Variants: []syncer.Candidate{{ID: "two:a"}, {ID: "two:b"}}},
	}, selected: map[string]bool{"one:a": true, "two:a": true}, variants: map[string]string{"one": "one:a", "two": "two:a"}}
	w.move(1)
	if w.index != 1 {
		t.Fatalf("index = %d", w.index)
	}
	w.toggle(&Model{})
	if w.variants["one"] != "one:a" || w.variants["two"] != "two:b" {
		t.Fatalf("variants = %#v", w.variants)
	}
}
