package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/buildinfo"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/repo"
)

type ViewName string

const (
	ViewActive  ViewName = "active"
	ViewRepo    ViewName = "repo"
	ViewDoctor  ViewName = "doctor"
	ViewInstall ViewName = "install"
)

type Model struct {
	cfg            config.Config
	opts           Options
	symbols        symbols
	view           ViewName
	width          int
	height         int
	cursor         int
	animationFrame int
	selected       map[ViewName]map[string]bool
	filter         filterState

	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	install   installState
	repoUsage map[string][]string

	modal  modal
	status string
	err    error

	previewToken   int
	previewCancel  context.CancelFunc
	previewLoading bool
	resolvePreview previewResolver

	reloadToken            int
	reloadInFlight         bool
	reloadPending          tea.Cmd
	reloadReportsStatus    bool
	buildInfo              buildinfo.Info
	latestRelease          string
	startupLoadCmd         tea.Cmd
	updateCheckCmd         tea.Cmd
	reloadCancel           context.CancelFunc
	updateCancel           context.CancelFunc
	updateToken            int
	updating               bool
	doctorFixToken         int
	doctorFixInFlight      bool
	doctorFixCancel        context.CancelFunc
	mutationToken          uint64
	mutationInFlight       bool
	mutationProjectTouched bool
	pendingMutationCmd     tea.Cmd
	recommendationToken    uint64
	recommendationInFlight bool
	renameToken            uint64
	renameInFlight         bool
	renameCancel           context.CancelFunc
	restoreToken           uint64
	restoreInFlight        bool
	restoreCancel          context.CancelFunc
	syncToken              uint64
	syncInFlight           bool
	syncCancel             context.CancelFunc
}

type reloadResultMsg struct {
	token     int
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
}

type latestReleaseMsg struct {
	token   int
	version string
}

