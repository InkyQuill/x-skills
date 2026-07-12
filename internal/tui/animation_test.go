package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
)

func TestInitStartsAnimationOnlyWhenUnicode(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	if cmd := New(cfg, Options{ASCII: true}).Init(); cmd != nil {
		t.Fatal("ASCII Init returned animation command, want nil")
	}
	if cmd := New(cfg).Init(); cmd == nil {
		t.Fatal("Unicode Init returned nil, want animation command")
	}
}

func TestAnimationTickAdvancesOnlyWhenUnicode(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	m := New(cfg)
	m.animationFrame = 4
	updated, cmd := m.Update(animationTickMsg(time.Time{}))
	m = mustModel(t, updated)
	if m.animationFrame != 5 {
		t.Fatalf("Unicode animationFrame = %d, want 5", m.animationFrame)
	}
	if cmd == nil {
		t.Fatal("Unicode animation tick returned nil command, want next tick")
	}

	ascii := New(cfg, Options{ASCII: true})
	ascii.animationFrame = 4
	updated, cmd = ascii.Update(animationTickMsg(time.Time{}))
	ascii = mustModel(t, updated)
	if ascii.animationFrame != 4 {
		t.Fatalf("ASCII animationFrame = %d, want unchanged 4", ascii.animationFrame)
	}
	if cmd != nil {
		t.Fatal("ASCII animation tick returned command, want nil")
	}
}

func TestAnimationFramesAffectUnicodeHeaderAndRowsOnly(t *testing.T) {
	unicode := animationRenderModel(Options{})
	firstUnicode := animationRenderOutput(unicode)
	unicode.animationFrame = 1
	secondUnicode := animationRenderOutput(unicode)
	if firstUnicode == secondUnicode {
		t.Fatalf("Unicode render did not change across animation frames:\n%s", firstUnicode)
	}

	ascii := animationRenderModel(Options{ASCII: true})
	firstASCII := animationRenderOutput(ascii)
	ascii.animationFrame = 1
	secondASCII := animationRenderOutput(ascii)
	if firstASCII != secondASCII {
		t.Fatalf("ASCII render changed across animation frames:\nfirst:\n%s\nsecond:\n%s", firstASCII, secondASCII)
	}
	if !strings.Contains(firstASCII, "* x-skills") || !strings.Contains(firstASCII, "> [x] zen-of-go") {
		t.Fatalf("ASCII render did not preserve expected symbols:\n%s", firstASCII)
	}
}

func TestAnimationFramesDoNotAnimateRowCursor(t *testing.T) {
	m := animationRenderModel(Options{})
	m.selected[ViewRepo] = map[string]bool{}

	first := plain(strings.Join(renderRepoRows(m, 80), "\n"))
	m.animationFrame = 1
	second := plain(strings.Join(renderRepoRows(m, 80), "\n"))

	if first != second {
		t.Fatalf("focused row cursor changed across animation frames:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !strings.Contains(first, "› ◇ zen-of-go") {
		t.Fatalf("focused row does not use static cursor marker:\n%s", first)
	}
}

func TestAnimationFramesUseStableDisplayWidths(t *testing.T) {
	tests := []struct {
		name   string
		frames []string
		width  int
	}{
		{name: "pulse", frames: pulseDiamondFrames, width: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, frame := range tt.frames {
				if got := lipgloss.Width(frame); got != tt.width {
					t.Fatalf("frame %q width = %d, want %d", frame, got, tt.width)
				}
			}
		})
	}
}

func animationRenderModel(opts Options) Model {
	return Model{
		opts:    opts,
		symbols: symbolsFor(opts),
		view:    ViewRepo,
		cursor:  0,
		selected: map[ViewName]map[string]bool{
			ViewActive:  {},
			ViewRepo:    {"repo:zen-of-go": true},
			ViewDoctor:  {},
			ViewInstall: {},
		},
		repo: []repo.Skill{{
			Name:        "zen-of-go",
			Description: "Go style guide",
		}},
		repoUsage: map[string][]string{"zen-of-go": {".Ag"}},
	}
}

func animationRenderOutput(m Model) string {
	return plain(strings.Join([]string{
		renderHeader(m, 80),
		strings.Join(renderRepoRows(m, 80), "\n"),
	}, "\n"))
}
