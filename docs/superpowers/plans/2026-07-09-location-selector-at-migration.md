# Location Selector `--at` Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `--target`/`--project`/`--global` and `add --to` with a single repeatable `--at` location selector across CLI commands.

**Architecture:** Add selector parsing in `internal/roots` or `internal/cli` using the dynamic managed roots registry from the core plan. All command handlers resolve `--at` into concrete `roots.ActiveRoot` values before calling `actions`.

**Tech Stack:** Go 1.26, Cobra, existing `config`, `roots`, `actions`, and CLI packages.

---

## Files

- Create: `internal/cli/location.go` for `--at` parsing and command helpers.
- Create: `internal/cli/location_test.go` for canonical and alias selectors.
- Modify: `internal/cli/add.go` to replace `--to` with `--at`.
- Modify: `internal/cli/link.go`, `internal/cli/migrate.go`, `internal/cli/unlink.go`, `internal/cli/list.go`, `internal/cli/doctor.go` to remove `--target`, `--project`, `--global`.
- Modify: related CLI tests to use `--at`.
- Modify: `README.md`, `/home/inky/.x-skills/skills/find-skills/SKILL.md`, and `/home/inky/.x-skills/skills/manage-skills/SKILL.md`.
- Modify: `docs/backlog.md` after verification.

### Task 1: Location Selector Parser

**Files:**
- Create: `internal/cli/location.go`
- Create: `internal/cli/location_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/cli/location_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
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
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli -count=1 -run TestResolveLocations
```

Expected: FAIL because `resolveLocations` is undefined.

- [ ] **Step 3: Implement parser**

Create `internal/cli/location.go`:

```go
package cli

import (
	"fmt"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func resolveLocations(cfg config.Config, selectors []string) ([]roots.ActiveRoot, error) {
	if len(selectors) == 0 {
		return nil, fmt.Errorf("at least one --at location is required")
	}
	var result []roots.ActiveRoot
	for _, selector := range selectors {
		root, err := resolveLocation(cfg, selector)
		if err != nil {
			return nil, err
		}
		result = append(result, root)
	}
	return result, nil
}

func resolveLocation(cfg config.Config, selector string) (roots.ActiveRoot, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		return roots.ActiveRoot{}, fmt.Errorf("empty --at location")
	}
	all := roots.ActiveRoots(cfg, roots.Filter{})
	var matches []roots.ActiveRoot
	for _, root := range all {
		if trimmed == root.Scope+":"+root.Target ||
			strings.EqualFold(trimmed, root.Label) ||
			trimmed == scopePrefix(root.Scope)+root.Target {
			matches = append(matches, root)
		}
	}
	if len(matches) == 0 {
		return roots.ActiveRoot{}, fmt.Errorf("unknown --at location %q; run x-skills list-roots", selector)
	}
	if len(matches) > 1 {
		return roots.ActiveRoot{}, fmt.Errorf("ambiguous --at location %q; use project:target or global:target", selector)
	}
	return matches[0], nil
}

func scopePrefix(scope string) string {
	if scope == config.ScopeGlobal {
		return "~"
	}
	return "."
}
```

- [ ] **Step 4: Run parser tests**

Run:

```bash
go test ./internal/cli -count=1 -run TestResolveLocations
```

Expected: PASS.

### Task 2: Replace Mutation Command Flags

**Files:**
- Modify: `internal/cli/add.go`
- Modify: `internal/cli/link.go`
- Modify: `internal/cli/migrate.go`
- Modify: `internal/cli/unlink.go`
- Modify: command tests under `internal/cli/*_test.go`

- [ ] **Step 1: Write failing command tests for `--at`**

Update representative tests:

```go
err := Execute([]string{
	"--home", home,
	"--project-root", project,
	"add", "--git", repo, "alpha-skill", "--at", ".Ag", "-y",
}, strings.NewReader(""), &out, &bytes.Buffer{})
```

```go
err := Execute([]string{
	"--home", home,
	"--project-root", project,
	"link", "typescript-expert", "--at", "project:codex", "-y",
}, strings.NewReader(""), &out, &bytes.Buffer{})
```

```go
err := Execute([]string{
	"--home", home,
	"--project-root", project,
	"migrate", "local-only", "--at", "project:codex", "-y",
}, strings.NewReader(""), &out, &bytes.Buffer{})
```

