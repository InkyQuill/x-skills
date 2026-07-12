package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

type mutationReconcileMsg struct {
	token     uint64
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
}

type recommendationResultMsg struct {
	token     uint64
	names     []string
	promote   bool
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
	reloadErr error
}

type renameArchiveResultMsg struct {
	token     uint64
	oldName   string
	newName   string
	result    actions.RenameResult
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
	reloadErr error
}

func (m *Model) openRepoRenameModal() {
	names := m.selectedRepoSkillNames()
	if len(names) != 1 {
		m.status = "select exactly one archived skill to rename"
		return
	}
	oldName := names[0]
	lines := []string{"Other projects are not indexed and may still link to the old archive path."}
	usagePaths, err := actions.VisibleArchiveUsagePaths(m.cfg, oldName)
	if err != nil {
		m.status = "inspect visible archive usages: " + err.Error()
		return
	}
	if len(usagePaths) == 0 {
		lines = append(lines, "Visible managed usages: none")
	} else {
		names := make([]string, 0, len(usagePaths))
		for _, path := range usagePaths {
			names = append(names, filepath.Base(path))
		}
		lines = append(lines, "Visible: "+strings.Join(names, ", "))
	}
	m.modal = newTextModal("Rename archive: "+oldName, strings.Join(lines, "\n"), oldName, func(current *Model, newName string) tea.Cmd {
		return current.beginRepoRename(oldName, newName)
	})
}

func (m *Model) beginRepoRename(oldName, newName string) tea.Cmd {
	if m.renameInFlight {
		return nil
	}
	if err := repo.ValidateName(newName); err != nil {
		m.status = err.Error()
		return nil
	}
	m.renameToken++
	token := m.renameToken
	m.renameInFlight = true
	ctx, cancel := context.WithCancel(context.Background())
	m.renameCancel = cancel
	m.modal = nil
	m.status = "renaming " + oldName + "..."
	cfg := m.cfg
	return func() tea.Msg {
		msg := renameArchiveResultMsg{token: token, oldName: oldName, newName: newName}
		msg.result, msg.err = actions.RenameArchiveContext(ctx, cfg, oldName, newName)
		if ctx.Err() != nil {
			return msg
		}
		msg.active, msg.repo, msg.issues, msg.repoUsage, msg.reloadErr = loadTUIData(ctx, cfg)
		return msg
	}
}

func (m *Model) applyRenameArchiveResult(msg renameArchiveResultMsg) tea.Cmd {
	if msg.token != m.renameToken {
		return nil
	}
	m.renameInFlight = false
	m.renameCancel = nil
	if msg.err != nil {
		m.status = "archive rename failed: " + msg.err.Error()
		return nil
	}
	if msg.reloadErr == nil {
		m.active, m.repo, m.issues, m.repoUsage = msg.active, msg.repo, msg.issues, msg.repoUsage
		m.selected[ViewRepo] = map[string]bool{"repo:" + msg.newName: true}
		m.clampCursor()
	}
	m.status = "Renamed " + msg.oldName + " to " + msg.newName
	if msg.reloadErr != nil {
		m.status += ", but refreshing the TUI failed: " + msg.reloadErr.Error()
	}
	return nil
}

func (m *Model) cancelRenameWork() {
	if m.renameCancel != nil {
		m.renameCancel()
		m.renameCancel = nil
	}
}

func (m *Model) queueProjectReconciliation() bool {
	if !m.mutationProjectTouched {
		return false
	}
	m.mutationProjectTouched = false
	m.mutationToken++
	token := m.mutationToken
	m.mutationInFlight = true
	cfg := m.cfg
	m.pendingMutationCmd = func() tea.Msg {
		_, err := manifest.ReconcileLocal(cfg)
		active, repoSkills, issues, repoUsage, reloadErr := loadTUIData(context.Background(), cfg)
		if err == nil {
			err = reloadErr
		}
		return mutationReconcileMsg{token: token, active: active, repo: repoSkills, issues: issues, repoUsage: repoUsage, err: err}
	}
	return true
}

func (m *Model) applyMutationReconcileResult(msg mutationReconcileMsg) tea.Cmd {
	if msg.token != m.mutationToken {
		return nil
	}
	m.mutationInFlight = false
	m.active = msg.active
	m.repo = msg.repo
	m.issues = msg.issues
	m.repoUsage = msg.repoUsage
	m.clampCursor()
	if msg.err != nil {
		line := "x skill mutation succeeded but local manifest reconciliation failed: " + msg.err.Error()
		if result, ok := m.modal.(resultModal); ok {
			result.lines = append(result.lines, line)
			m.modal = result
		} else {
			m.status = strings.TrimPrefix(line, "x ")
		}
	}
	return nil
}

