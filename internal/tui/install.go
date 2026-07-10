package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	tea "github.com/charmbracelet/bubbletea"
)

type installInputMode int

const (
	installInputNone installInputMode = iota
	installInputQuery
	installInputOwner
)

type installState struct {
	Query                string
	Owner                string
	Searching            bool
	Results              []installResultView
	Audit                map[string]remote.AuditSummary
	Message              string
	InputMode            installInputMode
	searchToken          int
	previewToken         int
	archiveToken         int
	archiveInFlight      bool
	archiveInFlightToken int
	useToken             int
	useGeneration        *installUseGeneration
	useInFlight          bool
	useInFlightToken     int
	searchClient         remote.SearchClient
	checkouts            *remote.CheckoutCache
	testCloneURL         string
	pendingUse           *pendingInstallUseIntent
	pendingArchiveBatch  *installArchiveBatchContinuation
	pendingUseBatch      *installUseBatchContinuation
}

type installUseGeneration struct {
	value atomic.Int64
}

func (g *installUseGeneration) next() int {
	return int(g.value.Add(1))
}

func (g *installUseGeneration) isCurrent(token int) bool {
	return token != 0 && g.value.Load() == int64(token)
}

type installSearchResultMsg struct {
	token   int
	query   string
	results []remote.SearchResult
	err     error
}

type installPreviewMsg struct {
	token int
	name  string
	path  string
	err   error
}

type installUpdateDiffMsg struct {
	token int
	row   installResultView
	diff  directoryDiff
	err   error
}

type installArchiveMsg struct {
	token        int
	name         string
	identity     installArchiveIdentity
	archiveState string
	batch        *installArchiveBatchResult
	err          error
}

type installArchiveStateMsg struct {
	token    int
	identity installArchiveIdentity
	state    string
}

type installUseMsg struct {
	token        int
	name         string
	row          installResultView
	identity     installArchiveIdentity
	archiveState string
	destinations []installDestination
	batch        *installUseBatchResult
	stale        bool
	err          error
}

type installArchiveIdentity struct {
	name  string
	owner string
	repo  string
	path  string
}

type installArchiveBatchResult struct {
	total    int
	success  []string
	failures []string
	next     *installArchiveBatchNext
}

type installArchiveBatchNext struct {
	row       installResultView
	action    string
	remaining []installResultView
}

type installUseBatchResult struct {
	total    int
	success  []string
	failures []string
	next     *installUseBatchNext
}

type installUseBatchNext struct {
	row       installResultView
	remaining []installResultView
}

type pendingInstallUseIntent struct {
	row         installResultView
	updateToken int
}

type installArchiveBatchContinuation struct {
	identity    installArchiveIdentity
	updateToken int
	total       int
	remaining   []installResultView
	success     []string
	failures    []string
}

type installUseBatchContinuation struct {
	identity     installArchiveIdentity
	row          installResultView
	updateToken  int
	total        int
	remaining    []installResultView
	destinations []installDestination
	success      []string
	failures     []string
}

type installResultView struct {
	Result       remote.SearchResult
	ArchiveState string
	AuditPill    string
}

const (
	installSearchTimeout  = 30 * time.Second
	installPreviewTimeout = 60 * time.Second
	installArchiveTimeout = 60 * time.Second
)

var (
	installUseLink      = actions.Link
	installApplyArchive = remote.ApplyArchive
)

func newInstallState() installState {
	return installState{
		Message:       "type at least 2 characters",
		Audit:         map[string]remote.AuditSummary{},
		useGeneration: &installUseGeneration{},
		searchClient:  remote.NewSearchClient(remote.DefaultSearchEndpoint, http.DefaultClient),
	}
}

func (s *installState) ensureUseGeneration() *installUseGeneration {
	if s.useGeneration == nil {
		s.useGeneration = &installUseGeneration{}
		s.useGeneration.value.Store(int64(s.useToken))
	}
	return s.useGeneration
}

func (s *installState) bumpUseToken() int {
	generation := s.ensureUseGeneration()
	if current := generation.value.Load(); int64(s.useToken) != current {
		generation.value.Store(int64(s.useToken))
	}
	s.useToken = generation.next()
	return s.useToken
}

func (m Model) runInstallSearch() tea.Cmd {
	token := m.install.searchToken
	query := m.install.Query
	owner := m.install.Owner
	client := m.install.searchClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), installSearchTimeout)
		defer cancel()

		results, err := client.Search(ctx, remote.SearchRequest{Query: query, Owner: owner, Limit: remote.DefaultSearchLimit})
		return installSearchResultMsg{token: token, query: query, results: results, err: err}
	}
}

func (m *Model) startInstallSearch() tea.Cmd {
	m.install.searchToken++
	m.install.previewToken++
	clear(m.selected[ViewInstall])
	if len([]rune(strings.TrimSpace(m.install.Query))) < 2 {
		m.install.Searching = false
		m.install.Message = "type at least 2 characters"
		m.install.Results = nil
		m.status = m.install.Message
		return nil
	}
	m.install.Searching = true
	m.install.Message = "searching..."
	return m.runInstallSearch()
}

func (m *Model) updateInstallInput(msg tea.KeyMsg) tea.Cmd {
	beforeQuery := m.install.Query
	beforeOwner := m.install.Owner

	switch msg.String() {
	case "esc":
		m.install.InputMode = installInputNone
		return nil
	case "enter":
		m.install.InputMode = installInputNone
		m.cursor = 0
		return m.startInstallSearch()
	case "backspace":
		if m.install.InputMode == installInputQuery {
			m.install.Query = trimLastRune(m.install.Query)
		} else {
			m.install.Owner = trimLastRune(m.install.Owner)
		}
	default:
		if msg.Type == tea.KeyRunes {
			if m.install.InputMode == installInputQuery {
				m.install.Query += string(msg.Runes)
			} else {
				m.install.Owner += string(msg.Runes)
			}
		}
	}
	if m.install.Query != beforeQuery || m.install.Owner != beforeOwner {
		m.install.previewToken++
	}
	return nil
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func (m Model) selectedInstallResult() (installResultView, bool) {
	if m.cursor < 0 || m.cursor >= len(m.install.Results) {
		return installResultView{}, false
	}
	return m.install.Results[m.cursor], true
}

func (m Model) selectedInstallRows() []installResultView {
	selected := m.selected[ViewInstall]
	if len(selected) == 0 {
		return nil
	}
	rows := make([]installResultView, 0, len(selected))
	for _, row := range m.install.Results {
		if selected[installID(row.Result)] {
			rows = append(rows, row)
		}
	}
	return rows
}

func (m Model) installActionRows() []installResultView {
	if rows := m.selectedInstallRows(); len(rows) > 0 {
		return rows
	}
	row, ok := m.selectedInstallResult()
	if !ok {
		return nil
	}
	return []installResultView{row}
}

func (m *Model) previewInstallResult() tea.Cmd {
	row, ok := m.selectedInstallResult()
	if !ok {
		return nil
	}
	m.install.previewToken++
	token := m.install.previewToken
	checkouts := m.ensureInstallCheckoutCache()
	source, err := m.gitSourceForInstall(row.Result)
	if err != nil {
		return func() tea.Msg {
			return installPreviewMsg{token: token, name: row.Result.Name, err: err}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), installPreviewTimeout)
		defer cancel()

		checkout, err := checkouts.Checkout(ctx, source)
		if err != nil {
			return installPreviewMsg{token: token, name: row.Result.Name, err: err}
		}
		found, err := checkout.FindSkillContext(ctx, row.Result.Name, row.Result.Path)
		if err != nil {
			return installPreviewMsg{token: token, name: row.Result.Name, err: err}
		}
		return installPreviewMsg{token: token, name: row.Result.Name, path: found.SkillDir}
	}
}

