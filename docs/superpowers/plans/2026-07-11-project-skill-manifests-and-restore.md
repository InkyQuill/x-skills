# Project Skill Manifests And Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add committed recommendations, a machine-local skill overlay, explicit recommendation commands, safe restore/full-restore planning, and Git hygiene diagnostics.

**Architecture:** A new `internal/manifest` package owns versioned YAML parsing, deterministic writes, overlay merging, and local reconciliation. Restore is plan/apply: resolve every desired skill first, perform safe additions, and permit destructive `--full` cleanup only when every desired skill is available. Project mutation commands call one reconciliation service after success; the committed manifest changes only through explicit recommend/unrecommend operations.

**Tech Stack:** Go 1.26.5, YAML v3, Cobra, existing source metadata, actions, roots, fingerprint, and Doctor packages, Git CLI for tracked-file inspection.

## Global Constraints

- `.x-skills.yaml` is committed maintainer intent and contains reproducibly sourced skills only.
- `.x-skills.local.yaml` is gitignored local state and may contain archive-only skills.
- Effective desired state is their union; committed entries win source and compatibility conflicts.
- Automatic reconciliation never removes an unavailable archive-only local entry merely because this machine lacks its archive.
- Restore never stores machine-specific Skills Folder destinations in either manifest.
- `restore --full` touches only explicitly selected project Skills Folders, never global or unselected folders, and never deletes archived copies.
- Any unresolved desired skill blocks the entire destructive phase of `restore --full`.
- Doctor may edit `.gitignore`, but never runs `git add` or `git rm --cached`.

---

## File Structure

- Create `internal/manifest/model.go`, `io.go`, `merge.go`, `reconcile.go`, `restore.go` and tests.
- Create `internal/cli/manifest.go`, `recommend.go`, `restore.go` and tests.
- Modify `internal/cli/root.go`: register new commands.
- Modify project mutation handlers in `internal/cli/add.go`, `link.go`, `migrate.go`, `unlink.go`.
- Modify TUI mutation completion paths in `internal/tui/actions.go` and `install.go`.
- Modify `internal/doctor/doctor.go` and CLI/TUI Doctor tests for Git hygiene.
- Modify `.gitignore`, `README.md`, `CONTEXT.md`, and `docs/backlog.md`.

### Task 1: Define And Round-Trip Both Manifest Schemas

**Files:**
- Create: `internal/manifest/model.go`
- Create: `internal/manifest/io.go`
- Create: `internal/manifest/io_test.go`

**Interfaces:**
- Produces: `Manifest{Version int, Skills []Skill}`.
- Produces: `Skill{Name string, Source Source, Compatibility *remote.CompatibilityProfile, Fingerprint string}`.
- Produces: `LoadRecommended(projectRoot string)`, `LoadLocal(projectRoot string)`, `WriteRecommended`, and `WriteLocal`.

- [ ] **Step 1: Write strict parsing tests**

Use this canonical committed fixture:

```yaml
version: 1
skills:
  - name: using-svelte-5
    source:
      type: github
      repository: InkyQuill/x-skills
      path: skills/using-svelte-5
      ref: main
```

Use this local fixture:

```yaml
version: 1
skills:
  - name: private-review
    source:
      type: archive
    fingerprint: sha256:abc
```

Reject unknown fields, duplicate names, invalid names, unsupported versions, and an archive source in the Recommended Skill Manifest.

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/manifest -count=1 -run 'Load|Write'
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement deterministic atomic I/O**

