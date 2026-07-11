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