func (m *Model) activeTargets() []actions.ActiveSkill {
	return m.selectedActiveSkills("migrate")
}

func (m *Model) openMigrateModal() {
	targets := m.activeTargets()
	if len(targets) == 0 {
		m.modal = newResultModal("Migrate active skills", []string{"No unmanaged active skill directories selected."})
		return
	}
	m.modal = newMigrateConfirmModal(targets, func(current *Model) {
		current.applyMigrateTargets(targets, actions.ConflictResolutionAsk)
	})
}

func (m *Model) applyMigrateTargets(targets []actions.ActiveSkill, resolution string) {
	m.applyMigrateTargetsWithResults(targets, resolution, nil, nil)
}

func (m *Model) applyMigrateTargetsWithResults(targets []actions.ActiveSkill, resolution string, successes, failures []string) {
	for i, skill := range targets {
		result, err := actions.Migrate(m.cfg, actions.MigrateRequest{
			Name:               filepath.Base(skill.Path),
			Scope:              skill.Root.Scope,
			Target:             skill.Root.Target,
			Confirmed:          true,
			ConflictResolution: resolution,
		})
		if err != nil {
			var conflict *actions.ArchiveConflictError
			if errors.As(err, &conflict) {
				tail := append([]actions.ActiveSkill(nil), targets[i+1:]...)
				successesBeforeConflict := append([]string(nil), successes...)
				failuresBeforeConflict := append([]string(nil), failures...)
				m.openArchiveConflictModal(conflict, "Migration Results", func(current *Model, chosen string) {
					current.applyResolvedMigrateConflict(skill, tail, chosen, successesBeforeConflict, failuresBeforeConflict)
				})
				return
			}
			failures = append(failures, "x "+filepath.Base(skill.Path)+"  "+err.Error())
			continue
		}
		successes = append(successes, "✓ "+result.Name+"  "+result.Status)
		m.mutationProjectTouched = m.mutationProjectTouched || skill.Root.Scope == config.ScopeProject
	}
	m.finishMigrateTargets(successes, failures)
}

func (m *Model) applyResolvedMigrateConflict(skill actions.ActiveSkill, remaining []actions.ActiveSkill, resolution string, successes, failures []string) {
	result, err := actions.Migrate(m.cfg, actions.MigrateRequest{
		Name:               filepath.Base(skill.Path),
		Scope:              skill.Root.Scope,
		Target:             skill.Root.Target,
		Confirmed:          true,
		ConflictResolution: resolution,
	})
	if err != nil {
		failures = append(failures, "x "+filepath.Base(skill.Path)+"  "+err.Error())
	} else {
		successes = append(successes, "✓ "+result.Name+"  "+result.Status)
		m.mutationProjectTouched = m.mutationProjectTouched || skill.Root.Scope == config.ScopeProject
	}
	m.applyMigrateTargetsWithResults(remaining, actions.ConflictResolutionAsk, successes, failures)
}

func (m *Model) openArchiveConflictModal(conflict *actions.ArchiveConflictError, resultTitle string, apply func(*Model, string)) {
	diff, err := buildDirectoryDiff(conflict.ActivePath, conflict.ArchivedPath)
	if err != nil {
		m.modal = newResultModal(resultTitle, []string{fmt.Sprintf("failed to build conflict diff: %v", err)})
		return
	}
	m.modal = newConflictDiffModalWithModelApply(conflict.Name, diff, "Incoming active", apply)
}

func (m *Model) finishMigrateTargets(successes, failures []string) {
	if !m.queueProjectReconciliation() {
		m.reload()
	}
	if len(failures) == 0 {
		m.modal = nil
		m.status = mutationSuccessStatus(successes, "migrated")
		return
	}
	lines := append(successes, failures...)
	m.modal = newResultModal("Migration Results", lines)
}

type migrateConfirmModal struct {
	targets []actions.ActiveSkill
	scroll  int
	choice  int
	apply   func(*Model)
}

func newMigrateConfirmModal(targets []actions.ActiveSkill, apply func(*Model)) modal {
	return migrateConfirmModal{targets: targets, apply: apply}
}

func (c migrateConfirmModal) Title() string {
	return "Migrate active skills"
}

