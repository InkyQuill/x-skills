package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestInstallArchiveStateChecksAreBoundedAndCoalesced(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	results := []remote.SearchResult{}
	checkoutPaths := map[string]string{}
	for repoIndex := range 6 {
		owner := "acme"
		repoName := fmt.Sprintf("skills-%d", repoIndex)
		checkoutPath := t.TempDir()
		checkoutPaths["https://github.com/"+owner+"/"+repoName+".git"] = checkoutPath
		for skillIndex := range 2 {
			name := fmt.Sprintf("skill-%d-%d", repoIndex, skillIndex)
			path := "skills/" + name
			writeTUITestRemoteSkill(t, checkoutPath, path, name, "Incoming.")
			archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), name, "Archived.")
			if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
				SourceType: remote.SourceTypeGitHub,
				Owner:      owner,
				Repo:       repoName,
				SkillPath:  path,
			}); err != nil {
				t.Fatal(err)
			}
			results = append(results, remote.SearchResult{Name: name, Owner: owner, Repo: repoName, Path: path})
		}
	}

	var active atomic.Int64
	var maxActive atomic.Int64
	var mu sync.Mutex
	calls := map[string]int{}
	previousCheckout := installArchiveStateCheckout
	installArchiveStateCheckout = func(ctx context.Context, _ *remote.CheckoutCache, source remote.GitSource) (remote.Checkout, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		mu.Lock()
		calls[source.CloneURL+"@"+source.Ref]++
		mu.Unlock()
		select {
		case <-ctx.Done():
			return remote.Checkout{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		return remote.Checkout{Path: checkoutPaths[source.CloneURL], Source: source}, nil
	}
	t.Cleanup(func() { installArchiveStateCheckout = previousCheckout })

	m := New(cfg)
	m.install.checkouts = remote.NewCheckoutCache(t.TempDir())
	msg := m.checkInstallArchiveStates(t.Context(), 7, results, 3)

	if maxActive.Load() > 3 {
		t.Fatalf("max concurrent checks = %d, want <= 3", maxActive.Load())
	}
	for repoIndex := range 6 {
		key := fmt.Sprintf("https://github.com/acme/skills-%d.git@", repoIndex)
		if got := calls[key]; got != 1 {
			t.Fatalf("checkout calls for %q = %d, want 1", key, got)
		}
	}
	if len(msg.results) != len(results) {
		t.Fatalf("state results = %d, want %d", len(msg.results), len(results))
	}
	for i, result := range msg.results {
		if result.Identity != installArchiveIdentityFromResult(results[i]) {
			t.Fatalf("result %d identity = %#v, want input order %#v", i, result.Identity, installArchiveIdentityFromResult(results[i]))
		}
		if result.Err != nil || result.State == "" {
			t.Fatalf("result %d = %#v, want successful state for matching input", i, result)
		}
	}
}

func TestInstallArchiveStateChecksDistinguishSourceRefs(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	checkoutPaths := map[string]string{}
	results := []remote.SearchResult{}
	for _, ref := range []string{"main", "next"} {
		checkoutPath := t.TempDir()
		name := "skill-" + ref
		path := "skills/" + name
		writeTUITestRemoteSkill(t, checkoutPath, path, name, "Incoming.")
		checkoutPaths[ref] = checkoutPath
		results = append(results, remote.SearchResult{
			Name:  name,
			Owner: "acme",
			Repo:  "skills",
			Path:  path,
			Ref:   ref,
		})
	}

	var mu sync.Mutex
	calls := map[string]int{}
	previousCheckout := installArchiveStateCheckout
	installArchiveStateCheckout = func(_ context.Context, _ *remote.CheckoutCache, source remote.GitSource) (remote.Checkout, error) {
		mu.Lock()
		calls[source.CloneURL+"@"+source.Ref]++
		mu.Unlock()
		return remote.Checkout{Path: checkoutPaths[source.Ref], Source: source}, nil
	}
	t.Cleanup(func() { installArchiveStateCheckout = previousCheckout })

	m := New(cfg)
	m.install.checkouts = remote.NewCheckoutCache(t.TempDir())
	msg := m.checkInstallArchiveStates(t.Context(), 1, results, 3)

	for _, key := range []string{
		"https://github.com/acme/skills.git@main",
		"https://github.com/acme/skills.git@next",
	} {
		if got := calls[key]; got != 1 {
			t.Fatalf("checkout calls for %q = %d, want 1", key, got)
		}
	}
	if len(msg.results) != 2 || msg.results[0].Err != nil || msg.results[1].Err != nil {
		t.Fatalf("state results = %#v, want two successful checks", msg.results)
	}
}

func TestInstallSearchRetainsInitializedCheckoutCache(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "skill", "Archived.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "acme",
		Repo:       "skills",
		SkillPath:  "skills/skill",
	}); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.install.searchToken = 1
	if m.install.checkouts != nil {
		t.Fatal("checkout cache initialized before search result")
	}

	updated, cmd := m.Update(installSearchResultMsg{
		token: 1,
		query: "skill",
		results: []remote.SearchResult{{
			Name:  "skill",
			Owner: "acme",
			Repo:  "skills",
			Path:  "skills/skill",
		}},
	})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive state command is nil")
	}
	if m.install.checkouts == nil {
		t.Fatal("checkout cache was not retained on model")
	}
	retained := m.install.checkouts

	updated, _ = m.Update(installSearchResultMsg{
		token: 1,
		query: "skill",
		results: []remote.SearchResult{{
			Name:  "skill",
			Owner: "acme",
			Repo:  "skills",
			Path:  "skills/skill",
		}},
	})
	m = mustModel(t, updated)
	if m.install.checkouts != retained {
		t.Fatal("later search replaced retained checkout cache")
	}
}

func TestInstallArchiveStateResultsRejectStaleTokenAndIdentity(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.install.searchToken = 2
	current := remote.SearchResult{Name: "current", Owner: "acme", Repo: "skills", Path: "skills/current", Ref: "main"}
	m.install.Results = []installResultView{{Result: current, ArchiveState: remote.ArchiveStateArchived}}

	m.applyInstallArchiveStateResults(installArchiveStatesMsg{
		token: 1,
		results: []installArchiveStateResult{{
			Identity: installArchiveIdentityFromResult(current),
			State:    remote.ArchiveStateUpdateAvailable,
		}},
	})
	m.applyInstallArchiveStateResults(installArchiveStatesMsg{
		token: 2,
		results: []installArchiveStateResult{{
			Identity: installArchiveIdentityFromResult(remote.SearchResult{Name: "other"}),
			State:    remote.ArchiveStateUpdateAvailable,
		}},
	})
	m.applyInstallArchiveStateResults(installArchiveStatesMsg{
		token: 2,
		results: []installArchiveStateResult{{
			Identity: installArchiveIdentityFromResult(remote.SearchResult{
				Name: "current", Owner: "acme", Repo: "skills", Path: "skills/current", Ref: "next",
			}),
			State: remote.ArchiveStateUpdateAvailable,
		}},
	})

	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateArchived {
		t.Fatalf("archive state = %q, want stale results rejected", got)
	}
}

func TestInstallRowShowsCheckFailed(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 120
	m.height = 35
	m.install.Results = []installResultView{{
		Result:            remote.SearchResult{Name: "broken", Owner: "acme", Repo: "skills"},
		ArchiveState:      remote.ArchiveStateArchived,
		ArchiveCheckError: "checkout failed",
	}}

	view := plain(m.View())
	if !strings.Contains(view, "check failed") {
		t.Fatalf("install row missing check failed pill:\n%s", view)
	}
	if !strings.Contains(view, "checkout failed") {
		t.Fatalf("install inspector missing check error:\n%s", view)
	}
}

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
	for _, want := range []string{"I:Install", "Install: search", "type at least 2 characters", "space select", "/ search", "i install & use", "a archive only", "c clear"} {
		if !strings.Contains(view, want) {
			t.Fatalf("install shell missing %q:\n%s", want, view)
		}
	}
}

