# Plan 5 Task 3 Fix Report

## Outcome

Added focused regression coverage for inspector block values with unequal line
lengths. The test proves that each rendered line keeps its exact content and
ANSI-aware display width without style-added trailing whitespace. The existing
per-line production renderer is retained unchanged.

## TDD evidence

- **Red:** temporarily restored the pre-`a3677c3` multiline `Style.Render` call.
  `go test ./internal/tui -run '^TestBlockInspectorValueDoesNotPadUnequalLines$' -count=1`
  failed because Lip Gloss padded `"short"` from width 5 to width 13.
- **Green:** restored the per-line renderer from `a3677c3`; the same focused
  command passed.

The test forces an ANSI256 profile and a colored inspector value style so it is
stable even when the test environment sets `NO_COLOR`. It then checks each raw
line for independent ANSI styling, exact stripped content, absence of a trailing
space, and exact ANSI-aware display width.

## Verification

- `gofmt -l .` — PASS (no output)
- focused inspector regression — PASS
- `go test -race -count=1 ./...` — PASS
- `go vet ./...` — PASS
- `staticcheck ./...` — PASS
- `git diff --check` — PASS

## Review

No blocking Go findings. The test exercises the behavioral contract through the
real renderer, restores global style/profile state with `t.Cleanup`, and does not
run in parallel while those globals are overridden.