func (m *Model) archiveInstallResult() tea.Cmd {
	rows := m.installActionRows()
	if len(rows) == 0 {
		return nil
	}
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	if len(rows) == 1 {
		return m.archiveInstallSingleResult(rows[0])
	}
	return m.archiveInstallBatch(rows)
}

func (m *Model) archiveInstallSingleResult(row installResultView) tea.Cmd {
	if row.ArchiveState == remote.ArchiveStateNameConflict {
		m.openInstallNameConflictModal(row)
		return nil
	}
	if row.ArchiveState == remote.ArchiveStateUpdateAvailable {
		return m.openInstallUpdateDiff(row)
	}
	cmd := m.archiveInstallRow(row)
	if cmd == nil {
		return nil
	}
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = m.install.archiveToken
	return cmd
}

func (m *Model) archiveInstallBatch(rows []installResultView) tea.Cmd {
	actionRows := rows
	var next *installArchiveBatchNext
	for i, row := range rows {
		switch row.ArchiveState {
		case remote.ArchiveStateNameConflict:
			actionRows = rows[:i]
			next = &installArchiveBatchNext{
				row:       row,
				action:    remote.ArchiveStateNameConflict,
				remaining: append([]installResultView(nil), rows[i+1:]...),
			}
		case remote.ArchiveStateUpdateAvailable:
			actionRows = rows[:i]
			next = &installArchiveBatchNext{
				row:       row,
				action:    remote.ArchiveStateUpdateAvailable,
				remaining: append([]installResultView(nil), rows[i+1:]...),
			}
		}
		if next != nil {
			break
		}
	}
	if len(actionRows) == 0 {
		m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
			identity:  installArchiveIdentityFromResult(next.row.Result),
			total:     len(rows),
			remaining: append([]installResultView(nil), next.remaining...),
		}
		return m.openInstallArchiveBatchNext(next)
	}
	cmd := m.archiveInstallRows(actionRows, next, len(rows))
	if cmd == nil {
		return nil
	}
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = m.install.archiveToken
	m.status = fmt.Sprintf("archiving %d skills...", len(actionRows))
	m.install.Message = m.status
	return cmd
}

func (m *Model) archiveInstallRows(rows []installResultView, next *installArchiveBatchNext, total int) tea.Cmd {
	commands := make([]tea.Cmd, 0, len(rows))
	for _, row := range rows {
		cmd := m.archiveInstallRow(row)
		if cmd == nil {
			continue
		}
		commands = append(commands, cmd)
	}
	if len(commands) == 0 {
		return nil
	}
	token := m.install.archiveToken
	return func() tea.Msg {
		result := &installArchiveBatchResult{total: total, next: next}
		for i, cmd := range commands {
			msg := cmd().(installArchiveMsg)
			if msg.err != nil {
				switch msg.archiveState {
				case remote.ArchiveStateNameConflict:
					result.next = &installArchiveBatchNext{
						row:       rows[i],
						action:    remote.ArchiveStateNameConflict,
						remaining: append(append([]installResultView(nil), rows[i+1:]...), installArchiveBatchNextRows(next)...),
					}
					return installArchiveMsg{token: token, batch: result}
				case remote.ArchiveStateUpdateAvailable:
					result.next = &installArchiveBatchNext{
						row:       rows[i],
						action:    remote.ArchiveStateUpdateAvailable,
						remaining: append(append([]installResultView(nil), rows[i+1:]...), installArchiveBatchNextRows(next)...),
					}
					return installArchiveMsg{token: token, batch: result}
				default:
					result.failures = append(result.failures, rows[i].Result.Name+": "+msg.err.Error())
					continue
				}
			}
			result.success = append(result.success, msg.name)
		}
		return installArchiveMsg{token: token, batch: result}
	}
}

func (m *Model) openInstallArchiveBatchNext(next *installArchiveBatchNext) tea.Cmd {
	if next == nil {
		return nil
	}
	switch next.action {
	case remote.ArchiveStateNameConflict:
		m.openInstallNameConflictModal(next.row)
		return nil
	case remote.ArchiveStateUpdateAvailable:
		cmd := m.openInstallUpdateDiff(next.row)
		if cmd != nil {
			identity := installArchiveIdentityFromResult(next.row.Result)
			if m.pendingInstallArchiveBatchMatches(identity) {
				m.install.pendingArchiveBatch.updateToken = m.install.previewToken
			}
		}
		return cmd
	default:
		return nil
	}
}

func installArchiveBatchNextRows(next *installArchiveBatchNext) []installResultView {
	if next == nil {
		return nil
	}
	rows := make([]installResultView, 0, 1+len(next.remaining))
	rows = append(rows, next.row)
	rows = append(rows, next.remaining...)
	return rows
}

func (m *Model) archiveInstallRow(row installResultView) tea.Cmd {
	return m.archiveInstallRowWithConflict(row, row.Result.Name, remote.ConflictArchiveOnly)
}

func (m *Model) archiveInstallRowWithConflict(row installResultView, archiveName, conflict string) tea.Cmd {
	m.install.previewToken++
	m.install.archiveToken++
	token := m.install.archiveToken
	identity := installArchiveIdentityFromResult(row.Result)
	checkouts := m.ensureInstallCheckoutCache()
	source, err := m.gitSourceForInstall(row.Result)
	if err != nil {
		return func() tea.Msg {
			return installArchiveMsg{token: token, name: archiveName, identity: identity, err: err}
		}
	}
	cfg := m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), installArchiveTimeout)
		defer cancel()

		checkout, err := checkouts.Checkout(ctx, source)
		if err != nil {
			return installArchiveMsg{token: token, name: archiveName, identity: identity, err: err}
		}
		found, err := checkout.FindSkillContext(ctx, row.Result.Name, row.Result.Path)
		if err != nil {
			return installArchiveMsg{token: token, name: archiveName, identity: identity, err: err}
		}
		plan, err := remote.PlanArchive(cfg, found.SkillDir, archiveName, found.Metadata)
		if err != nil {
			return installArchiveMsg{token: token, name: archiveName, identity: identity, err: err}
		}
		if conflict == remote.ConflictArchiveOnly {
			switch plan.State {
			case remote.ArchiveStateNotArchived:
			case remote.ArchiveStateArchived:
				return installArchiveMsg{token: token, name: archiveName, identity: identity, archiveState: plan.State}
			case remote.ArchiveStateNameConflict:
				return installArchiveMsg{
					token:        token,
					name:         archiveName,
					identity:     identity,
					archiveState: plan.State,
					err:          fmt.Errorf("archive conflict for %s", archiveName),
				}
			case remote.ArchiveStateUpdateAvailable:
				return installArchiveMsg{
					token:        token,
					name:         archiveName,
					identity:     identity,
					archiveState: plan.State,
					err:          fmt.Errorf("update available for %s", archiveName),
				}
			default:
				return installArchiveMsg{
					token:        token,
					name:         archiveName,
					identity:     identity,
					archiveState: plan.State,
					err:          fmt.Errorf("unknown archive state %q for %s", plan.State, archiveName),
				}
			}
		}
		_, err = installApplyArchive(remote.AddRequest{
			Config:      cfg,
			IncomingDir: found.SkillDir,
			ArchiveName: archiveName,
			Metadata:    found.Metadata,
			Conflict:    conflict,
		})
		if err != nil {
			plan, planErr := remote.PlanArchive(cfg, found.SkillDir, archiveName, found.Metadata)
			if planErr == nil {
				return installArchiveMsg{token: token, name: archiveName, identity: identity, archiveState: plan.State, err: err}
			}
			return installArchiveMsg{token: token, name: archiveName, identity: identity, err: err}
		}
		return installArchiveMsg{token: token, name: archiveName, identity: identity, archiveState: remote.ArchiveStateArchived}
	}
}

