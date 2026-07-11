# Plan 5 Task 5 Fix Report

## Outcome

Addressed the review finding with regression-test changes only. The existing production status renderers already implement the required contract, so no production code was changed.

## Changes

- Made the managed, unmanaged, and broken `NO_COLOR` row fixtures identical in every non-status field by setting the broken `Reason` to the same text as `Description`.
- Added a table-driven exact-contract test for both status rendering surfaces:
  - Unicode: `✓ managed`, `◇ unmanaged`, `× broken`; compact markers `✓`, `◇`, `×`.
  - ASCII: `+ managed`, `? unmanaged`, `x broken`; compact markers `+`, `?`, `x`.
- Exercised each exact mapping with every cursor/selection combination to prove status output is independent of row state.

## TDD Evidence

After adding the tests, temporarily changed the Unicode managed glyph from `✓` to `◇`.

- `TestStatusRowsDistinguishableWithoutColor/unicode` failed because the managed and unmanaged rows became identical.
- `TestStatusRenderersUseExactSymbolsIndependentlyOfRowState/unicode_managed` failed for both the status chip and compact marker under all cursor/selection combinations.

The temporary mutation was then reverted. The final diff contains no production changes.

## Verification

- `NO_COLOR=1 go test ./internal/tui -count=1 -run 'NoColor|Status'` — pass
- `go test ./internal/tui -count=1` — pass
- `go test ./... -count=1` — pass
- `go test ./... -race -count=1` — pass
- `go vet ./...` — pass
- `staticcheck ./...` — pass
- `gofmt -l .` — pass; no files listed
- `git diff --check` — pass
