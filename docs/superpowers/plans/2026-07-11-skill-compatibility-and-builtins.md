# Skill Compatibility And Built-Ins Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Model Skills Folder consumers, classify skill compatibility without overclaiming, ship `x-`-prefixed Built-In Skills, and let Doctor archive and globally link missing built-ins.

**Architecture:** Extend global root configuration with consumer agent IDs and extend archived skill metadata with an optional explicit compatibility profile. A new pure `internal/compatibility` package computes effective compatibility from explicit metadata first and conservative content inference second. Built-In Skills remain canonical under repository `skills/`, are embedded into the binary from a root package, and are installed through Doctor using the same archive/link primitives as other skills.

**Tech Stack:** Go 1.26.5, YAML v3, JSON metadata, `go:embed`, Cobra, Bubble Tea, existing actions/repo/doctor packages.

## Global Constraints

- Use the canonical terms in `CONTEXT.md`: Skills Folder, Built-In Skill, Agent-Agnostic Skill, and Compatibility Profile.
- Explicit compatibility metadata always overrides inference.
- Mere mentions of an agent name never produce a high-confidence incompatible result.
- Skill rewriting is never automatic during sync, install, Doctor, or restore.
- Built-In Skill names must start with `x-`.
- Non-interactive Doctor never guesses a global Skills Folder.

---

## File Structure

- Create `builtins.go`: root-package embedded Built-In Skill filesystem.
- Rename `skills/find-skills/` to `skills/x-find-skills/` and `skills/manage-skills/` to `skills/x-manage-skills/`.
- Create `skills/x-port-skill/SKILL.md` and `skills/x-port-skill/agents/openai.yaml`.
- Create `internal/builtin/catalog.go` and `internal/builtin/catalog_test.go`: enumerate and materialize embedded skills.
- Create `internal/compatibility/compatibility.go`, `infer.go`, and tests.
- Modify `internal/config/config.go` and tests: Skills Folder consumers.
- Modify `internal/roots/roots.go`: expose consumers to callers.
- Modify `internal/remote/source.go` and tests: metadata schema and explicit profile.
- Modify `internal/doctor/doctor.go` and tests: missing/inactive built-in issues.
- Modify `internal/cli/doctor.go` and tests: global destination selection and archive-only reporting.
- Modify `internal/tui/actions.go`, `modal_choice.go`, and tests: Doctor built-in workbench.
- Modify `README.md`, `CONTEXT.md`, and `docs/backlog.md`.

### Task 1: Add Consumer Agents To Skills Folder Configuration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/roots/roots.go`
- Modify: `internal/roots/roots_test.go`
- Modify: `README.md`

**Interfaces:**
- Produces: `ManagedRoot.Consumers []string` and `ActiveRoot.Consumers []string`.
- Produces: `config.NormalizeConsumers([]string) ([]string, error)`.

- [ ] **Step 1: Write parsing and default tests**

Assert the defaults are `.agents -> [codex, pi, opencode, crush]`, `.claude -> [claude]`, and `.codex -> [codex]` for both scopes. Assert YAML overrides accept lowercase IDs, deduplicate them, and reject empty or malformed IDs.

```go
data := []byte("version: 1\nactive_roots:\n  - scope: project\n    target: agents\n    path: .agents/skills\n    consumers: [codex, pi, codex]\n")
```

- [ ] **Step 2: Verify tests fail**

Run:

```bash
go test ./internal/config ./internal/roots -count=1 -run Consumers
```

Expected: FAIL because roots have no `Consumers` field.

- [ ] **Step 3: Implement consumer parsing**

Add `Consumers []string` to `ManagedRoot`, `ActiveRoot`, and `activeRootYAML`. Normalize with `^[a-z][a-z0-9-]*$`, trim, lowercase, deduplicate, and sort. A custom root with omitted consumers has an unknown consumer set (`nil`), not a guessed target-based consumer.

- [ ] **Step 4: Document configuration**

Add this exact shape to README:

