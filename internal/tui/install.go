package tui

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
	searchClient remote.SearchClient
	checkouts    *remote.CheckoutCache
}

type installSearchResultMsg struct {
	token   int
	query   string
	results []remote.SearchResult
	err     error
}

type installResultView struct {
	Result       remote.SearchResult
	ArchiveState string
	AuditPill    string
}

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
		results, err := client.Search(context.Background(), remote.SearchRequest{Query: query, Owner: owner, Limit: remote.DefaultSearchLimit})
		return installSearchResultMsg{token: token, query: query, results: results, err: err}
	}
}

func (m *Model) startInstallSearch() tea.Cmd {
	m.install.searchToken++
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
	return nil
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
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

func (m Model) installArchiveState(result remote.SearchResult) string {
	meta := remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      result.Owner,
		Repo:       result.Repo,
		SkillPath:  result.Path,
	}
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