func (m *Model) archiveInstallRowRenamingExisting(row installResultView, oldPath, newPath string) tea.Cmd {
	m.install.previewToken++
	m.install.archiveToken++
	token := m.install.archiveToken
	identity := installArchiveIdentityFromResult(row.Result)
	checkouts := m.ensureInstallCheckoutCache()
	source, err := m.gitSourceForInstall(row.Result)
	if err != nil {
		return func() tea.Msg {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, err: err}
		}
	}
	cfg := m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), installArchiveTimeout)
		defer cancel()

		checkout, err := checkouts.Checkout(ctx, source)
		if err != nil {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, err: err}
		}
		found, err := checkout.FindSkillContext(ctx, row.Result.Name, row.Result.Path)
		if err != nil {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, err: err}
		}
		if _, err := os.Lstat(newPath); err == nil {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: remote.ArchiveStateNameConflict, err: fmt.Errorf("archive name already exists: %s", filepath.Base(newPath))}
		} else if !os.IsNotExist(err) {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: remote.ArchiveStateNameConflict, err: fmt.Errorf("inspect archive destination %q: %w", newPath, err)}
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: remote.ArchiveStateNameConflict, err: err}
		}
		_, err = installApplyArchive(remote.AddRequest{
			Config:      cfg,
			IncomingDir: found.SkillDir,
			ArchiveName: row.Result.Name,
			Metadata:    found.Metadata,
			Conflict:    remote.ConflictReplaceArchive,
		})
		if err != nil {
			if rollbackErr := rollbackExistingArchiveRename(oldPath, newPath); rollbackErr != nil {
				err = fmt.Errorf("apply incoming archive after renaming existing archive: %w; rollback rename: %v", err, rollbackErr)
			}
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: remote.ArchiveStateNameConflict, err: err}
		}
		return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: remote.ArchiveStateArchived}
	}
}

func rollbackExistingArchiveRename(oldPath, newPath string) error {
	if _, err := os.Lstat(oldPath); err == nil {
		return fmt.Errorf("original archive path already exists: %s", oldPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect original archive path %q: %w", oldPath, err)
	}
	if err := os.Rename(newPath, oldPath); err != nil {
		return fmt.Errorf("restore original archive: %w", err)
	}
	return nil
}

func (m *Model) openInstallNameConflictModal(row installResultView) {
	m.modal = newChoiceModalWithCommand(
		"Name conflict: "+row.Result.Name,
		[]string{
			"Archive already contains a skill with this name from a different source.",
			"Choose how to archive the incoming remote skill.",
		},
		[]string{"Replace archive", "Rename existing archive", "Rename incoming archive", "Cancel"},
		0,
		func(current *Model, choice int) tea.Cmd {
			return current.applyInstallNameConflictChoice(row, choice)
		},
	)
}

func (m *Model) applyInstallNameConflictChoice(row installResultView, choice int) tea.Cmd {
	switch choice {
	case 0:
		m.modal = nil
		return m.applyInstallArchiveWithConflict(row, row.Result.Name, remote.ConflictReplaceArchive)
	case 1:
		m.modal = newInstallRenameModal(row, true)
	case 2:
		m.modal = newInstallRenameModal(row, false)
	default:
		m.modal = nil
		m.status = "cancelled install " + row.Result.Name
		m.install.Message = m.status
		m.install.pendingUse = nil
		m.install.pendingUseBatch = nil
		m.install.pendingArchiveBatch = nil
	}
	return nil
}

func newInstallRenameModal(row installResultView, renameExisting bool) modal {
	title := "Rename incoming archive"
	suggestion := row.Result.Name + "-remote"
	if renameExisting {
		title = "Rename existing archive"
		suggestion = row.Result.Name + "-local"
	}
	return newTextModal(title, "Archive name", suggestion, func(m *Model, name string) tea.Cmd {
		if name == "" {
			m.status = "archive name is required"
			m.install.Message = m.status
			return nil
		}
		if renameExisting {
			return m.renameExistingArchiveThenInstall(row, name)
		}
		if repo.HasSkill(m.cfg, name) {
			path, err := repo.SkillPath(m.cfg, name)
			if err != nil {
				m.status = err.Error()
			} else {
				m.status = "archive destination already exists: " + path
			}
			m.install.Message = m.status
			return nil
		}
		return m.applyInstallArchiveWithConflict(row, name, remote.ConflictRenameIncoming)
	})
}

func (m *Model) renameExistingArchiveThenInstall(row installResultView, newName string) tea.Cmd {
	oldPath, err := repo.SkillPath(m.cfg, row.Result.Name)
	if err != nil {
		m.status = err.Error()
		m.install.Message = m.status
		return nil
	}
	newPath, err := repo.SkillPath(m.cfg, newName)
	if err != nil {
		m.status = err.Error()
		m.install.Message = m.status
		return nil
	}
	if repo.HasSkill(m.cfg, newName) {
		m.status = "archive name already exists: " + newName
		m.install.Message = m.status
		return nil
	}
	cmd := m.archiveInstallRowRenamingExisting(row, oldPath, newPath)
	if cmd == nil {
		return nil
	}
	m.modal = nil
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = m.install.archiveToken
	m.status = "archiving " + row.Result.Name + "..."
	m.install.Message = m.status
	return cmd
}

func (m *Model) openInstallUpdateDiff(row installResultView) tea.Cmd {
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	m.install.previewToken++
	token := m.install.previewToken
	checkouts := m.ensureInstallCheckoutCache()
	source, err := m.gitSourceForInstall(row.Result)
	if err != nil {
		return func() tea.Msg {
			return installUpdateDiffMsg{token: token, row: row, err: err}
		}
	}
	cfg := m.cfg
	m.status = "comparing update for " + row.Result.Name + "..."
	m.install.Message = m.status
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), installArchiveTimeout)
		defer cancel()

		checkout, err := checkouts.Checkout(ctx, source)
		if err != nil {
			return installUpdateDiffMsg{token: token, row: row, err: err}
		}
		found, err := checkout.FindSkillContext(ctx, row.Result.Name, row.Result.Path)
		if err != nil {
			return installUpdateDiffMsg{token: token, row: row, err: err}
		}
		archivePath, err := repo.SkillPath(cfg, row.Result.Name)
		if err != nil {
			return installUpdateDiffMsg{token: token, row: row, err: err}
		}
		diff, err := buildDirectoryDiff(found.SkillDir, archivePath)
		if err != nil {
			return installUpdateDiffMsg{token: token, row: row, err: fmt.Errorf("failed to build conflict diff: %w", err)}
		}
		return installUpdateDiffMsg{token: token, row: row, diff: diff}
	}
}