func TestInstallHelpShowsRealInstallKeys(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	view := plain(newHelpModal().View(100, 40, m))
	for _, want := range []string{"switch to Install view", "Install: / search", "Install: i install and use", "Install: a archive only", "Install too"} {
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

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	for _, key := range []string{"s", "v", "e", "l", "t", "e"} {
		updated, _ = m.Update(keyRunes(key))
		m = mustModel(t, updated)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	m.install.Audit[installAuditKey(remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"})] = remote.AuditSummary{Available: true, Alerts: 1}
	m.install.Audit[installAuditKey(remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/other-svelte-coder"})] = remote.AuditSummary{Available: true, Critical: 1}

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

func TestInstallSearchResultRendersAuditPillInASCIIMode(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()), Options{ASCII: true})
	m.setView(ViewInstall)
	m.width = 120
	m.height = 30
	m.install.searchToken = 1
	result := remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"}
	m.install.Audit[installAuditKey(result)] = remote.AuditSummary{Available: true, Critical: 1}

	updated, _ := m.Update(installSearchResultMsg{
		token:   1,
		query:   "coder",
		results: []remote.SearchResult{result},
	})
	m = mustModel(t, updated)

	if got := m.install.Results[0].AuditPill; got != "!! risky" {
		t.Fatalf("AuditPill = %q, want %q", got, "!! risky")
	}
	view := plain(m.View())
	if strings.Contains(view, "✓") || strings.Contains(view, "⚠") || strings.Contains(view, "‼") {
		t.Fatalf("install view rendered unicode audit pill in ASCII mode:\n%s", view)
	}
	if !strings.Contains(view, "!! risky") {
		t.Fatalf("install view missing ASCII audit pill:\n%s", view)
	}
}

func TestInstallRowsRenderRichStateSourceAndDescription(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.cursor = 0
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "svelte-coder",
			Description: "Svelte help.",
			Owner:       "vercel-labs",
			Repo:        "skills",
			Path:        "skills/svelte-coder",
			Installs:    812,
		},
		ArchiveState: remote.ArchiveStateArchived,
	}}

	rows := renderInstallRows(m, 120)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	got := strings.TrimRight(ansi.Strip(rows[0]), " ")
	for _, want := range []string{"› ◇", "svelte-coder", "vercel-labs/skills", "812 installs", "archived", "Svelte help."} {
		if !strings.Contains(got, want) {
			t.Fatalf("install row missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "  0 installs") {
		t.Fatalf("install row rendered empty count:\n%s", got)
	}
	if colorAvailableForTest() {
		requireInstallRowStyledSegment(t, rows[0], installSourceStyle, cursorBg.GetBackground(), "vercel-labs/skills")
		requireInstallRowStyledSegment(t, rows[0], installCountStyle, cursorBg.GetBackground(), "812 installs")
		requireInstallRowStyledSegment(t, rows[0], okStyle, cursorBg.GetBackground(), remote.ArchiveStateArchived)
	}
}

func TestInstallRowsRenderAuditAndArchiveStatePills(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.cursor = 0
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "svelte-coder",
			Description: "Svelte help.",
			Owner:       "vercel-labs",
			Repo:        "skills",
			Path:        "skills/svelte-coder",
		},
		ArchiveState: remote.ArchiveStateNameConflict,
		AuditPill:    "⚠ warn",
	}}
	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true

	rows := renderInstallRows(m, 120)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	got := strings.TrimRight(ansi.Strip(rows[0]), " ")
	for _, want := range []string{"› ◆", "svelte-coder", "vercel-labs/skills", "name conflict", "⚠ warn", "Svelte help."} {
		if !strings.Contains(got, want) {
			t.Fatalf("install row missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "installs") {
		t.Fatalf("install row rendered zero install count:\n%s", got)
	}
	if colorAvailableForTest() {
		requireInstallRowStyledSegment(t, rows[0], dangerStyle, cursorBg.GetBackground(), remote.ArchiveStateNameConflict)
		requireInstallRowStyledSegment(t, rows[0], archiveStyle, cursorBg.GetBackground(), "⚠ warn")
	}
}

func TestInstallRowsPreserveHighlightBackgroundAcrossRichSegments(t *testing.T) {
	if colorAvailableForTest() && (!selectedBackgroundConfigured() || !cursorBackgroundConfigured()) {
		t.Fatal("row background styles are not configured")
	}
	if !colorAvailableForTest() {
		t.Skip("color disabled")
	}

	tests := []struct {
		name       string
		cursor     int
		selected   bool
		background lipgloss.TerminalColor
	}{
		{
			name:       "cursor",
			cursor:     0,
			background: cursorBg.GetBackground(),
		},
		{
			name:       "selected",
			cursor:     -1,
			selected:   true,
			background: selectedBg.GetBackground(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(config.Default(t.TempDir(), t.TempDir()))
			m.setView(ViewInstall)
			m.cursor = tt.cursor
			m.install.Results = []installResultView{{
				Result: remote.SearchResult{
					Name:        "svelte-coder",
					Description: "Svelte help.",
					Owner:       "vercel-labs",
					Repo:        "skills",
					Path:        "skills/svelte-coder",
					Installs:    812,
				},
				ArchiveState: remote.ArchiveStateArchived,
				AuditPill:    "⚠ warn",
			}}
			if tt.selected {
				m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true
			}

			rows := renderInstallRows(m, 120)
			if len(rows) != 1 {
				t.Fatalf("rows = %d, want 1", len(rows))
			}
			requireInstallRowStyledSegment(t, rows[0], installSourceStyle, tt.background, "vercel-labs/skills")
			requireInstallRowStyledSegment(t, rows[0], installCountStyle, tt.background, "812 installs")
			requireInstallRowStyledSegment(t, rows[0], okStyle, tt.background, remote.ArchiveStateArchived)
			requireInstallRowStyledSegment(t, rows[0], archiveStyle, tt.background, "⚠ warn")
			requireInstallRowStyledSegment(t, rows[0], mutedStyle, tt.background, "Svelte help.")
		})
	}
}

func requireInstallRowStyledSegment(
	t *testing.T,
	row string,
	style lipgloss.Style,
	background lipgloss.TerminalColor,
	text string,
) {
	t.Helper()
	styled := style.Background(background).Render(text)
	if !strings.Contains(row, styled) {
		t.Fatalf("install row missing styled segment %q:\n%q", styled, row)
	}
}

func TestInstallSearchResultCachesAuditFromResult(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 120
	m.height = 30
	m.install.searchToken = 1
	result := remote.SearchResult{
		Name:        "svelte-coder",
		Description: "Svelte help.",
		Owner:       "vercel-labs",
		Repo:        "skills",
		Path:        "skills/svelte-coder",
		Audit:       &remote.AuditSummary{Available: true, Critical: 1},
	}

	updated, _ := m.Update(installSearchResultMsg{
		token:   1,
		query:   "coder",
		results: []remote.SearchResult{result},
	})
	m = mustModel(t, updated)

	if got := m.install.Results[0].AuditPill; got != "‼ risky" {
		t.Fatalf("AuditPill = %q, want %q", got, "‼ risky")
	}
	if got := m.install.Audit[installAuditKey(result)]; !got.Available || got.Critical != 1 {
		t.Fatalf("cached audit = %#v", got)
	}
}

func TestInstallSpaceTogglesResultSelection(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"}},
		{Result: remote.SearchResult{Name: "react-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/react-coder"}},
	}

	firstID := installID(m.install.Results[0].Result)
	updated, _ := m.Update(keyRunes(" "))
	m = mustModel(t, updated)
	if !m.selected[ViewInstall][firstID] {
		t.Fatalf("selected[%q] = false, want true", firstID)
	}

	updated, _ = m.Update(keyRunes("j"))
	m = mustModel(t, updated)
	secondID := installID(m.install.Results[1].Result)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)
	if !m.selected[ViewInstall][firstID] || !m.selected[ViewInstall][secondID] {
		t.Fatalf("selected = %#v, want both install rows selected", m.selected[ViewInstall])
	}

	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)
	if m.selected[ViewInstall][secondID] {
		t.Fatalf("selected[%q] = true after second toggle", secondID)
	}
	rows := m.selectedInstallRows()
	if len(rows) != 1 || rows[0].Result.Name != "svelte-coder" {
		t.Fatalf("selected rows = %#v, want first row only", rows)
	}
}

