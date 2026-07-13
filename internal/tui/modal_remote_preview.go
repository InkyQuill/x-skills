package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/remote"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type previewResolver func(
	context.Context,
	*remote.CheckoutCache,
	remote.PreviewRequest,
) (remote.PreviewResult, error)

type remotePreviewMsg struct {
	token  int
	result remote.PreviewResult
	err    error
}

type remotePreviewModal struct {
	token      int
	title      string
	repository string
	skill      string
	preview    *previewModal
	err        error
}

func (r *remotePreviewModal) Title() string {
	return r.title
}

func (r *remotePreviewModal) View(width, height int, m Model) string {
	if r.preview != nil {
		return r.preview.View(width, height, m)
	}

	bodyWidth := max(modalContentWidth(width), 20)
	lines := []string{
		accentStyle.Render(tuiui.TruncateANSI(r.title, bodyWidth)),
		mutedStyle.Render(tuiui.TruncateANSI(r.repository+"  |  "+r.skill, bodyWidth)),
		strings.Repeat("-", bodyWidth),
	}
	if r.err != nil {
		lines = append(lines,
			dangerStyle.Render("Could not load this preview."),
			fmt.Sprintf("%v", r.err),
			"",
			"Check the repository, skill path, and your access, then try again.",
		)
	} else {
		lines = append(lines,
			fmt.Sprintf("%s Loading preview...", m.pulseDiamond()),
			"",
			"Checking out the repository and reading SKILL.md.",
		)
	}
	lines = append(lines,
		"",
		tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		}),
	)
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (r *remotePreviewModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if r.preview == nil {
		if closeOnEscapeOrQuit(msg) {
			m.closeRemotePreview()
			return true, nil
		}
		return false, nil
	}

	closed, cmd := r.preview.Update(msg, m)
	if closed {
		m.closeRemotePreview()
		return true, cmd
	}
	m.modal = r
	return false, cmd
}

func (r *remotePreviewModal) routesKeyToModel(msg tea.KeyMsg) bool {
	if r.preview != nil {
		return false
	}
	if _, ok := viewForKey(msg); ok {
		return true
	}
	return listCursorDelta(msg) != 0 || msg.String() == "enter"
}

func (m *Model) openRemotePreview() tea.Cmd {
	if m.install.useInFlight || m.install.archiveInFlight {
		return nil
	}
	row, ok := m.selectedInstallResult()
	if !ok {
		return nil
	}

	m.closeRemotePreview()
	token := m.previewToken
	repository := row.Result.Owner + "/" + row.Result.Repo
	if m.install.testCloneURL != "" {
		repository = m.install.testCloneURL
	} else if row.Result.Owner == "" || row.Result.Repo == "" {
		repository = "unknown repository"
	}
	preview := &remotePreviewModal{
		token:      token,
		title:      "Preview: " + row.Result.Name,
		repository: repository,
		skill:      row.Result.Name,
	}
	m.modal = preview
	m.previewLoading = true

	checkouts := m.ensureInstallCheckoutCache()
	source, sourceErr := m.gitSourceForInstall(row.Result)
	if checkouts == nil || sourceErr != nil {
		if sourceErr == nil {
			sourceErr = errors.New(m.status)
		}
		return func() tea.Msg { return remotePreviewMsg{token: token, err: sourceErr} }
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.previewCancel = cancel
	resolver := m.resolvePreview
	if resolver == nil {
		resolver = remote.ResolvePreview
	}
	request := remote.PreviewRequest{
		Source:        source,
		Name:          row.Result.Name,
		PreferredPath: row.Result.Path,
	}
	return func() tea.Msg {
		defer cancel()
		previewCtx, timeoutCancel := context.WithTimeout(ctx, installPreviewTimeout)
		defer timeoutCancel()
		result, err := resolver(previewCtx, checkouts, request)
		return remotePreviewMsg{token: token, result: result, err: err}
	}
}

func (m *Model) applyRemotePreview(msg remotePreviewMsg) {
	preview, ok := m.modal.(*remotePreviewModal)
	if !ok || msg.token != m.previewToken || preview.token != msg.token || m.view != ViewInstall {
		return
	}
	m.previewCancel = nil
	m.previewLoading = false
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.modal = nil
			return
		}
		preview.err = msg.err
		return
	}
	preview.preview = newPreviewModalFromDocument(
		preview.title,
		msg.result.SkillPath,
		msg.result.SkillMD,
	)
}

func (m *Model) cancelRemotePreview() {
	if m.previewCancel != nil {
		m.previewCancel()
		m.previewCancel = nil
	}
	m.previewLoading = false
	m.previewToken++
}

func (m *Model) closeRemotePreview() {
	m.cancelRemotePreview()
	if _, ok := m.modal.(*remotePreviewModal); ok {
		m.modal = nil
	}
}
