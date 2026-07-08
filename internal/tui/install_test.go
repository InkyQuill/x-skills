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

	"github.com/InkyQuill/x-skills/internal/actions"
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

func TestInstallSearchResultRendersAuditPillFromCache(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 120
	m.height = 30
	m.install.searchToken = 1
	m.install.Audit["vercel-labs/skills@svelte-coder"] = remote.AuditSummary{Available: true, Alerts: 1}

	updated, _ := m.Update(installSearchResultMsg{
		token: 1,
		query: "coder",
		results: []remote.SearchResult{
			{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder", Installs: 812},
			{Name: "react-coder", Description: "React help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/react-coder", Installs: 120},
		},
	})
	m = mustModel(t, updated)

	if got := m.install.Results[0].AuditPill; got != "⚠ warn" {
		t.Fatalf("first AuditPill = %q, want %q", got, "⚠ warn")
	}
	if got := m.install.Results[1].AuditPill; got != "" {
		t.Fatalf("second AuditPill = %q, want empty", got)
	}
	view := plain(m.View())
	if !strings.Contains(view, "⚠ warn") {
		t.Fatalf("install view missing audit pill:\n%s", view)
	}
	if strings.Contains(view, "✓ safe") || strings.Contains(view, "‼ risky") {
		t.Fatalf("install view rendered unexpected audit pill:\n%s", view)
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
	if m.install.Message != "archived svelte-coder" {
		t.Fatalf("message = %q", m.install.Message)
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

func TestInstallAndUseLinksProjectAgentsByDefault(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("destination modal is nil")
	}
	view := plain(m.modal.View(120, 30, m))
	if !strings.Contains(view, "Install and use svelte-coder") || !strings.Contains(view, "[x] .Ag") {
		t.Fatalf("destination modal missing default project agents:\n%s", view)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archive {
		t.Fatalf("resolved = %q, want %q", resolved, archive)
	}
	if m.modal != nil {
		t.Fatal("modal is still open after install and use")
	}
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseEnterClosesDestinationModalBeforeCommand(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if m.modal != nil {
		t.Fatal("destination modal remained active after submit")
	}

	updated, duplicateCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if duplicateCmd != nil {
		if _, ok := duplicateCmd().(installUseMsg); ok {
			t.Fatal("duplicate enter submitted install-use again")
		}
	}

	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseIgnoresStaleSuccess(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{
		{
			Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
			ArchiveState: remote.ArchiveStateArchived,
		},
		{
			Result:       remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"},
			ArchiveState: remote.ArchiveStateArchived,
		},
	}
	m.install.useToken = 2
	m.modal = newInstallDestinationModal(m.install.Results[1])
	m.status = "new install pending"
	m.install.Message = "new install pending"

	updated, _ := m.Update(installUseMsg{
		token: 1,
		name:  "svelte-coder",
		destinations: []installDestination{
			{Scope: config.ScopeProject, Target: config.TargetAgents, Label: ".Ag"},
		},
	})
	m = mustModel(t, updated)

	if m.modal == nil {
		t.Fatal("stale install-use success closed newer modal")
	}
	if m.status != "new install pending" {
		t.Fatalf("status = %q, want newer status preserved", m.status)
	}
	if m.install.Message != "new install pending" {
		t.Fatalf("message = %q, want newer message preserved", m.install.Message)
	}
}

func TestInstallAndUseBlocksDestinationModalWhileInFlightAndCompletes(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{
			Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
		{
			Result:       remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"},
			ArchiveState: remote.ArchiveStateArchived,
		},
	}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}

	token := m.install.useToken
	m.cursor = 1
	updated, blockedCmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if blockedCmd != nil {
		t.Fatalf("blocked cmd = %#v, want nil", blockedCmd)
	}
	if m.modal != nil {
		t.Fatal("destination modal opened while install-use was in flight")
	}
	if m.install.useToken != token {
		t.Fatalf("use token = %d, want still %d", m.install.useToken, token)
	}
	if m.status != "install already running" {
		t.Fatalf("status = %q", m.status)
	}

	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.modal != nil {
		t.Fatal("modal is open after install-use completes")
	}
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseBlocksArchiveOnlyWhileInFlightAndCompletes(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, installCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if installCmd == nil {
		t.Fatal("install cmd is nil")
	}
	archiveToken := m.install.archiveToken

	updated, archiveCmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if archiveCmd != nil {
		if _, ok := archiveCmd().(installArchiveMsg); ok {
			t.Fatal("archive-only command started while install-use was in flight")
		}
		t.Fatalf("archive cmd = %#v, want nil", archiveCmd)
	}
	if m.status != "install already running" {
		t.Fatalf("status = %q", m.status)
	}
	if m.install.Message != "install already running" {
		t.Fatalf("message = %q", m.install.Message)
	}
	if m.install.archiveToken != archiveToken {
		t.Fatalf("archive token = %d, want %d", m.install.archiveToken, archiveToken)
	}

	msg := installCmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archive {
		t.Fatalf("resolved = %q, want %q", resolved, archive)
	}
}

func TestInstallAndUseStaleCommandDoesNotArchiveOrLink(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "React help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{
			Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
		{
			Result:       remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
	}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, oldCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if oldCmd == nil {
		t.Fatal("old cmd is nil")
	}

	m.install.bumpUseToken()
	m.status = "newer state"
	m.install.Message = "newer state"

	originalLink := installUseLink
	linkCalls := 0
	installUseLink = func(cfg config.Config, req actions.LinkRequest) (actions.MutationResult, error) {
		linkCalls++
		return originalLink(cfg, req)
	}
	t.Cleanup(func() {
		installUseLink = originalLink
	})

	oldMsg := oldCmd().(installUseMsg)
	updated, _ = m.Update(oldMsg)
	m = mustModel(t, updated)

	if linkCalls != 0 {
		t.Fatalf("link calls = %d, want 0 for stale command", linkCalls)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("stale archive exists or unexpected error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("stale link exists or unexpected error: %v", err)
	}
	if m.status != "newer state" {
		t.Fatalf("status = %q, want newer state preserved", m.status)
	}
	if m.install.Message != "newer state" {
		t.Fatalf("message = %q, want newer state preserved", m.install.Message)
	}
}

func TestInstallDestinationChecklistNavigationAndToggle(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)

	view := plain(m.modal.View(120, 30, m))
	if !strings.Contains(view, "[x] .Ag") || !strings.Contains(view, "[x] .Cl") {
		t.Fatalf("checklist did not keep default and toggle second destination:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)

	view = plain(m.modal.View(120, 30, m))
	if !strings.Contains(view, "[ ] .Ag") || !strings.Contains(view, "[x] .Cl") {
		t.Fatalf("checklist did not move up and toggle default destination:\n%s", view)
	}
}

func TestInstallAndUseRequiresDestination(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil with no destination selected", cmd)
	}
	if m.modal == nil {
		t.Fatal("modal closed with no destination selected")
	}
	if m.status != "select at least one destination" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseLinkErrorClosesModalAndShowsStatus(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder", "Existing active.")
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil after destination preflight failure", cmd)
	}

	if m.modal != nil {
		t.Fatal("modal reopened after link error")
	}
	if !strings.Contains(m.status, "destination exists") {
		t.Fatalf("status = %q, want link error", m.status)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("archive exists after destination preflight failure or unexpected error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")); err != nil {
		t.Fatalf("existing active destination changed unexpectedly: %v", err)
	}
}

func TestInstallAndUseRollsBackPartialLinksAfterLateFailure(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)

	originalLink := installUseLink
	calls := 0
	installUseLink = func(cfg config.Config, req actions.LinkRequest) (actions.MutationResult, error) {
		calls++
		if calls == 2 {
			return actions.MutationResult{}, errors.New("late link failure")
		}
		return originalLink(cfg, req)
	}
	t.Cleanup(func() {
		installUseLink = originalLink
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if calls != 2 {
		t.Fatalf("link calls = %d, want 2", calls)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf(".Ag link remains after rollback: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf(".Cl link exists after failed link: %v", err)
	}
	if m.status != "late link failure" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseRollsBackLinksAfterMidLinkInvalidation(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)

	generation := m.install.ensureUseGeneration()
	originalLink := installUseLink
	calls := 0
	installUseLink = func(cfg config.Config, req actions.LinkRequest) (actions.MutationResult, error) {
		calls++
		result, err := originalLink(cfg, req)
		if calls == 1 {
			generation.next()
		}
		return result, err
	}
	t.Cleanup(func() {
		installUseLink = originalLink
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installUseMsg)
	if !msg.stale {
		t.Fatalf("stale = false, want true")
	}
	m.status = "newer state"
	m.install.Message = "newer state"
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if calls != 1 {
		t.Fatalf("link calls = %d, want 1", calls)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf(".Ag link remains after stale rollback: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf(".Cl link exists after stale rollback: %v", err)
	}
	if m.status != "newer state" {
		t.Fatalf("status = %q, want newer state preserved", m.status)
	}
	if m.install.Message != "newer state" {
		t.Fatalf("message = %q, want newer state preserved", m.install.Message)
	}
	if m.install.useInFlight {
		t.Fatal("useInFlight remained set after stale result")
	}
}

func TestInstallAndUsePreflightsAllDestinationsBeforeLinking(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "svelte-coder", "Existing active.")
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil after destination preflight failure", cmd)
	}

	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf(".Ag link was created before .Cl preflight failure: %v", err)
	}
	if !strings.Contains(m.status, "destination exists") || !strings.Contains(m.status, ".claude") {
		t.Fatalf("status = %q, want existing .Cl destination", m.status)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("archive exists after destination preflight failure or unexpected error: %v", err)
	}
}

func TestInstallArchiveOnlyBlocksInstallAndUseWhileInFlightAndCompletes(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, archiveCmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if archiveCmd == nil {
		t.Fatal("archive cmd is nil")
	}
	useToken := m.install.useToken

	updated, installCmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if installCmd != nil {
		t.Fatalf("install cmd = %#v, want nil", installCmd)
	}
	if m.modal != nil {
		t.Fatal("destination modal opened while archive-only was in flight")
	}
	if m.install.useToken != useToken {
		t.Fatalf("use token = %d, want %d", m.install.useToken, useToken)
	}
	if m.status != "install already running" {
		t.Fatalf("status = %q", m.status)
	}
	if m.install.Message != "install already running" {
		t.Fatalf("message = %q", m.install.Message)
	}

	msg := archiveCmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateArchived {
		t.Fatalf("archive state = %q, want archived", got)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallArchiveOnlyStaleResultClearsInFlightWithoutApplying(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, archiveCmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if archiveCmd == nil {
		t.Fatal("archive cmd is nil")
	}
	m.install.archiveToken++
	m.status = "newer state"
	m.install.Message = "newer state"

	msg := archiveCmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.status != "newer state" {
		t.Fatalf("status = %q, want stale archive result ignored", m.status)
	}

	updated, installCmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if installCmd != nil {
		t.Fatalf("install cmd = %#v, want nil", installCmd)
	}
	if m.modal == nil {
		t.Fatal("destination modal did not open after stale archive result cleared in-flight state")
	}
}

func TestInstallAndUseKeyNoOpsOutsideInstallOrWithoutSelection(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd outside install = %#v, want nil", cmd)
	}
	if m.modal != nil {
		t.Fatal("modal opened outside install view")
	}

	m.setView(ViewInstall)
	updated, cmd = m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd without selected result = %#v, want nil", cmd)
	}
	if m.modal != nil {
		t.Fatal("modal opened without selected result")
	}
}

func TestInstallArchiveOnlyNameConflictShowsChoice(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Local archived.")
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:  "svelte-coder",
			Owner: "vercel-labs",
			Repo:  "skills",
			Path:  "skills/svelte-coder",
		},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil for conflict choice", cmd)
	}
	if m.modal == nil {
		t.Fatal("conflict modal is nil")
	}
	view := plain(m.modal.View(120, 35, m))
	for _, want := range []string{"Name conflict: svelte-coder", "Replace archive", "Rename existing archive", "Rename incoming archive", "Cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("name conflict modal missing %q:\n%s", want, view)
		}
	}
}

func TestInstallArchiveOnlyNameConflictReplaceArchive(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "someone-else",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "svelte-coder",
			Description: "Incoming help.",
			Path:        "skills/svelte-coder",
		},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil for archive conflict modal", cmd)
	}
	if m.modal == nil {
		t.Fatal("conflict modal is nil")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("replace archive cmd is nil")
	}
	if !m.install.archiveInFlight {
		t.Fatal("archive replace did not mark archive in-flight")
	}
	if m.status != "archiving svelte-coder..." {
		t.Fatalf("pending status = %q", m.status)
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if m.install.archiveInFlight {
		t.Fatal("archive in-flight remained set after replace")
	}

	info, err := skills.Read(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Incoming help." {
		t.Fatalf("description = %q, want incoming archive", info.Description)
	}
	meta, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("source metadata missing")
	}
	if meta.SourceType != remote.SourceTypeGit || meta.CloneURL != repoDir {
		t.Fatalf("metadata = %#v, want incoming source metadata", meta)
	}
}

func TestInstallArchiveOnlyNameConflictRenameExisting(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "someone-else",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, _ := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "svelte-coder-local") {
		t.Fatalf("rename existing modal missing suggestion:\n%s", plain(m.modal.View(120, 35, m)))
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("rename existing archive cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	localInfo, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder-local"))
	if err != nil {
		t.Fatal(err)
	}
	if localInfo.Description != "Existing help." {
		t.Fatalf("local description = %q", localInfo.Description)
	}
	incomingInfo, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if incomingInfo.Description != "Incoming help." {
		t.Fatalf("incoming description = %q", incomingInfo.Description)
	}
}

func TestInstallArchiveOnlyNameConflictRenameExistingRollsBackOnApplyFailure(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "someone-else",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	originalApplyArchive := installApplyArchive
	installApplyArchive = func(remote.AddRequest) (remote.AddResult, error) {
		return remote.AddResult{}, errors.New("injected archive failure")
	}
	t.Cleanup(func() {
		installApplyArchive = originalApplyArchive
	})

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, _ := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("rename existing archive cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if !strings.Contains(m.status, "injected archive failure") {
		t.Fatalf("status = %q, want injected archive failure", m.status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Existing help." {
		t.Fatalf("original archive description = %q, want rollback to existing archive", info.Description)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder-local")); !os.IsNotExist(err) {
		t.Fatalf("renamed archive still exists or unexpected error: %v", err)
	}
}

func TestInstallArchiveOnlyNameConflictRenameIncoming(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "someone-else"}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, _ := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "svelte-coder-remote") {
		t.Fatalf("rename incoming modal missing suggestion:\n%s", plain(m.modal.View(120, 35, m)))
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("rename incoming archive cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.status != "archived svelte-coder-remote" {
		t.Fatalf("status = %q", m.status)
	}
	existingInfo, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if existingInfo.Description != "Existing help." {
		t.Fatalf("existing description = %q", existingInfo.Description)
	}
	incomingInfo, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder-remote"))
	if err != nil {
		t.Fatal(err)
	}
	if incomingInfo.Description != "Incoming help." {
		t.Fatalf("incoming description = %q", incomingInfo.Description)
	}
}

func TestInstallArchiveOnlyNameConflictCancelEmptyAndDuplicate(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder-remote", "Duplicate help.")

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, _ := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	for range 3 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.status != "cancelled install svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}

	updated, _ = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	for range 2 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	text, ok := m.modal.(textModal)
	if !ok {
		t.Fatalf("modal = %T, want textModal", m.modal)
	}
	text.input.SetValue("")
	m.modal = text
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.status != "archive name is required" {
		t.Fatalf("empty status = %q", m.status)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder-remote")); err != nil {
		t.Fatal(err)
	}

	text, ok = m.modal.(textModal)
	if !ok {
		t.Fatalf("modal after empty = %T, want textModal", m.modal)
	}
	text.input.SetValue("svelte-coder-remote")
	m.modal = text
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if !strings.Contains(m.status, "archive destination already exists") {
		t.Fatalf("duplicate status = %q", m.status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder-remote"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Duplicate help." {
		t.Fatalf("duplicate archive changed: %q", info.Description)
	}
}

func TestInstallAndUseNameConflictReplaceThenContinuesToDestinations(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "someone-else"}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while conflict modal opens", cmd)
	}
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "Name conflict: svelte-coder") {
		t.Fatalf("install-use did not open conflict modal:\n%s", plain(m.modal.View(120, 35, m)))
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("replace archive cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "Install and use svelte-coder") {
		t.Fatalf("install-use did not continue to destinations:\n%s", plain(m.modal.View(120, 35, m)))
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use cmd is nil after conflict resolution")
	}
	useMsg := cmd().(installUseMsg)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseUpdateAcceptIncomingThenContinuesToDestinations(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	diffMsg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 40, m)), "Incoming remote") {
		t.Fatalf("install-use did not open update diff:\n%s", plain(m.modal.View(120, 40, m)))
	}

	updated, cmd = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("accept incoming archive cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "Install and use svelte-coder") {
		t.Fatalf("install-use did not continue to destinations:\n%s", plain(m.modal.View(120, 35, m)))
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use cmd is nil after update resolution")
	}
	useMsg := cmd().(installUseMsg)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseUpdateKeepArchiveThenContinuesToDestinations(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	diffMsg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	updated, cmd = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("keep archive cmd = %#v, want nil", cmd)
	}
	if view := installTestModalView(m, 120, 35); !strings.Contains(view, "Install and use svelte-coder") {
		t.Fatalf("install-use did not continue to destinations after keeping archive:\n%s", view)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use cmd is nil after keeping archive")
	}
	useMsg := cmd().(installUseMsg)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Old." {
		t.Fatalf("description = %q, want archive kept", info.Description)
	}
}

