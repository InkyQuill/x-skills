package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const animationInterval = 140 * time.Millisecond

var (
	pulseDiamondFrames = []string{"◆", "◇", "◈", "◇"}
)

type animationTickMsg time.Time

func animationTick() tea.Cmd {
	return tea.Tick(animationInterval, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}

func (m Model) animationsEnabled() bool {
	return !m.opts.ASCII
}

func (m Model) pulseDiamond() string {
	if !m.animationsEnabled() {
		return m.symbols.ProductMark
	}
	return pulseDiamondFrames[m.animationFrame%len(pulseDiamondFrames)]
}