func (c migrateConfirmModal) View(width, height int, m Model) string {
	targetRows := c.targetRows(width, m)
	body := []string{
		fmt.Sprintf("Targets (%d)", len(c.targets)),
	}
	body = append(body, targetRows...)
	apply := "[ Apply ]"
	cancel := "Cancel"
	if c.choice == 1 {
		apply = "Apply"
		cancel = "[ Cancel ]"
	}
	if c.choice == 0 {
		apply = selectedBg.Render(apply)
	} else {
		cancel = selectedBg.Render(cancel)
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: "Migrate active skills",
		Body:  body,
		Footer: []string{
			apply + "   " + cancel,
			tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
				{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
				{ASCII: "left/right", Unicode: "←/→", Label: "choose"},
				{ASCII: "enter", Unicode: "↵", Label: "apply"},
				{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
			}),
		},
		Scroll:    c.scroll,
		UseScroll: true,
	})
}

func (c migrateConfirmModal) targetRows(width int, m Model) []string {
	rows := make([]string, 0, len(c.targets)+1)
	for _, target := range c.targets {
		chip := rootLabel(target.Root)
		rows = append(rows, tuiui.TruncateANSI("  "+filepath.Base(target.Path)+"  "+renderRootChip(m.symbols, chip, lipgloss.NoColor{}), width-16))
	}
	return rows
}

func (c migrateConfirmModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		c.scroll = tuiui.ClampIndex(c.scroll+delta, len(c.targets))
		m.modal = c
		return false, nil
	}
	switch msg.String() {
	case "esc", "q", "n":
		return true, nil
	case "left", "right":
		if c.choice == 0 {
			c.choice = 1
		} else {
			c.choice = 0
		}
		m.modal = c
	case "enter":
		if c.choice == 0 {
			c.apply(m)
			return false, nil
		}
		return true, nil
	case "y":
		c.apply(m)
	}
	return false, nil
}

func (m *Model) openUnlinkModal() {
	targets := m.selectedActiveSkills("unlink")
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink active skills", []string{"No active skills selected."})
		return
	}
	m.modal = repoUsageModal{
		name:     activeUnlinkTitle(targets),
		targets:  activeUsageTargets(targets),
		selected: selectedIndexes(len(targets)),
		apply: func(current *Model, modal repoUsageModal) {
			current.applyActiveUsageUnlink(modal)
		},
	}
}

func (m Model) selectedActiveSkills(action string) []actions.ActiveSkill {
	selected := map[string]bool{}
	for _, id := range m.selectedIDsForView() {
		selected[id] = true
	}

	var skills []actions.ActiveSkill
	seen := map[string]bool{}
	for _, group := range m.active {
		if selected[group.ID] {
			for _, skill := range group.Members {
				if action == "migrate" && skill.Status != actions.StatusUnmanaged {
					continue
				}
				key := skill.Path
				if action == "migrate" {
					if resolved, err := filepath.EvalSymlinks(skill.Path); err == nil {
						key = resolved
					}
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				skills = append(skills, skill)
			}
		}
	}
	return skills
}

func activeUsageTargets(targets []actions.ActiveSkill) []repoUsageTarget {
	usageTargets := make([]repoUsageTarget, 0, len(targets))
	for _, target := range targets {
		usageTargets = append(usageTargets, repoUsageTarget{
			Name:   filepath.Base(target.Path),
			Scope:  target.Root.Scope,
			Target: target.Root.Target,
			Chip:   rootLabel(target.Root),
			Path:   target.Path,
			Status: target.Status,
		})
	}
	return usageTargets
}

func selectedIndexes(count int) map[int]bool {
	selected := map[int]bool{}
	for i := 0; i < count; i++ {
		selected[i] = true
	}
	return selected
}

func activeUnlinkTitle(targets []actions.ActiveSkill) string {
	if len(targets) == 0 {
		return "selected skills"
	}
	return targets[0].Name
}

func (m *Model) applyActiveUsageUnlink(r repoUsageModal) {
	targets := selectedUsageTargets(r)
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink Results", []string{"No locations selected."})
		return
	}
	if usageTargetsContainUnmanaged(targets) {
		lines := []string{"Selected locations"}
		for _, target := range targets {
			lines = append(lines, "  "+target.Chip+"  "+target.Path)
		}
		choices := []string{"Copy selected unmanaged skills to repo, then unlink", "Unlink selected without copying to repo", "Cancel"}
		m.modal = newChoiceModal("Unlink active skills", lines, choices, 0, func(current *Model, choice int) {
			if choice == 2 {
				current.modal = nil
				return
			}
			current.applyUsageTargets(targets, choice == 1)
		})
		return
	}
	m.applyUsageTargets(targets, false)
}

type repoLinkModal struct {
	names     []string
	locations []roots.ActiveRoot
	index     int
}

func (m Model) selectedRepoSkillNames() []string {
	if m.view != ViewRepo {
		return nil
	}
	names := make([]string, 0)
	for _, id := range m.selectedIDsForView() {
		if name, ok := strings.CutPrefix(id, "repo:"); ok {
			names = append(names, name)
		}
	}
	return names
}