```go
err := Execute([]string{
	"--home", home,
	"--project-root", project,
	"unlink", "local-only", "--at", "project:codex", "--delete-unmanaged", "-y",
}, strings.NewReader(""), &out, &bytes.Buffer{})
```

- [ ] **Step 2: Run selected CLI tests to verify they fail**

Run:

```bash
go test ./internal/cli -count=1 -run 'TestAddArchivesAndLinksDefaultProjectAgents|TestLink|TestMigrate|TestUnlink'
```

Expected: FAIL where commands do not recognize `--at`.

- [ ] **Step 3: Replace flags in command structs**

For each mutation command:

- Add `at []string` to the command options.
- Register `cmd.Flags().StringArrayVar(&opts.at, "at", nil, "managed root location; repeat for multiple locations")`.
- Remove `--target`, `--project`, `--global`, and `--to`.
- Resolve locations with `resolveLocations(rootOptions.config(), opts.at)`.
- For commands that require one location (`migrate`, `unlink` for a single active root), error when `len(locations) != 1`.
- For commands that support multiple locations (`add`, `link`), iterate over all resolved locations.

Use this mapping:

```go
actions.LinkRequest{
	Name:   name,
	Scope:  location.Scope,
	Target: location.Target,
}
```

Do not use `location.Path` in action requests yet; actions still resolve by scope/target through config.

- [ ] **Step 4: Run CLI tests**

Run:

```bash
go test ./internal/cli -count=1
```

Expected: PASS after all tests are migrated.

### Task 3: Replace Read-Only Command Filters

**Files:**
- Modify: `internal/cli/list.go`
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/list_test.go`
- Modify: `internal/cli/doctor_test.go`

- [ ] **Step 1: Write/adjust tests for `list --at` and `doctor --at`**

Use:

```go
err := Execute([]string{
	"--home", home,
	"--project-root", project,
	"list", "--at", "project:codex",
}, strings.NewReader(""), &out, &bytes.Buffer{})
```

Use:

```go
err := Execute([]string{
	"--home", home,
	"--project-root", project,
	"doctor", "--at", ".Cl",
}, strings.NewReader(""), &out, &bytes.Buffer{})
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli -count=1 -run 'TestList|TestDoctor'
```

Expected: FAIL until `--at` is wired.

- [ ] **Step 3: Implement read-only `--at` filters**

For `list` and `doctor`, accept a single `--at` for now. If multiple `--at` values are provided, scan all listed roots. Convert locations into `actions.ScanFilter` or `doctor.Filter` only when one location is selected; otherwise add a small multi-root scan helper that filters by root set.

Recommended minimal approach:

```go
locations, err := resolveLocations(cfg, opts.at)
if err != nil {
	return err
}
allowed := map[string]bool{}
for _, location := range locations {
	allowed[location.Scope+":"+location.Target] = true
}
```

Filter returned active skills/issues by `skill.Root.Scope+":"+skill.Root.Target`.

- [ ] **Step 4: Run read-only tests**

Run:

```bash
go test ./internal/cli -count=1 -run 'TestList|TestDoctor'
```

Expected: PASS.

### Task 4: Documentation And Backlog Cleanup

**Files:**
- Modify: `README.md`
- Modify: `/home/inky/.x-skills/skills/find-skills/SKILL.md`
- Modify: `/home/inky/.x-skills/skills/manage-skills/SKILL.md`
- Modify: `docs/backlog.md`

- [ ] **Step 1: Update command examples**

Replace examples:

```bash
x-skills link svelte-coder --target codex --project
x-skills migrate next-best-practices --target codex --project
x-skills unlink opentui-react --target agents --global
x-skills add owner/repo --all --to .Ag --to ~Cl
```

with:

```bash
x-skills link svelte-coder --at project:codex
x-skills migrate next-best-practices --at project:codex
x-skills unlink opentui-react --at global:agents
x-skills add owner/repo --all --at .Ag --at ~Cl
```

- [ ] **Step 2: Remove completed backlog item**

Remove the `--at` migration backlog item only after tests and docs pass.

- [ ] **Step 3: Run verification**

Run:

```bash
go test ./cmd/x-skills ./internal/... -count=1
go build -o bin/x-skills ./cmd/x-skills
git diff --check
```

Expected: all commands exit 0.