func TestInstallAndUseNameConflictEscClearsPendingUse(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "someone-else"}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNameConflict,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while conflict modal opens", cmd)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("conflict modal remained open after escape")
	}

	updated, cmd = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while archive-only conflict modal opens", cmd)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only replace cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("stale pending install-use opened modal after archive-only resolution:\n%s", plain(m.modal.View(120, 35, m)))
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseUpdateQClearsPendingUse(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use update diff cmd is nil")
	}
	diffMsg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("q"))
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatal("update diff modal remained open after q")
	}

	updated, cmd = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only update diff cmd is nil")
	}
	diffMsg = cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	updated, cmd = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only accept incoming cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("stale pending install-use opened modal after archive-only update:\n%s", plain(m.modal.View(120, 35, m)))
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseUpdateStaleDiffClearsPendingUse(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use update diff cmd is nil")
	}
	m.setView(ViewRepo)
	diffMsg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if m.install.pendingUse != nil {
		t.Fatal("pending install-use remained after stale update diff")
	}

	m.setView(ViewInstall)
	updated, cmd = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only update diff cmd is nil")
	}
	diffMsg = cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	updated, cmd = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only accept incoming cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("stale pending install-use opened modal after stale update diff:\n%s", plain(m.modal.View(120, 35, m)))
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseUpdateRepeatedRequestKeepsNewestPendingUse(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, firstCmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if firstCmd == nil {
		t.Fatal("first install-use update diff cmd is nil")
	}
	updated, secondCmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if secondCmd == nil {
		t.Fatal("second install-use update diff cmd is nil")
	}

	firstMsg := firstCmd().(installUpdateDiffMsg)
	updated, _ = m.Update(firstMsg)
	m = mustModel(t, updated)
	if m.install.pendingUse == nil {
		t.Fatal("newest pending install-use was cleared by stale update diff")
	}

	secondMsg := secondCmd().(installUpdateDiffMsg)
	updated, _ = m.Update(secondMsg)
	m = mustModel(t, updated)
	updated, cmd := m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("keep archive cmd = %#v, want nil", cmd)
	}
	if m.modal == nil {
		t.Fatal("destination modal is nil after keeping archive from newest update diff")
	}
	if _, ok := m.modal.(installDestinationModal); !ok {
		t.Fatalf("modal = %T, want installDestinationModal", m.modal)
	}
}