func (m *Model) applyInstallUpdateDiffResult(msg installUpdateDiffMsg) tea.Cmd {
	if msg.token != m.install.previewToken || m.view != ViewInstall {
		m.clearPendingInstallUseForUpdateDiff(msg.row, msg.token)
		return nil
	}
	if msg.err != nil {
		identity := installArchiveIdentityFromResult(msg.row.Result)
		if isMissingSkillInRepoError(msg.err) {
			if m.pendingInstallArchiveBatchUpdateMatches(identity, msg.token) {
				return m.continueInstallArchiveBatchAfterResolved(identity, msg.row.Result.Name, msg.err)
			}
			if m.pendingInstallUseBatchUpdateMatches(identity, msg.token) {
				return m.continueInstallUseBatchAfterArchiveResolution(identity, msg.row.Result.Name, msg.err)
			}
		}
		m.clearPendingInstallUseForUpdateDiff(msg.row, msg.token)
		if m.showMissingSkillInRepoModal(msg.err) {
			return nil
		}
		m.status = msg.err.Error()
		m.install.Message = m.status
		return nil
	}
	row := msg.row
	m.modal = newConflictDiffModalWithModelCommandApply(row.Result.Name, msg.diff, "Incoming remote", func(current *Model, chosen string) tea.Cmd {
		if chosen == actions.ConflictResolutionUseActive {
			return current.applyInstallArchiveWithConflict(row, row.Result.Name, remote.ConflictReplaceArchive)
		}
		if pending := current.install.pendingUse; pending != nil && installArchiveIdentityFromResult(pending.row.Result) == installArchiveIdentityFromResult(row.Result) {
			row := pending.row
			row.ArchiveState = remote.ArchiveStateArchived
			current.install.pendingUse = nil
			current.install.bumpUseToken()
			current.modal = newInstallDestinationModalUsingExistingArchive(current.cfg, row)
			current.status = "kept archive " + row.Result.Name + "; choose destinations"
			current.install.Message = current.status
			return nil
		}
		identity := installArchiveIdentityFromResult(row.Result)
		if current.pendingInstallUseBatchUpdateMatches(identity, msg.token) {
			current.status = "kept archive " + row.Result.Name + "; continuing install"
			current.install.Message = current.status
			return current.continueInstallUseBatchAfterArchiveResolution(identity, row.Result.Name, nil)
		}
		if current.pendingInstallArchiveBatchUpdateMatches(identity, msg.token) {
			current.status = "kept archive " + row.Result.Name + "; continuing archive"
			current.install.Message = current.status
			return current.continueInstallArchiveBatchAfterResolved(identity, row.Result.Name, nil)
		}
		current.status = "kept archive " + row.Result.Name
		current.install.Message = current.status
		current.install.pendingUse = nil
		return nil
	})
	return nil
}

func (m *Model) applyInstallArchiveWithConflict(row installResultView, archiveName, conflict string) tea.Cmd {
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	cmd := m.archiveInstallRowWithConflict(row, archiveName, conflict)
	if cmd == nil {
		return nil
	}
	m.modal = nil
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = m.install.archiveToken
	m.status = "archiving " + archiveName + "..."
	m.install.Message = m.status
	return cmd
}

func (m *Model) installAndUse(row installResultView, destinations []installDestination, useExistingArchive bool) tea.Cmd {
	if len(destinations) == 0 {
		m.status = "select at least one destination"
		m.install.Message = m.status
		return nil
	}
	if m.install.useInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	if m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	if err := preflightInstallUseDestinations(m.cfg, row.Result.Name, destinations); err != nil {
		m.status = err.Error()
		m.install.Message = m.status
		return nil
	}
	token := m.install.bumpUseToken()
	var archiveCmd tea.Cmd
	if !useExistingArchive {
		archiveCmd = m.archiveInstallRow(row)
		if archiveCmd == nil {
			return nil
		}
	}
	m.install.useInFlight = true
	m.install.useInFlightToken = token
	cfg := m.cfg
	useGeneration := m.install.ensureUseGeneration()
	return func() tea.Msg {
		if !useGeneration.isCurrent(token) {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, stale: true}
		}
		if archiveCmd != nil {
			archiveMsg := archiveCmd().(installArchiveMsg)
			if archiveMsg.err != nil {
				return installUseMsg{
					token:        token,
					name:         row.Result.Name,
					row:          row,
					identity:     archiveMsg.identity,
					archiveState: archiveMsg.archiveState,
					destinations: destinations,
					err:          archiveMsg.err,
				}
			}
		}
		if !useGeneration.isCurrent(token) {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, stale: true}
		}
		if err := preflightInstallUseDestinations(cfg, row.Result.Name, destinations); err != nil {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: err}
		}
		createdPaths := make([]string, 0, len(destinations))
		for _, dest := range destinations {
			if !useGeneration.isCurrent(token) {
				if err := rollbackInstallUseLinks(createdPaths); err != nil {
					return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: fmt.Errorf("rollback stale install-use links: %w", err)}
				}
				return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, stale: true}
			}
			result, err := installUseLink(cfg, actions.LinkRequest{Name: row.Result.Name, Scope: dest.Scope, Target: dest.Target})
			if err != nil {
				if rollbackErr := rollbackInstallUseLinks(createdPaths); rollbackErr != nil {
					err = errors.Join(err, fmt.Errorf("rollback partial install-use links: %w", rollbackErr))
				}
				return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: err}
			}
			if result.Path != "" {
				createdPaths = append(createdPaths, result.Path)
			}
			if !useGeneration.isCurrent(token) {
				if err := rollbackInstallUseLinks(createdPaths); err != nil {
					return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: fmt.Errorf("rollback stale install-use links: %w", err)}
				}
				return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, stale: true}
			}
		}
		return installUseMsg{token: token, name: row.Result.Name, destinations: destinations}
	}
}

func (m *Model) installAndUseRows(rows []installResultView, destinations []installDestination) tea.Cmd {
	return m.installAndUseRowsWithProgress(rows, destinations, nil, false)
}

