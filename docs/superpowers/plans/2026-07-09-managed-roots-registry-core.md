# Managed Roots Registry Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add global `~/.x-skills/config.yaml` support for managed active skill roots and expose them through `x-skills list-roots`.

**Architecture:** Keep path/config parsing in `internal/config`, root expansion in `internal/roots`, and Cobra wiring in `internal/cli`. Built-in roots remain defaults, config entries override by `(scope, target)`, `enabled: false` disables a root, and invalid config fails fast.

**Tech Stack:** Go 1.26, Cobra, `gopkg.in/yaml.v3`, existing `config`, `roots`, and CLI packages.

---

## Files

- Modify: `internal/config/config.go` for root registry structs, config loading, target validation, path expansion, and label generation.
- Modify: `internal/config/config_test.go` for config parsing and validation tests.
- Modify: `internal/roots/roots.go` for dynamic root expansion.
- Modify: `internal/roots/roots_test.go` for custom root and disabled default coverage.
- Modify: `internal/cli/root.go` to load global config and remove legacy root override fields in this phase only if tests are updated in the same task.
- Create: `internal/cli/list_roots.go` for `x-skills list-roots`.
- Create: `internal/cli/list_roots_test.go` for human and JSON output.
- Modify: `README.md` to document `~/.x-skills/config.yaml` and `list-roots`.
- Modify: `docs/backlog.md` only after this plan is fully verified.

### Task 1: Config Model And Built-In Defaults

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for built-in managed roots**

Add to `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/config -count=1 -run TestManagedRootsDefaults
```

Expected: FAIL because `ManagedRoots` is undefined.

- [ ] **Step 3: Implement root model and defaults**

In `internal/config/config.go`, add:

```go
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

func (c Config) ManagedRoots() []ManagedRoot {
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
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/config -count=1 -run TestManagedRootsDefaults
```

Expected: PASS.

### Task 2: Global YAML Config Loading

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for `~/.x-skills/config.yaml`**

Add to `internal/config/config_test.go`:

```go
func TestLoadGlobalConfigAddsAndOverridesManagedRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

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

func TestLoadGlobalConfigRejectsInvalidTarget(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := Default(project, home)
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: global\n    target: Open Claw\n    path: ~/.openclaw/skills\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGlobal(cfg)
	if err == nil || !strings.Contains(err.Error(), `invalid target "Open Claw"`) {
		t.Fatalf("err = %v, want invalid target", err)
	}
}
```

Add imports: `os`, `strings`.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/config -count=1 -run 'TestLoadGlobalConfig'
```

Expected: FAIL because `LoadGlobal` is undefined.

- [ ] **Step 3: Implement config parsing and merge**

In `internal/config/config.go`, add imports:

```go
import (
	"errors"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)
```

Add config fields:

```go
type Config struct {
	ProjectRoot      string
	HomeDir          string
	ArchiveRoot      string
	GlobalAgentsRoot string
	GlobalClaudeRoot string
	GlobalCodexRoot  string
	managedRoots     []ManagedRoot
}
```

Add YAML model and loader:

```go
type fileConfig struct {
	Version     int              `yaml:"version"`
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

func LoadGlobal(cfg Config) (Config, error) {
	path := filepath.Join(cfg.ArchiveRoot, "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg.managedRoots = cfg.defaultManagedRoots()
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}
	var parsed fileConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}
	if parsed.Version != 0 && parsed.Version != 1 {
		return Config{}, fmt.Errorf("load config %s: unsupported version %d", path, parsed.Version)
	}
	roots := cfg.defaultManagedRoots()
	for i, entry := range parsed.ActiveRoots {
		root, err := cfg.managedRootFromYAML(entry)
		if err != nil {
			return Config{}, fmt.Errorf("load config %s: active_roots[%d]: %w", path, i, err)
		}
		roots = upsertManagedRoot(roots, root)
	}
	cfg.managedRoots = roots
	return cfg, nil
}
```

Add helpers:

```go
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

