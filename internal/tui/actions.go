package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
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
		lines = append(lines, "  "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target))
	}
	lines = append(lines, "", "Plan", "  1. Compare active content with archive", "  2. If identical, relink active copies", "  3. If different, review full-file diff")
	m.modal = newConfirmModal("Migrate active skills", lines, false, func(current *Model) {
		current.applyMigrateTargets(targets, actions.ConflictResolutionAsk)
	})
}

func (m *Model) applyMigrateTargets(targets []actions.ActiveSkill, resolution string) {
	var lines []string
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
		lines = append(lines, "✓ "+result.Name+"  "+result.Status)
		m.status = result.Status + " " + result.Name
	}
	m.reload()
	m.modal = newResultModal("Migration Results", lines)
}

func (m *Model) openUnlinkModal() {
	targets := m.selectedActiveSkills("unlink")
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink active skills", []string{"No active skills selected."})
		return
	}
	if allActiveTargetsAlreadyArchived(targets) {
		m.modal = repoUsageModal{name: activeUnlinkTitle(targets), targets: activeUsageTargets(targets), selected: selectedIndexes(len(targets))}
		return
	}
	lines := []string{"Managed links"}
	for _, target := range targets {
		if target.Status == actions.StatusManaged {
			lines = append(lines, "  ✓ "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target)+"  remove symlink only")
		}
	}
	lines = append(lines, "", "Broken links")
	for _, target := range targets {
		if target.Status == actions.StatusBroken {
			lines = append(lines, "  ▲ "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target)+"  remove broken symlink")
		}
	}
	lines = append(lines, "", "Unmanaged directories")
	for _, target := range targets {
		if target.Status == actions.StatusUnmanaged {
			lines = append(lines, "  ◆ "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target))
		}
	}
	choices := []string{"Migrate to repo, then unlink active copies", "Delete active copies without archiving", "Cancel"}
	m.modal = newChoiceModal("Unlink active skills", lines, choices, 0, func(current *Model, choice int) {
		if choice == 2 {
			current.modal = nil
			return
		}
		current.applyUnlinkTargets(targets, choice == 1)
	})
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

func allActiveTargetsAlreadyArchived(targets []actions.ActiveSkill) bool {
	for _, target := range targets {
		if target.Status == actions.StatusUnmanaged {
			return false
		}
	}
	return true
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

func (m *Model) applyUnlinkTargets(targets []actions.ActiveSkill, deleteUnmanaged bool) {
	var lines []string
	for _, skill := range targets {
		result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{
			Name:            filepath.Base(skill.Path),
			Scope:           skill.Root.Scope,
			Target:          skill.Root.Target,
			Confirmed:       true,
			DeleteUnmanaged: deleteUnmanaged,
		})
		if err != nil {
			lines = append(lines, "x "+filepath.Base(skill.Path)+"  "+err.Error())
			continue
		}
		lines = append(lines, "✓ "+result.Name+"  "+result.Status)
	}
	m.reload()
	m.modal = newResultModal("Unlink Results", lines)
}

type repoLinkModal struct {
	name        string
	scope       string
	target      string
	field       int
	destination string
}

func (m *Model) currentRepoSkillName() (string, bool) {
	skills := m.visibleRepoSkills()
	if m.view != ViewRepo || m.cursor < 0 || m.cursor >= len(skills) {
		return "", false
	}
	return skills[m.cursor].Name, true
}

func (m *Model) openRepoLinkModal() {
	name, ok := m.currentRepoSkillName()
	if !ok {
		return
	}
	linkModal := repoLinkModal{name: name, scope: config.ScopeProject, target: config.TargetAgents}
	linkModal.destination = linkModal.destinationPath(m)
	m.modal = linkModal
}

func (r repoLinkModal) Title() string {
	return "Link repo skill"
}

func (r repoLinkModal) destinationPath(m *Model) string {
	root, err := m.cfg.ActiveRoot(r.scope, r.target)
	if err != nil {
		return err.Error()
	}
	return filepath.Join(root, r.name)
}

