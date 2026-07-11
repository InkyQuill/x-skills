package roots

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestActiveRootsCanBeFiltered(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	all := ActiveRoots(cfg, Filter{})
	if len(all) != 6 {
		t.Fatalf("len(all) = %d, want 6", len(all))
	}

	filtered := ActiveRoots(cfg, Filter{Scope: "project", Target: "codex"})
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].Label != ".Cd" {
		t.Fatalf("Label = %q", filtered[0].Label)
	}
}

func TestActiveRootsIncludesConsumers(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")
	want := map[string][]string{
		config.TargetAgents: {"codex", "pi", "opencode", "crush"},
		config.TargetClaude: {"claude"},
		config.TargetCodex:  {"codex"},
	}

	for _, root := range ActiveRoots(cfg, Filter{}) {
		if !slices.Equal(root.Consumers, want[root.Target]) {
			t.Fatalf("%s:%s consumers = %v, want %v", root.Scope, root.Target, root.Consumers, want[root.Target])
		}
	}
}

func TestActiveRootsReturnsIndependentConsumerSlices(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("active_roots:\n  - scope: project\n    target: agents\n    path: .agents/skills\n    consumers: [codex, pi]\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	first := ActiveRoots(loaded, Filter{Scope: config.ScopeProject, Target: config.TargetAgents})
	first[0].Consumers[0] = "invalid_id"

	second := ActiveRoots(loaded, Filter{Scope: config.ScopeProject, Target: config.TargetAgents})
	if want := []string{"codex", "pi"}; !slices.Equal(second[0].Consumers, want) {
		t.Fatalf("Consumers = %v, want %v", second[0].Consumers, want)
	}
}

func TestActiveRootsCanBeFilteredByScopeOnly(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	filtered := ActiveRoots(cfg, Filter{Scope: "project"})
	if len(filtered) != 3 {
		t.Fatalf("len(filtered) = %d, want 3", len(filtered))
	}
	for _, root := range filtered {
		if root.Scope != "project" {
			t.Fatalf("Scope = %q, want project", root.Scope)
		}
	}
}

func TestActiveRootsCanBeFilteredByTargetOnly(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	filtered := ActiveRoots(cfg, Filter{Target: "codex"})
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}
	for _, root := range filtered {
		if root.Target != "codex" {
			t.Fatalf("Target = %q, want codex", root.Target)
		}
	}
}

func TestActiveRootsRejectInvalidFilter(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	if roots := ActiveRoots(cfg, Filter{Scope: "workspace"}); len(roots) != 0 {
		t.Fatalf("roots = %#v, want none", roots)
	}
	if roots := ActiveRoots(cfg, Filter{Target: "cursor"}); len(roots) != 0 {
		t.Fatalf("roots = %#v, want none", roots)
	}
}

func TestActiveRootsIncludesConfiguredRootsAndSkipsDisabled(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n  - scope: global\n    target: claude\n    enabled: false\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	all := ActiveRoots(loaded, Filter{})
	if len(all) != 6 {
		t.Fatalf("len(all) = %d, want 6", len(all))
	}
	var foundOpenCode bool
	for _, root := range all {
		if root.Target == "opencode" {
			foundOpenCode = true
			if root.Label != ".Oc" || root.Path != filepath.Join(project, ".opencode", "skills") {
				t.Fatalf("opencode root = %#v", root)
			}
		}
		if root.Scope == config.ScopeGlobal && root.Target == config.TargetClaude {
			t.Fatalf("disabled global claude root was returned: %#v", root)
		}
	}
	if !foundOpenCode {
		t.Fatal("opencode root missing")
	}
}
