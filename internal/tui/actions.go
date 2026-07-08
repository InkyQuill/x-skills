package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
	tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"
)

func (m *Model) activeTargets() []actions.ActiveSkill {
	return m.selectedActiveSkills("migrate")
}

func (m *Model) openMigrateModal() {
	targets := m.activeTargets()
	if len(targets) == 0 {
		m.modal = newResultModal("Migrate active skills", []string{"No unmanaged active skill directories selected."})
		return
	}
	lines := []string{"Targets"}
	for _, target := range targets {
		lines = append(lines, "  "+filepath.Base(target.Path)+"  "+renderRootChip(m.symbols, rootChip(target.Root.Scope, target.Root.Target), lipgloss.NoColor{}))
	}
	lines = append(lines, "", "Plan", "  1. Compare active content with archive", "  2. If identical, relink active copies", "  3. If different, review full-file diff")
	m.modal = newConfirmModal("Migrate active skills", lines, false, func(current *Model) {
		current.applyMigrateTargets(targets, actions.ConflictResolutionAsk)
	})
}

func (m *Model) applyMigrateTargets(targets []actions.ActiveSkill, resolution string) {
	var lines []string
	var successes []string
	for _, skill := range targets {
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
				diff, diffErr := buildDirectoryDiff(conflict.ActivePath, conflict.ArchivedPath)
				if diffErr != nil {
					m.modal = newResultModal("Migration Results", []string{fmt.Sprintf("failed to build conflict diff: %v", diffErr)})
					return
				}
				m.modal = newConflictDiffModal(conflict.Name, diff, func(chosen string) {
					m.applyMigrateTargets([]actions.ActiveSkill{skill}, chosen)
				})
				return
			}
			lines = append(lines, "x "+filepath.Base(skill.Path)+"  "+err.Error())
			continue
		}
		successes = append(successes, "✓ "+result.Name+"  "+result.Status)
	}
	m.reload()
	if len(lines) == 0 {
		m.modal = nil
		m.status = mutationSuccessStatus(successes, "migrated")
		return
	}
	lines = append(successes, lines...)
	m.modal = newResultModal("Migration Results", lines)
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
			Chip:   rootChip(target.Root.Scope, target.Root.Target),
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
	names       []string
	scope       string
	target      string
	field       int
	destination string
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
	linkModal := repoLinkModal{names: names, scope: config.ScopeProject, target: config.TargetAgents}
	linkModal.destination = linkModal.destinationPath(m)
	m.modal = linkModal
}

func (r repoLinkModal) Title() string {
	return "Link repo skill"
}

func (r repoLinkModal) destinationPath(m *Model) string {
	if len(r.names) != 1 {
		return ""
	}
	root, err := m.cfg.ActiveRoot(r.scope, r.target)
	if err != nil {
		return err.Error()
	}
	return filepath.Join(root, r.names[0])
}

func (r repoLinkModal) View(width, height int, m Model) string {
	scopeCursor := " "
	targetCursor := " "
	if r.field == 0 {
		scopeCursor = m.symbols.Cursor
	}
	if r.field == 1 {
		targetCursor = m.symbols.Cursor
	}
	lines := []string{
		accentStyle.Render("Link repo skill"),
		repoSelectionLabel("Skill", len(r.names)),
	}
	for _, name := range r.names {
		lines = append(lines, "  "+name)
	}
	lines = append(lines,
		"",
		"Destination",
		"  "+scopeCursor+" scope   "+linkChoice(r.scope == config.ScopeProject, "project")+"    "+linkChoice(r.scope == config.ScopeGlobal, "global"),
		"  "+targetCursor+" target  "+linkChoice(r.target == config.TargetAgents, ".Ag")+"        "+linkChoice(r.target == config.TargetClaude, ".Cl")+"        "+linkChoice(r.target == config.TargetCodex, ".Cd"),
		"",
		"Will create",
	)
	lines = append(lines, r.linkPreviewLines(m)...)
	lines = append(lines,
		"",
		mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "left/right", Unicode: "←/→", Label: "change option"},
			{ASCII: "tab", Unicode: "⇥", Label: "field"},
			{ASCII: "enter", Unicode: "↵", Label: "link"},
			{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
		})),
	)
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (r repoLinkModal) linkPreviewLines(m Model) []string {
	root, err := m.cfg.ActiveRoot(r.scope, r.target)
	if err != nil {
		return []string{"  " + err.Error()}
	}
	lines := make([]string, 0, len(r.names))
	for _, name := range r.names {
		lines = append(lines, "  "+filepath.Join(root, name)+" -> "+filepath.Join(m.cfg.ArchiveSkillsRoot(), name))
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
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "tab":
		if r.field == 0 {
			r.field = 1
		} else {
			r.field = 0
		}
		r.destination = r.destinationPath(m)
		m.modal = r
	case "left", "right":
		r.move(msg.String())
		r.destination = r.destinationPath(m)
		m.modal = r
	case "enter":
		r.apply(m)
	}
	return false, nil
}

