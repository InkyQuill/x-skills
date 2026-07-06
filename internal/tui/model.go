package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
)

type ViewName string

const (
	ViewActive ViewName = "active"
	ViewRepo   ViewName = "repo"
	ViewDoctor ViewName = "doctor"
)

type Model struct {
	cfg      config.Config
	opts     Options
	symbols  symbols
	view     ViewName
	width    int
	height   int
	cursor   int
	selected map[string]bool
	filter   filterState

	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string

	modal  modal
	status string
	err    error
}

func New(cfg config.Config, opts ...Options) Model {
	options := defaultOptions()
	if len(opts) > 0 {
		options = opts[0]
	}
	m := Model{
		cfg:      cfg,
		opts:     options,
		symbols:  symbolsFor(options),
		view:     ViewActive,
		selected: map[string]bool{},
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
	default:
		return m, nil
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal != nil {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		close, cmd := m.modal.Update(msg, &m)
		if close {
			m.modal = nil
		}
		return m, cmd
	}

	if isRefreshKey(msg) {
		m.reload()
		m.status = "refreshed"
		return m, nil
	}

	if m.filter.Active {
		switch msg.String() {
		case keyActive:
			m.setView(ViewActive)
			return m, nil
		case keyRepo:
			m.setView(ViewRepo)
			return m, nil
		case keyDoctor:
			m.setView(ViewDoctor)
			return m, nil
		}
	}

	if m.filter.Active {
		if m.filter.update(msg) {
			m.cursor = 0
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case keyActive:
		m.setView(ViewActive)
	case keyRepo:
		m.setView(ViewRepo)
	case keyDoctor:
		m.setView(ViewDoctor)
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case " ":
		m.toggleSelection()
	case "/":
		if m.view == ViewActive || m.view == ViewRepo {
			m.filter.Active = true
			m.filter.Query = ""
		}
	case "enter":
		m.openDetailModal()
	case keyHelp:
		m.modal = newHelpModal()
	case "p":
		m.openPreviewModal()
	case "l":
		if m.view == ViewRepo {
			m.openRepoLinkModal()
		}
	case "m":
		m.openMigrateModal()
	case "u":
		if m.view == ViewRepo {
			m.openRepoUnlinkModal()
		} else {
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
		if m.cursor >= 0 && m.cursor < len(m.active) {
			m.modal = activeDetailModal(m.active[m.cursor])
		}
	}
}

func (m *Model) openPreviewModal() {
	switch m.view {
	case ViewActive:
		if m.cursor >= 0 && m.cursor < len(m.active) && len(m.active[m.cursor].Members) > 0 {
			m.modal = newPreviewModal("Preview: "+m.active[m.cursor].Name, resolvedSkillPath(m.active[m.cursor].Members[0].Path))
		}
	case ViewRepo:
		if m.cursor >= 0 && m.cursor < len(m.repo) {
			if path, err := repo.SkillPath(m.cfg, m.repo[m.cursor].Name); err == nil {
				m.modal = newPreviewModal("Preview: "+m.repo[m.cursor].Name, path)
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

func (m *Model) reload() {
	m.err = nil
	activeSkills, err := actions.ScanActive(m.cfg, actions.ScanFilter{})
	if err != nil {
		m.err = err
	}
	m.active = groupActiveSkills(activeSkills)
	m.repoUsage = usageByRepoName(m.active)

	repoSkills, err := repo.List(m.cfg)
	if err != nil && m.err == nil {
		m.err = err
	}
	m.repo = repoSkills

	issues, err := doctor.Diagnose(m.cfg, doctor.Filter{})
	if err != nil && m.err == nil {
		m.err = err
	}
	m.issues = issues
	m.clampCursor()
}

func (m *Model) setView(view ViewName) {
	if m.view == view {
		return
	}
	m.view = view
	m.cursor = 0
	m.selected = map[string]bool{}
	m.filter = filterState{}
}

func (m *Model) moveCursor(delta int) {
	count := m.itemCount()
	if count == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = count - 1
	}
	if m.cursor >= count {
		m.cursor = 0
	}
}

func (m *Model) toggleSelection() {
	id, ok := m.currentID()
	if !ok {
		return
	}
	m.selected[id] = !m.selected[id]
}

func (m *Model) selectedIDsForView() []string {
	var ids []string
	switch m.view {
	case ViewActive:
		for _, group := range m.active {
			if m.selected[group.ID] {
				ids = append(ids, group.ID)
			}
		}
	case ViewRepo:
		for _, skill := range m.repo {
			id := repoID(skill.Name)
			if m.selected[id] {
				ids = append(ids, id)
			}
		}
	case ViewDoctor:
		for _, issue := range m.issues {
			id := issueID(issue)
			if m.selected[id] {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		return ids
	}
	id, ok := m.currentID()
	if !ok {
		return nil
	}
	return []string{id}
}

func (m *Model) currentID() (string, bool) {
	switch m.view {
	case ViewActive:
		if m.cursor < 0 || m.cursor >= len(m.active) {
			return "", false
		}
		return m.active[m.cursor].ID, true
	case ViewRepo:
		if m.cursor < 0 || m.cursor >= len(m.repo) {
			return "", false
		}
		return repoID(m.repo[m.cursor].Name), true
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
		return len(m.active)
	case ViewRepo:
		return len(m.repo)
	case ViewDoctor:
		return len(m.issues)
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
