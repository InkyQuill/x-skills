# Plan 3 Task 5 Report

## Outcome

- Added resolve-first project restore planning with explicit project Skills Folder validation.
- Effective Recommended and Local Skill Manifest intent is resolved from an existing archive or a staged Git/GitHub checkout before full cleanup is enabled.
- Plans retain safe additions and report unavailable desired skills while clearing and blocking the destructive removal phase.
- Additive restore creates only missing managed links in explicitly selected destinations.
- Full restore inspects only explicitly selected project roots. Managed extras are removed as links; unmanaged extras are represented and applied as migrate-before-remove changes so their archived copy is preserved.
- Apply publishes staged archives and creates desired links before considering cleanup, skips all removals whenever any desired skill is unavailable, preserves archived copies, and reconciles the Local Skill Manifest after successful filesystem changes.

## Safety invariants

- Zero destinations, global destinations, disabled/unconfigured destinations, and caller-crafted paths are rejected.
- Global and unselected project Skills Folders are never scanned for removal.
- Archive directories are never included in removal plans.
- A missing archive-only source is an unavailable desired skill, not an instruction to remove local state.
- Cleanup uses existing mutation operations, including conflict detection for unmanaged migrations; ambiguous archive conflicts fail without deleting the active copy.

## TDD evidence

- Restore tests first failed because `PlanRestore`, `ApplyRestore`, and their plan types were undefined.
- The minimal implementation then passed tests covering unavailable-source blocking, explicit-root scoping, additive links, unmanaged migration, active-copy removal, and archive preservation.

## Verification

- `go test ./internal/manifest -count=1` — pass.
- `gofmt -l internal/manifest` — pass, no files listed.
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `go test -race -count=1 ./...` — pass.
- `git diff --check` — pass.

## Notes

- The pre-existing untracked `x-skills` binary was left untouched and is not part of the commit.

## Review remediation

- Existing archives are no longer trusted by name alone. Archive-only entries verify their declared fingerprint, while Git/GitHub entries verify source type, repository identity, ref, and skill path before becoming available.
- Added explicit desired-entry normalization. Managed links are retained, broken links are removed, identical unmanaged copies are replaced by managed links, and divergent unmanaged copies require a resolved preserve name.
- Full cleanup now distinguishes managed and broken removals from unmanaged migrations. Divergent archive collisions expose `MigrationConflict` with an unused suggested preserve name; apply preflights every resolution and path before its first mutation.
- Restore changes are bound to the exact planned path under an enabled configured project Skills Folder. Caller-edited traversal names, changed paths, occupied preserve names, and discarded staging are rejected before mutation.
- `RestorePlan.Close`/`Discard` deterministically removes staged checkouts; apply always closes its plan copy. Planning errors also discard staging.
- Reconciliation is deferred after the first successful mutation and runs on success, cancellation, or later failure. Reconciliation errors are joined with the original apply error so partial filesystem success is never hidden.
- Added regression coverage for local Git fetch/apply, wrong archive identity, fingerprint mismatch, full managed/broken/unmanaged classification, unavailable desired skills with safe additions, non-mutating migration conflicts and resolved preserve names, desired-entry normalization, staged cleanup, exact-path tampering, and partial-apply reconciliation.

### Remediation verification

- `go test ./internal/manifest -count=1` — pass.
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `go test -race -count=1 ./...` — pass.
- `git diff --check` — pass.
