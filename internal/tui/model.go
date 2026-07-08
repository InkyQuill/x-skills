package tui

import (
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
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
	cfg         config.Config
	opts        Options
	symbols     symbols
	view        ViewName
	width       int
	height      int
	cursor      int
	reloadToken int
	selected    map[ViewName]map[string]bool
	filter      filterState

	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	install   installState
	repoUsage map[string][]string

	modal  modal
	status string
	err    error
}

type reloadResultMsg struct {
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
	token     int
}

func New(cfg config.Config, opts ...Options) Model {
	options := defaultOptions()
	if len(opts) > 0 {
		options = opts[0]
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
		filter:  newFilterState(),
		install: newInstallState(),
	}
	m.reload()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case reloadResultMsg:
		m.applyReloadResult(msg)
		return m, nil
	case installSearchResultMsg:
		m.applyInstallSearchResult(msg)
		return m, nil
	case installPreviewMsg:
		if msg.token != m.install.previewToken || m.view != ViewInstall {
			return m, nil
		}
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.modal = newPreviewModal("Preview: "+msg.name, msg.path)
		return m, nil
	case installArchiveMsg:
		m.applyInstallArchiveResult(msg)
		return m, nil
	case installUseMsg:
		m.applyInstallUseResult(msg)
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.modal != nil {
		close, cmd := m.modal.Update(msg, &m)
		if close {
			m.modal = nil
		}
		return m, cmd
	}

	if m.view == ViewInstall && m.install.InputMode != installInputNone {
		cmd := m.updateInstallInput(msg)
		return m, cmd
	}

	if isRefreshKey(msg) {
		m.reload()
		m.status = "refreshed"
		return m, nil
	}

	if m.filter.Active {
		if m.filter.update(msg) {
			m.cursor = 0
			return m, nil
		}
	}

	switch msg.String() {
	case "q":
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
		if m.view == ViewActive || m.view == ViewRepo {
			m.selected[m.view] = map[string]bool{}
			m.status = "selection cleared"
		}
	case "/":
		if m.view == ViewInstall {
			m.install.InputMode = installInputQuery
			m.status = ""
		} else if m.view == ViewActive || m.view == ViewRepo {
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
		m.openDetailModal()
	case "a":
		if m.view == ViewInstall {
			return m, m.archiveInstallResult()
		}
	case "i":
		if m.view == ViewInstall {
			if row, ok := m.selectedInstallResult(); ok {
				m.modal = newInstallDestinationModal(row)
			}
		}
	case keyHelp:
		m.modal = newHelpModal()
	case "p":
		m.openPreviewModal()
	case "l":
		if m.view == ViewRepo {
			m.openRepoLinkModal()
		}
	case "m":
		if m.view == ViewActive {
			m.openMigrateModal()
		}
	case "u":
		if m.view == ViewRepo {
			m.openRepoUnlinkModal()
		} else if m.view == ViewActive {
			m.openUnlinkModal()
		}
	case "d":
		if m.view == ViewRepo {
			m.openRepoDeleteModal()
		}
	case "f":
		if m.view == ViewDoctor {
			m.openDoctorFixModal()
		}
	}

	return m, nil
}

func (m *Model) openDetailModal() {
	switch m.view {
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor >= 0 && m.cursor < len(groups) {
			m.modal = activeDetailModal(groups[m.cursor], m.symbols)
		}
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor >= 0 && m.cursor < len(skills) {
			if skill, ok := m.repoSkillByName(skills[m.cursor].Name); ok {
				m.modal = repoDetailModal(skill, m.repoUsage[skill.Name], m.symbols)
			}
		}
	case ViewDoctor:
		if m.cursor >= 0 && m.cursor < len(m.issues) {
			m.modal = doctorDetailModal(m.issues[m.cursor])
		}
	}
}

func (m *Model) repoSkillByName(name string) (repo.Skill, bool) {
	for _, skill := range m.repo {
		if skill.Name == name {
			return skill, true
		}
	}
	return repo.Skill{}, false
}

func (m *Model) openPreviewModal() {
	switch m.view {
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor >= 0 && m.cursor < len(groups) && len(groups[m.cursor].Members) > 0 {
			m.modal = newPreviewModal("Preview: "+groups[m.cursor].Name, resolvedSkillPath(groups[m.cursor].Members[0].Path))
		}
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor >= 0 && m.cursor < len(skills) {
			if path, err := repo.SkillPath(m.cfg, skills[m.cursor].Name); err == nil {
				m.modal = newPreviewModal("Preview: "+skills[m.cursor].Name, path)
			}
		}
	}
}

func resolvedSkillPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func loadTUIData(cfg config.Config) ([]ActiveGroup, []repo.Skill, []doctor.Issue, map[string][]string, error) {
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

	issues, err := doctor.Diagnose(cfg, doctor.Filter{})
	if err != nil && firstErr == nil {
		firstErr = err
	}
	sort.Slice(issues, func(i, j int) bool {
		return skillNameLess(issues[i].Name, issues[j].Name)
	})

	return activeGroups, repoSkills, issues, repoUsage, firstErr
}

func (m *Model) reload() {
	m.active, m.repo, m.issues, m.repoUsage, m.err = loadTUIData(m.cfg)
	m.clampCursor()
}

func (m *Model) reloadCmd() tea.Cmd {
	m.reloadToken++
	token := m.reloadToken
	cfg := m.cfg

	return func() tea.Msg {
		active, repoSkills, issues, repoUsage, err := loadTUIData(cfg)
		return reloadResultMsg{
			active:    active,
			repo:      repoSkills,
			issues:    issues,
			repoUsage: repoUsage,
			err:       err,
			token:     token,
		}
	}
}

func (m *Model) applyReloadResult(msg reloadResultMsg) {
	if msg.token != m.reloadToken {
		return
	}
	m.active = msg.active
	m.repo = msg.repo
	m.issues = msg.issues
	m.repoUsage = msg.repoUsage
	m.err = msg.err
	m.clampCursor()
}

func (m *Model) setView(view ViewName) {
	if m.view == view {
		return
	}
	if m.view == ViewInstall && view != ViewInstall {
		m.install.previewToken++
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