func (m *Model) installAndUseRowsWithProgress(
	rows []installResultView,
	destinations []installDestination,
	progress *installUseBatchResult,
	useExistingFirstArchive bool,
) tea.Cmd {
	if len(rows) == 0 {
		return nil
	}
	if len(rows) == 1 && progress == nil && !useExistingFirstArchive {
		return m.installAndUse(rows[0], destinations, false)
	}
	if len(destinations) == 0 {
		m.status = "select at least one destination"
		m.install.Message = m.status
		return nil
	}
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	for _, row := range rows {
		if err := preflightInstallUseDestinations(m.cfg, row.Result.Name, destinations); err != nil {
			m.status = err.Error()
			m.install.Message = m.status
			return nil
		}
	}
	token := m.install.bumpUseToken()
	archiveCommands := make([]tea.Cmd, 0, len(rows))
	for i, row := range rows {
		if i == 0 && useExistingFirstArchive {
			archiveCommands = append(archiveCommands, nil)
			continue
		}
		cmd := m.archiveInstallRow(row)
		if cmd == nil {
			return nil
		}
		archiveCommands = append(archiveCommands, cmd)
	}
	m.install.useInFlight = true
	m.install.useInFlightToken = token
	cfg := m.cfg
	useGeneration := m.install.ensureUseGeneration()
	return func() tea.Msg {
		result := &installUseBatchResult{total: len(rows)}
		if progress != nil {
			result.total = progress.total
			result.success = append(result.success, progress.success...)
			result.failures = append(result.failures, progress.failures...)
		}
		createdPaths := make([]string, 0, len(rows)*len(destinations))
		for i, row := range rows {
			if !useGeneration.isCurrent(token) {
				if err := rollbackInstallUseLinks(createdPaths); err != nil {
					return installUseMsg{token: token, destinations: destinations, err: fmt.Errorf("rollback stale install-use links: %w", err)}
				}
				return installUseMsg{token: token, destinations: destinations, stale: true}
			}
			if archiveCommands[i] != nil {
				archiveMsg := archiveCommands[i]().(installArchiveMsg)
				if archiveMsg.err != nil {
					switch archiveMsg.archiveState {
					case remote.ArchiveStateNameConflict, remote.ArchiveStateUpdateAvailable:
						result.next = &installUseBatchNext{
							row:       row,
							remaining: append([]installResultView(nil), rows[i+1:]...),
						}
						return installUseMsg{
							token:        token,
							name:         row.Result.Name,
							row:          row,
							identity:     archiveMsg.identity,
							archiveState: archiveMsg.archiveState,
							destinations: destinations,
							batch:        result,
							err:          archiveMsg.err,
						}
					default:
						result.failures = append(result.failures, row.Result.Name+": "+archiveMsg.err.Error())
						continue
					}
				}
			}
			rowFailed := false
			rowCreatedPaths := make([]string, 0, len(destinations))
			for _, dest := range destinations {
				if !useGeneration.isCurrent(token) {
					rollbackPaths := append(append([]string(nil), createdPaths...), rowCreatedPaths...)
					if err := rollbackInstallUseLinks(rollbackPaths); err != nil {
						return installUseMsg{token: token, destinations: destinations, err: fmt.Errorf("rollback stale install-use links: %w", err)}
					}
					return installUseMsg{token: token, destinations: destinations, stale: true}
				}
				linkResult, err := installUseLink(cfg, actions.LinkRequest{Name: row.Result.Name, Scope: dest.Scope, Target: dest.Target})
				if err != nil {
					if rollbackErr := rollbackInstallUseLinks(rowCreatedPaths); rollbackErr != nil {
						err = errors.Join(err, fmt.Errorf("rollback partial install-use links: %w", rollbackErr))
					}
					result.failures = append(result.failures, row.Result.Name+": "+err.Error())
					rowFailed = true
					break
				}
				if linkResult.Path != "" {
					rowCreatedPaths = append(rowCreatedPaths, linkResult.Path)
				}
				if !useGeneration.isCurrent(token) {
					rollbackPaths := append(append([]string(nil), createdPaths...), rowCreatedPaths...)
					if err := rollbackInstallUseLinks(rollbackPaths); err != nil {
						return installUseMsg{token: token, destinations: destinations, err: fmt.Errorf("rollback stale install-use links: %w", err)}
					}
					return installUseMsg{token: token, destinations: destinations, stale: true}
				}
			}
			if !rowFailed {
				createdPaths = append(createdPaths, rowCreatedPaths...)
				result.success = append(result.success, row.Result.Name)
			}
		}
		return installUseMsg{token: token, destinations: destinations, batch: result}
	}
}

func rollbackInstallUseLinks(paths []string) error {
	var errs []error
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("inspect rollback path %q: %w", path, err))
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			errs = append(errs, fmt.Errorf("rollback path is not a symlink: %s", path))
			continue
		}
		if err := os.Remove(path); err != nil {
			errs = append(errs, fmt.Errorf("remove rollback path %q: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func preflightInstallUseDestinations(cfg config.Config, name string, destinations []installDestination) error {
	for _, dest := range destinations {
		root, err := cfg.ActiveRoot(dest.Scope, dest.Target)
		if err != nil {
			return err
		}
		destination := filepath.Join(root, name)
		if _, err := os.Lstat(destination); err == nil {
			return fmt.Errorf("destination exists: %s", destination)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect destination %q: %w", destination, err)
		}
	}
	return nil
}

type installDestination struct {
	Scope   string
	Target  string
	Label   string
	Checked bool
}

type installDestinationModal struct {
	name               string
	row                installResultView
	destinations       []installDestination
	cursor             int
	useExistingArchive bool
}

func newInstallDestinationModal(cfg config.Config, row installResultView) modal {
	return newInstallDestinationModalWithArchiveMode(cfg, row, false)
}

func newInstallDestinationModalUsingExistingArchive(cfg config.Config, row installResultView) modal {
	return newInstallDestinationModalWithArchiveMode(cfg, row, true)
}

func newInstallDestinationModalWithArchiveMode(cfg config.Config, row installResultView, useExistingArchive bool) modal {
	return installDestinationModal{
		name:               row.Result.Name,
		row:                row,
		useExistingArchive: useExistingArchive,
		destinations:       installDestinations(cfg),
	}
}

func installDestinations(cfg config.Config) []installDestination {
	activeRoots := roots.ActiveRoots(cfg, roots.Filter{})
	destinations := make([]installDestination, 0, len(activeRoots))
	defaultIndex := defaultInstallDestinationIndex(activeRoots)
	for i, root := range activeRoots {
		destinations = append(destinations, installDestination{
			Scope:   root.Scope,
			Target:  root.Target,
			Label:   rootLabel(root),
			Checked: i == defaultIndex,
		})
	}
	return destinations
}

func defaultInstallDestinationIndex(activeRoots []roots.ActiveRoot) int {
	firstProject := -1
	firstAvailable := -1
	for i, root := range activeRoots {
		if firstAvailable == -1 {
			firstAvailable = i
		}
		if root.Scope != config.ScopeProject {
			continue
		}
		if firstProject == -1 {
			firstProject = i
		}
		if root.Target == config.TargetAgents {
			return i
		}
	}
	if firstProject != -1 {
		return firstProject
	}
	return firstAvailable
}

func (m *Model) openInstallDestinationModal(row installResultView) tea.Cmd {
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	switch row.ArchiveState {
	case remote.ArchiveStateNameConflict:
		m.install.pendingUse = &pendingInstallUseIntent{row: row}
		m.openInstallNameConflictModal(row)
		return nil
	case remote.ArchiveStateUpdateAvailable:
		cmd := m.openInstallUpdateDiff(row)
		m.install.pendingUse = &pendingInstallUseIntent{row: row, updateToken: m.install.previewToken}
		return cmd
	}
	m.install.bumpUseToken()
	m.modal = newInstallDestinationModal(m.cfg, row)
	return nil
}

func (m *Model) openInstallDestinationModalForRows(rows []installResultView) tea.Cmd {
	if len(rows) == 0 {
		return nil
	}
	if len(rows) == 1 {
		return m.openInstallDestinationModal(rows[0])
	}
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
	m.install.bumpUseToken()
	m.modal = newInstallBatchDestinationModal(m.cfg, rows)
	return nil
}

func (d installDestinationModal) Title() string {
	return "Install and use " + d.name
}

func (d installDestinationModal) View(width, height int, m Model) string {
	body := make([]string, 0, len(d.destinations))
	for i, dest := range d.destinations {
		cursor := " "
		if i == d.cursor {
			cursor = m.symbols.Cursor
		}
		check := "[ ]"
		if dest.Checked {
			check = "[x]"
		}
		row := "  " + cursor + " " + check + " " + dest.Label
		if i == d.cursor {
			row = selectedBg.Render(row)
		}
		body = append(body, row)
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title:  d.Title(),
		Body:   body,
		Footer: []string{mutedStyle.Render("up/down move  space toggle  enter install  esc cancel")},
		Focus:  d.cursor,
	})
}

func (d installDestinationModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		d.cursor = clampModalIndex(d.cursor+delta, len(d.destinations))
		m.modal = d
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		m.install.bumpUseToken()
		return true, nil
	case " ":
		if d.cursor >= 0 && d.cursor < len(d.destinations) {
			d.destinations[d.cursor].Checked = !d.destinations[d.cursor].Checked
		}
		m.modal = d
	case "enter":
		destinations := checkedInstallDestinations(d.destinations)
		if len(destinations) == 0 {
			m.status = "select at least one destination"
			m.install.Message = m.status
			m.modal = d
			return false, nil
		}
		m.status = "installing " + d.name + "..."
		m.install.Message = m.status
		return true, m.installAndUse(d.row, destinations, d.useExistingArchive)
	}
	return false, nil
}