func TestInstallAndUseUpdateDiffErrorClearsPendingUse(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = filepath.Join(t.TempDir(), "missing")
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use update diff cmd is nil")
	}
	diffMsg := cmd().(installUpdateDiffMsg)
	if diffMsg.err == nil {
		t.Fatal("diff msg err is nil")
	}
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if m.install.pendingUse != nil {
		t.Fatal("pending install-use remained after update diff error")
	}

	m.install.testCloneURL = repoDir
	updated, cmd = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only update diff cmd is nil")
	}
	diffMsg = cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	updated, cmd = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive-only accept incoming cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("stale pending install-use opened modal after update diff error:\n%s", plain(m.modal.View(120, 35, m)))
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseArchivedRowDiscoveredUpdateRoutesToDiff(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType:   remote.SourceTypeGit,
		CloneURL:     repoDir,
		SkillPath:    "skills/svelte-coder",
		UpstreamName: "svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateArchived,
	}}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while destination modal opens", cmd)
	}
	if view := installTestModalView(m, 120, 35); !strings.Contains(view, "Install and use svelte-coder") {
		t.Fatalf("destination modal did not open:\n%s", view)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use cmd is nil")
	}
	useMsg := cmd().(installUseMsg)
	updated, cmd = m.Update(useMsg)
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update conflict did not start diff command")
	}
	diffMsg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if view := installTestModalView(m, 120, 40); !strings.Contains(view, "Incoming remote") {
		t.Fatalf("install-use archive-step update did not open diff:\n%s", view)
	}

	updated, cmd = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("keep archive cmd = %#v, want nil", cmd)
	}
	if view := installTestModalView(m, 120, 35); !strings.Contains(view, "Install and use svelte-coder") {
		t.Fatalf("install-use did not return to destinations after keeping archive:\n%s", view)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("install-use cmd is nil after keeping archive")
	}
	useMsg = cmd().(installUseMsg)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)
	if m.status != "installed svelte-coder to .Ag" {
		t.Fatalf("status = %q", m.status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Old." {
		t.Fatalf("description = %q, want archive kept", info.Description)
	}
}

