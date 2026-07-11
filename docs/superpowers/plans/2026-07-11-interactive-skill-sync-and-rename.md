# Interactive Skill Sync And Rename Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users interactively synchronize the aggregate project skill set into selected Skills Folders, preserve every conflict through migration/rename, support safe non-interactive selection, and expose general archive rename in the TUI.

**Architecture:** A new `internal/syncer` package scans all non-destination project Skills Folders, groups identical occurrences by fingerprint, and produces compatibility-aware candidates plus divergent-name variants. CLI and TUI are adapters over a shared plan/apply engine. Rename is implemented in `internal/actions` as an archive transaction that relinks visible usages and updates both project manifests.

**Tech Stack:** Go 1.26.5, Huh for CLI multi-select, Bubble Tea for TUI workbenches, existing roots/actions/fingerprint/compatibility/manifest packages.

## Global Constraints

- `sync` is additive; unselected candidates never remove destination skills.
- Candidates come from every configured non-destination project Skills Folder.
- Identical occurrences collapse by fingerprint; divergent same-name variants require an explicit source choice.
- Every selected unmanaged source is migrated before it is linked elsewhere.
- Destination conflicts preserve both contents; no divergent skill is overwritten silently.
- Compatible, partial, and unknown candidates start selected; incompatible candidates start unselected.
- `-y` confirms a resolved plan and never resolves selection or conflict ambiguity.
- Successful project changes reconcile `.x-skills.local.yaml`.

---

## File Structure

- Create `internal/syncer/candidates.go`, `plan.go`, `apply.go` and tests.
- Create `internal/cli/sync.go` and tests; modify `internal/cli/root.go`.
- Create `internal/tui/sync.go`, `modal_sync.go`, and tests; modify model/actions/help/views.
- Create `internal/actions/rename.go` and tests.
- Modify `internal/tui/actions.go`, `modal_text.go`, and tests for Repo rename.
- Modify `README.md`, `CONTEXT.md`, and `docs/backlog.md`.

### Task 1: Aggregate And Group Sync Candidates

**Files:**
- Create: `internal/syncer/candidates.go`
- Create: `internal/syncer/candidates_test.go`

**Interfaces:**
- Produces: `Candidate{ID, Name, Fingerprint string; Occurrences []actions.ActiveSkill; Compatibility compatibility.Assessment}`.
- Produces: `NameGroup{Name string, Variants []Candidate}`.
- Produces: `Discover(cfg config.Config, destinations []roots.ActiveRoot) ([]NameGroup, error)`.

- [ ] **Step 1: Write discovery tests**

Create `.agents`, `.pi`, and `.opencode` project roots. Assert identical content groups into one candidate with three occurrences, divergent same-name content yields two variants, destination roots are excluded, and global roots are never sources.

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/syncer -count=1 -run Discover
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement deterministic discovery**

Resolve destinations by canonical root location, scan all enabled project roots except those destinations, fingerprint skill directories and symlink targets through existing fingerprint semantics, then sort groups by name and variants by fingerprint. Candidate IDs are `name + ":" + fingerprint`.

- [ ] **Step 4: Assess destination compatibility**

Combine consumer IDs from every destination, deduplicate them, read explicit metadata from the matching archive when present, and call `compatibility.Assess`. A multi-destination candidate is compatible only if it covers every combined consumer; partial means a non-empty subset.

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/syncer -count=1
git add internal/syncer
git commit -m "feat(sync): discover aggregate project candidates"
```

### Task 2: Plan Migration, Linking, And Destination Conflicts

**Files:**
- Create: `internal/syncer/plan.go`
- Create: `internal/syncer/plan_test.go`

**Interfaces:**
- Produces: `Selection{CandidateIDs []string, VariantByName map[string]string}`.
- Produces: `ConflictResolution{DestinationPath, PreserveAs, Action string}` where Action is `replace`, `keep`, or `cancel`.
- Produces: `Plan{Migrations, Links []Change; Conflicts []Conflict; Skipped []Skip}`.

- [ ] **Step 1: Write plan tests**

Cover managed source reuse, unmanaged source migration, same-fingerprint destination no-op/normalization, different unmanaged destination conflict, different managed destination conflict, explicit keep, and cancellation. Assert every preserve name is validated and unique.

- [ ] **Step 2: Implement preflight-only planning**

For each selected candidate, choose an existing matching archive or plan source migration. For each destination: missing entry becomes link; matching content becomes no-op or normalize-to-managed; divergent content becomes a conflict. Do not mutate while unresolved conflicts remain.

- [ ] **Step 3: Suggest editable conflict names**

Build suggestions from skill and destination target, such as `review-from-claude`; if occupied, append `-2`, `-3`, and so on. Validate using the same archive-name/path-safety rules as remote installs.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/syncer -count=1 -run Plan
git add internal/syncer/plan.go internal/syncer/plan_test.go
git commit -m "feat(sync): plan safe destination reconciliation"
```

### Task 3: Apply Sync Transactionally Per Skill

**Files:**
- Create: `internal/syncer/apply.go`
- Create: `internal/syncer/apply_test.go`

