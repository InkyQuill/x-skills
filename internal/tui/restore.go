package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type restoreDestination struct {
	root    roots.ActiveRoot
	checked bool
}

type restoreWorkbenchModal struct {
	destinations []restoreDestination
	index        int
	full         bool
}

func newRestoreWorkbenchModal(cfg config.Config) restoreWorkbenchModal {
	projectRoots := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject})
	destinations := make([]restoreDestination, 0, len(projectRoots))
	for _, root := range projectRoots {
		destinations = append(destinations, restoreDestination{root: root, checked: true})
	}
	return restoreWorkbenchModal{destinations: destinations}
}

func (r restoreWorkbenchModal) Title() string { return "Restore project skills" }

func (r restoreWorkbenchModal) View(width, height int, m Model) string {
	body := []string{"Project Skills Folders"}
	for i, destination := range r.destinations {
		cursor, check := " ", m.symbols.Unchecked
		if i == r.index {
			cursor = m.symbols.Cursor
		}
		if destination.checked {
			check = m.symbols.Checked
		}
		body = append(body, fmt.Sprintf("  %s %s %s  %s", cursor, check, destination.root.Label, destination.root.Path))
	}
	full := "off"
	if r.full {
		full = "ON"
	}
	body = append(
		body,
		"",
		"Full restore: "+full,
		"Full restore may migrate or remove extra skills only in the selected folders.",
	)
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: r.Title(),
		Body:  body,
		Footer: []string{mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "move"},
			{ASCII: "space", Label: "toggle destination"},
			{ASCII: "f", Label: "full"},
			{ASCII: "enter", Unicode: "↵", Label: "plan"},
			{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
		}))},
		Focus: 1 + r.index,
	})
}

func (r restoreWorkbenchModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		r.index = clampModalIndex(r.index+delta, len(r.destinations))
		m.modal = r
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case " ":
		if len(r.destinations) > 0 {
			r.destinations[r.index].checked = !r.destinations[r.index].checked
			m.modal = r
		}
	case "f":
		r.full = !r.full
		m.modal = r
	case "enter":
		destinations := []roots.ActiveRoot{}
		for _, destination := range r.destinations {
			if destination.checked {
				destinations = append(destinations, destination.root)
			}
		}
		if len(destinations) == 0 {
			m.status = "select at least one project Skills Folder"
			return false, nil
		}
		m.modal = nil
		return false, m.beginRestorePlan(r, destinations, r.full)
	}
	return false, nil
}

type restorePlanMsg struct {
	token uint64
	plan  manifest.RestorePlan
	setup restoreWorkbenchModal
	err   error
}

type restoreApplyMsg struct {
	token     uint64
	result    manifest.RestoreResult
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
	reloadErr error
}

func (m *Model) beginRestorePlan(setup restoreWorkbenchModal, destinations []roots.ActiveRoot, full bool) tea.Cmd {
	m.cancelRestoreWork()
	m.restoreToken++
	token := m.restoreToken
	ctx, cancel := context.WithCancel(context.Background())
	m.restoreCancel = cancel
	m.restoreInFlight = true
	m.status = "planning project restore..."
	cfg := m.cfg
	return func() tea.Msg {
		defer cancel()
		plan, err := manifest.PlanRestore(ctx, cfg, manifest.RestoreRequest{Destinations: destinations, Full: full})
		return restorePlanMsg{token: token, plan: plan, setup: setup, err: err}
	}
}

func (m *Model) applyRestorePlanResult(msg restorePlanMsg) tea.Cmd {
	if msg.token != m.restoreToken {
		_ = msg.plan.Close()
		return nil
	}
	m.restoreInFlight = false
	m.restoreCancel = nil
	if msg.err != nil {
		_ = msg.plan.Close()
		m.status = "restore planning failed: " + msg.err.Error()
		return nil
	}
	m.modal = restorePlanModal{plan: msg.plan, setup: msg.setup}
	return nil
}

type restorePlanModal struct {
	plan          manifest.RestorePlan
	setup         restoreWorkbenchModal
	conflictIndex int
}

func (m *Model) openRestorePlan(plan manifest.RestorePlan) { m.modal = restorePlanModal{plan: plan} }

func (r restorePlanModal) Title() string { return "Project restore plan" }

