package compatibility

import (
	"slices"

	"github.com/InkyQuill/x-skills/internal/remote"
)

type State string

const (
	StateCompatible   State = "compatible"
	StatePartial      State = "partial"
	StateUnknown      State = "unknown"
	StateIncompatible State = "incompatible"
)

type Confidence string

const (
	ConfidenceUnknown Confidence = "unknown"
	ConfidenceLow     Confidence = "low"
	ConfidenceHigh    Confidence = "high"
)

type Assessment struct {
	State      State
	Confidence Confidence
	Agents     []string
	Reasons    []string
	Explicit   bool
}

func Assess(
	skillDir string,
	explicit *remote.CompatibilityProfile,
	consumers []string,
) (Assessment, error) {
	if explicit != nil {
		return assessExplicit(explicit, consumers), nil
	}

	inferred, err := infer(skillDir)
	if err != nil {
		return Assessment{}, err
	}
	if inferred.Confidence != ConfidenceHigh {
		return Assessment{
			State:      StateUnknown,
			Confidence: inferred.Confidence,
			Reasons:    inferred.Reasons,
		}, nil
	}

	assessment := assessAgents(inferred.Agents, consumers)
	assessment.Confidence = ConfidenceHigh
	assessment.Reasons = inferred.Reasons
	return assessment, nil
}

func assessExplicit(profile *remote.CompatibilityProfile, consumers []string) Assessment {
	assessment := Assessment{
		State:      StateUnknown,
		Confidence: ConfidenceHigh,
		Explicit:   true,
	}
	if profile.Agnostic {
		if len(consumers) > 0 {
			assessment.State = StateCompatible
		}
		return assessment
	}

	assessment = assessAgents(profile.Agents, consumers)
	assessment.Confidence = ConfidenceHigh
	assessment.Explicit = true
	return assessment
}

func assessAgents(agents, consumers []string) Assessment {
	knownAgents := sortedUnique(agents)
	assessment := Assessment{State: StateUnknown, Agents: knownAgents}
	if len(consumers) == 0 {
		return assessment
	}

	matches := 0
	for _, consumer := range sortedUnique(consumers) {
		if slices.Contains(knownAgents, consumer) {
			matches++
		}
	}
	switch {
	case matches == 0:
		assessment.State = StateIncompatible
	case matches == len(sortedUnique(consumers)):
		assessment.State = StateCompatible
	default:
		assessment.State = StatePartial
	}
	return assessment
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}
