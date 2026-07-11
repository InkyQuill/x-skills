package tui

import (
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