**Interfaces:**
- Produces: `Apply(ctx context.Context, cfg config.Config, plan Plan) Result`.
- Produces: `Progress{Completed, Total int; Skill, Action string}` callback in `ApplyOptions`.

- [ ] **Step 1: Write apply/rollback tests**

Assert unmanaged sources are archived before linking, divergent destinations are migrated under the accepted name, later-link failure rolls back links created for that skill, preserved archives remain, cancellation stops before the next skill, and partial successes are reported.

- [ ] **Step 2: Implement per-skill ordering**

For one skill: materialize/migrate the selected source archive; preserve each replacing destination; preflight all links; replace destination entries; emit progress; reconcile the local manifest. If a later destination fails, remove links created for that skill and restore destination backups while keeping already preserved archive copies.

- [ ] **Step 3: Verify and commit**

```bash
go test ./internal/syncer -race -count=1 -run Apply
git add internal/syncer/apply.go internal/syncer/apply_test.go
git commit -m "feat(sync): apply skill synchronization"
```

### Task 4: Add Interactive And Non-Interactive CLI Sync

**Files:**
- Create: `internal/cli/sync.go`
- Create: `internal/cli/sync_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- CLI: `x-skills sync --at LOCATION... [--all | --skill NAME...] [-y]`.

- [ ] **Step 1: Write CLI contract tests**

Assert destinations are required and project-scoped; interactive mode preselects compatible/partial/unknown and leaves incompatible unchecked; no-TTY mode requires `--all` or `--skill`; `--all` skips incompatible; `--skill` selects explicit names; divergent variants still require explicit resolution; and `-y` does not choose a variant.

- [ ] **Step 2: Implement the Huh checklist**

Render name, source chips, and compatibility status. Incompatible candidates remain available but unchecked. After selection, show one choice form per divergent variant and one editable preserve-name field per destination conflict, followed by a summary confirmation.

- [ ] **Step 3: Implement non-interactive flags**

`--all` chooses every non-incompatible unique candidate. Repeated `--skill` chooses exact names. If a requested name has multiple fingerprints, return an error listing source Skills Folder labels; do not fall back to the first occurrence.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/cli ./internal/syncer -count=1 -run Sync
git add internal/cli internal/syncer
git commit -m "feat(cli): add interactive skill sync"
```

### Task 5: Add The TUI Sync Workbench

**Files:**
- Create: `internal/tui/sync.go`
- Create: `internal/tui/modal_sync.go`
- Create: `internal/tui/sync_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/modal_help.go`

**Interfaces:**
- Produces TUI messages `syncCandidatesMsg`, `syncPlanMsg`, `syncProgressMsg`, and `syncResultMsg`, all generation guarded.

- [ ] **Step 1: Write workbench tests**

Open Sync, choose destination Skills Folders, assert candidate defaults, unselect one, resolve a divergent variant, edit a destination preservation name, preview the plan, apply, and verify links/manifests. Add Esc/cancellation and small-terminal render tests.

- [ ] **Step 2: Build the modal sequence**

Use five explicit stages: destinations, candidates, divergent variants, destination conflicts, confirmation/progress. Back navigation preserves prior choices. The summary shows migrations, links, preserved conflicts, skips, and compatibility warnings.

- [ ] **Step 3: Keep background work outside Update**

Candidate discovery, compatibility scanning, and apply run through commands. Every message includes a sync token; closing the modal or quitting cancels its context and discards stale results.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/tui ./internal/syncer -race -count=1 -run Sync
git add internal/tui internal/syncer
git commit -m "feat(tui): add skill sync workbench"
```

### Task 6: Add General Archive Rename

**Files:**
- Create: `internal/actions/rename.go`
- Create: `internal/actions/rename_test.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/modal_help.go`

**Interfaces:**
- Produces: `actions.RenameArchive(cfg, oldName, newName string) (RenameResult, error)`.
- `RenameResult` includes archive path, relinked visible paths, undiscoverable-project warning, and manifest updates.

- [ ] **Step 1: Write transaction tests**

Cover path/name validation, archive rename, relinking current-project/global managed usages, rollback after a failed relink, Recommended/Local manifest identity update, and no changes to unmanaged same-name entries.

- [ ] **Step 2: Implement transactional rename**

Preflight the new archive and every visible managed usage. Rename the archive, create replacement symlinks through temporary siblings, update manifests, and roll everything back on failure. Preserve the accepted limitation that other projects are not indexed.

- [ ] **Step 3: Expose Repo rename**

Choose an unused key after a keymap assertion test. Open an editable text modal prefilled with the old name, show affected visible usages and the other-project warning, then apply asynchronously and refresh.

- [ ] **Step 4: Reuse rename in sync conflicts**

The sync conflict workbench calls the same archive-name validator and rename transaction where an existing archive must move. Remove any separate conflict-only rename implementation that would diverge.

- [ ] **Step 5: Final verification**

```bash
go test ./internal/actions ./internal/syncer ./internal/cli ./internal/tui -race -count=1
go test ./... -count=1
go vet ./...
go build -o /tmp/x-skills ./cmd/x-skills
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/actions internal/syncer internal/cli internal/tui README.md CONTEXT.md docs/backlog.md
git commit -m "feat: add archive rename workflows"
```