func New(cfg config.Config, opts ...Options) Model {
	options := defaultOptions()
	if len(opts) > 0 {
		provided := opts[0]
		options.ASCII = provided.ASCII
		if provided.BuildInfo != (buildinfo.Info{}) {
			options.BuildInfo = provided.BuildInfo
		}
		if provided.LatestReleaseChecker != nil {
			options.LatestReleaseChecker = provided.LatestReleaseChecker
		}
		if provided.loadData != nil {
			options.loadData = provided.loadData
		}
	}
	m := Model{
		cfg:     cfg,
		opts:    options,
		symbols: symbolsFor(options),
		view:    ViewActive,
		selected: map[ViewName]map[string]bool{
			ViewActive:  {},
			ViewRepo:    {},
			ViewDoctor:  {},
			ViewInstall: {},
		},
		filter:         newFilterState(),
		install:        newInstallState(),
		buildInfo:      options.BuildInfo,
		resolvePreview: remote.ResolvePreview,
	}
	m.startupLoadCmd = m.beginReloadWithStatus(false)
	if m.opts.LatestReleaseChecker != nil && m.buildInfo.IsRelease() {
		ctx, cancel := context.WithCancel(context.Background())
		m.updateCancel = cancel
		m.updateToken++
		m.updateCheckCmd = m.latestReleaseCmd(ctx, m.updateToken)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.startupLoadCmd, m.updateCheckCmd}
	if m.animationsEnabled() {
		cmds = append(cmds, animationTick())
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	m.updating = true
	defer func() {
		current, ok := updated.(Model)
		if !ok {
			return
		}
		current.updating = false
		if current.reloadPending == nil {
			updated = current
			return
		}
		reloadCmd := current.reloadPending
		current.reloadPending = nil
		updated = current
		cmd = tea.Batch(cmd, reloadCmd)
	}()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case animationTickMsg:
		if !m.animationsEnabled() {
			return m, nil
		}
		m.animationFrame++
		return m, animationTick()
	case installSearchResultMsg:
		return m, m.applyInstallSearchResult(msg)
	case remotePreviewMsg:
		m.applyRemotePreview(msg)
		return m, nil
	case installUpdateDiffMsg:
		return m, m.applyInstallUpdateDiffResult(msg)
	case installArchiveMsg:
		return m, m.applyInstallArchiveResult(msg)
	case installBatchProgressMsg:
		if msg.token == m.install.archiveInFlightToken {
			m.status = fmt.Sprintf("archiving %d/%d: %s", msg.completed, msg.total, msg.name)
			m.install.Message = m.status
			return m, msg.next
		}
		return m, nil
	case installBatchCancelledMsg:
		if msg.token == m.install.archiveInFlightToken {
			m.install.archiveInFlight = false
			m.install.archiveInFlightToken = 0
		}
		return m, nil
	case installArchiveStatesMsg:
		m.applyInstallArchiveStateResults(msg)
		return m, nil
	case installUseMsg:
		return m, m.applyInstallUseResult(msg)
	case doctorFixResultMsg:
		return m, m.applyDoctorFixResult(msg)
	case mutationReconcileMsg:
		return m, m.applyMutationReconcileResult(msg)
	case recommendationResultMsg:
		return m, m.applyRecommendationResult(msg)
	case renameArchiveResultMsg:
		return m, m.applyRenameArchiveResult(msg)
	case restorePlanMsg:
		return m, m.applyRestorePlanResult(msg)
	case restoreApplyMsg:
		return m, m.applyRestoreResult(msg)
	case syncCandidatesMsg:
		return m, m.applySyncCandidates(msg)
	case syncPlanMsg:
		return m, m.applySyncPlan(msg)
	case syncProgressMsg:
		return m, m.applySyncProgress(msg)
	case syncResultMsg:
		return m, m.applySyncResult(msg)
	case reloadResultMsg:
		if msg.token != m.reloadToken {
			return m, nil
		}
		m.active = msg.active
		m.repo = msg.repo
		m.issues = msg.issues
		m.repoUsage = msg.repoUsage
		m.err = msg.err
		m.reloadInFlight = false
		m.reloadCancel = nil
		m.clampCursor()
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		if m.reloadReportsStatus {
			m.status = "refreshed"
		}
		return m, nil
	case latestReleaseMsg:
		if msg.token != m.updateToken {
			return m, nil
		}
		m.updateCancel = nil
		if version, ok := m.buildInfo.NewerStable(msg.version); ok {
			m.latestRelease = version
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.cancelRemotePreview()
		m.cancelStartupWork()
		m.cancelRenameWork()
		closeRestoreModalPlan(m.modal)
		m.cancelInstallWork()
		m.cancelRestoreWork()
		m.cancelSyncWork()
		m.cancelDoctorFixWork()
		return m, tea.Quit
	}

	if m.modal != nil {
		closedModal := m.modal
		modalClosed, cmd := m.modal.Update(msg, &m)
		if m.pendingMutationCmd != nil {
			cmd = tea.Batch(cmd, m.pendingMutationCmd)
			m.pendingMutationCmd = nil
		}
		if modalClosed {
			m.clearPendingInstallUseOnModalClose(closedModal)
			m.modal = nil
		}
		return m, cmd
	}

	if m.view == ViewInstall && m.install.InputMode != installInputNone {
		cmd := m.updateInstallInput(msg)
		return m, cmd
	}

	if isRefreshKey(msg) {
		return m, m.beginReload()
	}

	if m.filter.Active {
		previousQuery := m.filter.Query
		if handled, cmd := m.filter.update(msg); handled {
			m.cursor = 0
			if m.filter.Query != previousQuery {
				m.selected[m.view] = map[string]bool{}
			}
			return m, cmd
		}
	}

	switch msg.String() {
	case "q":
		m.cancelRemotePreview()
		m.cancelStartupWork()
		m.cancelRenameWork()
		m.cancelInstallWork()
		m.cancelRestoreWork()
		m.cancelSyncWork()
		m.cancelDoctorFixWork()
		return m, tea.Quit
	case keyActive:
		m.setView(ViewActive)
	case keyRepo:
		m.setView(ViewRepo)
	case keyDoctor:
		m.setView(ViewDoctor)
	case keyInstall:
		m.setView(ViewInstall)
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case " ":
		m.toggleSelection()
	case "c":
		if m.view == ViewActive || m.view == ViewRepo || m.view == ViewInstall {
			m.selected[m.view] = map[string]bool{}
			m.status = "selection cleared"
		}
	case "/":
		switch m.view {
		case ViewInstall:
			m.install.InputMode = installInputQuery
			m.status = ""
		case ViewActive, ViewRepo:
			m.filter = newFilterState()
			m.filter.Active = true
			m.filter.input.Focus()
		}
	case "o":
		if m.view == ViewInstall {
			m.install.InputMode = installInputOwner
			m.status = ""
		}
	case "enter":
		if m.view == ViewInstall {
			return m, m.previewInstallResult()
		}
		if m.view == ViewDoctor {
			m.openDetailModal()
		} else {
			m.openPreviewModal()
		}
	case "a":
		if m.view == ViewInstall {
			return m, m.archiveInstallResult()
		}
	case "i":
		if m.view == ViewInstall {
			return m, m.openInstallDestinationModalForRows(m.installActionRows())
		}
	case keyHelp:
		m.modal = newHelpModal()
	case "p":
		m.openPreviewModal()
	case "l":
		if m.view == ViewRepo {
			m.openRepoLinkModal()
		}
	case "r":
		if m.view == ViewRepo {
			return m, m.toggleRepoRecommendations()
		}
	case keyRepoRename:
		if m.view == ViewRepo {
			if m.renameInFlight {
				m.status = "rename already running"
				return m, nil
			}
			m.openRepoRenameModal()
		}
	case "s":
		if m.restoreInFlight {
			m.status = "restore already running"
			return m, nil
		}
		m.modal = newRestoreWorkbenchModal(m.cfg)
	case "S":
		if m.syncInFlight {
			m.status = "sync already running"
			return m, nil
		}
		m.modal = newSyncWorkbenchModal(m.cfg)
	case "m":
		if m.view == ViewActive {
			m.openMigrateModal()
		}
	case "u":
		switch m.view {
		case ViewRepo:
			m.openRepoUnlinkModal()
		case ViewActive:
			m.openUnlinkModal()
		}
	case "d":
		if m.view == ViewRepo {
			m.openRepoDeleteModal()
		}
	case "f":
		if m.view == ViewDoctor {
			if m.doctorFixInFlight {
				m.status = "doctor fix already running"
				return m, nil
			}
			m.openDoctorFixModal()
		}
	}

	return m, nil
}

func (m *Model) openDetailModal() {
	if m.view == ViewDoctor && m.cursor >= 0 && m.cursor < len(m.issues) {
		m.modal = doctorDetailModal(m.issues[m.cursor])
	}
}

func (m *Model) openPreviewModal() {
	switch m.view {
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor >= 0 && m.cursor < len(groups) && len(groups[m.cursor].Members) > 0 {
			m.modal = newPreviewModal("Preview: "+groups[m.cursor].Identity, resolvedSkillPath(groups[m.cursor].Members[0].Path))
		}
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor >= 0 && m.cursor < len(skills) {
			path, err := repo.SkillPath(m.cfg, skills[m.cursor].Name)
			if err != nil {
				m.status = "could not open preview: " + err.Error()
				return
			}
			m.modal = newPreviewModal("Preview: "+skills[m.cursor].Name, path)
		}
	}
}

func resolvedSkillPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func loadTUIData(ctx context.Context, cfg config.Config) ([]ActiveGroup, []repo.Skill, []doctor.Issue, map[string][]string, error) {
	var firstErr error
	activeSkills, err := actions.ScanActive(cfg, actions.ScanFilter{})
	if err != nil {
		firstErr = err
	}
	activeGroups := groupActiveSkills(activeSkills)
	repoUsage := usageByRepoName(activeGroups)

	repoSkills, err := repo.List(cfg)
	if err != nil && firstErr == nil {
		firstErr = err
	}

	issues, err := doctor.Diagnose(ctx, cfg, doctor.Filter{})
	if err != nil && firstErr == nil {
		firstErr = err
	}
	sort.Slice(issues, func(i, j int) bool {
		return skillNameLess(issues[i].Name, issues[j].Name)
	})

	return activeGroups, repoSkills, issues, repoUsage, firstErr
}

func (m *Model) reload() {
	if m.updating {
		m.reloadPending = m.beginReload()
		m.reloadReportsStatus = false
		return
	}
	m.reloadSynchronously()
}

func (m *Model) reloadSynchronously() {
	m.active, m.repo, m.issues, m.repoUsage, m.err = m.opts.loadData(context.Background(), m.cfg)
	m.clampCursor()
}

func (m *Model) reloadCmd(ctx context.Context) tea.Cmd {
	token := m.reloadToken
	cfg := m.cfg
	loader := m.opts.loadData
	return func() tea.Msg {
		active, repoSkills, issues, repoUsage, err := loader(ctx, cfg)
		return reloadResultMsg{
			token:     token,
			active:    active,
			repo:      repoSkills,
			issues:    issues,
			repoUsage: repoUsage,
			err:       err,
		}
	}
}

func (m *Model) beginReload() tea.Cmd {
	return m.beginReloadWithStatus(true)
}

func (m *Model) beginReloadWithStatus(report bool) tea.Cmd {
	if m.reloadCancel != nil {
		m.reloadCancel()
	}
	m.reloadToken++
	m.reloadInFlight = true
	m.reloadReportsStatus = report
	if report {
		m.status = "refreshing..."
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.reloadCancel = cancel
	return m.reloadCmd(ctx)
}

func (m Model) latestReleaseCmd(ctx context.Context, token int) tea.Cmd {
	checker := m.opts.LatestReleaseChecker
	return func() tea.Msg {
		version, err := checker.LatestRelease(ctx)
		if err != nil {
			return latestReleaseMsg{token: token}
		}
		return latestReleaseMsg{token: token, version: version}
	}
}

func (m *Model) cancelStartupWork() {
	if m.reloadCancel != nil {
		m.reloadCancel()
		m.reloadCancel = nil
	}
	if m.updateCancel != nil {
		m.updateCancel()
		m.updateCancel = nil
	}
}

func (m *Model) setView(view ViewName) {
	if m.view == view {
		return
	}
	if m.view == ViewInstall && view != ViewInstall {
		m.closeRemotePreview()
		m.cancelInstallWork()
		m.install.InputMode = installInputNone
	}
	m.view = view
	m.cursor = 0
	m.selected = map[ViewName]map[string]bool{
		ViewActive:  {},
		ViewRepo:    {},
		ViewDoctor:  {},
		ViewInstall: {},
	}
	m.filter = newFilterState()
}

func (m *Model) cancelInstallWork() {
	m.install.cancelOperations()
	m.install.previewToken++
	m.install.archiveToken++
	m.install.bumpUseToken()
	m.install.archiveInFlight = false
	m.install.archiveInFlightToken = 0
	m.install.useInFlight = false
	m.install.useInFlightToken = 0
}

func (m *Model) moveCursor(delta int) {
	count := m.itemCount()
	if count == 0 {
		m.cursor = 0
		return
	}
	previous := m.cursor
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = count - 1
	}
	if m.cursor >= count {
		m.cursor = 0
	}
	if m.view == ViewInstall && m.cursor != previous {
		m.closeRemotePreview()
		m.install.previewToken++
	}
}

func (m *Model) toggleSelection() {
	if m.view == ViewDoctor {
		return
	}
	id, ok := m.currentID()
	if !ok {
		return
	}
	if m.selected[m.view] == nil {
		m.selected[m.view] = map[string]bool{}
	}
	m.selected[m.view][id] = !m.selected[m.view][id]
}

func (m *Model) selectedIDsForView() []string {
	var ids []string
	switch m.view {
	case ViewActive:
		for _, group := range m.visibleActiveGroups() {
			if m.selected[ViewActive][group.ID] {
				ids = append(ids, group.ID)
			}
		}
	case ViewRepo:
		for _, skill := range m.visibleRepoSkills() {
			id := repoID(skill.Name)
			if m.selected[ViewRepo][id] {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		return ids
	}
	switch m.view {
	case ViewActive, ViewRepo:
		id, ok := m.currentID()
		if !ok {
			return nil
		}
		return []string{id}
	default:
		return nil
	}
}

func (m *Model) currentID() (string, bool) {
	switch m.view {
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor < 0 || m.cursor >= len(groups) {
			return "", false
		}
		return groups[m.cursor].ID, true
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor < 0 || m.cursor >= len(skills) {
			return "", false
		}
		return repoID(skills[m.cursor].Name), true
	case ViewInstall:
		if m.cursor < 0 || m.cursor >= len(m.install.Results) {
			return "", false
		}
		return installID(m.install.Results[m.cursor].Result), true
	case ViewDoctor:
		if m.cursor < 0 || m.cursor >= len(m.issues) {
			return "", false
		}
		return issueID(m.issues[m.cursor]), true
	default:
		return "", false
	}
}

func (m *Model) itemCount() int {
	switch m.view {
	case ViewActive:
		return len(m.visibleActiveGroups())
	case ViewRepo:
		return len(m.visibleRepoSkills())
	case ViewDoctor:
		return len(m.issues)
	case ViewInstall:
		return len(m.install.Results)
	default:
		return 0
	}
}

func (m *Model) clampCursor() {
	count := m.itemCount()
	if count == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= count {
		m.cursor = count - 1
	}
}

func repoID(name string) string {
	return "repo:" + name
}

func issueID(issue doctor.Issue) string {
	return "doctor:" + issue.Path
}