func (m *Model) toggleRepoRecommendations() tea.Cmd {
	if m.recommendationInFlight {
		return nil
	}
	names := m.selectedRepoSkillNames()
	if len(names) == 0 {
		return nil
	}
	m.recommendationToken++
	token := m.recommendationToken
	m.recommendationInFlight = true
	m.status = "updating project recommendations..."
	cfg := m.cfg
	queuedNames := slices.Clone(names)
	return func() tea.Msg {
		msg := recommendationResultMsg{token: token, names: queuedNames}
		recommended, err := manifest.LoadRecommended(cfg.ProjectRoot)
		if err == nil {
			recommendedNames := make(map[string]struct{}, len(recommended.Skills))
			for _, skill := range recommended.Skills {
				recommendedNames[skill.Name] = struct{}{}
			}
			recommendedCount := 0
			for _, name := range queuedNames {
				if _, ok := recommendedNames[name]; ok {
					recommendedCount++
				}
			}
			switch {
			case recommendedCount != 0 && recommendedCount != len(queuedNames):
				err = errors.New("select only recommended skills to remove, or only local skills to promote")
			case recommendedCount == len(queuedNames):
				err = manifest.Unrecommend(cfg, queuedNames)
			default:
				msg.promote = true
				err = manifest.Recommend(cfg, queuedNames)
			}
		}
		msg.err = err
		msg.active, msg.repo, msg.issues, msg.repoUsage, msg.reloadErr = loadTUIData(context.Background(), cfg)
		return msg
	}
}

func (m *Model) applyRecommendationResult(msg recommendationResultMsg) tea.Cmd {
	if msg.token != m.recommendationToken {
		return nil
	}
	m.recommendationInFlight = false
	if msg.reloadErr == nil {
		m.active = msg.active
		m.repo = msg.repo
		m.issues = msg.issues
		m.repoUsage = msg.repoUsage
		m.clampCursor()
	}
	if msg.err != nil {
		m.status = "project recommendation update failed: " + msg.err.Error()
		return nil
	}
	action := "Removed " + repoSelectionTitle(msg.names) + " from project recommendations"
	if msg.promote {
		action = "Promoted " + repoSelectionTitle(msg.names) + " to project recommendations"
	}
	if msg.reloadErr != nil {
		m.status = action + ", but refreshing the TUI failed: " + msg.reloadErr.Error()
		return nil
	}
	m.status = action
	return nil
}

func repoSelectionLabel(label string, count int) string {
	if count == 1 {
		return label
	}
	return label + "s"
}

func repoSelectionTitle(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%d selected repo skills", len(names))
}

func (m *Model) openRepoLinkModal() {
	names := m.selectedRepoSkillNames()
	if len(names) == 0 {
		return
	}
	linkModal := repoLinkModal{names: names, locations: roots.ActiveRoots(m.cfg, roots.Filter{})}
	m.modal = linkModal
}

func (r repoLinkModal) Title() string {
	return "Link repo skill"
}

func (r repoLinkModal) View(width, height int, m Model) string {
	lines := []string{
		repoSelectionLabel("Skill", len(r.names)),
	}
	for _, name := range r.names {
		lines = append(lines, "  "+name)
	}
	lines = append(lines,
		"",
		"Destination",
	)
	if len(r.locations) == 0 {
		lines = append(lines, "  no active roots configured")
	} else {
		for i, location := range r.locations {
			cursor := " "
			if i == r.index {
				cursor = m.symbols.Cursor
			}
			line := "  " + cursor + " " + linkChoice(i == r.index, rootLabel(location)+"  "+location.Scope+":"+location.Target)
			lines = append(lines, tuiui.TruncateANSI(line, width-10))
		}
	}
	lines = append(lines, "", "Will create")
	lines = append(lines, r.linkPreviewLines(m)...)
	lines = append(lines,
		"",
	)
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: "Link repo skill",
		Body:  lines,
		Footer: []string{tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "destination"},
			{ASCII: "enter", Unicode: "↵", Label: "link"},
			{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
		})},
		Focus: 3 + r.index,
	})
}

func (r repoLinkModal) linkPreviewLines(m Model) []string {
	location, ok := r.selectedLocation()
	if !ok {
		return []string{"  no active roots configured"}
	}
	lines := make([]string, 0, len(r.names))
	for _, name := range r.names {
		lines = append(lines, "  "+filepath.Join(location.Path, name)+" -> "+filepath.Join(m.cfg.ArchiveSkillsRoot(), name))
	}
	return lines
}