func TestInstallClearKeyClearsResultSelection(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"}},
		{Result: remote.SearchResult{Name: "react-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/react-coder"}},
	}
	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true
	m.selected[ViewInstall][installID(m.install.Results[1].Result)] = true

	updated, _ := m.Update(keyRunes("c"))
	m = mustModel(t, updated)

	if len(m.selected[ViewInstall]) != 0 {
		t.Fatalf("selected install rows = %#v, want none", m.selected[ViewInstall])
	}
	if m.status != "selection cleared" {
		t.Fatalf("status = %q, want selection cleared", m.status)
	}
}

func TestInstallSearchResetsSelectionOnSubmitAndCurrentResult(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.Query = "svelte"
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}},
	}
	m.cursor = 1
	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true

	if cmd := m.startInstallSearch(); cmd == nil {
		t.Fatal("search cmd is nil")
	}
	if len(m.selected[ViewInstall]) != 0 {
		t.Fatalf("selection after search submit = %#v, want empty", m.selected[ViewInstall])
	}
	if rows := m.installActionRows(); len(rows) != 1 || rows[0].Result.Name != "react-coder" {
		t.Fatalf("action rows after submit = %#v, want cursor row only", rows)
	}

	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true
	updated, _ := m.Update(installSearchResultMsg{
		token:   m.install.searchToken,
		query:   "svelte",
		results: []remote.SearchResult{{Name: "vue-coder", Path: "skills/vue-coder"}},
	})
	m = mustModel(t, updated)
	if len(m.selected[ViewInstall]) != 0 {
		t.Fatalf("selection after current search result = %#v, want empty", m.selected[ViewInstall])
	}
}

func TestInstallStaleSearchResultPreservesSelection(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.searchToken = 2
	result := remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}
	m.install.Results = []installResultView{{Result: result}}
	m.selected[ViewInstall][installID(result)] = true

	updated, _ := m.Update(installSearchResultMsg{
		token:   1,
		query:   "stale",
		results: []remote.SearchResult{{Name: "react-coder", Path: "skills/react-coder"}},
	})
	m = mustModel(t, updated)
	if !m.selected[ViewInstall][installID(result)] {
		t.Fatalf("stale search result cleared selection: %#v", m.selected[ViewInstall])
	}
}

func TestInstallArchiveUsesSelectedRows(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "React help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	m.cursor = 2

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("fallback archive cmd is nil")
	}
	m = runInstallArchiveBatchCommands(t, m, cmd)
	if m.status != "archived vue-coder" {
		t.Fatalf("fallback status = %q, want cursor row archived", m.status)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "vue-coder")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("unselected first archive exists or unexpected error: %v", err)
	}

	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true
	m.selected[ViewInstall][installID(m.install.Results[1].Result)] = true
	m.cursor = 2
	updated, cmd = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("selected archive cmd is nil")
	}
	m = runInstallArchiveBatchCommands(t, m, cmd)
	if m.status != "archived 2 skills" {
		t.Fatalf("selected status = %q, want batch status", m.status)
	}
	for _, name := range []string{"svelte-coder", "react-coder", "vue-coder"} {
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), name)); err != nil {
			t.Fatalf("archive %s missing: %v", name, err)
		}
	}
}