```yaml
version: 1
active_roots:
  - scope: project
    target: agents
    path: .agents/skills
    label: .Ag
    consumers: [codex, pi, opencode, crush]
```

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/config ./internal/roots -count=1
git add internal/config internal/roots README.md
git commit -m "feat(config): declare skills folder consumers"
```

### Task 2: Add Explicit Compatibility Metadata

**Files:**
- Modify: `internal/remote/source.go`
- Modify: `internal/remote/source_test.go`

**Interfaces:**
- Produces: `CompatibilityProfile{Agnostic bool, Agents []string}`.
- Produces: `SourceMetadata.SchemaVersion int` and `SourceMetadata.Compatibility *CompatibilityProfile`.

- [ ] **Step 1: Write metadata compatibility tests**

Cover an agnostic profile, a sorted agent list, invalid simultaneous `agnostic: true` plus agents, and backward-compatible loading of current metadata without `schema_version`.

```go
meta := SourceMetadata{
	SchemaVersion: 2,
	SourceType: SourceTypeGitHub,
	Owner: "acme",
	Repo: "skills",
	CloneURL: "https://github.com/acme/skills.git",
	Commit: "abc",
	SkillPath: "skills/review",
	Compatibility: &CompatibilityProfile{Agents: []string{"claude"}},
}
```

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/remote -count=1 -run 'Compatibility|SourceMetadataRoundTrip'
```

Expected: FAIL because the fields and validation do not exist.

- [ ] **Step 3: Implement schema v2 without breaking v1**

Treat missing/zero schema version as v1 when reading. Write version 2 for new or updated metadata. Validate that exactly one of `Agnostic` or a non-empty `Agents` list is set when a profile exists. Compatibility fields do not participate in `SameIdentity`.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/remote -count=1
git add internal/remote/source.go internal/remote/source_test.go
git commit -m "feat(metadata): record skill compatibility"
```

### Task 3: Infer Compatibility Conservatively

**Files:**
- Create: `internal/compatibility/compatibility.go`
- Create: `internal/compatibility/infer.go`
- Create: `internal/compatibility/compatibility_test.go`
- Create: `internal/compatibility/testdata/claude-only/SKILL.md`
- Create: `internal/compatibility/testdata/mentions-claude/SKILL.md`

**Interfaces:**
- Produces: `Assessment{State State, Confidence Confidence, Agents []string, Reasons []string, Explicit bool}`.
- Produces: `Assess(skillDir string, explicit *remote.CompatibilityProfile, consumers []string) (Assessment, error)`.
- States: `compatible`, `partial`, `unknown`, `incompatible`.

- [ ] **Step 1: Write table-driven assessment tests**

Cover explicit agnostic, explicit full match, explicit partial match, explicit no match, unknown consumers, ordinary agent mention, and strong Claude-only instructions. The ordinary mention must remain unknown.

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/compatibility -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement explicit evaluation**

If a profile exists, return `Explicit: true`; agnostic is compatible for every known destination. Agent lists are compatible when all consumers match, partial when the intersection is non-empty, and incompatible when the intersection is empty. Unknown consumers always return unknown.

- [ ] **Step 4: Implement high-confidence inference rules**

Read `SKILL.md` plus known metadata files under `agents/`. High-confidence signals require executable semantics such as `$CLAUDE_PROJECT_DIR`, Claude-only hook configuration, or instructions that mandate a named exclusive tool. Agent names in prose, examples, titles, source URLs, and comparison tables add a low-confidence reason only and cannot yield incompatible.

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/compatibility -count=1
git add internal/compatibility
git commit -m "feat: assess agent compatibility"
```

### Task 4: Rename And Add Built-In Skills

**Files:**
- Rename: `skills/find-skills/` to `skills/x-find-skills/`
- Rename: `skills/manage-skills/` to `skills/x-manage-skills/`
- Create: `skills/x-port-skill/SKILL.md`
- Create: `skills/x-port-skill/agents/openai.yaml`
- Modify: references in `README.md` and tests/docs found by `rg`.

**Interfaces:**
- Produces the Built-In Skill IDs `x-find-skills`, `x-manage-skills`, and `x-port-skill`.

