package compatibility_test

import (
	"reflect"
	"testing"

	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestAssess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		skillDir     string
		explicit     *remote.CompatibilityProfile
		consumers    []string
		wantState    compatibility.State
		wantConf     compatibility.Confidence
		wantAgents   []string
		wantReasons  []string
		wantExplicit bool
	}{
		{
			name:         "explicit agnostic",
			explicit:     &remote.CompatibilityProfile{Agnostic: true},
			consumers:    []string{"codex", "claude"},
			wantState:    compatibility.StateCompatible,
			wantConf:     compatibility.ConfidenceHigh,
			wantExplicit: true,
		},
		{
			name:         "explicit full match",
			explicit:     &remote.CompatibilityProfile{Agents: []string{"codex", "claude"}},
			consumers:    []string{"claude", "codex"},
			wantState:    compatibility.StateCompatible,
			wantConf:     compatibility.ConfidenceHigh,
			wantAgents:   []string{"claude", "codex"},
			wantExplicit: true,
		},
		{
			name:         "explicit partial match",
			explicit:     &remote.CompatibilityProfile{Agents: []string{"claude"}},
			consumers:    []string{"claude", "codex"},
			wantState:    compatibility.StatePartial,
			wantConf:     compatibility.ConfidenceHigh,
			wantAgents:   []string{"claude"},
			wantExplicit: true,
		},
		{
			name:         "explicit no match",
			explicit:     &remote.CompatibilityProfile{Agents: []string{"claude"}},
			consumers:    []string{"codex"},
			wantState:    compatibility.StateIncompatible,
			wantConf:     compatibility.ConfidenceHigh,
			wantAgents:   []string{"claude"},
			wantExplicit: true,
		},
		{
			name:         "unknown consumers override explicit profile",
			explicit:     &remote.CompatibilityProfile{Agnostic: true},
			wantState:    compatibility.StateUnknown,
			wantConf:     compatibility.ConfidenceHigh,
			wantExplicit: true,
		},
		{
			name:        "ordinary agent mention remains unknown",
			skillDir:    "testdata/mentions-claude",
			consumers:   []string{"codex"},
			wantState:   compatibility.StateUnknown,
			wantConf:    compatibility.ConfidenceLow,
			wantReasons: []string{"mentions an agent without exclusive executable semantics"},
		},
		{
			name:        "strong claude-only instruction is incompatible with codex",
			skillDir:    "testdata/claude-only",
			consumers:   []string{"codex"},
			wantState:   compatibility.StateIncompatible,
			wantConf:    compatibility.ConfidenceHigh,
			wantAgents:  []string{"claude"},
			wantReasons: []string{"uses the Claude-only $CLAUDE_PROJECT_DIR runtime variable"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := compatibility.Assess(tt.skillDir, tt.explicit, tt.consumers)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tt.wantState || got.Confidence != tt.wantConf || got.Explicit != tt.wantExplicit {
				t.Fatalf("assessment = %#v, want state=%q confidence=%q explicit=%v", got, tt.wantState, tt.wantConf, tt.wantExplicit)
			}
			if !reflect.DeepEqual(got.Agents, tt.wantAgents) {
				t.Fatalf("agents = %#v, want %#v", got.Agents, tt.wantAgents)
			}
			if !reflect.DeepEqual(got.Reasons, tt.wantReasons) {
				t.Fatalf("reasons = %#v, want %#v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestAssessExplicitMetadataOverridesInference(t *testing.T) {
	t.Parallel()

	got, err := compatibility.Assess(
		"testdata/claude-only",
		&remote.CompatibilityProfile{Agnostic: true},
		[]string{"codex"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != compatibility.StateCompatible || !got.Explicit {
		t.Fatalf("assessment = %#v, want explicit compatible", got)
	}
}
