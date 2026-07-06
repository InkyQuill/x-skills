package config

import (
	"fmt"
	"path/filepath"
	"slices"
)

const (
	ScopeProject = "project"
	ScopeGlobal  = "global"

	TargetAgents = "agents"
	TargetClaude = "claude"
	TargetCodex  = "codex"
)

var Scopes = []string{ScopeProject, ScopeGlobal}
var Targets = []string{TargetAgents, TargetClaude, TargetCodex}

type Config struct {
	ProjectRoot      string
	HomeDir          string
	ArchiveRoot      string
	GlobalAgentsRoot string
	GlobalClaudeRoot string
	GlobalCodexRoot  string
}

func Default(projectRoot, homeDir string) Config {
	return Config{
		ProjectRoot:      projectRoot,
		HomeDir:          homeDir,
		ArchiveRoot:      filepath.Join(homeDir, ".x-skills"),
		GlobalAgentsRoot: filepath.Join(homeDir, ".agents", "skills"),
		GlobalClaudeRoot: filepath.Join(homeDir, ".claude", "skills"),
		GlobalCodexRoot:  filepath.Join(homeDir, ".codex", "skills"),
	}
}

func (c Config) ArchiveSkillsRoot() string {
	return filepath.Join(c.ArchiveRoot, "skills")
}

func (c Config) ActiveRoot(scope, target string) (string, error) {
	if !validScope(scope) {
		return "", fmt.Errorf("unknown scope %q", scope)
	}
	if !validTarget(target) {
		return "", fmt.Errorf("unknown target %q", target)
	}
	if scope == ScopeProject {
		return filepath.Join(c.ProjectRoot, "."+target, "skills"), nil
	}
	switch target {
	case TargetAgents:
		return c.GlobalAgentsRoot, nil
	case TargetClaude:
		return c.GlobalClaudeRoot, nil
	case TargetCodex:
		return c.GlobalCodexRoot, nil
	default:
		return "", fmt.Errorf("unknown target %q", target)
	}
}

func (c Config) MustActiveRoot(scope, target string) string {
	root, err := c.ActiveRoot(scope, target)
	if err != nil {
		panic(err)
	}
	return root
}

func LocationLabel(scope, target string) string {
	if !validScope(scope) || !validTarget(target) {
		return ""
	}
	prefix := "./"
	if scope == ScopeGlobal {
		prefix = "~/"
	}
	return prefix + "." + target
}

func validScope(scope string) bool {
	return slices.Contains(Scopes, scope)
}

func validTarget(target string) bool {
	return slices.Contains(Targets, target)
}