- [ ] **Step 1: Rename the existing built-ins and metadata names**

Use filesystem moves, then set frontmatter `name` to the directory name. Update OpenAI metadata to use display names `X Skills: Manage Skills` and prompts that invoke `$x-manage-skills`.

- [ ] **Step 2: Create `x-port-skill`**

Its workflow must: inspect all skill files; identify agent-exclusive instructions; preserve semantics; rewrite common instructions into agent-agnostic language; add explicit Compatibility Profile metadata only after verification; add agent-specific metadata such as `agents/openai.yaml` when useful; show a diff; and never overwrite without approval.

- [ ] **Step 3: Validate names and stale references**

Run:

```bash
rg -n 'name: (find-skills|manage-skills)|\$(find-skills|manage-skills)' skills README.md docs internal
```

Expected: no stale built-in names.

- [ ] **Step 4: Commit**

```bash
git add skills README.md docs internal
git commit -m "feat(skills): prefix built-in skills"
```

### Task 5: Embed And Materialize Built-In Skills

**Files:**
- Create: `builtins.go`
- Create: `internal/builtin/catalog.go`
- Create: `internal/builtin/catalog_test.go`

**Interfaces:**
- Produces: root package `var BuiltInSkills embed.FS`.
- Produces: `builtin.List() ([]Skill, error)` and `builtin.Archive(cfg config.Config, names []string) ([]string, error)`.

- [ ] **Step 1: Write catalog tests**

Assert the catalog returns exactly the three `x-` names, rejects any non-prefixed directory, and archives a complete directory including `agents/openai.yaml`.

- [ ] **Step 2: Add the root embed**

```go
package xskills

import "embed"

// BuiltInSkills contains the canonical skills shipped with x-skills.
//
//go:embed skills/*
var BuiltInSkills embed.FS
```

- [ ] **Step 3: Implement safe archive materialization**

Read only direct `skills/<name>` children, validate the `x-` prefix and archive name, copy through a temporary sibling directory, then rename into the archive. Existing divergent archives return a conflict; identical content is a no-op.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/builtin -count=1
go test ./... -count=1
git add builtins.go internal/builtin
git commit -m "feat: embed built-in skills"
```

### Task 6: Diagnose And Install Missing Built-Ins

**Files:**
- Modify: `internal/doctor/doctor.go`
- Modify: `internal/doctor/doctor_test.go`
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/doctor_test.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`

**Interfaces:**
- Adds Doctor kinds `missing-builtin` and `inactive-builtin`.
- Adds `FixOptions.BuiltInDestinations []roots.ActiveRoot` and `FixOptions.ArchiveOnlyBuiltIns bool`.

- [ ] **Step 1: Add diagnosis tests**

Missing archive yields `missing-builtin`; archived but not linked in any global Skills Folder yields `inactive-builtin`; a built-in linked globally yields neither.

- [ ] **Step 2: Add CLI behavior tests**

Interactive `doctor --fix` shows enabled global destinations with `~Ag` preselected and an explicit archive-only option. `doctor --fix -y` without global `--at` archives built-ins and prints `archived but inactive`; with `--at global:agents` it archives and links them. Project destinations are rejected for this fix.

- [ ] **Step 3: Implement diagnosis and safe fixes**

Archive through `builtin.Archive`, link through existing action primitives, and preserve partial-success results. Do not replace divergent archives or destination entries without the normal conflict workflow.

- [ ] **Step 4: Add the TUI Doctor workbench**

When fixing built-in issues, show a checklist of global Skills Folders plus `Archive only`. The ordinary Doctor list remains usable while the notice is pending; do not open a modal on every refresh after the user dismisses it in the current session.

- [ ] **Step 5: Verify**

```bash
go test ./internal/doctor ./internal/cli ./internal/tui -count=1 -run 'BuiltIn|Builtin'
go test ./... -count=1
go vet ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/doctor internal/cli internal/tui README.md CONTEXT.md docs/backlog.md
git commit -m "feat(doctor): install missing built-in skills"
```