func TestRunInstallArchiveRowReturnsOperationMessage(t *testing.T) {
	op := installArchiveRowOperation(func(context.Context) installArchiveMsg {
		return installArchiveMsg{name: "beta", err: errors.New("boom")}
	})

	msg := runInstallArchiveRow(context.Background(), op)
	if msg.name != "beta" || msg.err == nil {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestRunInstallArchiveRowRejectsNilOperation(t *testing.T) {
	msg := runInstallArchiveRow(context.Background(), nil)
	if msg.err == nil || msg.err.Error() != "nil install archive row operation" {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestInstallArchiveBatchAttributesFailureToOperationRow(t *testing.T) {
	commands := []installArchiveRowCommand{
		{
			row: installResultView{Result: remote.SearchResult{Name: "alpha"}},
			operation: func(context.Context) installArchiveMsg {
				return installArchiveMsg{name: "beta", err: errors.New("boom")}
			},
		},
	}

	msg := runInstallArchiveBatch(context.Background(), commands, nil, 1, 7, nil)
	if msg.batch == nil || !reflect.DeepEqual(msg.batch.failures, []string{"alpha: boom"}) {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestArchiveInstallRowsStopsWhenGenerationChanges(t *testing.T) {
	generation := &installUseGeneration{}
	token := generation.next()
	var first, second atomic.Int32
	commands := []installArchiveRowCommand{
		{row: installResultView{Result: remote.SearchResult{Name: "one"}}, operation: func(context.Context) installArchiveMsg {
			first.Add(1)
			generation.value.Add(1)
			return installArchiveMsg{name: "one"}
		}},
		{row: installResultView{Result: remote.SearchResult{Name: "two"}}, operation: func(context.Context) installArchiveMsg {
			second.Add(1)
			return installArchiveMsg{name: "two"}
		}},
	}

	msg := runInstallArchiveBatch(context.Background(), commands, nil, 2, token, generation)

	if first.Load() != 1 || second.Load() != 0 {
		t.Fatalf("operation calls = (%d, %d), want (1, 0)", first.Load(), second.Load())
	}
	if !msg.stale || msg.batch == nil || msg.batch.completed != 1 {
		t.Fatalf("msg = %#v, want stale after one completed row", msg)
	}
}

func TestInstallBatchProgress(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	generation := m.install.ensureUseGeneration()
	token := m.install.bumpUseToken()
	m.install.archiveToken = token
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = token
	var first, second atomic.Int32
	commands := []installArchiveRowCommand{
		{row: installResultView{Result: remote.SearchResult{Name: "one"}}, operation: func(context.Context) installArchiveMsg {
			first.Add(1)
			return installArchiveMsg{name: "one"}
		}},
		{row: installResultView{Result: remote.SearchResult{Name: "two"}}, operation: func(context.Context) installArchiveMsg {
			second.Add(1)
			return installArchiveMsg{name: "two"}
		}},
	}

	cmd := newInstallArchiveBatchCmd(t.Context(), commands, nil, 2, token, generation)
	progress, ok := cmd().(installBatchProgressMsg)
	if !ok {
		t.Fatalf("first batch message = %T, want installBatchProgressMsg", cmd())
	}
	if first.Load() != 1 || second.Load() != 0 {
		t.Fatalf("operation calls before progress = (%d, %d), want (1, 0)", first.Load(), second.Load())
	}

	updated, next := m.Update(progress)
	m = mustModel(t, updated)
	if next == nil {
		t.Fatal("next row command is nil")
	}
	if m.status != "archiving 1/2: one" {
		t.Fatalf("status = %q, want per-item progress", m.status)
	}
	_ = next()
	if second.Load() != 1 {
		t.Fatalf("second operation calls = %d, want 1 after progress applied", second.Load())
	}
}

func TestInstallBatchCancellationSuppressesStaleConflict(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	generation := m.install.ensureUseGeneration()
	token := m.install.bumpUseToken()
	m.install.archiveToken = token
	m.install.archiveInFlight = true
	m.install.archiveInFlightToken = token
	commands := []installArchiveRowCommand{{
		row: installResultView{Result: remote.SearchResult{Name: "one"}},
		operation: func(context.Context) installArchiveMsg {
			generation.next()
			return installArchiveMsg{name: "one", archiveState: remote.ArchiveStateNameConflict, err: errors.New("conflict")}
		},
	}}

	msg := newInstallArchiveBatchCmd(t.Context(), commands, nil, 1, token, generation)()
	cancelled, ok := msg.(installBatchCancelledMsg)
	if !ok {
		t.Fatalf("batch message = %T, want installBatchCancelledMsg", msg)
	}
	updated, cmd := m.Update(cancelled)
	m = mustModel(t, updated)
	if cmd != nil || m.modal != nil || m.install.pendingArchiveBatch != nil {
		t.Fatalf("stale conflict continued: cmd=%v modal=%T pending=%#v", cmd, m.modal, m.install.pendingArchiveBatch)
	}
}

func TestInstallBatchProgressContinuesAfterResolvedConflict(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	identity := installArchiveIdentity{name: "conflict"}
	m.install.archiveToken = 1
	m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
		identity:    identity,
		total:       4,
		completed:   1,
		currentName: "first",
		success:     []string{"first"},
		remaining: []installResultView{
			{Result: remote.SearchResult{Name: "third"}},
			{Result: remote.SearchResult{Name: "fourth"}},
		},
	}
	cmd := m.continueInstallArchiveBatchAfterResolved(identity, "second", nil)
	if cmd == nil {
		t.Fatal("continuation command is nil")
	}
	progress, ok := cmd().(installBatchProgressMsg)
	if !ok {
		t.Fatalf("continuation message = %T, want progress", cmd())
	}
	if progress.completed != 3 || progress.total != 4 || progress.name != "third" {
		t.Fatalf("progress = %#v, want 3/4 at third", progress)
	}
}

func TestLeavingInstallInvalidatesMutationGeneration(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	generation := m.install.ensureUseGeneration()
	token := m.install.bumpUseToken()
	previewToken := m.install.previewToken

	m.setView(ViewRepo)

	if generation.isCurrent(token) {
		t.Fatal("mutation generation remains current after leaving Install")
	}
	if m.install.previewToken == previewToken {
		t.Fatal("preview generation was not invalidated after leaving Install")
	}
}

func commandMessage[T tea.Msg](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	msg := cmd()
	if typed, ok := msg.(T); ok {
		return typed
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("command message = %T, want requested message or tea.BatchMsg", msg)
	}
	for _, batchedCmd := range batch {
		if typed, ok := batchedCmd().(T); ok {
			return typed
		}
	}
	var zero T
	t.Fatalf("tea.BatchMsg does not contain %T", zero)
	return zero
}

func TestInstallArchiveBatchContinuesAfterMiddleNameConflict(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "Incoming React help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "react-coder", "Existing React help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "someone-else"}); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateNameConflict},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("initial archive batch cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "Name conflict: react-coder") {
		t.Fatalf("archive batch did not pause on middle conflict:\n%s", installTestModalView(m, 120, 35))
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("rename existing archive cmd is nil")
	}
	msg = commandMessage[installArchiveMsg](t, cmd)
	updated, cmd = m.Update(msg)
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("continuation and reload batch cmd is nil after resolving middle conflict")
	}
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("continuation cmd message = %T, want tea.BatchMsg with reload", batchMsg)
	}
	if len(batch) != 2 {
		t.Fatalf("batch command count = %d, want continuation and reload", len(batch))
	}
	msg = batch[0]().(installArchiveMsg)
	reloadMsg := batch[1]()
	if _, ok := reloadMsg.(reloadResultMsg); !ok {
		t.Fatalf("second batch message = %T, want reloadResultMsg", reloadMsg)
	}
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	for _, name := range []string{"svelte-coder", "react-coder", "react-coder-local", "vue-coder"} {
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), name)); err != nil {
			t.Fatalf("archive %s missing after continuation: %v", name, err)
		}
	}
	if m.status != "archived 3 skills" {
		t.Fatalf("status = %q, want archived 3 skills", m.status)
	}
}

func TestInstallArchiveBatchContinuesAfterMiddleUpdate(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "Incoming React help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "react-coder", "Existing React help.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{
		SourceType: remote.SourceTypeGit,
		CloneURL:   repoDir,
		SkillPath:  "skills/react-coder",
	}); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateUpdateAvailable},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("initial archive batch cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, cmd = m.Update(msg)
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("middle update diff cmd is nil")
	}
	diffMsg := commandMessage[installUpdateDiffMsg](t, cmd)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 40, m)), "Archive conflict: react-coder") {
		t.Fatalf("archive batch did not pause on middle update:\n%s", installTestModalView(m, 120, 40))
	}

	updated, cmd = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("accept incoming archive cmd is nil")
	}
	msg = commandMessage[installArchiveMsg](t, cmd)
	updated, cmd = m.Update(msg)
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail archive cmd is nil after resolving middle update")
	}
	msg = commandMessage[installArchiveMsg](t, cmd)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	for _, name := range []string{"svelte-coder", "react-coder", "vue-coder"} {
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), name)); err != nil {
			t.Fatalf("archive %s missing after update continuation: %v", name, err)
		}
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "react-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Incoming React help." {
		t.Fatalf("react archive description = %q, want incoming update", info.Description)
	}
	if m.status != "archived 3 skills" {
		t.Fatalf("status = %q, want archived 3 skills", m.status)
	}
}

func TestInstallArchiveBatchMissingSkillContinuesWithTail(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.archiveToken = 1

	missing := remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}
	tail := installResultView{
		Result:       remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}
	m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
		identity:  installArchiveIdentityFromResult(missing),
		total:     2,
		remaining: []installResultView{tail},
	}

	updated, cmd := m.Update(installArchiveMsg{
		token:    1,
		name:     missing.Name,
		identity: installArchiveIdentityFromResult(missing),
		err:      &remote.MissingSkillError{Name: missing.Name, PreferredPath: missing.Path},
	})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail archive cmd is nil after missing skill")
	}
	msg := commandMessage[installArchiveMsg](t, cmd)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "vue-coder")); err != nil {
		t.Fatalf("tail archive missing after continuation: %v", err)
	}
	if m.status != "archived 1 of 2 skills" {
		t.Fatalf("status = %q, want partial archive status", m.status)
	}
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), `skill "react-coder" not found`) {
		t.Fatalf("result modal missing failed row:\n%s", installTestModalView(m, 120, 35))
	}
	if strings.Contains(plain(m.modal.View(120, 35, m)), "Couldn't find the requested skill in repo") {
		t.Fatalf("batch missing skill opened single-row stale registry modal:\n%s", installTestModalView(m, 120, 35))
	}
}

