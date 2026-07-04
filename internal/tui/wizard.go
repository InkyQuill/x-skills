package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
)

type WizardAction string

const (
	ActionInstall   WizardAction = "install"
	ActionMigrate   WizardAction = "migrate"
	ActionUnlink    WizardAction = "unlink"
	ActionFixDoctor WizardAction = "fix"
)

type Wizard struct {
	Open    bool
	Action  WizardAction
	Preview string

	RepoNames []string
	Active    []actions.ActiveSkill
	Issues    []doctor.Issue

	DestinationScope  string
	DestinationTarget string
}

func (m *Model) openWizard(action WizardAction) {
	wizard := Wizard{
		Open:              true,
		Action:            action,
		DestinationScope:  config.ScopeProject,
		DestinationTarget: config.TargetAgents,
	}

	switch action {
	case ActionInstall:
		wizard.RepoNames = m.selectedRepoNames()
	case ActionMigrate, ActionUnlink:
		wizard.Active = m.selectedActiveSkills()
	case ActionFixDoctor:
		wizard.Issues = m.selectedIssues()
	}

	wizard.Preview = buildPreview(m.cfg, wizard)
	m.wizard = wizard
}

func (m Model) selectedRepoNames() []string {
	selected := map[string]bool{}
	for _, id := range m.selectedIDsForView() {
		selected[strings.TrimPrefix(id, "repo:")] = true
	}

	var names []string
	for _, skill := range m.repo {
		if selected[skill.Name] {
			names = append(names, skill.Name)
		}
	}
	return names
}

func (m Model) selectedActiveSkills() []actions.ActiveSkill {
	selected := map[string]bool{}
	for _, id := range m.selectedIDsForView() {
		selected[id] = true
	}

	var skills []actions.ActiveSkill
	for _, group := range m.active {
		if selected[group.ID] {
			skills = append(skills, group.Members...)
		}
	}
	return skills
}

func (m Model) selectedIssues() []doctor.Issue {
	selected := map[string]bool{}
	for _, id := range m.selectedIDsForView() {
		selected[id] = true
	}

	var issues []doctor.Issue
	for _, issue := range m.issues {
		if selected[issueID(issue)] {
			issues = append(issues, issue)
		}
	}
	return issues
}

func buildPreview(cfg config.Config, wizard Wizard) string {
	switch wizard.Action {
	case ActionInstall:
		destination := config.LocationLabel(wizard.DestinationScope, wizard.DestinationTarget)
		if len(wizard.RepoNames) == 0 {
			return "No repo skills selected."
		}
		return fmt.Sprintf("Install %s to %s\n%s", strings.Join(wizard.RepoNames, ", "), destination, cfg.ActiveRoot(wizard.DestinationScope, wizard.DestinationTarget))
	case ActionMigrate:
		if len(wizard.Active) == 0 {
			return "No active skills selected."
		}
		return fmt.Sprintf("Migrate %d active skill instance(s) into %s and link back.", len(wizard.Active), cfg.ArchiveSkillsRoot())
	case ActionUnlink:
		if len(wizard.Active) == 0 {
			return "No active skills selected."
		}
		return fmt.Sprintf("Unlink %d active skill instance(s). Unmanaged directories migrate into repo first.", len(wizard.Active))
	case ActionFixDoctor:
		if len(wizard.Issues) == 0 {
			return "No doctor issues selected."
		}
		return fmt.Sprintf("Apply safe doctor fixes for %d issue(s). Broken symlinks with repo matches are relinked; others are removed.", len(wizard.Issues))
	default:
		return "Unknown action."
	}
}

func applyWizard(cfg config.Config, wizard Wizard) ([]string, error) {
	var results []string
	switch wizard.Action {
	case ActionInstall:
		for _, name := range wizard.RepoNames {
			result, err := actions.Link(cfg, actions.LinkRequest{
				Name:   name,
				Scope:  wizard.DestinationScope,
				Target: wizard.DestinationTarget,
			})
			if err != nil {
				return results, err
			}
			results = append(results, result.Name)
		}
	case ActionMigrate:
		for _, skill := range wizard.Active {
			result, err := actions.Migrate(cfg, actions.MigrateRequest{
				Name:      filepath.Base(skill.Path),
				Scope:     skill.Root.Scope,
				Target:    skill.Root.Target,
				Confirmed: true,
			})
			if err != nil {
				return results, err
			}
			results = append(results, result.Name)
		}
	case ActionUnlink:
		for _, skill := range wizard.Active {
			result, err := actions.Unlink(cfg, actions.UnlinkRequest{
				Name:      filepath.Base(skill.Path),
				Scope:     skill.Root.Scope,
				Target:    skill.Root.Target,
				Confirmed: true,
			})
			if err != nil {
				return results, err
			}
			results = append(results, result.Name)
		}
	case ActionFixDoctor:
		fixResults, err := doctor.FixIssues(wizard.Issues)
		for _, result := range fixResults {
			results = append(results, result.Name)
		}
		if err != nil {
			return results, err
		}
	}
	return results, nil
}