func linkChoice(selected bool, label string) string {
	if selected {
		return selectedBg.Render(selectedStyle.Render("● " + label))
	}
	return mutedStyle.Render("○ " + label)
}

func (r repoLinkModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		r.index = tuiui.ClampIndex(r.index+delta, len(r.locations))
		m.modal = r
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "enter":
		r.apply(m)
	}
	return false, nil
}

func (r repoLinkModal) apply(m *Model) {
	location, ok := r.selectedLocation()
	if !ok {
		m.modal = newResultModal("Link Results", []string{"No active roots configured."})
		return
	}
	var lines []string
	var successes []string
	for _, name := range r.names {
		result, err := actions.Link(m.cfg, actions.LinkRequest{Name: name, Scope: location.Scope, Target: location.Target})
		if err != nil {
			lines = append(lines, "x "+name+": "+err.Error())
			continue
		}
		successes = append(successes, "✓ "+result.Name+" linked")
	}
	if len(successes) > 0 && location.Scope == config.ScopeProject {
		m.mutationProjectTouched = true
		m.queueProjectReconciliation()
	}
	if !m.mutationInFlight {
		m.reload()
	}
	lines = append(successes, lines...)
	m.modal = newResultModal("Link Results", lines)
}

func (r repoLinkModal) selectedLocation() (roots.ActiveRoot, bool) {
	if r.index < 0 || r.index >= len(r.locations) {
		return roots.ActiveRoot{}, false
	}
	return r.locations[r.index], true
}

type repoUsageTarget struct {
	Name   string
	Scope  string
	Target string
	Chip   string
	Path   string
	Status string
}

type repoUsageModal struct {
	name     string
	targets  []repoUsageTarget
	selected map[int]bool
	index    int
	apply    func(*Model, repoUsageModal)
}

func (m *Model) openRepoUnlinkModal() {
	names := m.selectedRepoSkillNames()
	if len(names) == 0 {
		return
	}
	targets := m.repoUsageTargets(names...)
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink Results", []string{"No current usages for " + repoSelectionTitle(names) + "."})
		return
	}
	selected := map[int]bool{}
	for i := range targets {
		selected[i] = true
	}
	m.modal = repoUsageModal{name: repoSelectionTitle(names), targets: targets, selected: selected}
}

func (m Model) repoUsageTargets(names ...string) []repoUsageTarget {
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	var targets []repoUsageTarget
	for _, group := range m.active {
		for _, member := range group.Members {
			if selected[member.Name] && member.Status == actions.StatusManaged {
				targets = append(targets, repoUsageTarget{
					Name:   filepath.Base(member.Path),
					Scope:  member.Root.Scope,
					Target: member.Root.Target,
					Chip:   rootLabel(member.Root),
					Path:   member.Path,
					Status: member.Status,
				})
			}
		}
	}
	return targets
}

func (r repoUsageModal) Title() string {
	return "Unlink usages: " + r.name
}

func (r repoUsageModal) View(width, height int, m Model) string {
	body := []string{"Select current usages to remove.", ""}
	focus := 2 + r.index
	for i, target := range r.targets {
		cursor := " "
		if i == r.index {
			cursor = m.symbols.Cursor
		}
		check := m.symbols.Unchecked
		if r.selected[i] {
			check = m.symbols.Checked
		}
		var background lipgloss.TerminalColor = lipgloss.NoColor{}
		if r.selected[i] {
			background = selectedBg.GetBackground()
		}
		line := cursor + " " + selectedStyle.Render(check) + " " + renderRootChip(m.symbols, target.Chip, background) + "  " + target.Path
		if r.selected[i] {
			line = selectedBg.Render(line)
		}
		body = append(body, line)
	}
	return renderConstrainedModal(width, height, constrainedModalOptions{
		Title: "Unlink usages: " + r.name,
		Body:  body,
		Footer: []string{
			"[ Unlink selected ]   Cancel",
			tuiui.FooterLine(m.opts.ASCII, kbdStyle, mutedStyle, []tuiui.Shortcut{
				{ASCII: "up/down", Unicode: "↑/↓", Label: "move"},
				{ASCII: "space", Label: "toggle"},
				{ASCII: "enter", Unicode: "↵", Label: "choose"},
				{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
			}),
		},
		Focus: focus,
	})
}

func (r repoUsageModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		r.index = tuiui.ClampIndex(r.index+delta, len(r.targets))
		m.modal = r
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case " ":
		r.selected[r.index] = !r.selected[r.index]
		m.modal = r
	case "enter":
		if r.apply != nil {
			r.apply(m, r)
		} else {
			m.applyRepoUsageUnlink(r)
		}
	}
	return false, nil
}

func (m *Model) applyRepoUsageUnlink(r repoUsageModal) {
	m.applyUsageTargets(selectedUsageTargets(r), false)
}

