package config

import "path/filepath"

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

func (c Config) ActiveRoot(scope, target string) string {
	if scope == ScopeProject {
		return filepath.Join(c.ProjectRoot, "."+target, "skills")
	}
	switch target {
	case TargetAgents:
		return c.GlobalAgentsRoot
	case TargetClaude:
		return c.GlobalClaudeRoot
	case TargetCodex:
		return c.GlobalCodexRoot
	default:
		return ""
	}
}

func LocationLabel(scope, target string) string {
	prefix := "./"
	if scope == ScopeGlobal {
		prefix = "~/"
	}
	return prefix + "." + target
}
