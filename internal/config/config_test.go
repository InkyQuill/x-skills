package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDefaultConfigUsesProjectRootAndHome(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := Default(project, home)

	if cfg.ProjectRoot != project {
		t.Fatalf("ProjectRoot = %q, want %q", cfg.ProjectRoot, project)
	}
	if got := cfg.ArchiveSkillsRoot(); got != filepath.Join(home, ".x-skills", "skills") {
		t.Fatalf("ArchiveSkillsRoot = %q", got)
	}
	if got := cfg.MustActiveRoot("project", "agents"); got != filepath.Join(project, ".agents", "skills") {
		t.Fatalf("project agents root = %q", got)
	}
	if got := cfg.MustActiveRoot("global", "claude"); got != filepath.Join(home, ".claude", "skills") {
		t.Fatalf("global claude root = %q", got)
	}
}

func TestActiveRoot(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)

	cases := []struct {
		name   string
		scope  string
		target string
		want   string
	}{
		{
			name:   "project agents",
			scope:  ScopeProject,
			target: TargetAgents,
			want:   filepath.Join(project, ".agents", "skills"),
		},
		{
			name:   "project claude",
			scope:  ScopeProject,
			target: TargetClaude,
			want:   filepath.Join(project, ".claude", "skills"),
		},
		{
			name:   "project codex",
			scope:  ScopeProject,
			target: TargetCodex,
			want:   filepath.Join(project, ".codex", "skills"),
		},
		{
			name:   "global agents",
			scope:  ScopeGlobal,
			target: TargetAgents,
			want:   filepath.Join(home, ".agents", "skills"),
		},
		{
			name:   "global claude",
			scope:  ScopeGlobal,
			target: TargetClaude,
			want:   filepath.Join(home, ".claude", "skills"),
		},
		{
			name:   "global codex",
			scope:  ScopeGlobal,
			target: TargetCodex,
			want:   filepath.Join(home, ".codex", "skills"),
		},
		{
			name:   "invalid scope",
			scope:  "typo",
			target: TargetCodex,
		},
		{
			name:   "invalid target",
			scope:  ScopeProject,
			target: "bad",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cfg.ActiveRoot(tc.scope, tc.target)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("ActiveRoot(%q, %q) error = nil, want error", tc.scope, tc.target)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ActiveRoot(%q, %q) = %q, want %q", tc.scope, tc.target, got, tc.want)
			}
		})
	}
}

func TestLocationLabel(t *testing.T) {
	cases := []struct {
		name   string
		scope  string
		target string
		want   string
	}{
		{
			name:   "project agents",
			scope:  ScopeProject,
			target: TargetAgents,
			want:   "./.agents",
		},
		{
			name:   "project claude",
			scope:  ScopeProject,
			target: TargetClaude,
			want:   "./.claude",
		},
		{
			name:   "project codex",
			scope:  ScopeProject,
			target: TargetCodex,
			want:   "./.codex",
		},
		{
			name:   "global agents",
			scope:  ScopeGlobal,
			target: TargetAgents,
			want:   "~/.agents",
		},
		{
			name:   "global claude",
			scope:  ScopeGlobal,
			target: TargetClaude,
			want:   "~/.claude",
		},
		{
			name:   "global codex",
			scope:  ScopeGlobal,
			target: TargetCodex,
			want:   "~/.codex",
		},
		{
			name:   "invalid scope",
			scope:  "bad",
			target: TargetAgents,
			want:   "",
		},
		{
			name:   "invalid target",
			scope:  ScopeGlobal,
			target: "bad",
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LocationLabel(tc.scope, tc.target); got != tc.want {
				t.Fatalf("LocationLabel(%q, %q) = %q, want %q", tc.scope, tc.target, got, tc.want)
			}
		})
	}
}