func selectedUsageTargets(r repoUsageModal) []repoUsageTarget {
	var targets []repoUsageTarget
	for i, target := range r.targets {
		if r.selected[i] {
			targets = append(targets, target)
		}
	}
	return targets
}

func usageTargetsContainUnmanaged(targets []repoUsageTarget) bool {
	for _, target := range targets {
		if target.Status == actions.StatusUnmanaged {
			return true
		}
	}
	return false
}

func (m *Model) applyUsageTargets(targets []repoUsageTarget, deleteUnmanaged bool) {
	m.applyUsageTargetsWithResults(targets, deleteUnmanaged, actions.ConflictResolutionAsk, nil, nil)
}

func (m *Model) applyUsageTargetsWithResults(targets []repoUsageTarget, deleteUnmanaged bool, resolution string, successes, failures []string) {
	for i, target := range targets {
		result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{
			Name:               target.Name,
			Scope:              target.Scope,
			Target:             target.Target,
			Confirmed:          true,
			DeleteUnmanaged:    deleteUnmanaged,
			ConflictResolution: resolution,
		})
		if err != nil {
			var conflict *actions.ArchiveConflictError
			if errors.As(err, &conflict) {
				tail := append([]repoUsageTarget(nil), targets[i+1:]...)
				successesBeforeConflict := append([]string(nil), successes...)
				failuresBeforeConflict := append([]string(nil), failures...)
				m.openArchiveConflictModal(conflict, "Unlink Results", func(current *Model, chosen string) {
					current.applyResolvedUsageConflict(target, tail, deleteUnmanaged, chosen, successesBeforeConflict, failuresBeforeConflict)
				})
				return
			}
			failures = append(failures, "x "+target.Path+": "+err.Error())
			continue
		}
		successes = append(successes, "✓ "+result.Name+"  "+result.Status)
		m.mutationProjectTouched = m.mutationProjectTouched || target.Scope == config.ScopeProject
	}
	m.finishUsageTargets(successes, failures)
}

func (m *Model) applyResolvedUsageConflict(target repoUsageTarget, remaining []repoUsageTarget, deleteUnmanaged bool, resolution string, successes, failures []string) {
	result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{
		Name:               target.Name,
		Scope:              target.Scope,
		Target:             target.Target,
		Confirmed:          true,
		DeleteUnmanaged:    deleteUnmanaged,
		ConflictResolution: resolution,
	})
	if err != nil {
		failures = append(failures, "x "+target.Path+": "+err.Error())
	} else {
		successes = append(successes, "✓ "+result.Name+"  "+result.Status)
		m.mutationProjectTouched = m.mutationProjectTouched || target.Scope == config.ScopeProject
	}
	m.applyUsageTargetsWithResults(remaining, deleteUnmanaged, actions.ConflictResolutionAsk, successes, failures)
}

func (m *Model) finishUsageTargets(successes, failures []string) {
	if !m.queueProjectReconciliation() {
		m.reload()
	}
	if len(failures) == 0 {
		m.modal = nil
		m.status = unlinkSuccessStatus(successes)
		return
	}
	lines := append(successes, failures...)
	m.modal = newResultModal("Unlink Results", lines)
}

func unlinkSuccessStatus(successes []string) string {
	return mutationSuccessStatus(successes, "unlinked")
}

func mutationSuccessStatus(successes []string, verb string) string {
	switch len(successes) {
	case 0:
		return "no skills " + verb
	case 1:
		return strings.TrimPrefix(successes[0], "✓ ")
	default:
		return fmt.Sprintf("%s %d locations", verb, len(successes))
	}
}

func (m *Model) openRepoDeleteModal() {
	names := m.selectedRepoSkillNames()
	if len(names) == 0 {
		return
	}
	usageTargets := m.repoUsageTargets(names...)
	lines := repoDeleteIntroLines(names, len(usageTargets) > 0)
	if len(usageTargets) > 0 {
		lines = append(lines, "", "Visible usages")
		for _, target := range usageTargets {
			lines = append(lines, "  "+target.Chip+"  "+target.Path)
		}
		lines = append(lines, "", "Scope limit", "  Only current project roots and global roots are known. Other projects may need x-skills doctor afterwards.")
	}
	m.modal = newChoiceModal(repoDeleteTitle(names), lines, []string{"Cancel", repoDeleteActionLabel(names, len(usageTargets) > 0)}, 0, func(current *Model, choice int) {
		if choice == 0 {
			current.modal = nil
			return
		}
		current.applyRepoDeleteNames(names)
	})
}

