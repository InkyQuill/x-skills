package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
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