func (c Config) managedRootFromYAML(entry activeRootYAML) (ManagedRoot, error) {
	scope := strings.TrimSpace(entry.Scope)
	target := strings.TrimSpace(entry.Target)
	if scope != ScopeProject && scope != ScopeGlobal {
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
		if strings.TrimSpace(entry.Path) == "" {
			return ManagedRoot{}, fmt.Errorf("path is required when enabled is true")
		}
		root.Path = c.expandManagedRootPath(scope, entry.Path)
	}
	if root.Label == "" {
		root.Label = defaultManagedRootLabel(scope, target)
	}
	return root, nil
}

func (c Config) expandManagedRootPath(scope, value string) string {
	pathValue := strings.TrimSpace(value)
	if strings.HasPrefix(pathValue, "~/") {
		return filepath.Join(c.HomeDir, filepath.FromSlash(strings.TrimPrefix(pathValue, "~/")))
	}
	if filepath.IsAbs(pathValue) {
		return filepath.Clean(pathValue)
	}
	if scope == ScopeProject {
		return filepath.Join(c.ProjectRoot, filepath.FromSlash(pathValue))
	}
	return filepath.Join(c.HomeDir, filepath.FromSlash(pathValue))
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
```

Update `ManagedRoots`:

```go
func (c Config) ManagedRoots() []ManagedRoot {
	if c.managedRoots == nil {
		return c.defaultManagedRoots()
	}
	return append([]ManagedRoot(nil), c.managedRoots...)
}
```

Update `ActiveRoot` to search `ManagedRoots()` first and return an error for disabled/missing targets.

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/config -count=1
```

Expected: PASS.

### Task 3: Dynamic Roots Expansion

**Files:**
- Modify: `internal/roots/roots.go`
- Modify: `internal/roots/roots_test.go`

- [ ] **Step 1: Write failing roots tests**

Add to `internal/roots/roots_test.go`:

```go
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
```

Add imports: `os`, `path/filepath`.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/roots -count=1 -run TestActiveRootsIncludesConfiguredRootsAndSkipsDisabled
```

Expected: FAIL until `ActiveRoots` uses `cfg.ManagedRoots()`.

- [ ] **Step 3: Implement dynamic root expansion**

In `internal/roots/roots.go`, replace loops over `config.Scopes` and `config.Targets` with:

```go
func ActiveRoots(cfg config.Config, filter Filter) []ActiveRoot {
	if filter.Scope != "" && filter.Scope != config.ScopeProject && filter.Scope != config.ScopeGlobal {
		return nil
	}

	var roots []ActiveRoot
	for _, managed := range cfg.ManagedRoots() {
		if !managed.Enabled {
			continue
		}
		if filter.Scope != "" && managed.Scope != filter.Scope {
			continue
		}
		if filter.Target != "" && managed.Target != filter.Target {
			continue
		}
		roots = append(roots, ActiveRoot{
			Scope:  managed.Scope,
			Target: managed.Target,
			Path:   managed.Path,
			Label:  managed.Label,
		})
	}
	return roots
}
```

Remove the `slices` import if unused.

- [ ] **Step 4: Run roots tests**

Run:

```bash
go test ./internal/roots -count=1
```

Expected: PASS.

### Task 4: Load Config In CLI And Add `list-roots`

**Files:**
- Modify: `internal/cli/root.go`
- Create: `internal/cli/list_roots.go`
- Create: `internal/cli/list_roots_test.go`

- [ ] **Step 1: Write failing CLI tests**

Create `internal/cli/list_roots_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListRootsShowsConfiguredRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "list-roots"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, ".Oc") || !strings.Contains(output, "project:opencode") || !strings.Contains(output, filepath.Join(project, ".opencode", "skills")) {
		t.Fatalf("output missing configured root:\n%s", output)
	}
}

