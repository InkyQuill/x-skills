package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type filterState struct {
	Active bool
	Query  string
}

func (f filterState) matches(values ...string) bool {
	query := strings.TrimSpace(strings.ToLower(f.Query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (f *filterState) update(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		f.Active = false
		f.Query = ""
		return true
	case "enter":
		f.Active = false
		return true
	case "backspace":
		if len(f.Query) > 0 {
			f.Query = f.Query[:len(f.Query)-1]
		}
		return true
	}
	if len(msg.Runes) > 0 {
		f.Query += string(msg.Runes)
		return true
	}
	return true
}

func (m Model) visibleActiveGroups() []ActiveGroup {
	if strings.TrimSpace(m.filter.Query) == "" {
		return m.active
	}
	var groups []ActiveGroup
	for _, group := range m.active {
		if m.filter.matches(group.Name, group.Description, group.Status, strings.Join(group.Chips, " "), strings.Join(group.Aliases, " ")) {
			groups = append(groups, group)
		}
	}
	return groups
}

func (m Model) visibleRepoSkills() []repoSkillView {
	var skills []repoSkillView
	for _, skill := range m.repo {
		if m.filter.matches(skill.Name, skill.Description, strings.Join(m.repoUsage[skill.Name], " ")) {
			skills = append(skills, repoSkillView{Name: skill.Name, Description: skill.Description})
		}
	}
	return skills
}

type repoSkillView struct {
	Name        string
	Description string
}
