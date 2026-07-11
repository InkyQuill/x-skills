package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/syncer"
)

type syncCandidatesMsg struct {
	token  uint64
	groups []syncer.NameGroup
	err    error
}

type syncPlanMsg struct {
	token uint64
	plan  syncer.Plan
	err   error
}

type syncProgressMsg struct {
	token    uint64
	progress syncer.Progress
	next     tea.Cmd
}

type syncResultMsg struct {
	token     uint64
	result    syncer.Result
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	reloadErr error
}

func (m *Model) beginSyncCandidates(workbench syncWorkbenchModal) tea.Cmd {
	m.cancelSyncWork()
	m.syncToken++
	workbench.token = m.syncToken
	m.modal = workbench
	ctx, cancel := context.WithCancel(context.Background())
	m.syncCancel = cancel
	m.syncInFlight = true
	m.status = "scanning project Skills Folders..."
	workbench.isLoading = true
	m.modal = workbench
	cfg, destinations, token := m.cfg, workbench.selectedRoots(), m.syncToken
	return func() tea.Msg {
		groups, err := syncer.DiscoverContext(ctx, cfg, destinations)
		return syncCandidatesMsg{token: token, groups: groups, err: err}
	}
}

func (m *Model) applySyncCandidates(msg syncCandidatesMsg) tea.Cmd {
	if msg.token != m.syncToken {
		return nil
	}
	m.cancelSyncWork()
	w, ok := m.modal.(syncWorkbenchModal)
	if !ok || w.token != msg.token {
		return nil
	}
	if msg.err != nil {
		w.isLoading = false
		m.modal = w
		m.status = "sync scan failed: " + msg.err.Error()
		return nil
	}
	w.groups, w.stage, w.index = msg.groups, syncStageCandidates, 0
	w.isLoading = false
	w.setCandidateDefaults()
	m.modal = w
	m.status = ""
	return nil
}

func (m *Model) beginSyncPlan(workbench syncWorkbenchModal) tea.Cmd {
	m.cancelSyncWork()
	m.syncToken++
	workbench.token = m.syncToken
	m.modal = workbench
	ctx, cancel := context.WithCancel(context.Background())
	m.syncCancel = cancel
	m.syncInFlight = true
	m.status = "planning skill sync..."
	workbench.isLoading = true
	m.modal = workbench
	cfg, token := m.cfg, m.syncToken
	groups, destinations := workbench.groups, workbench.selectedRoots()
	selection, resolutions := workbench.selection(), workbench.resolutionsForPlan()
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return syncPlanMsg{token: token, err: ctx.Err()}
		default:
		}
		plan, err := syncer.PreflightContext(ctx, cfg, groups, destinations, selection, resolutions)
		return syncPlanMsg{token: token, plan: plan, err: err}
	}
}

func (m *Model) applySyncPlan(msg syncPlanMsg) tea.Cmd {
	if msg.token != m.syncToken {
		return nil
	}
	m.cancelSyncWork()
	w, ok := m.modal.(syncWorkbenchModal)
	if !ok || w.token != msg.token {
		return nil
	}
	if msg.err != nil {
		w.isLoading = false
		m.modal = w
		m.status = "sync planning failed: " + msg.err.Error()
		return nil
	}
	w.plan = msg.plan
	w.isLoading = false
	if syncPlanHasUnresolvedConflicts(msg.plan) {
		w.reconcileConflictNames(msg.plan)
		w.stage, w.index = syncStageConflicts, 0
	} else {
		w.stage, w.index = syncStageConfirmation, 0
	}
	m.modal = w
	m.status = ""
	return nil
}

func syncPlanHasUnresolvedConflicts(plan syncer.Plan) bool {
	for _, conflict := range plan.Conflicts {
		if conflict.Resolution.Action == "" {
			return true
		}
	}
	return false
}

func (m *Model) beginSyncApply(workbench syncWorkbenchModal) tea.Cmd {
	m.cancelSyncWork()
	m.syncToken++
	workbench.token, workbench.isApplying = m.syncToken, true
	m.modal = workbench
	ctx, cancel := context.WithCancel(context.Background())
	m.syncCancel = cancel
	m.syncInFlight = true
	m.status = "applying skill sync..."
	cfg, plan, token := m.cfg, workbench.plan, m.syncToken
	messages := make(chan tea.Msg, len(plan.Migrations)+len(plan.Links)+len(plan.Conflicts)+1)
	wait := syncWaitCmd(messages)
	return func() tea.Msg {
		go func() {
			result := syncer.Apply(ctx, cfg, plan, syncer.ApplyOptions{Progress: func(progress syncer.Progress) {
				select {
				case messages <- syncProgressMsg{token: token, progress: progress, next: wait}:
				case <-ctx.Done():
				}
			}})
			msg := syncResultMsg{token: token, result: result}
			if ctx.Err() == nil {
				msg.active, msg.repo, msg.issues, msg.repoUsage, msg.reloadErr = loadTUIData(cfg)
			}
			messages <- msg
		}()
		return <-messages
	}
}

func syncWaitCmd(messages <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-messages }
}

func (m *Model) applySyncProgress(msg syncProgressMsg) tea.Cmd {
	if msg.token == m.syncToken {
		m.status = fmt.Sprintf("sync %d/%d: %s %s", msg.progress.Completed, msg.progress.Total, msg.progress.Action, msg.progress.Skill)
		return msg.next
	}
	return nil
}

func (m *Model) applySyncResult(msg syncResultMsg) tea.Cmd {
	if msg.token != m.syncToken {
		return nil
	}
	if m.syncCancel != nil {
		m.syncCancel()
	}
	m.syncInFlight = false
	m.syncCancel = nil
	if msg.reloadErr == nil {
		m.active, m.repo, m.issues, m.repoUsage = msg.active, msg.repo, msg.issues, msg.repoUsage
		m.clampCursor()
	}
	m.modal = nil
	if msg.result.PlanError != nil {
		m.status = "sync failed: " + msg.result.PlanError.Error()
		return nil
	}
	if msg.result.ManifestError != nil {
		m.status = "sync failed: " + msg.result.ManifestError.Error()
		return nil
	}
	m.status = fmt.Sprintf("synced %d skills; %d failed", len(msg.result.Succeeded), len(msg.result.Failed))
	if msg.result.Cancelled {
		m.status = fmt.Sprintf("skill sync cancelled after %d skills; %d failed", len(msg.result.Succeeded), len(msg.result.Failed))
	}
	if msg.reloadErr != nil {
		m.status += "; refresh failed: " + msg.reloadErr.Error()
	}
	return nil
}

func (m *Model) cancelSyncWork() {
	if m.syncCancel != nil {
		m.syncCancel()
		m.syncCancel = nil
	}
	m.syncInFlight = false
}

func projectSyncRoots(cfg config.Config) []roots.ActiveRoot {
	return roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject})
}