func TestListRootsJSON(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "--json", "list-roots"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Roots []struct {
			Location string `json:"location"`
			Scope    string `json:"scope"`
			Target   string `json:"target"`
			Label    string `json:"label"`
			Path     string `json:"path"`
			Builtin  bool   `json:"builtin"`
			Enabled  bool   `json:"enabled"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if len(payload.Roots) != 6 {
		t.Fatalf("len(roots) = %d, want 6", len(payload.Roots))
	}
	if payload.Roots[0].Location == "" || payload.Roots[0].Path == "" {
		t.Fatalf("first root = %#v", payload.Roots[0])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli -count=1 -run TestListRoots
```

Expected: FAIL because `list-roots` is undefined.

- [ ] **Step 3: Load global config in `options.config`**

In `internal/cli/root.go`, update `options.config()`:

```go
func (o options) config() config.Config {
	cfg := config.Default(o.projectRoot, o.homeDir)
	if o.flags != nil && o.flags.Changed("archive-root") {
		cfg.ArchiveRoot = o.archiveRoot
	}
	loaded, err := config.LoadGlobal(cfg)
	if err != nil {
		panic(err)
	}
	return loaded
}
```

Then replace panic with proper error handling by adding:

```go
func (o options) configE() (config.Config, error) {
	cfg := config.Default(o.projectRoot, o.homeDir)
	if o.flags != nil && o.flags.Changed("archive-root") {
		cfg.ArchiveRoot = o.archiveRoot
	}
	return config.LoadGlobal(cfg)
}
```

Keep `config()` as a panic wrapper only for tests/helpers that cannot return errors:

```go
func (o options) config() config.Config {
	cfg, err := o.configE()
	if err != nil {
		panic(err)
	}
	return cfg
}
```

For command handlers touched in this plan, call `configE`.

- [ ] **Step 4: Implement `list-roots`**

Create `internal/cli/list_roots.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/spf13/cobra"
)

type listRootsPayload struct {
	Roots []listRootEntry `json:"roots"`
}

type listRootEntry struct {
	Location string `json:"location"`
	Scope    string `json:"scope"`
	Target   string `json:"target"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Builtin  bool   `json:"builtin"`
	Enabled  bool   `json:"enabled"`
}

func newListRootsCommand(rootOptions *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list-roots",
		Short: "List managed active skill roots",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := rootOptions.configE()
			if err != nil {
				return err
			}
			return writeListRoots(cmd.OutOrStdout(), cfg.ManagedRoots(), rootOptions.json)
		},
	}
}

func writeListRoots(out io.Writer, roots []config.ManagedRoot, asJSON bool) error {
	payload := listRootsPayload{Roots: make([]listRootEntry, 0, len(roots))}
	for _, root := range roots {
		if !root.Enabled {
			continue
		}
		payload.Roots = append(payload.Roots, listRootEntry{
			Location: root.Location(),
			Scope:    root.Scope,
			Target:   root.Target,
			Label:    root.Label,
			Path:     root.Path,
			Builtin:  root.Builtin,
			Enabled:  root.Enabled,
		})
	}
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	}
	for _, root := range payload.Roots {
		if _, err := fmt.Fprintf(out, "%-8s  %-18s  %s\n", root.Label, root.Location, root.Path); err != nil {
			return err
		}
	}
	return nil
}
```

Register in `newRootCommand`:

```go
newListRootsCommand(&opts),
```

- [ ] **Step 5: Run CLI tests**

Run:

```bash
go test ./internal/cli -count=1 -run TestListRoots
```

Expected: PASS.

### Task 5: Documentation And Backlog Cleanup

**Files:**
- Modify: `README.md`
- Modify: `docs/backlog.md`

- [ ] **Step 1: Update README**

Add under roots documentation:

```markdown
Custom managed roots live in `~/.x-skills/config.yaml`:

```yaml
version: 1
active_roots:
  - scope: global
    target: hermes
    path: ~/.config/hermes/skills
    label: ~Hm
  - scope: global
    target: claude
    enabled: false
```

Use `x-skills list-roots --json` for agent-readable root discovery.
```

- [ ] **Step 2: Update backlog**

Replace the broad managed-agent registry item in `docs/backlog.md` with two remaining precise items if they are not implemented yet:

```markdown
- Migrate root selection flags to `--at`. Context: registry core supports dynamic roots, but commands still expose older `--target`/`--project`/`--global` selectors until the selector migration plan is implemented. Evidence: `internal/cli`.
- Adapt TUI destination controls to dynamic managed roots. Context: registry core supports dynamic roots, but TUI destination checklists and labels still assume the built-in roots until the TUI adaptation plan is implemented. Evidence: `internal/tui/install.go`, `internal/tui/actions.go`.
```

- [ ] **Step 3: Run verification**

Run:

```bash
go test ./cmd/x-skills ./internal/... -count=1
go build -o bin/x-skills ./cmd/x-skills
git diff --check
```

Expected: all commands exit 0.
