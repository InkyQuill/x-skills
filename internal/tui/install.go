package tui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Query        string
	Owner        string
	Searching    bool
	Results      []installResultView
	Message      string
	InputMode    installInputMode
	searchToken  int
	previewToken int
	archiveToken int
	useToken     int
	searchClient remote.SearchClient
	checkouts    *remote.CheckoutCache
	testCloneURL string
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
	err          error
}

type installArchiveIdentity struct {
	name  string
	owner string
	repo  string
	path  string
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

func newInstallState() installState {
	return installState{
		Message:      "type at least 2 characters",
		searchClient: remote.NewSearchClient(remote.DefaultSearchEndpoint, http.DefaultClient),
	}
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
	return m.archiveInstallRow(row)
}

func (m *Model) archiveInstallRow(row installResultView) tea.Cmd {
	if row.ArchiveState == remote.ArchiveStateNameConflict {
		m.status = "archive conflict for " + row.Result.Name
		m.install.Message = m.status
		return nil
	}
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
		plan, err := remote.PlanArchive(cfg, found.SkillDir, row.Result.Name, found.Metadata)
		if err != nil {
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, err: err}
		}
		switch plan.State {
		case remote.ArchiveStateNotArchived:
		case remote.ArchiveStateArchived:
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: plan.State}
		case remote.ArchiveStateNameConflict:
			return installArchiveMsg{
				token:        token,
				name:         row.Result.Name,
				identity:     identity,
				archiveState: plan.State,
				err:          fmt.Errorf("archive conflict for %s", row.Result.Name),
			}
		case remote.ArchiveStateUpdateAvailable:
			return installArchiveMsg{
				token:        token,
				name:         row.Result.Name,
				identity:     identity,
				archiveState: plan.State,
				err:          fmt.Errorf("update available for %s", row.Result.Name),
			}
		default:
			return installArchiveMsg{
				token:        token,
				name:         row.Result.Name,
				identity:     identity,
				archiveState: plan.State,
				err:          fmt.Errorf("unknown archive state %q for %s", plan.State, row.Result.Name),
			}
		}
		_, err = remote.ApplyArchive(remote.AddRequest{
			Config:      cfg,
			IncomingDir: found.SkillDir,
			ArchiveName: row.Result.Name,
			Metadata:    found.Metadata,
			Conflict:    remote.ConflictArchiveOnly,
		})
		if err != nil {
			plan, planErr := remote.PlanArchive(cfg, found.SkillDir, row.Result.Name, found.Metadata)
			if planErr == nil {
				return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: plan.State, err: err}
			}
			return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, err: err}
		}
		return installArchiveMsg{token: token, name: row.Result.Name, identity: identity, archiveState: remote.ArchiveStateArchived}
	}
}

func (m *Model) installAndUse(row installResultView, destinations []installDestination) tea.Cmd {
	if len(destinations) == 0 {
		m.status = "select at least one destination"
		m.install.Message = m.status
		return nil
	}
	m.install.useToken++
	token := m.install.useToken
	archiveCmd := m.archiveInstallRow(row)
	if archiveCmd == nil {
		return nil
	}
	cfg := m.cfg
	return func() tea.Msg {
		archiveMsg := archiveCmd().(installArchiveMsg)
		if archiveMsg.err != nil {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: archiveMsg.err}
		}
		if err := preflightInstallUseDestinations(cfg, row.Result.Name, destinations); err != nil {
			return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: err}
		}
		for _, dest := range destinations {
			_, err := actions.Link(cfg, actions.LinkRequest{Name: row.Result.Name, Scope: dest.Scope, Target: dest.Target})
			if err != nil {
				return installUseMsg{token: token, name: row.Result.Name, destinations: destinations, err: err}
			}
		}
		return installUseMsg{token: token, name: row.Result.Name, destinations: destinations}
	}
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
		return false, m.installAndUse(d.row, destinations)
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
	if msg.token == 0 || msg.token != m.install.archiveToken {
		return
	}
	if msg.err != nil {
		m.reload()
		m.refreshInstallArchiveStates()
		m.updateInstallArchiveState(msg.identity, msg.archiveState)
		m.status = msg.err.Error()
		return
	}
	m.reload()
	m.refreshInstallArchiveStates()
	m.status = "archived " + msg.name
}

func (m *Model) applyInstallUseResult(msg installUseMsg) {
	if msg.token == 0 || msg.token != m.install.useToken {
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