func (r restorePlanModal) View(width, height int, m Model) string {
	body := restorePlanLines(r.plan)
	if len(r.plan.Conflicts) > 0 {
		body = append(body, "", "Rename decisions")
		for i, conflict := range r.plan.Conflicts {
			name := restoreArchiveName(r.plan, conflict.Path)
			if name == "" {
				name = conflict.SuggestedName + " (suggested)"
			}
			cursor := "  "
			if i == r.conflictIndex {
				cursor = m.symbols.Cursor + " "
			}
			body = append(body, cursor+conflict.Name+" → "+name)
		}
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: r.Title(), Body: body,
		Footer: []string{mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "e", Label: "edit rename"},
			{ASCII: "enter", Unicode: "↵", Label: "apply"},
			{ASCII: "esc", Unicode: "Esc", Label: "discard"},
		}))},
	})
}

func (r restorePlanModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 && len(r.plan.Conflicts) > 0 {
		r.conflictIndex += delta
		if r.conflictIndex < 0 {
			r.conflictIndex = len(r.plan.Conflicts) - 1
		}
		if r.conflictIndex >= len(r.plan.Conflicts) {
			r.conflictIndex = 0
		}
		m.modal = r
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		_ = r.plan.Close()
		m.modal = r.setup
		return false, nil
	case "e":
		if len(r.plan.Conflicts) > 0 {
			r.editConflict(m, r.conflictIndex)
		}
	case "enter":
		if len(r.plan.Conflicts) > 0 && !restoreConflictsResolved(r.plan) {
			index := firstUnresolvedRestoreConflict(r.plan)
			r.conflictIndex = index
			r.editConflict(m, index)
			return false, nil
		}
		if restorePlanIsDestructive(r.plan) {
			m.modal = restoreConfirmModal{parent: r, choice: 1}
			return false, nil
		}
		m.modal = nil
		return false, m.beginRestoreApply(r.plan)
	}
	return false, nil
}

func (r restorePlanModal) editConflict(m *Model, index int) {
	conflict := r.plan.Conflicts[index]
	value := restoreArchiveName(r.plan, conflict.Path)
	if value == "" {
		value = conflict.SuggestedName
	}
	input := textinput.New()
	input.SetValue(value)
	input.Focus()
	m.modal = restoreRenameModal{parent: r, conflictIndex: index, input: input}
}

type restoreRenameModal struct {
	parent        restorePlanModal
	conflictIndex int
	input         textinput.Model
}

func (r restoreRenameModal) Title() string {
	return "Preserve " + r.parent.plan.Conflicts[r.conflictIndex].Name
}

func (r restoreRenameModal) View(width, height int, m Model) string {
	r.input.Width = max(width-12, 1)
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: r.Title(),
		Body:  []string{"Archive name", r.input.View()},
		Footer: []string{mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "save"},
			{ASCII: "esc", Unicode: "Esc", Label: "back"},
		}))},
	})
}

func (r restoreRenameModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.modal = r.parent
		return false, nil
	case "enter":
		name := strings.TrimSpace(r.input.Value())
		if name == "" {
			m.status = "archive name is required"
			return false, nil
		}
		conflict := r.parent.plan.Conflicts[r.conflictIndex]
		setTUIRestoreArchiveName(&r.parent.plan, conflict.Path, name)
		m.modal = r.parent
		return false, nil
	}
	var cmd tea.Cmd
	r.input, cmd = r.input.Update(msg)
	m.modal = r
	return false, cmd
}

type restoreConfirmModal struct {
	parent restorePlanModal
	choice int
}

func (r restoreConfirmModal) Title() string { return "Confirm destructive restore" }

func (r restoreConfirmModal) View(width, height int, m Model) string {
	apply, back := "Apply", "[ Back ]"
	if r.choice == 0 {
		apply, back = "[ Apply ]", "Back"
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: r.Title(),
		Body:  restorePlanLines(r.parent.plan),
		Footer: []string{
			apply + "   " + back,
			mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
				{ASCII: "left/right", Unicode: "←/→", Label: "choose"},
				{ASCII: "enter", Unicode: "↵", Label: "select"},
				{ASCII: "esc", Unicode: "Esc", Label: "back"},
			})),
		},
	})
}

func (r restoreConfirmModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "n":
		m.modal = r.parent
		return false, nil
	case "left", "right":
		r.choice = 1 - r.choice
		m.modal = r
	case "y":
		m.modal = nil
		return false, m.beginRestoreApply(r.parent.plan)
	case "enter":
		if r.choice == 1 {
			m.modal = r.parent
			return false, nil
		}
		m.modal = nil
		return false, m.beginRestoreApply(r.parent.plan)
	}
	return false, nil
}