func repoDeleteTitle(names []string) string {
	if len(names) == 1 {
		return "Delete archive: " + names[0]
	}
	return "Delete archives: " + repoSelectionTitle(names)
}

func repoDeleteIntroLines(names []string, hasVisibleUsages bool) []string {
	lines := []string{}
	if len(names) > 1 {
		lines = append(lines, "Selected archives")
		for _, name := range names {
			lines = append(lines, "  "+name)
		}
		lines = append(lines, "")
	}
	switch {
	case hasVisibleUsages && len(names) == 1:
		lines = append(lines, "This archive is used in the current working set.")
	case hasVisibleUsages:
		lines = append(lines, "One or more selected archives are used in the current working set.")
	default:
		lines = append(lines, "No visible usages in the current working set.")
	}
	return lines
}

func repoDeleteActionLabel(names []string, hasVisibleUsages bool) string {
	noun := "archive"
	if len(names) > 1 {
		noun = "archives"
	}
	if !hasVisibleUsages {
		return "Delete " + noun
	}
	return "Unlink visible usages, then delete " + noun
}

func (m *Model) applyRepoDelete(name string) {
	m.applyRepoDeleteNames([]string{name})
}

func (m *Model) applyRepoDeleteNames(names []string) {
	var lines []string
	for _, name := range names {
		lines = append(lines, m.applySingleRepoDelete(name)...)
	}
	if !m.mutationInFlight {
		m.reload()
	}
	m.modal = newResultModal("Delete Results", lines)
}

func (m *Model) applySingleRepoDelete(name string) []string {
	var lines []string
	hasUnlinkError := false
	projectUnlinked := false
	defer func() {
		m.mutationProjectTouched = m.mutationProjectTouched || projectUnlinked
		m.queueProjectReconciliation()
	}()
	for _, target := range m.repoUsageTargets(name) {
		_, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: target.Name, Scope: target.Scope, Target: target.Target, Confirmed: true})
		if err != nil {
			hasUnlinkError = true
			lines = append(lines, "x unlink "+target.Path+": "+err.Error())
			continue
		}
		projectUnlinked = projectUnlinked || target.Scope == config.ScopeProject
	}
	if hasUnlinkError {
		lines = append(lines, "x delete "+name+": skipped because unlink failed")
		return lines
	}
	archivePath, err := repo.DeleteSkill(m.cfg, name)
	if err != nil {
		if archivePath == "" {
			lines = append(lines, "x delete "+name+": "+err.Error())
		} else {
			lines = append(lines, "x delete "+archivePath+": "+err.Error())
		}
	} else {
		lines = append(lines, "✓ deleted "+name)
	}
	return lines
}

func (m *Model) openDoctorFixModal() {
	if len(m.issues) == 0 {
		m.modal = newResultModal("Doctor Results", []string{"No doctor issues."})
		return
	}
	brokenCount := 0
	builtInCount := 0
	for _, issue := range m.issues {
		if issue.Kind == doctor.KindBrokenSymlink {
			brokenCount++
		} else if issue.Kind == doctor.KindMissingBuiltIn || issue.Kind == doctor.KindInactiveBuiltIn {
			builtInCount++
		}
	}
	if builtInCount > 0 {
		m.modal = newDoctorBuiltInFixModal(m.cfg, brokenCount, builtInCount)
		return
	}
	lines := []string{
		fmt.Sprintf("Apply %d Doctor fixes?", len(m.issues)),
		"",
		fmt.Sprintf("  - %d broken symlink issues", brokenCount),
	}
	m.modal = newConfirmModal("Confirm", lines, false, func(current *Model) {
		current.cancelDoctorFixWork()
		ctx, cancel := context.WithCancel(context.Background())
		current.doctorFixCancel = cancel
		current.doctorFixInFlight = true
		defer current.cancelDoctorFixWork()
		results, err := doctor.FixIssues(ctx, current.issues)
		var output []string
		for _, result := range results {
			output = append(output, "✓ "+result.Name+"  "+result.Action)
		}
		if err != nil {
			output = append(output, "x "+err.Error())
		}
		current.reload()
		current.modal = newResultModal("Doctor Results", output)
	})
}

type doctorBuiltInFixModal struct {
	destinations []roots.ActiveRoot
	checked      map[int]bool
	cursor       int
	brokenCount  int
	builtInCount int
}

func newDoctorBuiltInFixModal(cfg config.Config, brokenCount, builtInCount int) modal {
	destinations := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal})
	checked := map[int]bool{}
	for i, destination := range destinations {
		if destination.Target == config.TargetAgents {
			checked[i] = true
			break
		}
	}
	return doctorBuiltInFixModal{destinations: destinations, checked: checked, brokenCount: brokenCount, builtInCount: builtInCount}
}