func TestInstallArchiveBatchMixedFailuresShowsResults(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "missing-coder", Path: "skills/missing-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive batch cmd is nil")
	}
	m = runInstallArchiveBatchCommands(t, m, cmd)

	if m.status != "archived 1 of 2 skills" {
		t.Fatalf("status = %q, want archived 1 of 2 skills", m.status)
	}
	view := installTestModalView(m, 120, 40)
	for _, want := range []string{"Archive Results", "archived svelte-coder", "missing-coder"} {
		if !strings.Contains(view, want) {
			t.Fatalf("archive result modal missing %q:\n%s", want, view)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallUseUsesSelectedRows(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateArchived},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateArchived},
	}
	m.cursor = 2

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("fallback install-use cmd = %#v, want nil while modal opens", cmd)
	}
	single, ok := m.modal.(installDestinationModal)
	if !ok {
		t.Fatalf("fallback modal = %T, want installDestinationModal", m.modal)
	}
	if single.name != "vue-coder" {
		t.Fatalf("fallback modal name = %q, want cursor row", single.name)
	}

	m.modal = nil
	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true
	m.selected[ViewInstall][installID(m.install.Results[1].Result)] = true
	m.cursor = 2
	updated, cmd = m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("selected install-use cmd = %#v, want nil while modal opens", cmd)
	}
	batch, ok := m.modal.(installBatchDestinationModal)
	if !ok {
		t.Fatalf("selected modal = %T, want installBatchDestinationModal", m.modal)
	}
	if len(batch.rows) != 2 || batch.rows[0].Result.Name != "svelte-coder" || batch.rows[1].Result.Name != "react-coder" {
		t.Fatalf("batch rows = %#v, want selected rows in row order", batch.rows)
	}
	view := plain(batch.View(120, 35, m))
	if !strings.Contains(view, "Install and use 2 skills") || strings.Contains(view, "vue-coder") {
		t.Fatalf("batch destination modal did not use selected rows:\n%s", view)
	}
}

func TestInstallInspectorShowsDescriptionAndSourceDetails(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 120
	m.height = 30
	m.install.Results = []installResultView{
		{
			Result: remote.SearchResult{
				Name:        "svelte-coder",
				Description: "Svelte help.",
				Owner:       "vl",
				Repo:        "skills",
				Path:        "svelte",
				Installs:    812,
			},
			ArchiveState: remote.ArchiveStateArchived,
			AuditPill:    "⚠ warn",
		},
	}

	view := plain(m.View())
	for _, want := range []string{
		"Inspector",
		"svelte-coder",
		"Description",
		"Svelte help.",
		"Source",
		"vl/skills",
		"Installs",
		"812",
		"Archive",
		"archived",
		"Audit",
		"⚠ warn",
		"Owner",
		"vl",
		"Repo",
		"skills",
		"Path",
		"svelte",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("install inspector missing %q:\n%s", want, view)
		}
	}
}

func TestInstallInspectorShowsAvailableActions(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.width = 120
	m.height = 30
	m.install.Results = []installResultView{
		{
			Result: remote.SearchResult{
				Name:        "svelte-coder",
				Description: "Svelte help.",
				Owner:       "vercel-labs",
				Repo:        "skills",
				Path:        "skills/svelte-coder",
			},
			ArchiveState: remote.ArchiveStateNotArchived,
		},
	}

	view := plain(m.View())
	for _, want := range []string{"Actions", "enter preview", "i install & use", "a archive only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("install inspector missing action %q:\n%s", want, view)
		}
	}
}