func (m *Model) beginRestoreApply(plan manifest.RestorePlan) tea.Cmd {
	m.cancelRestoreWork()
	m.restoreToken++
	token := m.restoreToken
	ctx, cancel := context.WithCancel(context.Background())
	m.restoreCancel = cancel
	m.restoreInFlight = true
	m.status = "applying project restore..."
	cfg := m.cfg
	return func() tea.Msg {
		defer cancel()
		result, err := manifest.ApplyRestore(ctx, cfg, plan)
		msg := restoreApplyMsg{token: token, result: result, err: err}
		msg.active, msg.repo, msg.issues, msg.repoUsage, msg.reloadErr = loadTUIData(cfg)
		return msg
	}
}

func (m *Model) applyRestoreResult(msg restoreApplyMsg) tea.Cmd {
	if msg.token != m.restoreToken {
		return nil
	}
	m.restoreInFlight = false
	m.restoreCancel = nil
	if msg.reloadErr == nil {
		m.active, m.repo, m.issues, m.repoUsage = msg.active, msg.repo, msg.issues, msg.repoUsage
		m.clampCursor()
	}
	if msg.err != nil {
		m.status = "restore failed: " + msg.err.Error()
		return nil
	}
	m.status = fmt.Sprintf(
		"restored %d links, %d migrations, %d removals",
		len(msg.result.Additions),
		len(msg.result.Normalizations),
		len(msg.result.Removals),
	)
	if msg.reloadErr != nil {
		m.status += "; refresh failed: " + msg.reloadErr.Error()
	}
	return nil
}

func (m *Model) cancelRestoreWork() {
	if m.restoreCancel != nil {
		m.restoreCancel()
		m.restoreCancel = nil
	}
	m.restoreInFlight = false
}

func restorePlanLines(plan manifest.RestorePlan) []string {
	lines := []string{"Available"}
	for _, skill := range plan.Available {
		lines = append(lines, "  "+skill.Name)
	}
	lines = append(lines, "Unavailable")
	for _, skill := range plan.Unavailable {
		lines = append(lines, "  "+skill.Name+"  "+skill.Reason)
	}
	lines = append(lines, "Links")
	for _, change := range plan.Additions {
		lines = append(lines, "  "+restoreTUIChangeLine(change))
	}
	lines = append(lines, "Migrations")
	changes := append(append([]manifest.Change{}, plan.Normalizations...), plan.Removals...)
	for _, change := range changes {
		if change.Kind == manifest.ChangeMigrate {
			lines = append(lines, "  "+restoreTUIChangeLine(change))
		}
	}
	lines = append(lines, "Removals")
	for _, change := range changes {
		if change.Kind != manifest.ChangeMigrate {
			lines = append(lines, "  "+restoreTUIChangeLine(change))
		}
	}
	if plan.RemovalsBlocked {
		lines = append(lines, "", "Unavailable skills block destructive migrations and removals.")
	}
	return lines
}

func restoreTUIChangeLine(change manifest.Change) string {
	line := change.Name + "  " + change.Destination.Label + "  " + change.Path
	if change.Kind == manifest.ChangeMigrate {
		archiveName := change.ArchiveName
		if archiveName == "" {
			archiveName = "(rename required)"
		}
		line += " -> archive:" + archiveName
	}
	return line
}

func restorePlanIsDestructive(plan manifest.RestorePlan) bool {
	return len(plan.Normalizations) > 0 || len(plan.Removals) > 0
}

func closeRestoreModalPlan(current modal) {
	switch current := current.(type) {
	case restorePlanModal:
		_ = current.plan.Close()
	case restoreRenameModal:
		_ = current.parent.plan.Close()
	case restoreConfirmModal:
		_ = current.parent.plan.Close()
	}
}

func restoreArchiveName(plan manifest.RestorePlan, path string) string {
	for _, changes := range [][]manifest.Change{plan.Normalizations, plan.Removals} {
		for _, change := range changes {
			if change.Path == path && change.Kind == manifest.ChangeMigrate {
				return change.ArchiveName
			}
		}
	}
	return ""
}

func setTUIRestoreArchiveName(plan *manifest.RestorePlan, path, name string) {
	for _, changes := range [][]manifest.Change{plan.Normalizations, plan.Removals} {
		for i := range changes {
			if changes[i].Path == path && changes[i].Kind == manifest.ChangeMigrate {
				changes[i].ArchiveName = name
			}
		}
	}
}

func restoreConflictsResolved(plan manifest.RestorePlan) bool {
	for _, conflict := range plan.Conflicts {
		if restoreArchiveName(plan, conflict.Path) == "" {
			return false
		}
	}
	return true
}

func firstUnresolvedRestoreConflict(plan manifest.RestorePlan) int {
	for i, conflict := range plan.Conflicts {
		if restoreArchiveName(plan, conflict.Path) == "" {
			return i
		}
	}
	return -1
}
