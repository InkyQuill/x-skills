package tui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/roots"
)

type ActiveGroup struct {
	ID          string
	Name        string
	Status      string
	Description string
	Chips       []string
	Aliases     []string
	Members     []actions.ActiveSkill
	Reason      string
	Fingerprint string
}

func groupActiveSkills(skills []actions.ActiveSkill) []ActiveGroup {
	groups := map[string]*ActiveGroup{}
	var order []string

	for _, skill := range skills {
		key, fp := activeGroupKey(skill)
		group, ok := groups[key]
		if !ok {
			group = &ActiveGroup{
				ID:          "active:" + key,
				Name:        skill.Name,
				Status:      skill.Status,
				Description: skill.Description,
				Reason:      skill.Reason,
				Fingerprint: fp,
			}
			groups[key] = group
			order = append(order, key)
		}
		group.Members = append(group.Members, skill)
		group.Chips = appendUnique(group.Chips, rootLabel(skill.Root))
		if alias := filepath.Base(skill.Path); alias != "" && alias != group.Name {
			group.Aliases = appendUnique(group.Aliases, alias)
		}
		if group.Description == "" {
			group.Description = skill.Description
		}
		if group.Reason == "" {
			group.Reason = skill.Reason
		}
		group.Status = mergedStatus(group.Status, skill.Status)
	}

	result := make([]ActiveGroup, 0, len(order))
	for _, key := range order {
		sort.Strings(groups[key].Chips)
		sort.Strings(groups[key].Aliases)
		result = append(result, *groups[key])
	}
	sort.Slice(result, func(i, j int) bool {
		return skillNameLess(result[i].Name, result[j].Name)
	})
	return result
}

func skillNameLess(left, right string) bool {
	leftFolded := strings.ToLower(left)
	rightFolded := strings.ToLower(right)
	if leftFolded == rightFolded {
		return left < right
	}
	return leftFolded < rightFolded
}

func activeGroupKey(skill actions.ActiveSkill) (string, string) {
	if skill.Status == actions.StatusBroken {
		return "broken:" + skill.Path, ""
	}

	target := skill.Path
	if resolved, err := filepath.EvalSymlinks(skill.Path); err == nil {
		target = resolved
	}
	if fp, err := fingerprint.Directory(target); err == nil {
		return "sha:" + fp, fp
	}
	return "path:" + target, ""
}

func mergedStatus(current, next string) string {
	if current == actions.StatusBroken || next == actions.StatusBroken {
		return actions.StatusBroken
	}
	if current == actions.StatusUnmanaged || next == actions.StatusUnmanaged {
		return actions.StatusUnmanaged
	}
	return actions.StatusManaged
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	if value == "" || containsString(values, value) {
		return values
	}
	return append(values, value)
}

func usageByRepoName(groups []ActiveGroup) map[string][]string {
	usage := map[string][]string{}
	for _, group := range groups {
		for _, member := range group.Members {
			if member.Status != actions.StatusManaged {
				continue
			}
			chip := rootLabel(member.Root)
			usage[member.Name] = appendUnique(usage[member.Name], chip)
		}
	}
	for name := range usage {
		sort.Strings(usage[name])
	}
	return usage
}

func rootLabel(root roots.ActiveRoot) string {
	if root.Label != "" {
		return root.Label
	}
	return rootChip(root.Scope, root.Target)
}
