package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
)

func TestInstallTabSwitchesAndRendersShell(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.width = 120
	m.height = 30
	updated, _ := m.Update(keyRunes("I"))
	m = mustModel(t, updated)
	if m.view != ViewInstall {
		t.Fatalf("view = %q, want install", m.view)
	}
	view := plain(m.View())
	for _, want := range []string{"I:Install", "Install: search", "type at least 2 characters", "/ search", "i install & use", "a archive only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("install shell missing %q:\n%s", want, view)
		}
	}
}

func TestInstallHelpShowsRealInstallKeys(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	view := plain(newHelpModal().View(100, 40, m))
	for _, want := range []string{"switch to Install view", "Install: / search", "Install: i install and use", "Install: a archive only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "not yet available") {
		t.Fatalf("help still says install is unavailable:\n%s", view)
	}
}

func TestInstallScrollKeepsFocusedResultAndSearchVisible(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.view = ViewInstall
	m.width = 80
	m.height = 10
	m.install.Query = "skill"
	for i := range 12 {
		m.install.Results = append(m.install.Results, installResultView{
			Result: remote.SearchResult{
				Name:        fmt.Sprintf("skill-%02d", i),
				Description: fmt.Sprintf("description-%02d", i),
				Owner:       "owner",
				Repo:        "repo",
			},
			ArchiveState: "remote",
		})
	}
	m.cursor = len(m.install.Results) - 1

	view := plain(m.View())
	for _, want := range []string{"/ search:", "skill-11"} {
		if !strings.Contains(view, want) {
			t.Fatalf("install view missing %q with cursor at last result:\n%s", want, view)
		}
	}
}

