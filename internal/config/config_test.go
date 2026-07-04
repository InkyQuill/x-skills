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
			want:   "",
		},
		{
			name:   "invalid target",
			scope:  ScopeProject,
			target: "bad",
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.ActiveRoot(tc.scope, tc.target); got != tc.want {
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