func TestManagedRootsDefaults(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)

	roots := cfg.ManagedRoots()
	if len(roots) != 6 {
		t.Fatalf("len(roots) = %d, want 6", len(roots))
	}
	want := map[string]string{
		"project:agents": filepath.Join(project, ".agents", "skills"),
		"project:claude": filepath.Join(project, ".claude", "skills"),
		"project:codex":  filepath.Join(project, ".codex", "skills"),
		"global:agents":  filepath.Join(home, ".agents", "skills"),
		"global:claude":  filepath.Join(home, ".claude", "skills"),
		"global:codex":   filepath.Join(home, ".codex", "skills"),
	}
	for _, root := range roots {
		key := root.Scope + ":" + root.Target
		if got := root.Path; got != want[key] {
			t.Fatalf("%s path = %q, want %q", key, got, want[key])
		}
		if !root.Builtin || !root.Enabled {
			t.Fatalf("%s root = %#v, want builtin enabled", key, root)
		}
	}
}

func TestManagedRootsDefaultConsumers(t *testing.T) {
	cfg := Default(t.TempDir(), t.TempDir())
	want := map[string][]string{
		TargetAgents: {"codex", "pi", "opencode", "crush"},
		TargetClaude: {"claude"},
		TargetCodex:  {"codex"},
	}

	for _, root := range cfg.ManagedRoots() {
		if !slices.Equal(root.Consumers, want[root.Target]) {
			t.Fatalf("%s consumers = %v, want %v", root.Location(), root.Consumers, want[root.Target])
		}
	}
}

func TestLoadGlobalConfigNormalizesConsumers(t *testing.T) {
	home := t.TempDir()
	cfg := Default(t.TempDir(), home)
	data := []byte("version: 1\nactive_roots:\n  - scope: project\n    target: agents\n    path: .agents/skills\n    consumers: [codex, pi, codex]\n")
	writeGlobalConfig(t, home, data)

	loaded, err := LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range loaded.ManagedRoots() {
		if root.Scope == ScopeProject && root.Target == TargetAgents {
			if want := []string{"codex", "pi"}; !slices.Equal(root.Consumers, want) {
				t.Fatalf("Consumers = %v, want %v", root.Consumers, want)
			}
			return
		}
	}
	t.Fatal("project agents root missing")
}

func TestLoadGlobalConfigDefaultsOmittedConsumersForKnownTarget(t *testing.T) {
	home := t.TempDir()
	cfg := Default(t.TempDir(), home)
	writeGlobalConfig(t, home, []byte("active_roots:\n  - scope: project\n    target: claude\n    path: .claude/custom-skills\n"))

	loaded, err := LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range loaded.ManagedRoots() {
		if root.Scope == ScopeProject && root.Target == TargetClaude {
			if want := defaultConsumers(TargetClaude); !slices.Equal(root.Consumers, want) {
				t.Fatalf("Consumers = %v, want %v", root.Consumers, want)
			}
			return
		}
	}
	t.Fatal("project claude root missing")
}

