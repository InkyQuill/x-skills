package tui

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/actions"
)

func (m *Model) activeTargets() []actions.ActiveSkill {
	return m.selectedActiveSkills(ActionMigrate)
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
	targets := m.selectedActiveSkills(ActionUnlink)
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink active skills", []string{"No active skills selected."})
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