func (d doctorBuiltInFixModal) Title() string { return "Doctor fixes" }

func (d doctorBuiltInFixModal) View(width, height int, m Model) string {
	lines := []string{
		fmt.Sprintf("  - %d broken symlink issues", d.brokenCount),
		fmt.Sprintf("  - %d built-in skill issues", d.builtInCount),
		"",
		"Built-in skills",
	}
	for i, destination := range d.destinations {
		lines = append(lines, d.checkboxLine(i, rootLabel(destination), m))
	}
	lines = append(lines, d.checkboxLine(len(d.destinations), "Archive only", m), "", "Press Enter to apply")
	return renderConstrainedModal(width, height, constrainedModalOptions{Title: d.Title(), Body: lines})
}

func (d doctorBuiltInFixModal) checkboxLine(index int, label string, m Model) string {
	cursor := "  "
	if index == d.cursor {
		cursor = m.symbols.Cursor + " "
	}
	mark := "[ ]"
	if d.checked[index] {
		mark = "[x]"
	}
	return cursor + mark + " " + label
}

func (d doctorBuiltInFixModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if delta := modalMoveDelta(msg); delta != 0 {
		d.cursor = tuiui.ClampIndex(d.cursor+delta, len(d.destinations)+1)
		m.modal = d
		return false, nil
	}
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case " ":
		d.toggle()
		m.modal = d
	case "enter":
		return false, d.apply(m)
	}
	return false, nil
}

func (d *doctorBuiltInFixModal) toggle() {
	archiveOnly := len(d.destinations)
	if d.cursor == archiveOnly {
		d.checked = map[int]bool{archiveOnly: !d.checked[archiveOnly]}
		return
	}
	delete(d.checked, archiveOnly)
	d.checked[d.cursor] = !d.checked[d.cursor]
}

type doctorFixResultMsg struct {
	token     int
	results   []doctor.FixResult
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
}

func (d doctorBuiltInFixModal) apply(m *Model) tea.Cmd {
	destinations := make([]roots.ActiveRoot, 0, len(d.destinations))
	for i, destination := range d.destinations {
		if d.checked[i] {
			destinations = append(destinations, destination)
		}
	}
	if len(destinations) == 0 && !d.checked[len(d.destinations)] {
		m.status = "select at least one global Skills Folder or Archive only"
		m.modal = d
		return nil
	}
	m.cancelDoctorFixWork()
	m.doctorFixToken++
	token := m.doctorFixToken
	ctx, cancel := context.WithCancel(context.Background())
	m.doctorFixCancel = cancel
	m.doctorFixInFlight = true
	m.status = "applying Doctor fixes..."
	m.modal = nil
	cfg := m.cfg
	issues := append([]doctor.Issue(nil), m.issues...)
	archiveOnly := d.checked[len(d.destinations)]
	return func() tea.Msg {
		results, err := doctor.FixIssues(ctx, issues)
		builtInResults, builtInErr := doctor.FixBuiltIns(ctx, cfg, issues, doctor.FixOptions{
			BuiltInDestinations: destinations,
			ArchiveOnlyBuiltIns: archiveOnly,
		})
		results = append(results, builtInResults...)
		if err == nil {
			err = builtInErr
		}
		active, repoSkills, currentIssues, repoUsage, reloadErr := loadTUIData(ctx, cfg)
		if err == nil {
			err = reloadErr
		}
		return doctorFixResultMsg{token: token, results: results, active: active, repo: repoSkills, issues: currentIssues, repoUsage: repoUsage, err: err}
	}
}

func (m *Model) applyDoctorFixResult(msg doctorFixResultMsg) tea.Cmd {
	if msg.token != m.doctorFixToken {
		return nil
	}
	if m.doctorFixCancel != nil {
		m.doctorFixCancel()
	}
	m.doctorFixInFlight = false
	m.doctorFixCancel = nil
	m.active = msg.active
	m.repo = msg.repo
	m.issues = msg.issues
	m.repoUsage = msg.repoUsage
	m.err = msg.err
	m.clampCursor()
	output := make([]string, 0, len(msg.results)+1)
	for _, result := range msg.results {
		output = append(output, "✓ "+result.Name+"  "+result.Action)
	}
	if msg.err != nil {
		output = append(output, "x "+msg.err.Error())
	}
	m.modal = newResultModal("Doctor Results", output)
	m.status = "Doctor fixes complete"
	return nil
}

func (m *Model) cancelDoctorFixWork() {
	if m.doctorFixCancel != nil {
		m.doctorFixCancel()
		m.doctorFixCancel = nil
	}
	m.doctorFixInFlight = false
}