func TestLoadGlobalConfigLeavesOmittedCustomConsumersUnknown(t *testing.T) {
	home := t.TempDir()
	cfg := Default(t.TempDir(), home)
	writeGlobalConfig(t, home, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n"))

	loaded, err := LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range loaded.ManagedRoots() {
		if root.Target == "opencode" {
			if root.Consumers != nil {
				t.Fatalf("Consumers = %v, want nil", root.Consumers)
			}
			return
		}
	}
	t.Fatal("opencode root missing")
}

func TestManagedRootsReturnsIndependentConsumerSlices(t *testing.T) {
	home := t.TempDir()
	cfg := Default(t.TempDir(), home)
	writeGlobalConfig(t, home, []byte("active_roots:\n  - scope: project\n    target: agents\n    path: .agents/skills\n    consumers: [codex, pi]\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n"))

	loaded, err := LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	first := loaded.ManagedRoots()
	for i := range first {
		if first[i].Scope == ScopeProject && first[i].Target == TargetAgents {
			first[i].Consumers[0] = "invalid_id"
		}
	}

	second := loaded.ManagedRoots()
	for _, root := range second {
		switch {
		case root.Scope == ScopeProject && root.Target == TargetAgents:
			if want := []string{"codex", "pi"}; !slices.Equal(root.Consumers, want) {
				t.Fatalf("Consumers = %v, want %v", root.Consumers, want)
			}
		case root.Scope == ScopeProject && root.Target == "opencode":
			if root.Consumers != nil {
				t.Fatalf("Consumers = %v, want nil", root.Consumers)
			}
		}
	}
}

func TestNormalizeConsumers(t *testing.T) {
	t.Run("normalizes", func(t *testing.T) {
		got, err := NormalizeConsumers([]string{" Pi ", "codex", "PI", "open-code"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"codex", "open-code", "pi"}
		if !slices.Equal(got, want) {
			t.Fatalf("NormalizeConsumers() = %v, want %v", got, want)
		}
	})

	for _, consumers := range [][]string{{""}, {"open_code"}, {"9agent"}} {
		t.Run(strings.Join(consumers, ","), func(t *testing.T) {
			if _, err := NormalizeConsumers(consumers); err == nil {
				t.Fatalf("NormalizeConsumers(%q) error = nil, want error", consumers)
			}
		})
	}
}

func TestLoadGlobalConfigAddsAndOverridesManagedRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	data := []byte(`
version: 1
active_roots:
  - scope: global
    target: hermes
    path: ~/.config/hermes/skills
    label: ~Hm
  - scope: global
    target: claude
    enabled: false
  - scope: project
    target: opencode
    path: .opencode/skills
`)
	writeGlobalConfig(t, home, data)

	loaded, err := LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loaded.ActiveRoot(ScopeGlobal, TargetClaude); err == nil {
		t.Fatal("global claude root should be disabled")
	}
	hermes, err := loaded.ActiveRoot(ScopeGlobal, "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if hermes != filepath.Join(home, ".config", "hermes", "skills") {
		t.Fatalf("hermes path = %q", hermes)
	}
	opencode, err := loaded.ActiveRoot(ScopeProject, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	if opencode != filepath.Join(project, ".opencode", "skills") {
		t.Fatalf("opencode path = %q", opencode)
	}
}

func TestLoadGlobalConfigUsesHomeWhenArchiveRootIsOverridden(t *testing.T) {
	home := t.TempDir()
	cfg := Default(t.TempDir(), home)
	cfg.ArchiveRoot = filepath.Join(t.TempDir(), "archive")
	writeGlobalConfig(t, home, []byte("active_roots:\n  - scope: global\n    target: claude\n    enabled: false\n"))

	loaded, err := LoadGlobal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loaded.ActiveRoot(ScopeGlobal, TargetClaude); err == nil {
		t.Fatal("global claude root should be disabled by the home config")
	}
}

func writeGlobalConfig(t *testing.T, home string, data []byte) {
	t.Helper()
	configDir := filepath.Join(home, ".x-skills")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadGlobalConfigAcceptsMissingVersionAsVersionOne(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	writeGlobalConfig(t, home, []byte("active_roots: []\n"))

	if _, err := LoadGlobal(cfg); err != nil {
		t.Fatalf("LoadGlobal() error = %v, want nil", err)
	}
}

func TestLoadGlobalConfigAcceptsVersionOne(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	writeGlobalConfig(t, home, []byte("version: 1\nactive_roots: []\n"))

	if _, err := LoadGlobal(cfg); err != nil {
		t.Fatalf("LoadGlobal() error = %v, want nil", err)
	}
}

func TestLoadGlobalConfigRejectsInvalidTarget(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	writeGlobalConfig(t, home, []byte("active_roots:\n  - scope: global\n    target: Open Claw\n    path: ~/.openclaw/skills\n"))

	_, err := LoadGlobal(cfg)
	if err == nil || !strings.Contains(err.Error(), `invalid target "Open Claw"`) {
		t.Fatalf("err = %v, want invalid target", err)
	}
}

func TestLoadGlobalConfigRejectsVersionZero(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	writeGlobalConfig(t, home, []byte("version: 0\n"))

	_, err := LoadGlobal(cfg)
	if err == nil || !strings.Contains(err.Error(), "unsupported version 0") {
		t.Fatalf("err = %v, want unsupported version 0", err)
	}
}

func TestLoadGlobalConfigRejectsUnknownVersion(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	writeGlobalConfig(t, home, []byte("version: 2\n"))

	_, err := LoadGlobal(cfg)
	if err == nil || !strings.Contains(err.Error(), "unsupported version 2") {
		t.Fatalf("err = %v, want unsupported version", err)
	}
}
