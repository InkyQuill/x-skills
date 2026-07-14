package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type filterState struct {
	Active bool
	Query  string
	input  textinput.Model
}

func newFilterState() filterState {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 200
	return filterState{input: input}
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

func (f *filterState) update(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		f.Active = false
		f.Query = ""
		f.input.SetValue("")
		return true, nil
	case "enter":
		f.Active = false
		return true, nil
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	f.Query = f.input.Value()
	return true, cmd
}

func (m Model) visibleActiveGroups() []ActiveGroup {
	if strings.TrimSpace(m.filter.Query) == "" {
		return m.active
	}
	var groups []ActiveGroup
	for _, group := range m.active {
		if m.filter.matches(group.Identity, group.DeclaredName, group.Description, group.Status, strings.Join(group.Chips, " "), strings.Join(group.Aliases, " ")) {
			groups = append(groups, group)
		}
	}
	return groups
}

func (m Model) visibleRepoSkills() []repoSkillView {
	var skills []repoSkillView
	for _, skill := range m.repo {
		if m.filter.matches(skill.Identity, skill.DeclaredName, skill.Description, strings.Join(m.repoUsage[skill.Identity], " ")) {
			skills = append(skills, repoSkillView{Name: skill.Identity, Description: skill.Description})
		}
	}
	return skills
}

type repoSkillView struct {
	Name        string
	Description string
}