type installBatchDestinationModal struct {
	rows         []installResultView
	destinations []installDestination
	cursor       int
}

func newInstallBatchDestinationModal(cfg config.Config, rows []installResultView) modal {
	copiedRows := append([]installResultView(nil), rows...)
	return installBatchDestinationModal{rows: copiedRows, destinations: installDestinations(cfg)}
}

func (d installBatchDestinationModal) Title() string {
	return fmt.Sprintf("Install and use %d skills", len(d.rows))
}

func (d installBatchDestinationModal) View(width, height int, m Model) string {
	names := make([]string, 0, len(d.rows))
	for _, row := range d.rows {
		names = append(names, row.Result.Name)
	}
	body := []string{mutedStyle.Render(strings.Join(names, ", ")), ""}
	focus := 2 + d.cursor
	for i, dest := range d.destinations {
		cursor := " "
		if i == d.cursor {
			cursor = m.symbols.Cursor
		}
		check := "[ ]"
		if dest.Checked {
			check = "[x]"
		}
		row := "  " + cursor + " " + check + " " + dest.Label
		if i == d.cursor {
			row = selectedBg.Render(row)
		}
		body = append(body, row)
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title:  d.Title(),
		Body:   body,
		Footer: []string{mutedStyle.Render("up/down move  space toggle  enter install  esc cancel")},
		Focus:  focus,
	})
}

func (d installBatchDestinationModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		d.cursor = clampModalIndex(d.cursor+delta, len(d.destinations))
		m.modal = d
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		m.install.bumpUseToken()
		return true, nil
	case " ":
		if d.cursor >= 0 && d.cursor < len(d.destinations) {
			d.destinations[d.cursor].Checked = !d.destinations[d.cursor].Checked
		}
		m.modal = d
	case "enter":
		destinations := checkedInstallDestinations(d.destinations)
		if len(destinations) == 0 {
			m.status = "select at least one destination"
			m.install.Message = m.status
			m.modal = d
			return false, nil
		}
		m.status = fmt.Sprintf("installing %d skills...", len(d.rows))
		m.install.Message = m.status
		return true, m.installAndUseRows(d.rows, destinations)
	}
	return false, nil
}

func checkedInstallDestinations(available []installDestination) []installDestination {
	destinations := make([]installDestination, 0, len(available))
	for _, dest := range available {
		if dest.Checked {
			destinations = append(destinations, dest)
		}
	}
	return destinations
}

func (m *Model) ensureInstallCheckoutCache() *remote.CheckoutCache {
	if m.install.checkouts == nil {
		m.install.checkouts = remote.NewCheckoutCache(filepath.Join(os.TempDir(), "x-skills-tui-checkouts"))
	}
	return m.install.checkouts
}

func (m Model) gitSourceForInstall(result remote.SearchResult) (remote.GitSource, error) {
	if m.install.testCloneURL != "" {
		return remote.GitSource{CloneURL: m.install.testCloneURL}, nil
	}
	if result.Owner == "" || result.Repo == "" {
		return remote.GitSource{}, fmt.Errorf("missing source repository for %s", result.Name)
	}
	return remote.GitSource{
		Owner:    result.Owner,
		Repo:     result.Repo,
		CloneURL: "https://github.com/" + result.Owner + "/" + result.Repo + ".git",
	}, nil
}

func (m *Model) applyInstallSearchResult(msg installSearchResultMsg) tea.Cmd {
	if msg.token != m.install.searchToken {
		return nil
	}
	clear(m.selected[ViewInstall])
	m.install.Searching = false
	if msg.err != nil {
		m.install.Results = nil
		m.install.Message = msg.err.Error()
		m.status = msg.err.Error()
		return nil
	}
	m.install.Results = make([]installResultView, 0, len(msg.results))
	var stateChecks []tea.Cmd
	for _, result := range msg.results {
		auditKey := installAuditKey(result)
		audit := m.install.Audit[auditKey]
		if result.Audit != nil {
			audit = *result.Audit
			m.install.Audit[auditKey] = audit
		}
		state := m.installArchiveState(result)
		m.install.Results = append(m.install.Results, installResultView{
			Result:       result,
			ArchiveState: state,
			AuditPill:    installAuditPill(audit, m.opts),
		})
		if state == remote.ArchiveStateArchived {
			if cmd := m.installArchiveStateCheck(result, msg.token); cmd != nil {
				stateChecks = append(stateChecks, cmd)
			}
		}
	}
	count := len(msg.results)
	switch count {
	case 0:
		m.install.Message = "no results for " + strconv.Quote(msg.query)
		m.status = "found 0 results for " + strconv.Quote(msg.query)
	case 1:
		m.status = "found 1 result for " + strconv.Quote(msg.query)
		m.install.Message = m.status
	default:
		m.status = fmt.Sprintf("found %d results for %q", count, msg.query)
		m.install.Message = m.status
	}
	return tea.Batch(stateChecks...)
}

func (m *Model) installArchiveStateCheck(result remote.SearchResult, token int) tea.Cmd {
	archivePath, err := repo.SkillPath(m.cfg, result.Name)
	if err != nil {
		return nil
	}
	existing, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil || !ok || !existing.SameIdentity(m.installSourceMetadata(result)) {
		return nil
	}
	source, err := m.gitSourceForInstall(result)
	if err != nil {
		return nil
	}
	checkouts := m.ensureInstallCheckoutCache()
	cfg := m.cfg
	identity := installArchiveIdentityFromResult(result)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), installArchiveTimeout)
		defer cancel()
		checkout, err := checkouts.Checkout(ctx, source)
		if err != nil {
			return installArchiveStateMsg{token: token, identity: identity}
		}
		found, err := checkout.FindSkillContext(ctx, result.Name, result.Path)
		if err != nil {
			return installArchiveStateMsg{token: token, identity: identity}
		}
		plan, err := remote.PlanArchive(cfg, found.SkillDir, result.Name, found.Metadata)
		if err != nil {
			return installArchiveStateMsg{token: token, identity: identity}
		}
		return installArchiveStateMsg{token: token, identity: identity, state: plan.State}
	}
}

func (m *Model) applyInstallArchiveStateResult(msg installArchiveStateMsg) {
	if msg.token != m.install.searchToken || msg.state == "" {
		return
	}
	for i := range m.install.Results {
		if installArchiveIdentityFromResult(m.install.Results[i].Result) == msg.identity {
			m.install.Results[i].ArchiveState = msg.state
		}
	}
}