func TestInstallSearchResultAsyncStateCheckMarksUpdateAvailable(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{
		SourceType: remote.SourceTypeGit,
		CloneURL:   repoDir,
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.searchToken = 1
	result := remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}
	updated, cmd := m.Update(installSearchResultMsg{
		token:   1,
		query:   "svelte",
		results: []remote.SearchResult{result},
	})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("archive state check cmd is nil")
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateArchived {
		t.Fatalf("initial archive state = %q, want archived", got)
	}
	stateMsg := cmd().(installArchiveStatesMsg)
	updated, _ = m.Update(stateMsg)
	m = mustModel(t, updated)
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateUpdateAvailable {
		t.Fatalf("archive state = %q, want update available", got)
	}

	updated, cmd = m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("update diff cmd is nil")
	}
	diffMsg := commandMessage[installUpdateDiffMsg](t, cmd)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("update diff modal is nil")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, "Incoming remote") || !strings.Contains(view, "Archive conflict: svelte-coder") {
		t.Fatalf("update diff missing remote labels:\n%s", view)
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

func TestInstallPreviewMissingSkillInRepoShowsStaleRegistryModal(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/other", "other", "Other.")
	gitTUITestCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "next-best-practices", Description: "Next help.", Path: "next-best-practices"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	msg := cmd().(installPreviewMsg)
	if msg.err == nil {
		t.Fatal("preview error is nil")
	}
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("stale registry modal is nil")
	}
	view := plain(m.modal.View(120, 35, m))
	for _, want := range []string{
		"Uh-oh...",
		"Couldn't find the requested skill in repo.",
		"You might want to check the repo contents.",
		repoDir,
		"Remember that this sometimes happens with skills.sh - it's stale data.",
		"[ OK ]",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(m.status, "lstat") || strings.Contains(m.install.Message, "lstat") {
		t.Fatalf("raw lstat leaked into status=%q message=%q", m.status, m.install.Message)
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

func TestInstallArchiveOnlyFallsBackWhenRegistryPathIsStale(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/golang-cli", "golang-cli", "Go CLI help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{
			Name:        "golang-cli",
			Description: "Go CLI help.",
			Path:        "golang-cli",
		},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.status != "archived golang-cli" {
		t.Fatalf("status = %q", m.status)
	}
	meta, ok, err := remote.ReadSourceMetadata(filepath.Join(cfg.ArchiveSkillsRoot(), "golang-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("source metadata missing")
	}
	if meta.SkillPath != "skills/golang-cli" {
		t.Fatalf("SkillPath = %q, want skills/golang-cli", meta.SkillPath)
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

func TestInstallDestinationModalUsesConfiguredRoots(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := customRootConfig(t)
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
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, "[x] .Oc") || strings.Contains(view, ".Ag") {
		t.Fatalf("destination modal should use configured custom root only:\n%s", view)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if m.status != "installed svelte-coder to .Oc" {
		t.Fatalf("status = %q, want installed svelte-coder to .Oc", m.status)
	}
	if _, err := os.Lstat(filepath.Join(cfg.ProjectRoot, ".opencode", "skills", "svelte-coder")); err != nil {
		t.Fatalf("custom root link was not created: %v", err)
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
	m.modal = newInstallDestinationModal(cfg, m.install.Results[1])
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

func TestInstallBatchDestinationModalUsesConfiguredRoots(t *testing.T) {
	cfg := customRootConfig(t)
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateArchived},
	}
	m.selected[ViewInstall][installID(m.install.Results[0].Result)] = true
	m.selected[ViewInstall][installID(m.install.Results[1].Result)] = true

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("batch destination modal is nil")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, "[x] .Oc") || strings.Contains(view, ".Ag") {
		t.Fatalf("batch destination modal should use configured custom root only:\n%s", view)
	}
}

func TestInstallDestinationModalRequiresSelectionWhenNoRootsConfigured(t *testing.T) {
	cfg := customRootConfig(t)
	configPath := filepath.Join(cfg.HomeDir, ".x-skills", "config.yaml")
	if err := os.WriteFile(configPath, []byte(`active_roots:
  - scope: project
    target: agents
    enabled: false
  - scope: project
    target: claude
    enabled: false
  - scope: project
    target: codex
    enabled: false
  - scope: global
    target: agents
    enabled: false
  - scope: global
    target: claude
    enabled: false
  - scope: global
    target: codex
    enabled: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(config.Default(cfg.ProjectRoot, cfg.HomeDir))
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.status != "select at least one destination" {
		t.Fatalf("status = %q, want select at least one destination", m.status)
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
	if _, err := os.Lstat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("archive remains after failed install-and-use: %v", err)
	}
	if m.status != "late link failure" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestInstallAndUseRollsBackNewArchiveAfterLateFailure(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "svelte-coder")

	backupPath, err := prepareInstallUseArchiveRollback(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if backupPath != "" {
		t.Fatalf("backup path = %q, want empty", backupPath)
	}
	makeSkill(t, filepath.Dir(archivePath), filepath.Base(archivePath), "New.")

	if err := rollbackInstallUseArchive(archivePath, backupPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive remains after rollback or unexpected error: %v", err)
	}
}

func TestInstallAndUseRollsBackReplacedArchiveAfterLateFailure(t *testing.T) {
	archivePath := makeSkill(t, t.TempDir(), "svelte-coder", "Old.")
	oldSkillPath := filepath.Join(archivePath, "SKILL.md")
	oldBytes, err := os.ReadFile(oldSkillPath)
	if err != nil {
		t.Fatal(err)
	}

	backupPath, err := prepareInstallUseArchiveRollback(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive remains while rollback backup is prepared or unexpected error: %v", err)
	}
	makeSkill(t, filepath.Dir(archivePath), filepath.Base(archivePath), "New.")

	if err := rollbackInstallUseArchive(archivePath, backupPath); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, oldSkillPath, string(oldBytes))
}

func TestInstallAndUseRollsBackArchiveAddedBeforeMutationAfterLateFailure(t *testing.T) {
	cfg, m, row, destinations := installAndUseLateFailureFixture(t, remote.ArchiveStateNotArchived)
	archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), row.Result.Name)
	originalPrepare := installUsePrepareArchiveRollback
	installUsePrepareArchiveRollback = func(path string) (string, error) {
		makeSkill(t, filepath.Dir(path), filepath.Base(path), "Old.")
		return originalPrepare(path)
	}
	t.Cleanup(func() {
		installUsePrepareArchiveRollback = originalPrepare
	})

	msg := m.installAndUse(row, destinations, false)().(installUseMsg)

	if msg.err == nil || !strings.Contains(msg.err.Error(), "late link failure") {
		t.Fatalf("error = %v, want late link failure", msg.err)
	}
	assertFileContent(t, filepath.Join(archivePath, "SKILL.md"), "---\nname: svelte-coder\ndescription: Old.\n---\n")
}

func TestInstallAndUseRollsBackArchiveCreatedFromStaleArchivedRowAfterLateFailure(t *testing.T) {
	cfg, m, row, destinations := installAndUseLateFailureFixture(t, remote.ArchiveStateArchived)
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), row.Result.Name, "Old.")
	if err := os.RemoveAll(archivePath); err != nil {
		t.Fatal(err)
	}

	msg := m.installAndUse(row, destinations, false)().(installUseMsg)

	if msg.err == nil || !strings.Contains(msg.err.Error(), "late link failure") {
		t.Fatalf("error = %v, want late link failure", msg.err)
	}
	if _, err := os.Lstat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive remains after rollback or unexpected error: %v", err)
	}
}

func TestInstallAndUseReportsArchiveRollbackFailureAfterGenerationInvalidation(t *testing.T) {
	_, m, row, destinations := installAndUseLateFailureFixture(t, remote.ArchiveStateNotArchived)
	originalRollback := installUseRollbackArchive
	installUseRollbackArchive = func(string, string) error {
		return errors.New("injected archive rollback failure")
	}
	t.Cleanup(func() {
		installUseRollbackArchive = originalRollback
	})
	originalApply := installApplyArchive
	installApplyArchive = func(req remote.AddRequest) (remote.AddResult, error) {
		result, err := originalApply(req)
		m.install.ensureUseGeneration().next()
		return result, err
	}
	t.Cleanup(func() {
		installApplyArchive = originalApply
	})

	msg := m.installAndUse(row, destinations, false)().(installUseMsg)

	if msg.err == nil || !strings.Contains(msg.err.Error(), "injected archive rollback failure") {
		t.Fatalf("error = %v, want archive rollback failure", msg.err)
	}
}

func installAndUseLateFailureFixture(
	t *testing.T,
	archiveState string,
) (config.Config, *Model, installResultView, []installDestination) {
	t.Helper()
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	row := installResultView{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: archiveState,
	}
	destinations := []installDestination{
		{Scope: config.ScopeProject, Target: config.TargetAgents},
		{Scope: config.ScopeProject, Target: config.TargetClaude},
	}
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
	return cfg, &m, row, destinations
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func TestInstallAndUseBatchRollsBackFailedRowPartialLinks(t *testing.T) {
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
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

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
		if calls == 4 {
			return actions.MutationResult{}, errors.New("late batch link failure")
		}
		return originalLink(cfg, req)
	}
	t.Cleanup(func() {
		installUseLink = originalLink
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("batch install-use cmd is nil")
	}
	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if calls != 4 {
		t.Fatalf("link calls = %d, want 4", calls)
	}
	for _, target := range []string{config.TargetAgents, config.TargetClaude} {
		if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, target), "svelte-coder")); err != nil {
			t.Fatalf("successful row link %s missing: %v", target, err)
		}
	}
	for _, target := range []string{config.TargetAgents, config.TargetClaude} {
		if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, target), "react-coder")); !os.IsNotExist(err) {
			t.Fatalf("failed row link %s remains or unexpected error: %v", target, err)
		}
	}
	if m.status != "installed 1 of 2 skills" {
		t.Fatalf("status = %q, want partial batch status", m.status)
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

func TestInstallAndUseBatchContinuesAfterMiddleNameConflict(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "Incoming React help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "react-coder", "Existing React help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "someone-else"}); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateNameConflict},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while batch destination modal opens", cmd)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("initial batch install-use cmd is nil")
	}
	useMsg := cmd().(installUseMsg)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), "Name conflict: react-coder") {
		t.Fatalf("install-use batch did not pause on middle conflict:\n%s", installTestModalView(m, 120, 35))
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("rename existing archive cmd is nil")
	}
	archiveMsg := cmd().(installArchiveMsg)
	updated, cmd = m.Update(archiveMsg)
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail install-use cmd is nil after resolving middle conflict")
	}
	useMsg = commandMessage[installUseMsg](t, cmd)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)

	for _, name := range []string{"svelte-coder", "react-coder", "vue-coder"} {
		if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), name)); err != nil {
			t.Fatalf("active link %s missing after continuation: %v", name, err)
		}
	}
	if m.status != "installed 3 skills to .Ag" {
		t.Fatalf("status = %q, want installed 3 skills", m.status)
	}
}

func TestInstallAndUseBatchRenameIncomingLinksResolvedArchiveName(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "Incoming React help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "react-coder", "Existing React help.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "someone-else"}); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateNameConflict},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("initial batch install-use cmd is nil")
	}
	updated, _ = m.Update(cmd().(installUseMsg))
	m = mustModel(t, updated)
	for range 2 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("rename incoming archive cmd is nil")
	}
	updated, cmd = m.Update(cmd().(installArchiveMsg))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail install-use cmd is nil after renaming incoming archive")
	}
	updated, _ = m.Update(commandMessage[installUseMsg](t, cmd))
	m = mustModel(t, updated)

	activeRoot := cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents)
	for _, name := range []string{"react-coder-remote", "vue-coder"} {
		if _, err := os.Lstat(filepath.Join(activeRoot, name)); err != nil {
			t.Fatalf("active link %s missing after continuation: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(activeRoot, "react-coder")); !os.IsNotExist(err) {
		t.Fatalf("active link used unresolved archive name or unexpected error: %v", err)
	}
	if m.status != "installed 2 skills to .Ag" {
		t.Fatalf("status = %q, want installed 2 skills", m.status)
	}
}

func TestInstallAndUseBatchContinuesAfterMiddleUpdate(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/react-coder", "react-coder", "Incoming React help.")
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "react-coder", "Existing React help.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{
		SourceType: remote.SourceTypeGit,
		CloneURL:   repoDir,
		SkillPath:  "skills/react-coder",
	}); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.width = 120
	m.height = 40
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{
		{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
		{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}, ArchiveState: remote.ArchiveStateUpdateAvailable},
		{Result: remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"}, ArchiveState: remote.ArchiveStateNotArchived},
	}
	for _, row := range m.install.Results {
		m.selected[ViewInstall][installID(row.Result)] = true
	}

	updated, cmd := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if cmd != nil {
		t.Fatalf("cmd = %#v, want nil while batch destination modal opens", cmd)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes(" "))
	m = mustModel(t, updated)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("initial batch install-use cmd is nil")
	}
	useMsg := cmd().(installUseMsg)
	updated, cmd = m.Update(useMsg)
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("middle update diff cmd is nil")
	}
	diffMsg := commandMessage[installUpdateDiffMsg](t, cmd)
	updated, _ = m.Update(diffMsg)
	m = mustModel(t, updated)
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 40, m)), "Archive conflict: react-coder") {
		t.Fatalf("install-use batch did not pause on middle update:\n%s", installTestModalView(m, 120, 40))
	}

	updated, cmd = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail install-use cmd is nil after resolving middle update")
	}
	useMsg = commandMessage[installUseMsg](t, cmd)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)

	for _, target := range []string{config.TargetAgents, config.TargetClaude} {
		for _, name := range []string{"svelte-coder", "react-coder", "vue-coder"} {
			if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, target), name)); err != nil {
				t.Fatalf("active link %s/%s missing after update continuation: %v", target, name, err)
			}
		}
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "react-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Existing React help." {
		t.Fatalf("react archive description = %q, want kept archive", info.Description)
	}
	if m.status != "installed 3 skills to .Ag, .Cl" {
		t.Fatalf("status = %q, want installed 3 skills to .Ag, .Cl", m.status)
	}
}

func TestInstallAndUseBatchMissingSkillContinuesWithTail(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.archiveToken = 1

	missing := installResultView{
		Result:       remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}
	tail := installResultView{
		Result:       remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}
	destinations := []installDestination{{
		Scope:   config.ScopeProject,
		Target:  config.TargetAgents,
		Label:   ".Ag",
		Checked: true,
	}}
	m.install.pendingUseBatch = &installUseBatchContinuation{
		identity:     installArchiveIdentityFromResult(missing.Result),
		row:          missing,
		total:        2,
		remaining:    []installResultView{tail},
		destinations: destinations,
	}

	updated, cmd := m.Update(installArchiveMsg{
		token:    1,
		name:     missing.Result.Name,
		identity: installArchiveIdentityFromResult(missing.Result),
		err:      &remote.MissingSkillError{Name: missing.Result.Name, PreferredPath: missing.Result.Path},
	})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail install-use cmd is nil after missing skill")
	}
	useMsg := commandMessage[installUseMsg](t, cmd)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)

	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "vue-coder")); err != nil {
		t.Fatalf("tail active link missing after continuation: %v", err)
	}
	if m.status != "installed 1 of 2 skills" {
		t.Fatalf("status = %q, want partial install status", m.status)
	}
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), `react-coder: skill "react-coder" not found`) {
		t.Fatalf("result modal missing failed row:\n%s", installTestModalView(m, 120, 35))
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
	diffMsg := commandMessage[installUpdateDiffMsg](t, cmd)
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

func TestInstallArchiveOnlyResultOutsideInstallIsIgnoredAndKeepsView(t *testing.T) {
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
	if m.status != "" {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.install.Results[0].ArchiveState; got != remote.ArchiveStateNotArchived {
		t.Fatalf("archive state = %q, want stale result ignored", got)
	}
	if len(m.repo) != 0 {
		t.Fatalf("repo = %#v, want stale result not reloaded", m.repo)
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

func TestInstallArchiveBatchStaleUpdateDiffDoesNotClearNewerPendingBatch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	row := installResultView{
		Result:       remote.SearchResult{Name: "svelte-coder", Owner: "owner", Repo: "repo", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.previewToken = 2
	m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
		identity:    installArchiveIdentityFromResult(row.Result),
		updateToken: 2,
		total:       2,
		remaining: []installResultView{{
			Result:       remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		}},
	}

	updated, _ := m.Update(installUpdateDiffMsg{token: 1, row: row})
	m = mustModel(t, updated)

	if m.install.pendingArchiveBatch == nil {
		t.Fatal("stale update diff cleared newer pending archive batch")
	}
	if got := m.install.pendingArchiveBatch.updateToken; got != 2 {
		t.Fatalf("pending archive batch updateToken = %d, want 2", got)
	}
}

func TestInstallUseBatchStaleUpdateDiffDoesNotClearNewerPendingBatch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	row := installResultView{
		Result:       remote.SearchResult{Name: "svelte-coder", Owner: "owner", Repo: "repo", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.previewToken = 2
	m.install.pendingUseBatch = &installUseBatchContinuation{
		identity:     installArchiveIdentityFromResult(row.Result),
		row:          row,
		updateToken:  2,
		total:        2,
		destinations: []installDestination{{Scope: config.ScopeProject, Target: config.TargetAgents, Label: ".Ag", Checked: true}},
		remaining: []installResultView{{
			Result:       remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"},
			ArchiveState: remote.ArchiveStateNotArchived,
		}},
	}

	updated, _ := m.Update(installUpdateDiffMsg{token: 1, row: row})
	m = mustModel(t, updated)

	if m.install.pendingUseBatch == nil {
		t.Fatal("stale update diff cleared newer pending install-use batch")
	}
	if got := m.install.pendingUseBatch.updateToken; got != 2 {
		t.Fatalf("pending install-use batch updateToken = %d, want 2", got)
	}
}

func TestInstallBatchContinuationPreservesResolvedArchiveName(t *testing.T) {
	row := installResultView{Result: remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"}}
	identity := installArchiveIdentityFromResult(row.Result)

	t.Run("archive error", func(t *testing.T) {
		m := New(config.Default(t.TempDir(), t.TempDir()))
		m.setView(ViewInstall)
		m.install.archiveToken = 1
		m.install.pendingArchiveBatch = &installArchiveBatchContinuation{identity: identity, total: 1}

		updated, _ := m.Update(installArchiveMsg{
			token: 1, name: "react-coder-remote", identity: identity, err: errors.New("archive failed"),
		})
		m = mustModel(t, updated)
		if view := installTestModalView(m, 120, 35); !strings.Contains(view, "react-coder-remote: archive failed") {
			t.Fatalf("archive batch result lost resolved name:\n%s", view)
		}
	})

	t.Run("install error", func(t *testing.T) {
		m := New(config.Default(t.TempDir(), t.TempDir()))
		m.setView(ViewInstall)
		m.install.useToken = 1
		m.install.pendingUseBatch = &installUseBatchContinuation{
			identity: identity, row: row, total: 1,
		}

		updated, _ := m.Update(installUseMsg{
			token: 1, name: "react-coder-remote", row: row, identity: identity, err: errors.New("archive failed"),
		})
		m = mustModel(t, updated)
		if view := installTestModalView(m, 120, 35); !strings.Contains(view, "react-coder-remote: archive failed") {
			t.Fatalf("install batch result lost resolved name:\n%s", view)
		}
	})

	t.Run("keep existing", func(t *testing.T) {
		m := New(config.Default(t.TempDir(), t.TempDir()))
		m.setView(ViewInstall)
		m.width = 120
		m.height = 40
		m.install.previewToken = 1
		m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
			identity: identity, updateToken: 1, total: 1,
		}

		updated, _ := m.Update(installUpdateDiffMsg{token: 1, row: row})
		m = mustModel(t, updated)
		updated, _ = m.Update(keyRunes("k"))
		m = mustModel(t, updated)
		if m.status != "archived 1 skills" {
			t.Fatalf("status = %q, want kept archive counted in completed batch", m.status)
		}
	})
}

func TestInstallArchiveBatchUpdateDiffMissingSkillContinuesWithTail(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.previewToken = 1

	missing := installResultView{
		Result:       remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}
	tail := installResultView{
		Result:       remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}
	m.install.pendingArchiveBatch = &installArchiveBatchContinuation{
		identity:    installArchiveIdentityFromResult(missing.Result),
		updateToken: 1,
		total:       2,
		remaining:   []installResultView{tail},
	}

	updated, cmd := m.Update(installUpdateDiffMsg{
		token: 1,
		row:   missing,
		err:   &remote.MissingSkillError{Name: missing.Result.Name, PreferredPath: missing.Result.Path},
	})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail archive cmd is nil after missing skill update diff")
	}
	if m.modal != nil {
		t.Fatalf("missing skill update diff opened modal before tail command:\n%s", installTestModalView(m, 120, 35))
	}

	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)

	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "vue-coder")); err != nil {
		t.Fatalf("tail archive missing after update diff continuation: %v", err)
	}
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), `skill "react-coder" not found`) {
		t.Fatalf("result modal missing failed row:\n%s", installTestModalView(m, 120, 35))
	}
	if strings.Contains(plain(m.modal.View(120, 35, m)), "Couldn't find the requested skill in repo") {
		t.Fatalf("batch update diff missing skill opened single-row stale registry modal:\n%s", installTestModalView(m, 120, 35))
	}
}

func TestInstallUseBatchUpdateDiffMissingSkillContinuesWithTail(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/vue-coder", "vue-coder", "Vue help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.previewToken = 1

	missing := installResultView{
		Result:       remote.SearchResult{Name: "react-coder", Path: "skills/react-coder"},
		ArchiveState: remote.ArchiveStateUpdateAvailable,
	}
	tail := installResultView{
		Result:       remote.SearchResult{Name: "vue-coder", Path: "skills/vue-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}
	destinations := []installDestination{{
		Scope:   config.ScopeProject,
		Target:  config.TargetAgents,
		Label:   ".Ag",
		Checked: true,
	}}
	m.install.pendingUseBatch = &installUseBatchContinuation{
		identity:     installArchiveIdentityFromResult(missing.Result),
		row:          missing,
		updateToken:  1,
		total:        2,
		remaining:    []installResultView{tail},
		destinations: destinations,
	}

	updated, cmd := m.Update(installUpdateDiffMsg{
		token: 1,
		row:   missing,
		err:   &remote.MissingSkillError{Name: missing.Result.Name, PreferredPath: missing.Result.Path},
	})
	m = mustModel(t, updated)
	if cmd == nil {
		t.Fatal("tail install-use cmd is nil after missing skill update diff")
	}
	if m.modal != nil {
		t.Fatalf("missing skill update diff opened modal before tail command:\n%s", installTestModalView(m, 120, 35))
	}

	useMsg := cmd().(installUseMsg)
	updated, _ = m.Update(useMsg)
	m = mustModel(t, updated)

	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "vue-coder")); err != nil {
		t.Fatalf("tail active link missing after update diff continuation: %v", err)
	}
	if m.modal == nil || !strings.Contains(plain(m.modal.View(120, 35, m)), `react-coder: skill "react-coder" not found`) {
		t.Fatalf("result modal missing failed row:\n%s", installTestModalView(m, 120, 35))
	}
	if strings.Contains(plain(m.modal.View(120, 35, m)), "Couldn't find the requested skill in repo") {
		t.Fatalf("batch update diff missing skill opened single-row stale registry modal:\n%s", installTestModalView(m, 120, 35))
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

func TestInstallArchiveStateCheckoutCancelledWhenLeavingInstall(t *testing.T) {
	testInstallArchiveStateCheckoutCancellation(t, func(m *Model) {
		m.setView(ViewRepo)
	})
}

func TestInstallArchiveStateCheckoutCancelledWhenQuitting(t *testing.T) {
	testInstallArchiveStateCheckoutCancellation(t, func(m *Model) {
		updated, cmd := m.handleKey(keyRunes("q"))
		*m = mustModel(t, updated)
		if cmd == nil {
			t.Fatal("quit cmd is nil")
		}
	})
}

func testInstallArchiveStateCheckoutCancellation(t *testing.T, cancel func(*Model)) {
	t.Helper()
	cfg := config.Default(t.TempDir(), t.TempDir())
	archivePath := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Archived.")
	if err := remote.WriteSourceMetadata(archivePath, remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "vercel-labs",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	previousCheckout := installArchiveStateCheckout
	installArchiveStateCheckout = func(ctx context.Context, _ *remote.CheckoutCache, _ remote.GitSource) (remote.Checkout, error) {
		close(started)
		<-ctx.Done()
		return remote.Checkout{}, ctx.Err()
	}
	t.Cleanup(func() { installArchiveStateCheckout = previousCheckout })

	m := New(cfg)
	m.setView(ViewInstall)
	m.install.searchToken = 1
	m.install.checkouts = remote.NewCheckoutCache(t.TempDir())
	cmd := m.applyInstallSearchResult(installSearchResultMsg{
		token: 1,
		query: "svelte",
		results: []remote.SearchResult{{
			Name:  "svelte-coder",
			Owner: "vercel-labs",
			Repo:  "skills",
			Path:  "skills/svelte-coder",
		}},
	})
	if cmd == nil {
		t.Fatal("archive state check cmd is nil")
	}

	result := make(chan installArchiveStatesMsg, 1)
	go func() { result <- cmd().(installArchiveStatesMsg) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("checkout did not start")
	}

	cancel(&m)
	select {
	case msg := <-result:
		if len(msg.results) != 1 || !errors.Is(msg.results[0].Err, context.Canceled) {
			t.Fatalf("checkout error = %#v, want context.Canceled", msg.results)
		}
	case <-time.After(time.Second):
		t.Fatal("checkout was not cancelled promptly")
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

func runInstallArchiveBatchCommands(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for range 100 {
		if cmd == nil {
			return m
		}
		updated, next := m.Update(cmd())
		m = mustModel(t, updated)
		cmd = next
	}
	t.Fatal("archive batch did not finish after 100 messages")
	return m
}

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
