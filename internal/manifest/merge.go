package manifest

import (
	"cmp"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/remote"
)

type Notice struct {
	Skill   string
	Message string
}

func Effective(recommended, local Manifest) (Manifest, []Notice) {
	merged := make(map[string]Skill, len(local.Skills)+len(recommended.Skills))
	for _, skill := range cloneManifest(local).Skills {
		merged[skill.Name] = skill
	}

	notices := make([]Notice, 0)
	for _, skill := range cloneManifest(recommended).Skills {
		if localSkill, exists := merged[skill.Name]; exists && !sameIdentity(localSkill, skill) {
			notices = append(notices, Notice{
				Skill:   skill.Name,
				Message: "committed skill identity overrides local identity",
			})
		}
		merged[skill.Name] = skill
	}

	effective := Manifest{Version: manifestVersion, Skills: make([]Skill, 0, len(merged))}
	for _, skill := range merged {
		effective.Skills = append(effective.Skills, skill)
	}
	slices.SortStableFunc(effective.Skills, compareSkillNames)
	slices.SortStableFunc(notices, func(a, b Notice) int {
		return strings.Compare(strings.ToLower(a.Skill), strings.ToLower(b.Skill))
	})
	return effective, notices
}

func sameIdentity(a, b Skill) bool {
	return a.Source == b.Source &&
		a.Fingerprint == b.Fingerprint &&
		sameCompatibility(a.Compatibility, b.Compatibility)
}

func sameCompatibility(a, b *remote.CompatibilityProfile) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Agnostic == b.Agnostic && slices.Equal(a.Agents, b.Agents)
}

func compareSkillNames(a, b Skill) int {
	return cmp.Or(
		strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
		strings.Compare(a.Name, b.Name),
	)
}
