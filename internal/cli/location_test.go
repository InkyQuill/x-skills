package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestResolveLocationsSupportsCanonicalAndLabels(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	locations, err := resolveLocations(cfg, []string{"project:opencode", ".Ag", ".Oc"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(locations); got != 3 {
		t.Fatalf("len(locations) = %d, want 3", got)
	}
	if locations[0].Target != "opencode" || locations[1].Target != "agents" || locations[2].Target != "opencode" {
		t.Fatalf("locations = %#v", locations)
	}
}

func TestResolveLocationsSupportsCompactFullForms(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	cfg, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	locations, err := resolveLocations(cfg, []string{".agents", "~codex"})
	if err != nil {
		t.Fatal(err)
	}
	if locations[0].Scope != config.ScopeProject || locations[0].Target != config.TargetAgents {
		t.Fatalf("locations[0] = %#v, want project agents", locations[0])
	}
	if locations[1].Scope != config.ScopeGlobal || locations[1].Target != config.TargetCodex {
		t.Fatalf("locations[1] = %#v, want global codex", locations[1])
	}
}

func TestResolveLocationsRejectsAmbiguousLabel(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("active_roots:\n  - scope: project\n    target: alpha\n    path: .alpha/skills\n    label: .Ax\n  - scope: project\n    target: beta\n    path: .beta/skills\n    label: .Ax\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = resolveLocations(cfg, []string{".Ax"})
	if err == nil {
		t.Fatal("expected ambiguous selector error")
	}
	if !strings.Contains(err.Error(), "project:target or global:target") {
		t.Fatalf("error = %q, want canonical selector guidance", err)
	}
}
