# Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Go vertical slice of `x-skills`: cwd-based active/repo scanning, list/repo/link/migrate/unlink/doctor commands, and a Bubble Tea guided-manager TUI.

**Architecture:** Keep the Python implementation as a reference while adding a Go module beside it. Put all filesystem/domain behavior in internal packages used by both Cobra CLI commands and Bubble Tea models. CLI and TUI must not duplicate mutation logic.

**Tech Stack:** Go, Cobra, Bubble Tea, Bubbles, Lipgloss, standard library filesystem APIs, Go tests with temporary directories.

---

## File Structure

- Create `go.mod` and `go.sum`: Go module and Charm/Cobra dependencies.
- Create `cmd/x-skills/main.go`: executable entrypoint after `internal/cli.Execute` exists.
- Create `internal/config/config.go`: root path configuration, env var defaults, target/scope constants.
- Create `internal/skills/skill.go`: skill directory validation and `SKILL.md` frontmatter description parsing.
- Create `internal/roots/roots.go`: active root expansion and location labels.
- Create `internal/repo/repo.go`: archived repo skill listing and managed path helpers.
- Create `internal/fingerprint/fingerprint.go`: stable directory content SHA for grouping.
- Create `internal/actions/actions.go`: link, migrate, unlink, batch results, and typed operation outcomes.
- Create `internal/doctor/doctor.go`: diagnostics and safe fixes.
- Create `internal/cli/root.go`: Cobra root command, global flags, IO streams.
- Create `internal/cli/list.go`, `repo.go`, `link.go`, `migrate.go`, `unlink.go`, `doctor.go`, `tui.go`: subcommand wiring.
- Create `internal/tui/model.go`, `views.go`, `wizard.go`, `styles.go`: Bubble Tea guided manager.
- Create Go test files next to the packages they verify.
- Modify `README.md`: document experimental Go branch commands after implementation works.

## Task 1: Go Module And Config Foundations

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing config tests**

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/config`

Expected: FAIL because `internal/config` does not exist.

- [ ] **Step 3: Add the Go module**

```go
module github.com/InkyQuill/x-skills

go 1.23
```

Run: `go get github.com/spf13/cobra github.com/charmbracelet/bubbletea github.com/charmbracelet/bubbles github.com/charmbracelet/lipgloss`

Expected: dependencies are added to `go.mod` and `go.sum`.

- [ ] **Step 4: Implement config**

```go
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
```

- [ ] **Step 5: Run config tests**

Run: `go test ./internal/config`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/config.go internal/config/config_test.go
git commit -m "feat: add go module config"
```

## Task 2: Skill Parsing, Roots, Repo, And Fingerprints

**Files:**
- Create: `internal/skills/skill.go`
- Create: `internal/skills/skill_test.go`
- Create: `internal/roots/roots.go`
- Create: `internal/roots/roots_test.go`
- Create: `internal/repo/repo.go`
- Create: `internal/repo/repo_test.go`
- Create: `internal/fingerprint/fingerprint.go`
- Create: `internal/fingerprint/fingerprint_test.go`

- [ ] **Step 1: Write skill parsing tests**

```go
package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSkillDescription(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "react-state")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: react-state\ndescription: Manage React state.\n---\n# Body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Read(skill)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "react-state" {
		t.Fatalf("Name = %q", info.Name)
	}
	if info.Description != "Manage React state." {
		t.Fatalf("Description = %q", info.Description)
	}
}

func TestValidateRejectsMissingSkillMD(t *testing.T) {
	dir := t.TempDir()
	_, err := Read(dir)
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
}
```

- [ ] **Step 2: Write roots and repo tests**

```go
package roots

import (
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
	if filtered[0].Label != "./.codex" {
		t.Fatalf("Label = %q", filtered[0].Label)
	}
}
```

```go
package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestListRepoSkills(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	skill := filepath.Join(cfg.ArchiveSkillsRoot(), "golang-testing")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: golang-testing\ndescription: Test Go.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "golang-testing" {
		t.Fatalf("skills = %#v", skills)
	}
}
```

- [ ] **Step 3: Write fingerprint tests**

```go
package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryFingerprintIgnoresWalkOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("fingerprints differ: %q %q", first, second)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/skills ./internal/roots ./internal/repo ./internal/fingerprint`

Expected: FAIL because packages are not implemented.

- [ ] **Step 5: Implement focused packages**

Implement these public types and functions:

```go
// internal/skills
type Info struct {
	Name        string
	Path        string
	Description string
}
func Read(path string) (Info, error)
func IsDir(path string) bool

// internal/roots
type ActiveRoot struct {
	Scope  string
	Target string
	Path   string
	Label  string
}
type Filter struct {
	Scope  string
	Target string
}
func ActiveRoots(cfg config.Config, filter Filter) []ActiveRoot

// internal/repo
type Skill struct {
	Name        string
	Path        string
	Description string
}
func List(cfg config.Config) ([]Skill, error)
func SkillPath(cfg config.Config, name string) string

// internal/fingerprint
func Directory(root string) (string, error)
```

Use deterministic directory and file ordering in `fingerprint.Directory`.

- [ ] **Step 6: Run package tests**

Run: `go test ./internal/skills ./internal/roots ./internal/repo ./internal/fingerprint`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/skills internal/roots internal/repo internal/fingerprint
git commit -m "feat: scan skills and roots in go"
```

## Task 3: Active Skill Status And List Command

**Files:**
- Create: `internal/actions/scan.go`
- Create: `internal/actions/scan_test.go`
- Create: `cmd/x-skills/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/list.go`
- Create: `internal/cli/test_helpers_test.go`
- Create: `internal/cli/list_test.go`

- [ ] **Step 1: Write active scan tests**

```go
package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func makeSkill(t *testing.T, root, name, desc string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScanActiveStatusesAndBrokenReasons(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	managed := makeSkill(t, cfg.ArchiveSkillsRoot(), "managed-codex", "Managed.")
	codexRoot := cfg.ActiveRoot("project", "codex")
	if err := os.MkdirAll(codexRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, filepath.Join(codexRoot, "managed-codex")); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, cfg.ActiveRoot("project", "claude"), "local-claude", "Local.")
	globalAgents := cfg.ActiveRoot("global", "agents")
	if err := os.MkdirAll(globalAgents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(globalAgents, "broken-agents")); err != nil {
		t.Fatal(err)
	}

	skills, err := ScanActive(cfg, ScanFilter{})
	if err != nil {
		t.Fatal(err)
	}

	status := map[string]string{}
	reason := map[string]string{}
	for _, skill := range skills {
		status[skill.Name] = skill.Status
		reason[skill.Name] = skill.Reason
	}
	if status["managed-codex"] != StatusManaged {
		t.Fatalf("managed-codex status = %q", status["managed-codex"])
	}
	if status["local-claude"] != StatusUnmanaged {
		t.Fatalf("local-claude status = %q", status["local-claude"])
	}
	if status["broken-agents"] != StatusBroken {
		t.Fatalf("broken-agents status = %q", status["broken-agents"])
	}
	if reason["broken-agents"] == "" {
		t.Fatal("missing broken reason")
	}
}
```

- [ ] **Step 2: Write CLI list test**

Create `internal/cli/test_helpers_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func makeSkill(t *testing.T, root, name, desc string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
```

Create `internal/cli/list_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListShowsStatuses(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	archive := filepath.Join(home, ".x-skills", "skills")
	managed := makeSkill(t, archive, "managed-codex", "Managed codex skill.")
	root := filepath.Join(project, ".codex", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, filepath.Join(root, "managed-codex")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{
		"--project-root", project,
		"--home", home,
		"list", "--project", "--target", "codex",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"PROJECT codex", "./.codex", "managed-codex", "managed", "Managed codex skill."} {
		if !strings.Contains(text, want) {
			t.Fatalf("list output missing %q:\n%s", want, text)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/actions ./internal/cli`

Expected: FAIL because scan and CLI are not implemented.

- [ ] **Step 4: Implement scan and list**

Implement these types and functions:

```go
const (
	StatusManaged   = "managed"
	StatusUnmanaged = "unmanaged"
	StatusBroken    = "broken"
)

type ActiveSkill struct {
	Name        string
	Root        roots.ActiveRoot
	Path        string
	Status      string
	Description string
	Reason      string
}

type ScanFilter struct {
	Scope  string
	Target string
}

func ScanActive(cfg config.Config, filter ScanFilter) ([]ActiveSkill, error)
```

Implement Cobra root with global flags:

```go
func Execute(argv []string, stdin io.Reader, stdout, stderr io.Writer) error
```

Add hidden test-only `--home` and public `--project-root`, `--archive-root`,
`--global-root`, `--claude-global-root`, `--codex-global-root` flags. Add
`list --project --global --target`.

- [ ] **Step 5: Add the main entrypoint**

Create `cmd/x-skills/main.go`:

```go
package main

import (
	"os"

	"github.com/InkyQuill/x-skills/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(2)
	}
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/actions ./internal/cli`

Expected: PASS.

- [ ] **Step 7: Smoke-test command**

Run: `go run ./cmd/x-skills list --project --target agents`

Expected: command exits 0 and either lists project agents skills or prints that no active skills were found.

- [ ] **Step 8: Commit**

```bash
git add cmd/x-skills/main.go internal/actions/scan.go internal/actions/scan_test.go internal/cli
git commit -m "feat: add go list command"
```

## Task 4: Repo Command And Batch Link

**Files:**
- Modify: `internal/repo/repo.go`
- Create: `internal/actions/link.go`
- Create: `internal/actions/link_test.go`
- Create: `internal/cli/repo.go`
- Create: `internal/cli/link.go`
- Create: `internal/cli/repo_link_test.go`

- [ ] **Step 1: Write action tests for link**

```go
package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestLinkRepoSkillCreatesSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")

	result, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != filepath.Join(project, ".codex", "skills", "typescript-expert") {
		t.Fatalf("Path = %q", result.Path)
	}
	resolved, err := filepath.EvalSymlinks(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != source {
		t.Fatalf("resolved = %q, want %q", resolved, source)
	}
}

func TestLinkFailsWhenDestinationExists(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "typescript-expert", "TS.")
	makeSkill(t, cfg.ActiveRoot("project", "codex"), "typescript-expert", "Existing.")

	_, err := Link(cfg, LinkRequest{Name: "typescript-expert", Scope: "project", Target: "codex"})
	if err == nil {
		t.Fatal("expected destination exists error")
	}
}
```

- [ ] **Step 2: Write CLI repo/link tests**

```go
package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoListsArchivedSkills(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "unused-skill", "Not linked.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "repo"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unused-skill") || !strings.Contains(out.String(), "Not linked.") {
		t.Fatalf("repo output:\n%s", out.String())
	}
}

func TestLinkAcceptsMultipleNames(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	archive := filepath.Join(home, ".x-skills", "skills")
	first := makeSkill(t, archive, "first-skill", "First.")
	second := makeSkill(t, archive, "second-skill", "Second.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "link", "first-skill", "second-skill", "--project", "--target", "codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Summary:") || !strings.Contains(out.String(), "linked: first-skill, second-skill") {
		t.Fatalf("link output:\n%s", out.String())
	}
	for name, source := range map[string]string{"first-skill": first, "second-skill": second} {
		target := filepath.Join(project, ".codex", "skills", name)
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatal(err)
		}
		if resolved != source {
			t.Fatalf("%s resolved to %q, want %q", name, resolved, source)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/actions ./internal/cli`

Expected: FAIL because link and repo commands are not implemented.

- [ ] **Step 4: Implement repo command and link action**

Add:

```go
type LinkRequest struct {
	Name   string
	Scope  string
	Target string
}

type MutationResult struct {
	Name string
	Path string
}

func Link(cfg config.Config, req LinkRequest) (MutationResult, error)
```

`Link` validates that the repo skill exists, creates the destination root, and
creates a symlink. It must fail if destination exists.

- [ ] **Step 5: Run tests and smoke-test**

Run: `go test ./internal/actions ./internal/cli`

Expected: PASS.

Run: `go run ./cmd/x-skills repo`

Expected: command exits 0 and lists repo skills or no output if the repo is empty.

- [ ] **Step 6: Commit**

```bash
git add internal/repo internal/actions/link.go internal/actions/link_test.go internal/cli/repo.go internal/cli/link.go internal/cli/repo_link_test.go
git commit -m "feat: add go repo and link commands"
```

## Task 5: Migrate And Unlink Operations

**Files:**
- Create: `internal/actions/migrate.go`
- Create: `internal/actions/unlink.go`
- Create: `internal/actions/migrate_unlink_test.go`
- Create: `internal/cli/migrate.go`
- Create: `internal/cli/unlink.go`
- Create: `internal/cli/migrate_unlink_test.go`

- [ ] **Step 1: Write migrate/unlink action tests**

```go
package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestMigrateMovesDirectoryToRepoAndLinksBack(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.ActiveRoot("project", "codex"), "next-best-practices", "Next.")

	result, err := Migrate(cfg, MigrateRequest{Name: "next-best-practices", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(cfg.ArchiveSkillsRoot(), "next-best-practices")
	if result.Path != archived {
		t.Fatalf("Path = %q, want %q", result.Path, archived)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
}

func TestUnlinkManagedRemovesSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")
	root := cfg.ActiveRoot("project", "codex")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "opentui-react")
	if err := os.Symlink(source, active); err != nil {
		t.Fatal(err)
	}

	_, err := Unlink(cfg, UnlinkRequest{Name: "opentui-react", Scope: "project", Target: "codex", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}

func TestUnlinkUnmanagedDeleteRemovesActiveDirectory(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.ActiveRoot("project", "codex"), "local-only", "Local.")

	_, err := Unlink(cfg, UnlinkRequest{Name: "local-only", Scope: "project", Target: "codex", DeleteUnmanaged: true, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
}
```

- [ ] **Step 2: Write CLI migrate/unlink tests**

```go
package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateWithYesFlag(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "migrate", "local-only", "--project", "--target", "codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(home, ".x-skills", "skills", "local-only")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("resolved = %q, want %q", resolved, archived)
	}
}

func TestUnlinkUnmanagedDeleteWithYes(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "local-only", "--project", "--target", "codex", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), "removed unmanaged") {
		t.Fatalf("unlink output:\n%s", out.String())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/actions ./internal/cli`

Expected: FAIL because migrate and unlink are not implemented.

- [ ] **Step 4: Implement migrate and unlink**

Add:

```go
type MigrateRequest struct {
	Name      string
	Scope     string
	Target    string
	Confirmed bool
}

type UnlinkRequest struct {
	Name            string
	Scope           string
	Target          string
	Confirmed       bool
	DeleteUnmanaged bool
}

func Migrate(cfg config.Config, req MigrateRequest) (MutationResult, error)
func Unlink(cfg config.Config, req UnlinkRequest) (MutationResult, error)
```

Use `os.Rename` for moving an unmanaged active directory into the archive and
`os.Symlink` to link it back. Use `os.Remove` for symlink unlink and
`os.RemoveAll` only for `DeleteUnmanaged` after confirmation.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/actions ./internal/cli`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/migrate.go internal/actions/unlink.go internal/actions/migrate_unlink_test.go internal/cli/migrate.go internal/cli/unlink.go internal/cli/migrate_unlink_test.go
git commit -m "feat: add go migrate and unlink commands"
```

## Task 6: Doctor Diagnostics And Safe Fixes

**Files:**
- Create: `internal/doctor/doctor.go`
- Create: `internal/doctor/doctor_test.go`
- Create: `internal/cli/doctor.go`
- Create: `internal/cli/doctor_test.go`

- [ ] **Step 1: Write doctor package tests**

```go
package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func makeSkill(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiagnoseReportsBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.ActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(root, "chapter-drafter")); err != nil {
		t.Fatal(err)
	}

	issues, err := Diagnose(cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].Kind != KindBrokenSymlink {
		t.Fatalf("Kind = %q", issues[0].Kind)
	}
}

func TestFixBrokenSymlinkRelinksWhenRepoSkillExists(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	repoSkill := makeSkill(t, cfg.ArchiveSkillsRoot(), "chapter-drafter")
	root := cfg.ActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	results, err := Fix(cfg, FixOptions{Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "relinked" {
		t.Fatalf("results = %#v", results)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != repoSkill {
		t.Fatalf("resolved = %q, want %q", resolved, repoSkill)
	}
}

func TestFixBrokenSymlinkRemovesWhenRepoSkillMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.ActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	results, err := Fix(cfg, FixOptions{Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "removed" {
		t.Fatalf("results = %#v", results)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link still exists or unexpected err: %v", err)
	}
}
```

- [ ] **Step 2: Write CLI doctor tests**

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsAndFixesBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "doctor"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "broken") || !strings.Contains(out.String(), "chapter-drafter") {
		t.Fatalf("doctor output:\n%s", out.String())
	}

	out.Reset()
	err = Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link still exists or unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("fix output:\n%s", out.String())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/doctor ./internal/cli`

Expected: FAIL because doctor is not implemented.

- [ ] **Step 4: Implement doctor**

Add:

```go
const KindBrokenSymlink = "broken-symlink"

type Filter struct {
	Scope  string
	Target string
}

type Issue struct {
	Kind       string
	Name       string
	Location   string
	Path       string
	Reason     string
	SafeFix    string
	RepoTarget string
}

type FixOptions struct {
	Yes bool
}

type FixResult struct {
	Name   string
	Action string
	Path   string
}

func Diagnose(cfg config.Config, filter Filter) ([]Issue, error)
func Fix(cfg config.Config, opts FixOptions) ([]FixResult, error)
```

For the first slice, implement broken symlink diagnosis and safe fixes. Keep
the type shape open for invalid repo skills and mislinked symlinks.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/doctor ./internal/cli`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/doctor internal/cli/doctor.go internal/cli/doctor_test.go
git commit -m "feat: add go doctor fixes"
```

## Task 7: TUI Model, Views, And Wizard

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/views.go`
- Create: `internal/tui/wizard.go`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/model_test.go`
- Create: `internal/cli/tui.go`

- [ ] **Step 1: Write TUI model tests**

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelSwitchesViews(t *testing.T) {
	m := NewTestModel()
	if m.ViewName() != "active" {
		t.Fatalf("initial view = %q", m.ViewName())
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = next.(Model)
	if m.ViewName() != "repo" {
		t.Fatalf("view after r = %q", m.ViewName())
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = next.(Model)
	if m.ViewName() != "doctor" {
		t.Fatalf("view after d = %q", m.ViewName())
	}
}

func TestWizardPreviewIncludesDestination(t *testing.T) {
	m := NewTestModel()
	m.selected = map[string]bool{"repo:react-state": true}
	m.view = viewRepo

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = next.(Model)
	if !m.wizard.Open {
		t.Fatal("wizard is not open")
	}
	if m.wizard.Preview == "" {
		t.Fatal("wizard preview is empty")
	}
	if !strings.Contains(m.wizard.Preview, "./.agents") {
		t.Fatalf("preview missing default destination: %q", m.wizard.Preview)
	}
}
```

- [ ] **Step 2: Run TUI tests to verify they fail**

Run: `go test ./internal/tui`

Expected: FAIL because `internal/tui` is not implemented.

- [ ] **Step 3: Implement model state and view switching**

Add a Bubble Tea model with:

```go
type Model struct {
	cfg      config.Config
	view     viewName
	selected map[string]bool
	wizard   Wizard
}

type Wizard struct {
	Open    bool
	Action  string
	Preview string
}

func New(cfg config.Config) Model
func NewTestModel() Model
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
func (m Model) ViewName() string
```

Use keys:

- `a`: active view;
- `r`: repo view;
- `d`: doctor view;
- `i`: install/link wizard from repo view;
- `m`: migrate wizard from active view;
- `u`: unlink wizard from active view;
- `f`: doctor fix wizard from doctor view;
- `q`: quit.

- [ ] **Step 4: Implement guided visual rendering**

Use Lipgloss styles in `styles.go` for:

- selected card border;
- `managed`, `unmanaged`, `broken` status colors;
- path-like chips such as `./.agents` and `~/.claude`;
- wizard panel with preview text.

The default screen should render without panics even when no skills exist.

- [ ] **Step 5: Wire `x-skills tui`**

Create `internal/cli/tui.go` with a Cobra command that rejects `--no-input`,
constructs config, and runs:

```go
p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
_, err := p.Run()
return err
```

- [ ] **Step 6: Run TUI and CLI tests**

Run: `go test ./internal/tui ./internal/cli`

Expected: PASS.

- [ ] **Step 7: Smoke-test TUI startup**

Run: `go run ./cmd/x-skills tui`

Expected: full-screen TUI opens and can quit with `q`.

- [ ] **Step 8: Commit**

```bash
git add internal/tui internal/cli/tui.go
git commit -m "feat: add go tui manager"
```

## Task 8: Integration Pass, Docs, And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-07-04-go-rewrite-design.md` only if implementation reveals a design correction.

- [ ] **Step 1: Run full Go verification**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run Python reference tests**

Run: `uv run pytest`

Expected: PASS. If failures are unrelated to Go files, inspect before changing.

- [ ] **Step 3: Update README with experimental Go usage**

Add a short development section:

````markdown
### Experimental Go rewrite

The `go-rewrite-prototype` branch contains an experimental Go implementation.
Run it locally with:

```bash
go run ./cmd/x-skills list
go run ./cmd/x-skills repo
go run ./cmd/x-skills tui
```

The Python implementation remains the reference until Go reaches accepted
parity. The public one-liner installer on `main` still installs the Python
package during the rewrite.
````

- [ ] **Step 4: Run documentation-related tests**

Run: `uv run pytest tests/test_install_docs.py`

Expected: PASS.

- [ ] **Step 5: Run formatting**

Run: `gofmt -w cmd internal`

Expected: no output.

- [ ] **Step 6: Run final verification**

Run: `go test ./...`

Expected: PASS.

Run: `uv run pytest`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add README.md cmd internal go.mod go.sum
git commit -m "docs: document go rewrite prototype"
```

## Self-Review Notes

- Spec coverage: the plan covers the approved operation slice, Cobra/Charm stack, path-like location labels, hidden SHA in normal UI, `doctor --fix`, Python-as-reference migration, and deferred remote search/GitHub/update installer work.
- Scope: the plan intentionally does not implement skills.sh search, GitHub install/update metadata, or final release installer.
- Risk: the TUI task depends on enough core behavior existing first; it is intentionally placed after CLI operations and doctor.