func (m *Model) applyInstallArchiveResult(msg installArchiveMsg) tea.Cmd {
	if msg.token != 0 && msg.token == m.install.archiveInFlightToken {
		m.install.archiveInFlight = false
		m.install.archiveInFlightToken = 0
	}
	if msg.token == 0 || msg.token != m.install.archiveToken {
		return nil
	}
	if msg.batch != nil {
		return m.applyInstallArchiveBatchResult(msg.batch)
	}
	if msg.err != nil {
		m.reload()
		m.refreshInstallArchiveStates()
		m.updateInstallArchiveState(msg.identity, msg.archiveState)
		if isMissingSkillInRepoError(msg.err) {
			if m.pendingInstallArchiveBatchMatches(msg.identity) {
				return m.continueInstallArchiveBatchAfterResolved(msg.identity, msg.name, msg.err)
			}
			if m.pendingInstallUseBatchMatches(msg.identity) {
				return m.continueInstallUseBatchAfterArchiveResolution(msg.identity, msg.name, msg.err)
			}
		}
		if m.showMissingSkillInRepoModal(msg.err) {
			if m.pendingInstallUseMatches(msg.identity) {
				m.install.pendingUse = nil
			}
			if m.pendingInstallUseBatchMatches(msg.identity) {
				m.install.pendingUseBatch = nil
			}
			if m.pendingInstallArchiveBatchMatches(msg.identity) {
				m.install.pendingArchiveBatch = nil
			}
			return nil
		}
		m.status = msg.err.Error()
		m.install.Message = m.status
		if m.pendingInstallUseMatches(msg.identity) {
			m.install.pendingUse = nil
		}
		if m.pendingInstallArchiveBatchMatches(msg.identity) {
			return m.continueInstallArchiveBatchAfterResolved(msg.identity, msg.name, msg.err)
		}
		if m.pendingInstallUseBatchMatches(msg.identity) {
			return m.continueInstallUseBatchAfterArchiveResolution(msg.identity, msg.name, msg.err)
		}
		return nil
	}
	m.reload()
	m.refreshInstallArchiveStates()
	if m.pendingInstallArchiveBatchMatches(msg.identity) {
		return m.continueInstallArchiveBatchAfterResolved(msg.identity, msg.name, nil)
	}
	if m.pendingInstallUseBatchMatches(msg.identity) {
		return m.continueInstallUseBatchAfterArchiveResolution(msg.identity, msg.name, nil)
	}
	if pending := m.install.pendingUse; pending != nil && installArchiveIdentityFromResult(pending.row.Result) == msg.identity {
		row := pending.row
		if msg.name != "" {
			row.Result.Name = msg.name
		}
		row.ArchiveState = remote.ArchiveStateArchived
		m.install.pendingUse = nil
		m.install.bumpUseToken()
		m.modal = newInstallDestinationModal(m.cfg, row)
		m.status = "archived " + msg.name + "; choose destinations"
		m.install.Message = m.status
		return nil
	}
	m.status = "archived " + msg.name
	m.install.Message = m.status
	return nil
}

func (m *Model) applyInstallArchiveBatchResult(result *installArchiveBatchResult) tea.Cmd {
	m.reload()
	m.refreshInstallArchiveStates()
	if result.next != nil {
		m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
			identity:  installArchiveIdentityFromResult(result.next.row.Result),
			total:     result.total,
			remaining: append([]installResultView(nil), result.next.remaining...),
			success:   append([]string(nil), result.success...),
			failures:  append([]string(nil), result.failures...),
		}
		m.install.archiveInFlight = false
		m.install.archiveInFlightToken = 0
		return m.openInstallArchiveBatchNext(result.next)
	}
	if len(result.failures) > 0 {
		lines := make([]string, 0, len(result.success)+len(result.failures))
		for _, name := range result.success {
			lines = append(lines, "✓ archived "+name)
		}
		for _, failure := range result.failures {
			lines = append(lines, "x "+failure)
		}
		m.modal = newResultModal("Archive Results", lines)
		m.status = fmt.Sprintf("archived %d of %d skills", len(result.success), result.total)
		m.install.Message = m.status
		return nil
	}
	if len(result.success) > 0 {
		m.status = fmt.Sprintf("archived %d skills", len(result.success))
		m.install.Message = m.status
	}
	return nil
}

func (m Model) pendingInstallUseMatches(identity installArchiveIdentity) bool {
	return m.install.pendingUse != nil && installArchiveIdentityFromResult(m.install.pendingUse.row.Result) == identity
}

func (m Model) pendingInstallArchiveBatchMatches(identity installArchiveIdentity) bool {
	return m.install.pendingArchiveBatch != nil && m.install.pendingArchiveBatch.identity == identity
}

func (m Model) pendingInstallUseBatchMatches(identity installArchiveIdentity) bool {
	return m.install.pendingUseBatch != nil && m.install.pendingUseBatch.identity == identity
}

func (m Model) pendingInstallArchiveBatchUpdateMatches(identity installArchiveIdentity, updateToken int) bool {
	return m.install.pendingArchiveBatch != nil &&
		m.install.pendingArchiveBatch.identity == identity &&
		m.install.pendingArchiveBatch.updateToken == updateToken
}

func (m Model) pendingInstallUseBatchUpdateMatches(identity installArchiveIdentity, updateToken int) bool {
	return m.install.pendingUseBatch != nil &&
		m.install.pendingUseBatch.identity == identity &&
		m.install.pendingUseBatch.updateToken == updateToken
}

func (m *Model) continueInstallArchiveBatchAfterResolved(identity installArchiveIdentity, name string, err error) tea.Cmd {
	pending := m.install.pendingArchiveBatch
	if pending == nil || pending.identity != identity {
		return nil
	}
	m.install.pendingArchiveBatch = nil
	result := &installArchiveBatchResult{
		total:    pending.total,
		success:  append([]string(nil), pending.success...),
		failures: append([]string(nil), pending.failures...),
	}
	if err != nil {
		failure := err.Error()
		if name == "" {
			name = pending.identity.name
		}
		if name != "" {
			failure = name + ": " + failure
		}
		result.failures = append(result.failures, failure)
	} else if name != "" {
		result.success = append(result.success, name)
	}
	if len(pending.remaining) == 0 {
		return m.applyInstallArchiveBatchResult(result)
	}
	cmd := m.archiveInstallRows(pending.remaining, nil, pending.total)
	if cmd == nil {
		return m.applyInstallArchiveBatchResult(result)
	}
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = m.install.archiveToken
	m.status = fmt.Sprintf("archiving %d skills...", len(pending.remaining))
	m.install.Message = m.status
	return func() tea.Msg {
		msg := cmd().(installArchiveMsg)
		if msg.batch == nil {
			return msg
		}
		msg.batch.success = append(result.success, msg.batch.success...)
		msg.batch.failures = append(result.failures, msg.batch.failures...)
		return msg
	}
}

func (m *Model) continueInstallUseBatchAfterArchiveResolution(identity installArchiveIdentity, archiveName string, err error) tea.Cmd {
	pending := m.install.pendingUseBatch
	if pending == nil || pending.identity != identity {
		return nil
	}
	m.install.pendingUseBatch = nil
	row := pending.row
	result := &installUseBatchResult{
		total:    pending.total,
		success:  append([]string(nil), pending.success...),
		failures: append([]string(nil), pending.failures...),
	}
	if err != nil {
		if archiveName == "" {
			archiveName = row.Result.Name
		}
		result.failures = append(result.failures, archiveName+": "+err.Error())
		if len(pending.remaining) == 0 {
			return m.applyInstallUseResult(installUseMsg{
				token:        m.install.useToken,
				destinations: pending.destinations,
				batch:        result,
			})
		}
		return m.installAndUseRowsWithProgress(pending.remaining, pending.destinations, result, false)
	}
	if archiveName != "" {
		row.Result.Name = archiveName
	}
	row.ArchiveState = remote.ArchiveStateArchived
	rows := make([]installResultView, 0, 1+len(pending.remaining))
	rows = append(rows, row)
	rows = append(rows, pending.remaining...)
	return m.installAndUseRowsWithProgress(rows, pending.destinations, result, true)
}

func (m *Model) clearPendingInstallUseForUpdateDiff(row installResultView, token int) {
	identity := installArchiveIdentityFromResult(row.Result)
	if m.pendingInstallUseMatches(identity) && m.install.pendingUse.updateToken == token {
		m.install.pendingUse = nil
	}
	if m.pendingInstallUseBatchUpdateMatches(identity, token) {
		m.install.pendingUseBatch = nil
	}
	if m.pendingInstallArchiveBatchUpdateMatches(identity, token) {
		m.install.pendingArchiveBatch = nil
	}
}

