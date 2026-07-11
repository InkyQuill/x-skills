package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/syncer"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type syncStage int

const (
	syncStageDestinations syncStage = iota
	syncStageCandidates
	syncStageVariants
	syncStageConflicts
	syncStageConfirmation
)

type syncDestination struct {
	root    roots.ActiveRoot
	checked bool
}

type syncWorkbenchModal struct {
	stage         syncStage
	token         uint64
	destinations  []syncDestination
	groups        []syncer.NameGroup
	selected      map[string]bool
	variants      map[string]string
	conflictNames map[string]string
	plan          syncer.Plan
	index         int
	input         textinput.Model
	isEditing     bool
	isLoading     bool
	isApplying    bool
}

func newSyncWorkbenchModal(cfg config.Config) syncWorkbenchModal {
	roots := projectSyncRoots(cfg)
	destinations := make([]syncDestination, 0, len(roots))
	for _, root := range roots {
		destinations = append(destinations, syncDestination{root: root})
	}
	return syncWorkbenchModal{
		destinations: destinations,
		selected:     map[string]bool{},
		variants:     map[string]string{},
	}
}

func (w syncWorkbenchModal) Title() string { return "Sync project skills" }

func (w syncWorkbenchModal) View(width, height int, m Model) string {
	if width < 40 || height < 10 {
		return renderConstrainedModal(width, height, constrainedModalOptions{
			Title: w.Title(), Body: []string{"Terminal too small", "Resize to continue."},
			Footer: []string{"Esc cancel"},
		})
	}
	body := []string{fmt.Sprintf("Stage %d/5  %s", int(w.stage)+1, w.stageName())}
	switch w.stage {
	case syncStageDestinations:
		for i, destination := range w.destinations {
			body = append(body, w.checkLine(i, destination.checked, destination.root.Label+"  "+destination.root.Path, m))
		}
	case syncStageCandidates:
		for i, group := range w.groups {
			candidate := w.chosenCandidate(group)
			body = append(body, w.checkLine(i, w.groupSelected(group), group.Name+"  ["+string(candidate.Compatibility.State)+"]", m))
		}
	case syncStageVariants:
		groups := w.divergentGroups()
		if len(groups) == 0 {
			body = append(body, "No divergent variants.")
		} else {
			group := groups[w.index]
			body = append(body, "Choose "+group.Name+":")
			for _, candidate := range group.Variants {
				mark := " "
				if w.variants[group.Name] == candidate.ID {
					mark = "*"
				}
				body = append(body, fmt.Sprintf("  (%s) %s  [%s]  %s", mark, shortFingerprint(candidate.Fingerprint), candidate.Compatibility.State, candidateSourceLabels(candidate)))
			}
		}
	case syncStageConflicts:
		body = append(body, "Preserve destination conflicts")
		for i, conflict := range w.plan.Conflicts {
			body = append(body, w.cursorLine(i, conflict.Name+" → "+w.conflictNames[conflict.DestinationPath], m))
		}
		if w.isEditing {
			body = append(body, "", "Archive name", w.input.View())
		}
	case syncStageConfirmation:
		body = append(body, syncPlanLines(w.plan)...)
		body = append(body, w.compatibilityWarningLines()...)
		if w.isApplying {
			body = append(body, "", "Applying…")
		}
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: w.Title(), Body: body, Focus: w.index + 1,
		Footer: []string{mutedStyle.Render(renderCommandPalette(m.opts.ASCII, w.shortcuts()))},
	})
}

func (w syncWorkbenchModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if w.isApplying {
		if msg.String() == "esc" || msg.String() == "q" {
			m.cancelSyncWork()
			m.syncToken++
			return true, nil
		}
		return false, nil
	}
	if w.isLoading {
		if msg.String() == "esc" || msg.String() == "q" {
			m.cancelSyncWork()
			m.syncToken++
			return true, nil
		}
		return false, nil
	}
	if w.isEditing {
		return w.updateConflictInput(msg, m)
	}
	if msg.String() == "esc" || msg.String() == "q" {
		if w.stage == syncStageDestinations {
			m.cancelSyncWork()
			m.syncToken++
			return true, nil
		}
		w.back()
		w.index = 0
		m.modal = w
		return false, nil
	}
	if delta := modalMoveDelta(msg); delta != 0 {
		w.move(delta)
		m.modal = w
		return false, nil
	}
	switch msg.String() {
	case " ":
		w.toggle(m)
		m.modal = w
	case "e":
		if w.stage == syncStageConflicts && len(w.plan.Conflicts) > 0 {
			w.startConflictEdit(m)
		}
	case "enter":
		return false, w.advance(m)
	}
	return false, nil
}

