package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
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
	managedRoots     []ManagedRoot
}

type ManagedRoot struct {
	Scope   string
	Target  string
	Path    string
	Label   string
	Builtin bool
	Enabled bool
}

func (r ManagedRoot) Location() string {
	return r.Scope + ":" + r.Target
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
	for _, root := range c.ManagedRoots() {
		if root.Scope != scope || root.Target != target {
			continue
		}
		if !root.Enabled {
			return "", fmt.Errorf("root %s is disabled", root.Location())
		}
		return root.Path, nil
	}
	return "", fmt.Errorf("unknown target %q", target)
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

func ValidScope(scope string) bool {
	return validScope(scope)
}

func validTarget(target string) bool {
	return slices.Contains(Targets, target)
}

func (c Config) ManagedRoots() []ManagedRoot {
	if c.managedRoots == nil {
		return c.defaultManagedRoots()
	}
	return append([]ManagedRoot(nil), c.managedRoots...)
}

func LoadGlobal(cfg Config) (Config, error) {
	configPath := filepath.Join(cfg.HomeDir, ".x-skills", "config.yaml")
	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg.managedRoots = cfg.defaultManagedRoots()
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", configPath, err)
	}

	var parsed fileConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", configPath, err)
	}
	if parsed.Version != nil && *parsed.Version != 1 {
		return Config{}, fmt.Errorf("load config %s: unsupported version %d", configPath, *parsed.Version)
	}

	managedRoots := cfg.defaultManagedRoots()
	for i, entry := range parsed.ActiveRoots {
		root, err := cfg.managedRootFromYAML(entry)
		if err != nil {
			return Config{}, fmt.Errorf("load config %s: active_roots[%d]: %w", configPath, i, err)
		}
		managedRoots = upsertManagedRoot(managedRoots, root)
	}
	cfg.managedRoots = managedRoots
	return cfg, nil
}

type fileConfig struct {
	Version     *int             `yaml:"version"`
	ActiveRoots []activeRootYAML `yaml:"active_roots"`
}

type activeRootYAML struct {
	Scope   string `yaml:"scope"`
	Target  string `yaml:"target"`
	Path    string `yaml:"path"`
	Label   string `yaml:"label"`
	Enabled *bool  `yaml:"enabled"`
}

var targetPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (c Config) defaultManagedRoots() []ManagedRoot {
	return []ManagedRoot{
		c.defaultManagedRoot(ScopeProject, TargetAgents, filepath.Join(c.ProjectRoot, ".agents", "skills"), ".Ag"),
		c.defaultManagedRoot(ScopeProject, TargetClaude, filepath.Join(c.ProjectRoot, ".claude", "skills"), ".Cl"),
		c.defaultManagedRoot(ScopeProject, TargetCodex, filepath.Join(c.ProjectRoot, ".codex", "skills"), ".Cd"),
		c.defaultManagedRoot(ScopeGlobal, TargetAgents, c.GlobalAgentsRoot, "~Ag"),
		c.defaultManagedRoot(ScopeGlobal, TargetClaude, c.GlobalClaudeRoot, "~Cl"),
		c.defaultManagedRoot(ScopeGlobal, TargetCodex, c.GlobalCodexRoot, "~Cd"),
	}
}

func (c Config) defaultManagedRoot(scope, target, path, label string) ManagedRoot {
	return ManagedRoot{
		Scope:   scope,
		Target:  target,
		Path:    path,
		Label:   label,
		Builtin: true,
		Enabled: true,
	}
}

func (c Config) managedRootFromYAML(entry activeRootYAML) (ManagedRoot, error) {
	scope := strings.TrimSpace(entry.Scope)
	target := strings.TrimSpace(entry.Target)
	if !validScope(scope) {
		return ManagedRoot{}, fmt.Errorf("invalid scope %q", scope)
	}
	if !targetPattern.MatchString(target) {
		return ManagedRoot{}, fmt.Errorf("invalid target %q; expected ^[a-z][a-z0-9-]*$", target)
	}

	enabled := true
	if entry.Enabled != nil {
		enabled = *entry.Enabled
	}
	root := ManagedRoot{
		Scope:   scope,
		Target:  target,
		Label:   strings.TrimSpace(entry.Label),
		Enabled: enabled,
	}
	if enabled {
		pathValue := strings.TrimSpace(entry.Path)
		if pathValue == "" {
			return ManagedRoot{}, fmt.Errorf("path is required when enabled is true")
		}
		root.Path = c.expandManagedRootPath(scope, pathValue)
	}
	if root.Label == "" {
		root.Label = defaultManagedRootLabel(scope, target)
	}
	return root, nil
}

func (c Config) expandManagedRootPath(scope, value string) string {
	if strings.HasPrefix(value, "~/") {
		return filepath.Join(c.HomeDir, filepath.FromSlash(strings.TrimPrefix(value, "~/")))
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	if scope == ScopeProject {
		return filepath.Join(c.ProjectRoot, filepath.FromSlash(value))
	}
	return filepath.Join(c.HomeDir, filepath.FromSlash(value))
}

func defaultManagedRootLabel(scope, target string) string {
	prefix := "."
	if scope == ScopeGlobal {
		prefix = "~"
	}
	return prefix + targetLabel(target)
}

func targetLabel(target string) string {
	parts := strings.Split(target, "-")
	var label strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		label.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			label.WriteString(part[1:min(len(part), 2)])
		}
	}
	return label.String()
}

func upsertManagedRoot(roots []ManagedRoot, root ManagedRoot) []ManagedRoot {
	for i := range roots {
		if roots[i].Scope == root.Scope && roots[i].Target == root.Target {
			roots[i] = root
			return roots
		}
	}
	return append(roots, root)
}
