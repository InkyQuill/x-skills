# Plan 4 Task 3 Report

Implemented per-skill transactional sync application in `internal/syncer/apply.go` with contract tests in `internal/syncer/apply_test.go`.

## Delivered

- `Apply(ctx, cfg, plan, options...) Result`, `ApplyOptions`, and deterministic `Progress` callbacks.
- Durable materialization of unmanaged directories and symlink sources into the archive.
- Explicit preservation of replaced archive and destination content under accepted archive names.
- Per-skill active-link transactions: late failures remove links created for that skill and restore replaced destination entries.
- Cancellation checks between skills, continuation after individual skill failures, and separate succeeded/failed result lists.
- Local Skill Manifest reconciliation after successful or partially successful filesystem mutation, with reconciliation errors surfaced separately.

## TDD Evidence

The initial focused test run failed at compile time because `Apply`, `ApplyOptions`, and `Progress` did not exist. The first implementation then passed migration, preservation, rollback, cancellation, partial-result, progress, and manifest assertions. A later regression test failed when an unmanaged symlink source was copied as a symlink instead of materialized; resolving the source before the atomic copy made that test pass.

## Verification

Passed:

- `go test ./internal/syncer -race -count=1 -run Apply`
- `gofmt -l .`
- `go vet ./...`
- `staticcheck ./...`
- `go test -race -count=1 ./...`
- `go build -o /tmp/x-skills-plan4-task3 ./cmd/x-skills`
- `git diff --check`

The Zen of Go audit found no blocking issues. The implementation is synchronous, leaves concurrency to callers, checks cancellation at the promised skill boundary, wraps filesystem errors with operation context, and tests observable filesystem and result contracts.

## Review Remediation

The Critical and High review findings were reproduced with failing regressions and addressed comprehensively.

- Preflight plans now carry the approved candidate fingerprint on every migration, link, and conflict record.
- `Apply` performs an immutable whole-plan validation pass before mutation: current configured project Skills Folders, archive containment, direct-child names, candidate IDs, fingerprints, actions, unique source/destination/preservation paths, conflict resolution paths/statuses, and cancellation shape are rechecked.
- Live migration sources and link destinations are reclassified before writes. Source, archive, destination, resolution, and managed/unmanaged status drift abort through `Result.PlanError` without mutation.
- Migration-produced archives receive destination preflight without requiring the archive to exist prematurely. Source content is staged first, then an old archive is preserved and the staged tree is atomically published.
- If archive publication fails after preservation, the canonical archive is restored from the durable preservation copy. Preservation copies are retained.
- `SkillResult` exposes `ArchiveChanged`, `SourceRemoved`, and `LinksRolledBack`, making retained migration side effects on later failures explicit and coherent. Active destinations are restored and no managed link is left broken.
- Destination backup cleanup failures are returned rather than ignored; the successfully published link remains valid and the result accurately states that link rollback did not occur.
- Manifest reconciliation remains attempted after every success or failure that may have mutated filesystem state, with failure exposed through `ManifestError`.

Added regression coverage for tampered plan paths, identity and source/destination drift, unresolved conflicts, malformed cancellation, migration-produced archive destination preflight, retained migration after late link failure, partial source-removal failure, backup-cleanup failure, preservation durability, rollback state, progress, cancellation, and manifest reconciliation/error reporting.

Fresh remediation verification passed:

- `go test ./internal/syncer -race -count=1`
- `go test -race -count=1 ./...`
- `go vet ./...`
- `staticcheck ./...`
- `go build -o /tmp/x-skills-plan4-task3-remediation ./cmd/x-skills`
- `gofmt -l .`
- `git diff --check`