func (r repoLinkModal) View(width, height int, m Model) string {
	projectCursor := " "
	globalCursor := " "
	agentsCursor := " "
	claudeCursor := " "
	codexCursor := " "
	if r.field == 0 && r.scope == config.ScopeProject {
		projectCursor = m.symbols.Cursor
	}
	if r.field == 0 && r.scope == config.ScopeGlobal {
		globalCursor = m.symbols.Cursor
	}
	if r.field == 1 && r.target == config.TargetAgents {
		agentsCursor = m.symbols.Cursor
	}
	if r.field == 1 && r.target == config.TargetClaude {
		claudeCursor = m.symbols.Cursor
	}
	if r.field == 1 && r.target == config.TargetCodex {
		codexCursor = m.symbols.Cursor
	}
	lines := []string{
		accentStyle.Render("Link repo skill"),
		"Skill",
		"  " + r.name,
		"",
		"Destination",
		"  scope   " + projectCursor + " project    " + globalCursor + " global",
		"  target  " + agentsCursor + " .Ag        " + claudeCursor + " .Cl        " + codexCursor + " .Cd",
		"",
		"Will create",
		"  " + r.destination + " -> " + filepath.Join(m.cfg.ArchiveSkillsRoot(), r.name),
		"",
		mutedStyle.Render("left/right change option   tab field   enter link   esc cancel"),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
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
	if msg.Type == tea.KeyEnter {
		r.apply(m)
	}
	return false, nil
}

func (r repoLinkModal) apply(m *Model) {
	result, err := actions.Link(m.cfg, actions.LinkRequest{Name: r.name, Scope: r.scope, Target: r.target})
	if err != nil {
		m.modal = newResultModal("Link Results", []string{"x " + err.Error()})
		return
	}
	m.reload()
	m.modal = newResultModal("Link Results", []string{"✓ " + result.Name + " linked"})
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
}

type repoUsageModal struct {
	name     string
	targets  []repoUsageTarget
	selected map[int]bool
	index    int
}

func (m *Model) openRepoUnlinkModal() {
	name, ok := m.currentRepoSkillName()
	if !ok {
		return
	}
	targets := m.repoUsageTargets(name)
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink Results", []string{"No current usages for " + name + "."})
		return
	}
	selected := map[int]bool{}
	for i := range targets {
		selected[i] = true
	}
	m.modal = repoUsageModal{name: name, targets: targets, selected: selected}
}

func (m Model) repoUsageTargets(name string) []repoUsageTarget {
	var targets []repoUsageTarget
	for _, group := range m.active {
		for _, member := range group.Members {
			if member.Name == name && member.Status == actions.StatusManaged {
				targets = append(targets, repoUsageTarget{
					Name:   filepath.Base(member.Path),
					Scope:  member.Root.Scope,
					Target: member.Root.Target,
					Chip:   rootChip(member.Root.Scope, member.Root.Target),
					Path:   member.Path,
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
		lines = append(lines, cursor+" "+check+" "+target.Chip+"  "+target.Path)
	}
	lines = append(lines, "", "[ Unlink selected ]   Cancel", mutedStyle.Render("up/down move   space toggle   enter choose   esc cancel"))
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
		m.applyRepoUsageUnlink(r)
	}
	return false, nil
}

func (m *Model) applyRepoUsageUnlink(r repoUsageModal) {
	var lines []string
	for i, target := range r.targets {
		if !r.selected[i] {
			continue
		}
		result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: target.Name, Scope: target.Scope, Target: target.Target, Confirmed: true})
		if err != nil {
			lines = append(lines, "x "+target.Path+": "+err.Error())
			continue
		}
		lines = append(lines, "✓ "+result.Name+"  "+result.Status)
	}
	m.reload()
	m.modal = newResultModal("Unlink Results", lines)
}

func (m *Model) openRepoDeleteModal() {
	name, ok := m.currentRepoSkillName()
	if !ok {
		return
	}
	lines := []string{"This archive is used in the current working set.", "", "Visible usages"}
	for _, target := range m.repoUsageTargets(name) {
		lines = append(lines, "  "+target.Chip+"  "+target.Path)
	}
	lines = append(lines, "", "Scope limit", "  Only current project roots and global roots are known. Other projects may need x-skills doctor afterwards.")
	m.modal = newChoiceModal("Delete archive: "+name, lines, []string{"Cancel", "Unlink visible usages, then delete archive"}, 0, func(current *Model, choice int) {
		if choice == 0 {
			current.modal = nil
			return
		}
		current.applyRepoDelete(name)
	})
}

func (m *Model) applyRepoDelete(name string) {
	var lines []string
	for _, target := range m.repoUsageTargets(name) {
		_, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: target.Name, Scope: target.Scope, Target: target.Target, Confirmed: true})
		if err != nil {
			lines = append(lines, "x unlink "+target.Path+": "+err.Error())
		}
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
	m.reload()
	m.modal = newResultModal("Delete Results", lines)
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