Decode with `yaml.Decoder.KnownFields(true)`. Normalize Git paths to slash form. Sort skills case-insensitively by name. Write to a same-directory temporary file with mode `0644`, close it, then rename. Omit empty optional fields.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/manifest -count=1
git add internal/manifest
git commit -m "feat(manifest): add project manifest schemas"
```

### Task 2: Merge Recommended And Local Intent

**Files:**
- Create: `internal/manifest/merge.go`
- Create: `internal/manifest/merge_test.go`

**Interfaces:**
- Produces: `Effective(recommended, local Manifest) (Manifest, []Notice)`.

- [ ] **Step 1: Write overlay tests**

Assert local-only entries remain, recommended-only entries appear, and a duplicate name uses the recommended source/compatibility while retaining no machine-specific destination. Emit a notice when the local identity disagrees with the committed identity.

- [ ] **Step 2: Implement stable merge**

Index by exact normalized skill name. Insert local first, then overwrite with recommended. Return sorted output and conflict notices; do not mutate either input.

- [ ] **Step 3: Verify and commit**

```bash
go test ./internal/manifest -count=1 -run Effective
git add internal/manifest/merge.go internal/manifest/merge_test.go
git commit -m "feat(manifest): merge recommended and local skills"
```

### Task 3: Reconcile The Local Overlay After Project Mutations

**Files:**
- Create: `internal/manifest/reconcile.go`
- Create: `internal/manifest/reconcile_test.go`
- Modify: `internal/cli/add.go`
- Modify: `internal/cli/link.go`
- Modify: `internal/cli/migrate.go`
- Modify: `internal/cli/unlink.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/install.go`

**Interfaces:**
- Produces: `ReconcileLocal(cfg config.Config) (Result, error)`.
- Consumes: project roots only, archived source metadata, fingerprints, existing Local Skill Manifest.

- [ ] **Step 1: Write reconciliation tests**

Cover union across multiple project Skills Folders, removal of the last locally present sourced skill, retention of an unavailable archive-only existing entry, exclusion of global-only skills, and no write when normalized contents are unchanged.

- [ ] **Step 2: Verify tests fail**

```bash
go test ./internal/manifest -count=1 -run Reconcile
```

Expected: FAIL because `ReconcileLocal` is undefined.

- [ ] **Step 3: Implement union reconciliation**

Scan configured roots with `ScopeProject`; group by archive identity/fingerprint. Read source metadata from managed archives. For unmanaged skills, use archive source plus fingerprint. Begin from old archive-only entries and retain those whose archive is absent; replace the rest with observed local state. Remove entries also owned by the Recommended Skill Manifest from the local output.

- [ ] **Step 4: Hook only successful project mutations**

Call reconciliation after a command/TUI mutation reports success and touched at least one project root. Do not call on cancellation, preflight failure, global-only mutation, or failed rollback. If reconciliation fails after a successful filesystem mutation, report both facts clearly; do not roll back the skill operation.

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/manifest ./internal/cli ./internal/tui -count=1 -run 'Reconcile|Manifest'
git add internal/manifest internal/cli internal/tui
git commit -m "feat(manifest): reconcile local project skills"
```

### Task 4: Expose Recommend And Unrecommend

**Files:**
- Create: `internal/cli/recommend.go`
- Create: `internal/cli/recommend_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/modal_help.go`

**Interfaces:**
- CLI: `x-skills recommend NAME...` and `x-skills unrecommend NAME...`.
- Produces: `manifest.Recommend(cfg, names)` and `manifest.Unrecommend(cfg, names)`.

- [ ] **Step 1: Write CLI tests**

Assert recommendation copies Git/GitHub provenance and compatibility from archive metadata, rejects archive-only skills, removes the promoted entry from local overlay, and unrecommend moves a still-active project skill back to the local overlay.

- [ ] **Step 2: Implement recommendation operations**

Require every name to exist in the archive with reproducible Git/GitHub metadata. Plan all names before writing either manifest. Update both manifests atomically as a logical operation: write recommended first to a backup-aware temporary path, write local, and restore recommended on local write failure.

- [ ] **Step 3: Add TUI actions without hard-coding a conflicting key**