func (w *syncWorkbenchModal) setCandidateDefaults() {
	w.selected = map[string]bool{}
	if w.variants == nil {
		w.variants = map[string]string{}
	}
	for _, group := range w.groups {
		for _, candidate := range group.Variants {
			if candidate.Compatibility.State != compatibility.StateIncompatible {
				w.selected[candidate.ID] = true
				w.variants[group.Name] = candidate.ID
				break
			}
		}
	}
}

func (w syncWorkbenchModal) selectedRoots() []roots.ActiveRoot {
	result := []roots.ActiveRoot{}
	for _, destination := range w.destinations {
		if destination.checked {
			result = append(result, destination.root)
		}
	}
	return result
}

func (w syncWorkbenchModal) selection() syncer.Selection {
	ids := []string{}
	variants := map[string]string{}
	for _, group := range w.groups {
		if !w.groupSelected(group) {
			continue
		}
		id := w.variants[group.Name]
		if id == "" {
			id = group.Variants[0].ID
		}
		ids = append(ids, id)
		if len(group.Variants) > 1 {
			variants[group.Name] = id
		}
	}
	return syncer.Selection{CandidateIDs: ids, VariantByName: variants}
}

func (w syncWorkbenchModal) resolutions() []syncer.ConflictResolution {
	result := []syncer.ConflictResolution{}
	for path, name := range w.conflictNames {
		result = append(result, syncer.ConflictResolution{DestinationPath: path, PreserveAs: name, Action: syncer.ConflictReplace})
	}
	return result
}

func (w syncWorkbenchModal) advance(m *Model) tea.Cmd {
	switch w.stage {
	case syncStageDestinations:
		if len(w.selectedRoots()) == 0 {
			m.status = "select at least one project Skills Folder"
			return nil
		}
		return m.beginSyncCandidates(w)
	case syncStageCandidates:
		if len(w.selection().CandidateIDs) == 0 {
			m.status = "select at least one skill"
			return nil
		}
		w.stage, w.index = syncStageVariants, 0
		if len(w.divergentGroups()) == 0 {
			return m.beginSyncPlan(w)
		}
		m.modal = w
	case syncStageVariants:
		return m.beginSyncPlan(w)
	case syncStageConflicts:
		return m.beginSyncPlan(w)
	case syncStageConfirmation:
		return m.beginSyncApply(w)
	}
	return nil
}

func (w *syncWorkbenchModal) toggle(m *Model) {
	switch w.stage {
	case syncStageDestinations:
		if len(w.destinations) > 0 {
			w.destinations[w.index].checked = !w.destinations[w.index].checked
			w.groups = nil
			w.selected = map[string]bool{}
			w.variants = map[string]string{}
			w.invalidatePlan()
		}
	case syncStageCandidates:
		if len(w.groups) == 0 {
			return
		}
		group := w.groups[w.index]
		if w.groupSelected(group) {
			for _, c := range group.Variants {
				w.selected[c.ID] = false
			}
			w.invalidatePlan()
			return
		}
		candidate := defaultSyncCandidate(group)
		w.selected[candidate.ID], w.variants[group.Name] = true, candidate.ID
		w.invalidatePlan()
	case syncStageVariants:
		groups := w.divergentGroups()
		if len(groups) == 0 {
			return
		}
		group := groups[w.index]
		current := w.variants[group.Name]
		for i, c := range group.Variants {
			if c.ID == current {
				next := group.Variants[(i+1)%len(group.Variants)]
				w.variants[group.Name] = next.ID
				for _, variant := range group.Variants {
					w.selected[variant.ID] = variant.ID == next.ID
				}
				w.invalidatePlan()
				return
			}
		}
	}
}

func (w *syncWorkbenchModal) move(delta int) {
	count := len(w.destinations)
	switch w.stage {
	case syncStageCandidates:
		count = len(w.groups)
	case syncStageVariants:
		count = len(w.divergentGroups())
	case syncStageConflicts:
		count = len(w.plan.Conflicts)
	}
	if count == 0 {
		w.index = 0
		return
	}
	w.index = (w.index + delta + count) % count
}

func (w syncWorkbenchModal) groupSelected(group syncer.NameGroup) bool {
	for _, c := range group.Variants {
		if w.selected[c.ID] {
			return true
		}
	}
	return false
}

func defaultSyncCandidate(group syncer.NameGroup) syncer.Candidate {
	for _, candidate := range group.Variants {
		if candidate.Compatibility.State != compatibility.StateIncompatible {
			return candidate
		}
	}
	return group.Variants[0]
}

func (w syncWorkbenchModal) chosenCandidate(group syncer.NameGroup) syncer.Candidate {
	id := w.variants[group.Name]
	for _, candidate := range group.Variants {
		if candidate.ID == id {
			return candidate
		}
	}
	return defaultSyncCandidate(group)
}

func (w *syncWorkbenchModal) invalidatePlan() {
	w.plan = syncer.Plan{}
	w.conflictNames = nil
}