func (m *Model) clearPendingInstallUseOnModalClose(closed modal) {
	if !installModalClosesPendingUse(closed) {
		return
	}
	m.install.pendingUse = nil
	m.install.pendingUseBatch = nil
	m.install.pendingArchiveBatch = nil
}

func installModalClosesPendingUse(closed modal) bool {
	switch modal := closed.(type) {
	case choiceModal:
		return strings.HasPrefix(modal.title, "Name conflict: ")
	case conflictDiffModal:
		return modal.incomingLabel == "Incoming remote"
	case textModal:
		return modal.title == "Rename existing archive" || modal.title == "Rename incoming archive"
	default:
		return false
	}
}

func (m *Model) applyInstallUseResult(msg installUseMsg) tea.Cmd {
	if msg.token != 0 && msg.token == m.install.useInFlightToken {
		m.install.useInFlight = false
		m.install.useInFlightToken = 0
	}
	if msg.token == 0 || msg.token != m.install.useToken {
		return nil
	}
	if msg.stale {
		return nil
	}
	if msg.batch != nil {
		m.reload()
		m.refreshInstallArchiveStates()
		m.modal = nil
		if msg.batch.next != nil {
			m.install.pendingUseBatch = &installUseBatchContinuation{
				identity:     installArchiveIdentityFromResult(msg.batch.next.row.Result),
				row:          msg.batch.next.row,
				total:        msg.batch.total,
				remaining:    append([]installResultView(nil), msg.batch.next.remaining...),
				destinations: append([]installDestination(nil), msg.destinations...),
				success:      append([]string(nil), msg.batch.success...),
				failures:     append([]string(nil), msg.batch.failures...),
			}
			m.install.useInFlight = false
			m.install.useInFlightToken = 0
			return m.openInstallUseArchiveResolution(msg)
		}
		if len(msg.batch.failures) > 0 {
			lines := make([]string, 0, len(msg.batch.success)+len(msg.batch.failures))
			for _, name := range msg.batch.success {
				lines = append(lines, "✓ installed "+name)
			}
			for _, failure := range msg.batch.failures {
				lines = append(lines, "x "+failure)
			}
			m.modal = newResultModal("Install Results", lines)
			m.status = fmt.Sprintf("installed %d of %d skills", len(msg.batch.success), msg.batch.total)
			m.install.Message = m.status
			return nil
		}
		m.status = fmt.Sprintf("installed %d skills to %s", len(msg.batch.success), installDestinationLabels(msg.destinations))
		m.install.Message = m.status
		return nil
	}
	if msg.err != nil {
		m.reload()
		m.refreshInstallArchiveStates()
		m.updateInstallArchiveState(msg.identity, msg.archiveState)
		if m.showMissingSkillInRepoModal(msg.err) {
			return nil
		}
		m.status = msg.err.Error()
		m.install.Message = m.status
		if m.pendingInstallUseBatchMatches(msg.identity) {
			return m.continueInstallUseBatchAfterArchiveResolution(msg.identity, msg.name, msg.err)
		}
		return m.openInstallUseArchiveResolution(msg)
	}
	m.reload()
	m.refreshInstallArchiveStates()
	m.modal = nil
	m.status = "installed " + msg.name + " to " + installDestinationLabels(msg.destinations)
	m.install.Message = m.status
	return nil
}

func (m *Model) showMissingSkillInRepoModal(err error) bool {
	var missing *remote.MissingSkillError
	if !errors.As(err, &missing) {
		return false
	}
	repoURL := missing.RepoURL
	if repoURL == "" {
		repoURL = "unknown repo"
	}
	m.status = "couldn't find " + missing.Name + " in repo"
	m.install.Message = m.status
	m.modal = newResultModal("Uh-oh...", []string{
		"Couldn't find the requested skill in repo.",
		"You might want to check the repo contents.",
		"",
		"Repo",
		"  " + repoURL,
		"",
		"Remember that this sometimes happens with skills.sh - it's stale data.",
		"",
		"[ OK ]",
	})
	return true
}

func isMissingSkillInRepoError(err error) bool {
	var missing *remote.MissingSkillError
	return errors.As(err, &missing)
}

func (m *Model) openInstallUseArchiveResolution(msg installUseMsg) tea.Cmd {
	switch msg.archiveState {
	case remote.ArchiveStateNameConflict, remote.ArchiveStateUpdateAvailable:
	default:
		return nil
	}
	row := msg.row
	if row.Result.Name == "" {
		return nil
	}
	row.ArchiveState = msg.archiveState
	switch msg.archiveState {
	case remote.ArchiveStateNameConflict:
		if !m.pendingInstallUseBatchMatches(installArchiveIdentityFromResult(row.Result)) {
			m.install.pendingUse = &pendingInstallUseIntent{row: row}
		}
		m.openInstallNameConflictModal(row)
		return nil
	case remote.ArchiveStateUpdateAvailable:
		cmd := m.openInstallUpdateDiff(row)
		identity := installArchiveIdentityFromResult(row.Result)
		if m.pendingInstallUseBatchMatches(identity) {
			if cmd != nil {
				m.install.pendingUseBatch.updateToken = m.install.previewToken
			}
		} else {
			m.install.pendingUse = &pendingInstallUseIntent{row: row, updateToken: m.install.previewToken}
		}
		return cmd
	default:
		return nil
	}
}

func installDestinationLabels(destinations []installDestination) string {
	labels := make([]string, 0, len(destinations))
	for _, dest := range destinations {
		labels = append(labels, dest.Label)
	}
	return strings.Join(labels, ", ")
}

func (m *Model) refreshInstallArchiveStates() {
	for i := range m.install.Results {
		m.install.Results[i].ArchiveState = m.installArchiveState(m.install.Results[i].Result)
	}
}

func (m *Model) updateInstallArchiveState(identity installArchiveIdentity, state string) {
	if state == "" {
		return
	}
	for i := range m.install.Results {
		if installArchiveIdentityFromResult(m.install.Results[i].Result) == identity {
			m.install.Results[i].ArchiveState = state
		}
	}
}

func installArchiveIdentityFromResult(result remote.SearchResult) installArchiveIdentity {
	return installArchiveIdentity{
		name:  result.Name,
		owner: result.Owner,
		repo:  result.Repo,
		path:  result.Path,
	}
}

func installAuditKey(result remote.SearchResult) string {
	identity := installArchiveIdentityFromResult(result)
	return identity.owner + "/" + identity.repo + "@" + identity.path + "@" + identity.name
}

func installAuditPill(audit remote.AuditSummary, opts Options) string {
	if !audit.Available {
		return ""
	}
	if opts.ASCII {
		if audit.Critical > 0 {
			return "!! risky"
		}
		if audit.Alerts > 0 {
			return "! warn"
		}
		return "OK safe"
	}
	return audit.Pill()
}

func (m Model) installArchiveState(result remote.SearchResult) string {
	meta := m.installSourceMetadata(result)
	archivePath, err := repo.SkillPath(m.cfg, result.Name)
	if err != nil || !repo.HasSkill(m.cfg, result.Name) {
		return remote.ArchiveStateNotArchived
	}
	existing, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil || !ok || !existing.SameIdentity(meta) {
		return remote.ArchiveStateNameConflict
	}
	return remote.ArchiveStateArchived
}

func (m Model) installSourceMetadata(result remote.SearchResult) remote.SourceMetadata {
	meta := remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      result.Owner,
		Repo:       result.Repo,
		SkillPath:  result.Path,
	}
	if m.install.testCloneURL != "" {
		meta.SourceType = remote.SourceTypeGit
		meta.CloneURL = m.install.testCloneURL
	}
	return meta
}