func TestInstallArchiveOnlyRechecksStaleRowNameConflictWithoutReplacingArchive(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "svelte-coder",
			Description: "Incoming help.",
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

	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "someone-else",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.status != "archive conflict for svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	info, err := skills.Read(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Existing help." {
		t.Fatalf("description = %q, want existing archive unchanged", info.Description)
	}
	meta, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("source metadata missing")
	}
	if meta.Owner != "someone-else" || meta.SourceType != remote.SourceTypeGitHub {
		t.Fatalf("metadata = %#v, want existing source metadata unchanged", meta)
	}
}

func TestInstallArchiveOnlyAsyncConflictRefreshesStaleRowState(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Incoming help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "svelte-coder",
			Description: "Incoming help.",
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

	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Existing help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "someone-else",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.status != "archive conflict for svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateNameConflict {
		t.Fatalf("archive state = %q, want name conflict", got)
	}
}

func TestInstallArchiveOnlyAsyncConflictUpdatesOnlyMatchingDuplicateSource(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Archived help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "owner-two",
		Repo:       "skills-two",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	first := remote.SearchResult{
		Name:  "svelte-coder",
		Owner: "owner-one",
		Repo:  "skills-one",
		Path:  "skills/svelte-coder",
	}
	second := remote.SearchResult{
		Name:  "svelte-coder",
		Owner: "owner-two",
		Repo:  "skills-two",
		Path:  "skills/svelte-coder",
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.archiveToken = 1
	m.install.Results = []installResultView{
		{Result: first, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: second, ArchiveState: remote.ArchiveStateNotArchived},
	}

	updated, _ := m.Update(installArchiveMsg{
		token:        1,
		name:         "svelte-coder",
		identity:     installArchiveIdentityFromResult(first),
		archiveState: remote.ArchiveStateNameConflict,
		err:          fmt.Errorf("archive conflict for %s", first.Name),
	})
	m = mustModel(t, updated)

	if m.status != "archive conflict for svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateNameConflict {
		t.Fatalf("first archive state = %q, want name conflict", got)
	}
	if got := m.install.Results[1].ArchiveState; got != remote.ArchiveStateArchived {
		t.Fatalf("second archive state = %q, want archived", got)
	}
}