func (r repoLinkModal) apply(m *Model) {
	var lines []string
	var successes []string
	for _, name := range r.names {
		result, err := actions.Link(m.cfg, actions.LinkRequest{Name: name, Scope: r.scope, Target: r.target})
		if err != nil {
			lines = append(lines, "x "+name+": "+err.Error())
			continue
		}
		successes = append(successes, "✓ "+result.Name+" linked")
	}
	m.reload()
	lines = append(successes, lines...)
	m.modal = newResultModal("Link Results", lines)
}

func (r *repoLinkModal) move(direction string) {
	if r.field == 0 {
		if r.scope == config.ScopeProject {
			r.scope = config.ScopeGlobal
		} else {
			r.scope = config.ScopeProject
		}
		return
	}
	targets := []string{config.TargetAgents, config.TargetClaude, config.TargetCodex}
	current := 0
	for i, target := range targets {
		if target == r.target {
			current = i
			break
		}
	}
	if direction == "right" {
		current = (current + 1) % len(targets)
	} else {
		current = (current + len(targets) - 1) % len(targets)
	}
	r.target = targets[current]
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
					Chip:   rootChip(member.Root.Scope, member.Root.Target),
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
	lines := []string{
		accentStyle.Render("Unlink usages: " + r.name),
		"Select current usages to remove.",
		"",
	}
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
		lines = append(lines, line)
	}
	lines = append(lines, "", "[ Unlink selected ]   Cancel", mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
		{ASCII: "up/down", Unicode: "↑/↓", Label: "move"},
		{ASCII: "space", Label: "toggle"},
		{ASCII: "enter", Unicode: "↵", Label: "choose"},
		{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
	})))
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (r repoUsageModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up":
		if r.index > 0 {
			r.index--
		}
		m.modal = r
	case "down":
		if r.index+1 < len(r.targets) {
			r.index++
		}
		m.modal = r
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
	var lines []string
	var successes []string
	for _, target := range targets {
		result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: target.Name, Scope: target.Scope, Target: target.Target, Confirmed: true, DeleteUnmanaged: deleteUnmanaged})
		if err != nil {
			lines = append(lines, "x "+target.Path+": "+err.Error())
			continue
		}
		successes = append(successes, "✓ "+result.Name+"  "+result.Status)
	}
	m.reload()
	if len(lines) == 0 {
		m.modal = nil
		m.status = unlinkSuccessStatus(successes)
		return
	}
	lines = append(successes, lines...)
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
	m.reload()
	m.modal = newResultModal("Delete Results", lines)
}

func (m *Model) applySingleRepoDelete(name string) []string {
	var lines []string
	hasUnlinkError := false
	for _, target := range m.repoUsageTargets(name) {
		_, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: target.Name, Scope: target.Scope, Target: target.Target, Confirmed: true})
		if err != nil {
			hasUnlinkError = true
			lines = append(lines, "x unlink "+target.Path+": "+err.Error())
		}
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
	for _, issue := range m.issues {
		if issue.Kind == doctor.KindBrokenSymlink {
			brokenCount++
		}
	}
	lines := []string{
		fmt.Sprintf("Apply %d Doctor fixes?", len(m.issues)),
		"",
		fmt.Sprintf("  - %d broken symlink issues", brokenCount),
	}
	m.modal = newConfirmModal("Confirm", lines, false, func(current *Model) {
		results, err := doctor.FixIssues(current.issues)
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
