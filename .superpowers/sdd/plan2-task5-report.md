# Plan 2 Task 5 Report

## Summary

- Added the root `BuiltInSkills` embed containing the canonical `skills/*` tree.
- Added `internal/builtin` catalog listing with strict direct-child and `x-` name validation.
- Added safe archive materialization through sibling temporary directories.
- Existing identical archives are no-ops; divergent archives return `ErrArchiveConflict` without replacement.
- Complete skill trees are copied, including nested `agents/openai.yaml` metadata.

## TDD evidence

The initial focused run failed because `List`, `Archive`, `builtInSkills`, and `ErrArchiveConflict` did not exist. After the minimal implementation, the focused package passed.

## Verification

- `go test ./internal/builtin -count=1` — pass.
- `gofmt -l .` — pass (no files listed).
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `go test -race -count=1 ./...` — pass.
- `git diff --check` — pass.

## Notes

- `Archive` preserves successful earlier names when a later requested name fails, matching existing partial-success conventions.
- The pre-existing untracked `x-skills` file was left untouched and is not included in the commit.

## Review follow-up

- Replaced the check-then-rename publication on Linux with `renameat2` and `RENAME_NOREPLACE`, mapping a concurrent destination claim to `ErrArchiveConflict`.
- Added a portable non-Linux fallback using an exclusive publish lock and destination recheck.
- Changed the embed pattern to `all:skills/*` so future dot- or underscore-prefixed metadata is included.
- Added deterministic regression coverage for concurrent destination creation, mid-copy temporary cleanup, and partial-success preservation.
- Confirmed the non-Linux implementation compiles with `GOOS=darwin GOARCH=amd64 go test -c ./internal/builtin`.
- Follow-up verification passed: focused and full uncached tests, full race tests, `gofmt -l`, `go vet`, `staticcheck`, and `git diff --check`.

## Cross-platform publication follow-up

- Replaced the cooperative non-Linux fallback with kernel-enforced no-replace publication on Darwin (`renamex_np` with `RENAME_EXCL`) and Windows (`MoveFile` without replacement flags).
- Linux continues to use `renameat2` with `RENAME_NOREPLACE`.
- Other targets return `ErrAtomicPublishUnsupported` without renaming either path; `Archive` then removes its staged temporary directory through the existing deferred cleanup. No target uses a check-then-rename fallback.
- Added a regression test proving the unsupported publisher returns the sentinel and leaves both paths untouched.
- Cross-compiled `internal/builtin` for Darwin amd64/arm64, Windows amd64, FreeBSD amd64, OpenBSD amd64, NetBSD amd64, and DragonFly BSD amd64.
- Added build-tagged platform tests: Linux/Darwin/Windows require a concurrent destination to produce `ErrArchiveConflict`, while unsupported targets require `ErrAtomicPublishUnsupported` and verify staged/archive cleanup.
