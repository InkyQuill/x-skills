package config

import (
	"path/filepath"
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
	if got := cfg.ActiveRoot("project", "agents"); got != filepath.Join(project, ".agents", "skills") {
		t.Fatalf("project agents root = %q", got)
	}
	if got := cfg.ActiveRoot("global", "claude"); got != filepath.Join(home, ".claude", "skills") {
		t.Fatalf("global claude root = %q", got)
	}
}

func TestLocationLabel(t *testing.T) {
	cases := []struct {
		scope  string
		target string
		want   string
	}{
		{"project", "agents", "./.agents"},
		{"project", "codex", "./.codex"},
		{"global", "claude", "~/.claude"},
	}

	for _, tc := range cases {
		if got := LocationLabel(tc.scope, tc.target); got != tc.want {
			t.Fatalf("LocationLabel(%q, %q) = %q, want %q", tc.scope, tc.target, got, tc.want)
		}
	}
}