func (w *syncWorkbenchModal) back() {
	switch w.stage {
	case syncStageCandidates:
		w.stage = syncStageDestinations
	case syncStageVariants:
		w.stage = syncStageCandidates
	case syncStageConflicts:
		w.stage = syncStageVariants
	case syncStageConfirmation:
		if len(w.plan.Conflicts) > 0 {
			w.stage = syncStageConflicts
		} else {
			w.stage = syncStageVariants
		}
	}
}
func (w syncWorkbenchModal) divergentGroups() []syncer.NameGroup {
	result := []syncer.NameGroup{}
	for _, g := range w.groups {
		if len(g.Variants) > 1 && w.groupSelected(g) {
			result = append(result, g)
		}
	}
	return result
}

func (w syncWorkbenchModal) compatibilityWarningLines() []string {
	lines := []string{}
	for _, group := range w.groups {
		if !w.groupSelected(group) {
			continue
		}
		candidate := w.chosenCandidate(group)
		if candidate.Compatibility.State == compatibility.StateCompatible {
			continue
		}
		lines = append(lines, "Compatibility warning: "+group.Name+" is "+string(candidate.Compatibility.State))
		for _, reason := range candidate.Compatibility.Reasons {
			lines = append(lines, "  "+reason)
		}
	}
	return lines
}
func (w syncWorkbenchModal) checkLine(i int, checked bool, label string, m Model) string {
	check := m.symbols.Unchecked
	if checked {
		check = m.symbols.Checked
	}
	return w.cursorLine(i, check+" "+label, m)
}
func (w syncWorkbenchModal) cursorLine(i int, label string, m Model) string {
	cursor := " "
	if i == w.index {
		cursor = m.symbols.Cursor
	}
	return "  " + cursor + " " + label
}

func (w syncWorkbenchModal) stageName() string {
	return []string{"Destinations", "Candidates", "Divergent variants", "Destination conflicts", "Confirmation / progress"}[w.stage]
}
func (w syncWorkbenchModal) shortcuts() []tuiui.Shortcut {
	return []tuiui.Shortcut{{ASCII: "up/down", Unicode: "↑/↓", Label: "move"}, {ASCII: "space", Label: "toggle/choose"}, {ASCII: "e", Label: "edit"}, {ASCII: "enter", Unicode: "↵", Label: "continue"}, {ASCII: "esc", Unicode: "Esc", Label: "back"}}
}

func (w syncWorkbenchModal) startConflictEdit(m *Model) {
	conflict := w.plan.Conflicts[w.index]
	input := textinput.New()
	input.SetValue(w.conflictNames[conflict.DestinationPath])
	input.Focus()
	w.input, w.isEditing = input, true
	m.modal = w
}
func (w syncWorkbenchModal) updateConflictInput(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if msg.String() == "esc" {
		w.isEditing = false
		m.modal = w
		return false, nil
	}
	if msg.String() == "enter" {
		name := strings.TrimSpace(w.input.Value())
		if name == "" {
			m.status = "archive name is required"
			return false, nil
		}
		w.conflictNames[w.plan.Conflicts[w.index].DestinationPath] = name
		w.isEditing = false
		m.modal = w
		return false, nil
	}
	var cmd tea.Cmd
	w.input, cmd = w.input.Update(msg)
	m.modal = w
	return false, cmd
}

func shortFingerprint(fp string) string {
	if len(fp) > 8 {
		return fp[:8]
	}
	return fp
}
func candidateSourceLabels(candidate syncer.Candidate) string {
	labels := []string{}
	for _, occurrence := range candidate.Occurrences {
		labels = append(labels, occurrence.Root.Label)
	}
	return strings.Join(labels, ", ")
}
func syncPlanLines(plan syncer.Plan) []string {
	lines := []string{fmt.Sprintf("Migrations (%d)", len(plan.Migrations))}
	for _, change := range plan.Migrations {
		lines = append(lines, "  "+change.Name+"  "+change.SourcePath+" → "+change.ArchivePath)
	}
	lines = append(lines, fmt.Sprintf("Links (%d)", len(plan.Links)))
	for _, change := range plan.Links {
		lines = append(lines, "  "+change.Name+"  "+change.Action+"  "+change.DestinationPath)
	}
	lines = append(lines, fmt.Sprintf("Preserved conflicts (%d)", len(plan.Conflicts)))
	for _, conflict := range plan.Conflicts {
		lines = append(lines, "  "+conflict.Name+"  "+conflict.DestinationPath+" → "+conflict.Resolution.PreserveAs)
	}
	lines = append(lines, fmt.Sprintf("Skips (%d)", len(plan.Skipped)))
	for _, skip := range plan.Skipped {
		lines = append(lines, "  "+skip.Name+"  "+skip.Reason+"  "+skip.DestinationPath)
	}
	return lines
}