func TestInstallSearchRunsAfterEnterAndKeepsResults(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.install.searchClient = remote.NewSearchClient(testSearchServer(t, []remote.SearchResult{
		{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder", Installs: 812},
	}), http.DefaultClient)
	m.setView(ViewInstall)

	updated, cmd := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	for _, key := range []string{"s", "v", "e", "l", "t", "e"} {
		updated, cmd = m.Update(keyRunes(key))
		m = mustModel(t, updated)
		_ = cmd
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if !m.install.Searching {
		t.Fatal("searching = false")
	}
	msg := cmd().(installSearchResultMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if len(m.install.Results) != 1 || m.install.Results[0].Result.Name != "svelte-coder" {
		t.Fatalf("results = %#v", m.install.Results)
	}
	if m.status != "found 1 result for \"svelte\"" {
		t.Fatalf("status = %q", m.status)
	}
	if m.install.Message != "found 1 result for \"svelte\"" {
		t.Fatalf("message = %q", m.install.Message)
	}
}

func TestInstallSearchErrorClearsPreviousResults(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 100
	m.height = 30
	m.install.searchToken = 1

	updated, _ := m.Update(installSearchResultMsg{
		token: 1,
		query: "svelte",
		results: []remote.SearchResult{
			{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder", Installs: 812},
		},
	})
	m = mustModel(t, updated)
	if len(m.install.Results) != 1 {
		t.Fatalf("results after success = %#v", m.install.Results)
	}

	m.install.searchToken = 2
	updated, _ = m.Update(installSearchResultMsg{
		token: 2,
		query: "react",
		err:   errors.New("search failed"),
	})
	m = mustModel(t, updated)
	if len(m.install.Results) != 0 {
		t.Fatalf("results after error = %#v", m.install.Results)
	}
	view := plain(m.View())
	if !strings.Contains(view, "search failed") {
		t.Fatalf("install view missing error:\n%s", view)
	}
	if strings.Contains(view, "svelte-coder") {
		t.Fatalf("install view shows stale result after error:\n%s", view)
	}
}

func TestInstallSearchShortQueryInvalidatesPendingResult(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.install.searchClient = remote.NewSearchClient(testSearchServer(t, []remote.SearchResult{
		{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder", Installs: 812},
	}), http.DefaultClient)
	m.setView(ViewInstall)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	for _, key := range []string{"s", "v", "e", "l", "t", "e"} {
		updated, _ = m.Update(keyRunes(key))
		m = mustModel(t, updated)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	oldMsg := cmd().(installSearchResultMsg)

	updated, _ = m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	for range 5 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = mustModel(t, updated)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("short query command = %#v, want nil", cmd)
	}
	if m.install.Searching {
		t.Fatal("searching = true after short query")
	}

	updated, _ = m.Update(oldMsg)
	m = mustModel(t, updated)
	if len(m.install.Results) != 0 {
		t.Fatalf("stale results applied: %#v", m.install.Results)
	}
	if m.install.Message != "type at least 2 characters" {
		t.Fatalf("message = %q", m.install.Message)
	}
}

func TestInstallSearchShortQueryClearsPreviousResults(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 100
	m.height = 30
	m.install.searchToken = 1

	updated, _ := m.Update(installSearchResultMsg{
		token: 1,
		query: "svelte",
		results: []remote.SearchResult{
			{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder", Installs: 812},
		},
	})
	m = mustModel(t, updated)
	if len(m.install.Results) != 1 {
		t.Fatalf("results after success = %#v", m.install.Results)
	}

	m.install.InputMode = installInputQuery
	m.install.Query = "x"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("short query command = %#v, want nil", cmd)
	}
	if len(m.install.Results) != 0 {
		t.Fatalf("results after short query = %#v", m.install.Results)
	}
	if m.status != "type at least 2 characters" {
		t.Fatalf("status = %q", m.status)
	}
	view := plain(m.View())
	if !strings.Contains(view, "type at least 2 characters") {
		t.Fatalf("install view missing short-query message:\n%s", view)
	}
	if strings.Contains(view, "svelte-coder") {
		t.Fatalf("install view shows stale result after short query:\n%s", view)
	}
}

func TestInstallSearchZeroResultsUpdatesMessage(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.install.searchClient = remote.NewSearchClient(testSearchServer(t, nil), http.DefaultClient)
	m.setView(ViewInstall)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	for _, key := range []string{"m", "i", "s", "s"} {
		updated, _ = m.Update(keyRunes(key))
		m = mustModel(t, updated)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	msg := cmd().(installSearchResultMsg)

	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.install.Message != "no results for \"miss\"" {
		t.Fatalf("message = %q", m.install.Message)
	}
	if m.status != "found 0 results for \"miss\"" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallSearchCommandUsesBoundedContext(t *testing.T) {
	var sawDeadline bool
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			deadline, ok := r.Context().Deadline()
			if !ok {
				return nil, errors.New("missing search context deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > installSearchTimeout {
				return nil, fmt.Errorf("search context deadline = %s, want within %s", remaining, installSearchTimeout)
			}
			sawDeadline = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioNopCloser{strings.NewReader(`{"results":[]}`)},
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.install.searchClient = remote.NewSearchClient("https://skills.example/search", client)
	m.setView(ViewInstall)
	m.install.InputMode = installInputQuery
	m.install.Query = "svelte"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installSearchResultMsg)
	if msg.err != nil {
		t.Fatalf("search err = %v", msg.err)
	}
	if !sawDeadline {
		t.Fatal("search command did not reach bounded transport")
	}
}

func TestInstallEnterPreviewsRemoteSkill(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Owner: "", Repo: "", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	msg := cmd().(installPreviewMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("preview modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "Preview: svelte-coder") || !strings.Contains(view, "Svelte help.") {
		t.Fatalf("preview missing remote content:\n%s", view)
	}
}

func TestInstallPreviewInitializesAndReusesCheckoutCache(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.install.checkouts == nil {
		t.Fatal("checkout cache is nil after preview starts")
	}
	firstMsg := cmd().(installPreviewMsg)
	updated, _ = m.Update(firstMsg)
	m = mustModel(t, updated)
	m.modal = nil

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	secondMsg := cmd().(installPreviewMsg)
	if secondMsg.path != firstMsg.path {
		t.Fatalf("preview checkout path = %q, want reused path %q", secondMsg.path, firstMsg.path)
	}
}

func TestInstallPreviewMissingSourceRepository(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "no-source", Description: "Missing source."},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installPreviewMsg)
	if msg.err == nil {
		t.Fatal("preview error is nil")
	}
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.status != "missing source repository for no-source" {
		t.Fatalf("status = %q", m.status)
	}
	if m.modal != nil {
		t.Fatal("modal opened for missing source")
	}
}

func TestInstallPreviewIgnoresStaleAndNonInstallMessages(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "skill")
	writeTUITestRemoteSkill(t, filepath.Dir(skillDir), filepath.Base(skillDir), "skill", "Skill help.")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.previewToken = 2
	m.status = "before"

	updated, _ := m.Update(installPreviewMsg{token: 1, name: "skill", path: skillDir})
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("stale preview opened modal")
	}
	if m.status != "before" {
		t.Fatalf("status changed for stale preview: %q", m.status)
	}

	m.setView(ViewActive)
	updated, _ = m.Update(installPreviewMsg{token: 2, name: "skill", path: skillDir})
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("non-install preview opened modal")
	}
	if m.status != "before" {
		t.Fatalf("status changed outside install view: %q", m.status)
	}
}

func TestInstallPreviewIgnoresResultAfterInputEdit(t *testing.T) {
	tests := []struct {
		name         string
		openInputKey tea.KeyMsg
		editKey      tea.KeyMsg
		query        string
		owner        string
	}{
		{
			name:         "query rune",
			openInputKey: keyRunes("/"),
			editKey:      keyRunes("r"),
		},
		{
			name:         "query backspace",
			openInputKey: keyRunes("/"),
			editKey:      tea.KeyMsg{Type: tea.KeyBackspace},
			query:        "sv",
		},
		{
			name:         "owner rune",
			openInputKey: keyRunes("o"),
			editKey:      keyRunes("v"),
		},
		{
			name:         "owner backspace",
			openInputKey: keyRunes("o"),
			editKey:      tea.KeyMsg{Type: tea.KeyBackspace},
			owner:        "vercel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(config.Default(t.TempDir(), t.TempDir()))
			m.setView(ViewInstall)
			m.install.Query = tt.query
			m.install.Owner = tt.owner
			m.install.Results = []installResultView{{
				Result:       remote.SearchResult{Name: "no-source", Description: "Missing source."},
				ArchiveState: remote.ArchiveStateNotArchived,
			}}
			m.status = "before"

			updated, previewCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = mustModel(t, updated)
			if previewCmd == nil {
				t.Fatal("preview cmd is nil")
			}
			previewToken := m.install.previewToken

			updated, _ = m.Update(tt.openInputKey)
			m = mustModel(t, updated)
			updated, _ = m.Update(tt.editKey)
			m = mustModel(t, updated)
			if m.install.previewToken == previewToken {
				t.Fatalf("previewToken = %d, want increment after input edit", m.install.previewToken)
			}
			m.status = "after edit"

			previewMsg := previewCmd().(installPreviewMsg)
			updated, _ = m.Update(previewMsg)
			m = mustModel(t, updated)
			if m.status != "after edit" {
				t.Fatalf("status = %q, want stale preview ignored", m.status)
			}
			if m.modal != nil {
				t.Fatal("stale preview opened modal after input edit")
			}
		})
	}
}

func TestInstallPreviewIgnoresResultAfterLeavingAndReturning(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	msg := cmd().(installPreviewMsg)

	m.setView(ViewActive)
	m.setView(ViewInstall)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("preview modal opened after leaving and returning to install")
	}
}

func TestInstallPreviewIgnoresResultAfterNewSearch(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.searchClient = remote.NewSearchClient(testSearchServer(t, []remote.SearchResult{
		{Name: "react-coder", Description: "React help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/react-coder"},
	}), http.DefaultClient)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.testCloneURL = repoDir

	updated, previewCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	m.install.InputMode = installInputQuery
	m.install.Query = "react"
	updated, searchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	searchMsg := searchCmd().(installSearchResultMsg)
	updated, _ = m.Update(searchMsg)
	m = mustModel(t, updated)

	previewMsg := previewCmd().(installPreviewMsg)
	updated, _ = m.Update(previewMsg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("preview modal opened after new search")
	}
}

func TestInstallPreviewIgnoresResultAfterSelectionChange(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{
		{
			Result:       remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Path: "skills/svelte-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
		{
			Result:       remote.SearchResult{Name: "react-coder", Description: "React help.", Path: "skills/react-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
	}
	m.install.testCloneURL = repoDir
	m.status = "before"

	updated, previewCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	updated, _ = m.Update(keyRunes("j"))
	m = mustModel(t, updated)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}

	previewMsg := previewCmd().(installPreviewMsg)
	updated, _ = m.Update(previewMsg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("preview modal opened after install selection changed")
	}
	if m.status != "before" {
		t.Fatalf("status changed after stale selection preview: %q", m.status)
	}
}

func TestInstallArchiveOnlyArchivesRemoteSkillAndStaysOnInstall(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "svelte-coder",
			Description: "Svelte help.",
			Path:        "skills/svelte-coder",
		},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.view != ViewInstall {
		t.Fatalf("view = %q, want install", m.view)
	}
	if m.modal != nil {
		t.Fatal("modal opened after archive-only action")
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateArchived {
		t.Fatalf("archive state = %q, want archived", got)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Svelte help." {
		t.Fatalf("description = %q", info.Description)
	}
	meta, ok, err := remote.ReadSourceMetadata(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("source metadata missing")
	}
	if meta.UpstreamName != "svelte-coder" || meta.SkillPath != "skills/svelte-coder" {
		t.Fatalf("metadata = %#v", meta)
	}
}

func TestInstallInputCtrlCQuits(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("cmd = nil, want quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("cmd msg = %T, want tea.QuitMsg", msg)
	}
}

func testSearchServer(t *testing.T, results []remote.SearchResult) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	t.Cleanup(server.Close)
	return server.URL
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type ioNopCloser struct {
	*strings.Reader
}

func (c ioNopCloser) Close() error {
	return nil
}

func makeTUITestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runTUITestGit(t, dir, "init")
	runTUITestGit(t, dir, "config", "user.email", "test@example.com")
	runTUITestGit(t, dir, "config", "user.name", "Test")
	return dir
}

func writeTUITestRemoteSkill(t *testing.T, root, rel, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "---\nname: " + name + "\ndescription: " + desc + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitTUITestCommit(t *testing.T, repo, msg string) {
	t.Helper()
	runTUITestGit(t, repo, "add", ".")
	runTUITestGit(t, repo, "commit", "-m", msg)
}

func runTUITestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
