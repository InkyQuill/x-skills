package compatibility_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestAssessNormalizesAgentIDs(t *testing.T) {
	t.Parallel()

	got, err := compatibility.Assess(
		"unused",
		&remote.CompatibilityProfile{Agents: []string{" Codex ", "CLAUDE", "claude"}},
		[]string{" claude ", "CODEX", "codex"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != compatibility.StateCompatible {
		t.Fatalf("state = %q, want compatible", got.State)
	}
	wantAgents := []string{"claude", "codex"}
	if !reflect.DeepEqual(got.Agents, wantAgents) {
		t.Fatalf("agents = %#v, want %#v", got.Agents, wantAgents)
	}
}

func TestAssessTreatsBlankNormalizedIDsAsUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		profile   *remote.CompatibilityProfile
		consumers []string
	}{
		{name: "blank consumers", profile: &remote.CompatibilityProfile{Agnostic: true}, consumers: []string{"  "}},
		{name: "blank profile agents", profile: &remote.CompatibilityProfile{Agents: []string{"  "}}, consumers: []string{"codex"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := compatibility.Assess("unused", tt.profile, tt.consumers)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != compatibility.StateUnknown {
				t.Fatalf("assessment = %#v, want unknown", got)
			}
		})
	}
}

func TestAssessReasonsDoNotDependOnMetadataFilenameOrder(t *testing.T) {
	t.Parallel()

	want := []string{"uses the Claude-only $CLAUDE_PROJECT_DIR runtime variable"}
	for _, strongName := range []string{"openai.yaml", "claude.yaml"} {
		strongName := strongName
		t.Run(strongName, func(t *testing.T) {
			t.Parallel()
			dir := writeSkill(t, "This skill mentions Claude.")
			writeAgentMetadata(t, dir, strongName, "instructions: Read the project from $CLAUDE_PROJECT_DIR.\n")
			other := "claude.yaml"
			if strongName == other {
				other = "openai.yaml"
			}
			writeAgentMetadata(t, dir, other, "notes: Claude is also supported.\n")

			got, err := compatibility.Assess(dir, nil, []string{"codex"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Reasons, want) {
				t.Fatalf("reasons = %#v, want %#v", got.Reasons, want)
			}
		})
	}
}

func TestAssessScansOnlyKnownAgentMetadataWithoutFollowingSymlinks(t *testing.T) {
	t.Parallel()

	dir := writeSkill(t, "Works with coding agents.")
	agents := filepath.Join(dir, "agents")
	if err := os.MkdirAll(filepath.Join(agents, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	strong := "instructions: Read the project from $CLAUDE_PROJECT_DIR.\n"
	if err := os.WriteFile(filepath.Join(agents, "nested", "fixture.json"), []byte(strong), 0o644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "openai.yaml")
	if err := os.WriteFile(external, []byte(strong), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(agents, "openai.yaml")); err != nil {
		t.Fatal(err)
	}

	got, err := compatibility.Assess(dir, nil, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != compatibility.StateUnknown || got.Confidence == compatibility.ConfidenceHigh {
		t.Fatalf("assessment = %#v, want non-high unknown", got)
	}
}

func TestAssessDoesNotFollowAgentsDirectorySymlink(t *testing.T) {
	t.Parallel()

	dir := writeSkill(t, "Works with coding agents.")
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "openai.yaml"), []byte("instructions: Read from $CLAUDE_PROJECT_DIR.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dir, "agents")); err != nil {
		t.Fatal(err)
	}

	got, err := compatibility.Assess(dir, nil, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != compatibility.StateUnknown {
		t.Fatalf("assessment = %#v, want unknown", got)
	}
}

func TestAssessRequiresExecutableContextForHighConfidenceSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantState  compatibility.State
		wantReason []string
	}{
		{name: "imperative variable", content: "Read the project from `$CLAUDE_PROJECT_DIR`.", wantState: compatibility.StateIncompatible, wantReason: []string{"uses the Claude-only $CLAUDE_PROJECT_DIR runtime variable"}},
		{name: "must first use tool", content: "You must first use AskUserQuestion before editing.", wantState: compatibility.StateIncompatible, wantReason: []string{"mandates a Claude-only tool"}},
		{name: "backticked tool", content: "You must use `AskUserQuestion` before editing.", wantState: compatibility.StateIncompatible, wantReason: []string{"mandates a Claude-only tool"}},
		{name: "required call tool", content: "Required: call AskUserQuestion before editing.", wantState: compatibility.StateIncompatible, wantReason: []string{"mandates a Claude-only tool"}},
		{name: "negated variable", content: "Do not use `$CLAUDE_PROJECT_DIR`.", wantState: compatibility.StateUnknown},
		{name: "quoted counterexample", content: "> Read the project from `$CLAUDE_PROJECT_DIR`.", wantState: compatibility.StateUnknown},
		{name: "inline quoted counterexample", content: "The old guide said \"You must use AskUserQuestion\".", wantState: compatibility.StateUnknown},
		{name: "example", content: "Example: must use AskUserQuestion.", wantState: compatibility.StateUnknown},
		{name: "title", content: "# Must use AskUserQuestion", wantState: compatibility.StateUnknown},
		{name: "url", content: "See https://example.test/$CLAUDE_PROJECT_DIR for history.", wantState: compatibility.StateUnknown},
		{name: "comparison", content: "| Claude | `$CLAUDE_PROJECT_DIR` |", wantState: compatibility.StateUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := writeSkill(t, tt.content)
			got, err := compatibility.Assess(dir, nil, []string{"codex"})
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tt.wantState {
				t.Fatalf("state = %q, want %q; assessment=%#v", got.State, tt.wantState, got)
			}
			if tt.wantReason != nil && !reflect.DeepEqual(got.Reasons, tt.wantReason) {
				t.Fatalf("reasons = %#v, want %#v", got.Reasons, tt.wantReason)
			}
		})
	}
}

func TestAssessErrorBehavior(t *testing.T) {
	t.Parallel()

	t.Run("missing SKILL.md", func(t *testing.T) {
		t.Parallel()
		_, err := compatibility.Assess(t.TempDir(), nil, []string{"codex"})
		if err == nil || !strings.Contains(err.Error(), "read compatibility input") {
			t.Fatalf("error = %v, want wrapped SKILL.md read error", err)
		}
	})

	t.Run("known metadata read failure", func(t *testing.T) {
		t.Parallel()
		dir := writeSkill(t, "Works with coding agents.")
		if err := os.MkdirAll(filepath.Join(dir, "agents", "openai.yaml"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := compatibility.Assess(dir, nil, []string{"codex"})
		if err == nil || !strings.Contains(err.Error(), "read compatibility input") {
			t.Fatalf("error = %v, want wrapped metadata read error", err)
		}
	})

	t.Run("absent agents directory", func(t *testing.T) {
		t.Parallel()
		dir := writeSkill(t, "Works with coding agents.")
		if _, err := compatibility.Assess(dir, nil, []string{"codex"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unknown consumers skip invalid skill path", func(t *testing.T) {
		t.Parallel()
		got, err := compatibility.Assess(filepath.Join(t.TempDir(), "missing"), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != compatibility.StateUnknown {
			t.Fatalf("assessment = %#v, want unknown", got)
		}
	})
}

func writeSkill(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	data := "---\nname: test\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeAgentMetadata(t *testing.T, skillDir, name, content string) {
	t.Helper()
	dir := filepath.Join(skillDir, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
