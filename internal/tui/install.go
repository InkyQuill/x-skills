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
	err          error
}

type installUseMsg struct {
	token        int
	name         string
	destinations []installDestination
	stale        bool
	err          error
}

type installArchiveIdentity struct {
	name  string
	owner string
	repo  string
	path  string
}

type pendingInstallUseIntent struct {
	row installResultView
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
	row, ok := m.selectedInstallResult()
	if !ok {
		return nil
	}
	if m.install.useInFlight || m.install.archiveInFlight {
		m.status = "install already running"
		m.install.Message = m.status
		return nil
	}
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

func (m *Model) applyInstallUpdateDiffResult(msg installUpdateDiffMsg) {
	if msg.token != m.install.previewToken || m.view != ViewInstall {
		return
	}
	if msg.err != nil {
		m.status = msg.err.Error()
		m.install.Message = m.status
		return
	}
	row := msg.row
	m.modal = newConflictDiffModalWithModelCommandApply(row.Result.Name, msg.diff, "Incoming remote", func(current *Model, chosen string) tea.Cmd {
		if chosen == actions.ConflictResolutionUseActive {
			return current.applyInstallArchiveWithConflict(row, row.Result.Name, remote.ConflictReplaceArchive)
		}
		current.status = "kept archive " + row.Result.Name
		current.install.Message = current.status
		current.install.pendingUse = nil
		return nil
	})
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

func (m *Model) installAndUse(row installResultView, destinations []installDestination) tea.Cmd {
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
	archiveCmd := m.archiveInstallRow(row)
	if archiveCmd == nil {
		return nil
	}
	m.install.useInFlight = true
	m.install.useInFlightToken = token
	cfg := m.cfg
	useGeneration := m.install.ensureUseGeneration()
	return func() tea.Msg {
		if !useGeneration.isCurrent(token) {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, stale: true}
		}
		archiveMsg := archiveCmd().(installArchiveMsg)
		if archiveMsg.err != nil {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: archiveMsg.err}
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
	name         string
	row          installResultView
	destinations []installDestination
	cursor       int
}

func newInstallDestinationModal(row installResultView) modal {
	return installDestinationModal{name: row.Result.Name, row: row, destinations: []installDestination{
		{Scope: config.ScopeProject, Target: config.TargetAgents, Label: ".Ag", Checked: true},
		{Scope: config.ScopeProject, Target: config.TargetClaude, Label: ".Cl"},
		{Scope: config.ScopeProject, Target: config.TargetCodex, Label: ".Cd"},
		{Scope: config.ScopeGlobal, Target: config.TargetAgents, Label: "~Ag"},
		{Scope: config.ScopeGlobal, Target: config.TargetClaude, Label: "~Cl"},
		{Scope: config.ScopeGlobal, Target: config.TargetCodex, Label: "~Cd"},
	}}
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
		m.install.pendingUse = &pendingInstallUseIntent{row: row}
		return m.openInstallUpdateDiff(row)
	}
	m.install.bumpUseToken()
	m.modal = newInstallDestinationModal(row)
	return nil
}

func (d installDestinationModal) Title() string {
	return "Install and use " + d.name
}

func (d installDestinationModal) View(width, height int, m Model) string {
	lines := []string{accentStyle.Render(d.Title()), ""}
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
		lines = append(lines, row)
	}
	lines = append(lines, "", mutedStyle.Render("up/down move  space toggle  enter install  esc cancel"))
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (d installDestinationModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.install.bumpUseToken()
		return true, nil
	case "up", "k":
		if len(d.destinations) > 0 {
			d.cursor = (d.cursor + len(d.destinations) - 1) % len(d.destinations)
		}
		m.modal = d
	case "down", "j":
		if len(d.destinations) > 0 {
			d.cursor = (d.cursor + 1) % len(d.destinations)
		}
		m.modal = d
	case " ":
		if d.cursor >= 0 && d.cursor < len(d.destinations) {
			d.destinations[d.cursor].Checked = !d.destinations[d.cursor].Checked
		}
		m.modal = d
	case "enter":
		destinations := d.checked()
		if len(destinations) == 0 {
			m.status = "select at least one destination"
			m.install.Message = m.status
			m.modal = d
			return false, nil
		}
		m.status = "installing " + d.name + "..."
		m.install.Message = m.status
		return true, m.installAndUse(d.row, destinations)
	}
	return false, nil
}

func (d installDestinationModal) checked() []installDestination {
	destinations := make([]installDestination, 0, len(d.destinations))
	for _, dest := range d.destinations {
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

func (m *Model) applyInstallSearchResult(msg installSearchResultMsg) {
	if msg.token != m.install.searchToken {
		return
	}
	m.install.Searching = false
	if msg.err != nil {
		m.install.Results = nil
		m.install.Message = msg.err.Error()
		m.status = msg.err.Error()
		return
	}
	m.install.Results = make([]installResultView, 0, len(msg.results))
	for _, result := range msg.results {
		m.install.Results = append(m.install.Results, installResultView{
			Result:       result,
			ArchiveState: m.installArchiveState(result),
		})
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
}

func (m *Model) applyInstallArchiveResult(msg installArchiveMsg) {
	if msg.token != 0 && msg.token == m.install.archiveInFlightToken {
		m.install.archiveInFlight = false
		m.install.archiveInFlightToken = 0
	}
	if msg.token == 0 || msg.token != m.install.archiveToken {
		return
	}
	if msg.err != nil {
		m.reload()
		m.refreshInstallArchiveStates()
		m.updateInstallArchiveState(msg.identity, msg.archiveState)
		m.status = msg.err.Error()
		m.install.Message = m.status
		if m.pendingInstallUseMatches(msg.identity) {
			m.install.pendingUse = nil
		}
		return
	}
	m.reload()
	m.refreshInstallArchiveStates()
	if pending := m.install.pendingUse; pending != nil && installArchiveIdentityFromResult(pending.row.Result) == msg.identity {
		row := pending.row
		if msg.name != "" {
			row.Result.Name = msg.name
		}
		row.ArchiveState = remote.ArchiveStateArchived
		m.install.pendingUse = nil
		m.install.bumpUseToken()
		m.modal = newInstallDestinationModal(row)
		m.status = "archived " + msg.name + "; choose destinations"
		m.install.Message = m.status
		return
	}
	m.status = "archived " + msg.name
	m.install.Message = m.status
}

func (m Model) pendingInstallUseMatches(identity installArchiveIdentity) bool {
	return m.install.pendingUse != nil && installArchiveIdentityFromResult(m.install.pendingUse.row.Result) == identity
}

func (m *Model) applyInstallUseResult(msg installUseMsg) {
	if msg.token != 0 && msg.token == m.install.useInFlightToken {
		m.install.useInFlight = false
		m.install.useInFlightToken = 0
	}
	if msg.token == 0 || msg.token != m.install.useToken {
		return
	}
	if msg.stale {
		return
	}
	if msg.err != nil {
		m.reload()
		m.refreshInstallArchiveStates()
		m.status = msg.err.Error()
		m.install.Message = m.status
		return
	}
	m.reload()
	m.refreshInstallArchiveStates()
	m.modal = nil
	m.status = "installed " + msg.name + " to " + installDestinationLabels(msg.destinations)
	m.install.Message = m.status
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