Expose `Promote to project recommendations` and `Remove from project recommendations` through Repo action hints/help and the action handler. Choose the final key only after asserting it is absent from `model.go`; add a key-routing test for the selected key.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/cli ./internal/tui ./internal/manifest -count=1 -run 'Recommend|Unrecommend'
git add internal/cli internal/tui internal/manifest
git commit -m "feat: expose project skill recommendations"
```

### Task 5: Plan Safe Restore And Full Restore

**Files:**
- Create: `internal/manifest/restore.go`
- Create: `internal/manifest/restore_test.go`

**Interfaces:**
- Produces: `RestoreRequest{Destinations []roots.ActiveRoot, Full bool}`.
- Produces: `RestorePlan{Available []PlannedSkill, Unavailable []UnavailableSkill, Additions []Change, Removals []Change}`.
- Produces: `PlanRestore(ctx, cfg, request)`, `ApplyRestore(ctx, cfg, plan)`.

- [ ] **Step 1: Write plan tests**

Cover remote fetch, present archive source, missing archive source, additive restore, full removal scope, unmanaged-extra migration, and the rule that any unavailable desired skill empties/blocks `Removals` while retaining safe additions.

- [ ] **Step 2: Implement resolve-first planning**

Merge manifests, resolve every source into an existing or staged archive candidate, then inspect only explicit project destinations. For `Full`, classify extras as managed-link removal or unmanaged migrate-then-remove. Never include global/unselected paths or archive deletion.

- [ ] **Step 3: Implement apply ordering**

Apply available archives and desired links first. If `Unavailable` is non-empty, return a partial result without executing removals. Otherwise migrate unmanaged extras to conflict-safe user-confirmed archive names, remove managed extras, and reconcile the local manifest.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/manifest -count=1 -run Restore
git add internal/manifest/restore.go internal/manifest/restore_test.go
git commit -m "feat(manifest): plan safe project restore"
```

### Task 6: Expose Restore In CLI And TUI

**Files:**
- Create: `internal/cli/restore.go`
- Create: `internal/cli/restore_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`

**Interfaces:**
- CLI: `x-skills restore --at .Ag --at .Cl [--full] [-y]`.

- [ ] **Step 1: Add CLI contract tests**

Assert at least one explicit project `--at` is required, global selectors are rejected, normal restore never removes extras, `--full` prints its plan and requires confirmation, and unavailable entries block removals.

- [ ] **Step 2: Implement CLI plan/confirm/apply**

Print grouped `available`, `unavailable`, `links`, `migrations`, and `removals`. `-y` confirms only an unambiguous plan; conflict-safe rename decisions still require explicit input or fail in non-interactive mode.

- [ ] **Step 3: Add a TUI restore workbench**

Use a destination checklist limited to project Skills Folders, a Full toggle defaulting off, a plan preview, unavailable warnings, editable migration archive names, and a destructive confirmation screen only when removals remain enabled.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/cli ./internal/tui ./internal/manifest -count=1 -run Restore
git add internal/cli internal/tui
git commit -m "feat: expose project skill restore"
```

### Task 7: Add Git Hygiene Doctor Findings

**Files:**
- Modify: `internal/doctor/doctor.go`
- Modify: `internal/doctor/doctor_test.go`
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/doctor_test.go`
- Modify: `.gitignore`

**Interfaces:**
- Adds issue kinds `recommended-manifest-untracked`, `local-manifest-tracked`, and `skills-folder-tracked`.

- [ ] **Step 1: Write tests with a temporary Git repository**

Use `git init`, configure local test identity, and cover: untracked `.x-skills.yaml`; tracked `.x-skills.local.yaml`; tracked file under a configured project Skills Folder; no finding outside Git; and no finding for ignored/untracked local files.

- [ ] **Step 2: Implement tracked-file inspection**

Run `git -C <project> ls-files --error-unmatch -- <path>` for manifests and `git -C <project> ls-files -- <root-relative-path>/**` for Skills Folders. Pass arguments directly through `exec.CommandContext`; never invoke a shell.

- [ ] **Step 3: Implement conservative fixes**

Doctor may append normalized ignore entries for `.x-skills.local.yaml` and configured project Skills Folders. For tracked paths, print exact suggested `git rm -r --cached -- <path>` commands but do not execute or stage them. For untracked `.x-skills.yaml`, print `git add -- .x-skills.yaml` but do not execute it.

- [ ] **Step 4: Verify full feature**

```bash
go test ./internal/manifest ./internal/doctor ./internal/cli ./internal/tui -count=1
go test ./... -race -count=1
go vet ./...
go build -o /tmp/x-skills ./cmd/x-skills
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add .gitignore internal/doctor internal/cli README.md CONTEXT.md docs/backlog.md
git commit -m "feat(doctor): audit project skill manifests"
```