func TestInstallArchiveOnlyResultOutsideInstallReloadsStateAndKeepsView(t *testing.T) {
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

	m.setView(ViewActive)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.view != ViewActive {
		t.Fatalf("view = %q, want active", m.view)
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateArchived {
		t.Fatalf("archive state = %q, want archived", got)
	}
	if len(m.repo) != 1 || m.repo[0].Name != "svelte-coder" {
		t.Fatalf("repo = %#v, want archived skill after reload", m.repo)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallSameSourceUpdateOpensIncomingRemoteDiff(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "vercel-labs",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:  "svelte-coder",
			Owner: "vercel-labs",
			Repo:  "skills",
			Path:  "skills/svelte-coder",
		},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	if m.modal != nil {
		t.Fatal("diff modal opened before async command completed")
	}
	if m.status != "comparing update for svelte-coder..." {
		t.Fatalf("status = %q", m.status)
	}
	msg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("diff modal is nil after update diff command")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, "Incoming remote") || !strings.Contains(view, "Archive conflict: svelte-coder") {
		t.Fatalf("update diff missing remote labels:\n%s", view)
	}
}

func TestInstallSameSourceUpdateDiffIgnoresStaleResult(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "vercel-labs",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{
			Result:       remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"},
			ArchiveState: remote.ArchiveStateUpdateAvailable,
		},
		{
			Result:       remote.SearchResult{Name: "react-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/react-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
	}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	m.status = "newer selection"
	m.install.Message = "newer selection"
	msg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal == nil {
		// nil is the expected state; keep the branch explicit for the failure below.
	} else {
		t.Fatalf("stale update diff opened modal: %T", m.modal)
	}
	if m.status != "newer selection" {
		t.Fatalf("status = %q, want stale update diff ignored", m.status)
	}
}

func TestInstallSameSourceUpdateAcceptIncomingReplacesArchive(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	msg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	updated, cmd = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("accept incoming archive cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, _ = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "New." {
		t.Fatalf("description = %q, want incoming", info.Description)
	}
	if m.modal != nil {
		t.Fatal("modal remained open after accepting incoming")
	}
}

func TestInstallSameSourceUpdateKeepArchiveLeavesArchive(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	msg := cmd().(installUpdateDiffMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if m.status != "kept archive svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Old." {
		t.Fatalf("description = %q, want archive kept", info.Description)
	}
	if m.modal != nil {
		t.Fatal("modal remained open after keeping archive")
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

func installTestModalView(m Model, width, height int) string {
	if m.modal == nil {
		return "<nil>"
	}
	return plain(m.modal.View(width, height, m))
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
