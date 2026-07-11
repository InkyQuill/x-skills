# Late restore-flake review fix

## Outcome

Addressed all valid findings from `late-flake-review-report.md` while retaining the exact-plan staging assertion introduced by `a57a7eb`.

## Changes

- Replaced `RestorePlan.StagingRoot` with the explicitly test-only, read-only `StagingRootForTest` value-receiver accessor.
- Documented that the accessor returns the path recorded by that plan value and does not expose shared lifecycle state across copied plans.
- Consolidated restore staging cleanup coverage into `TestPlanRestoreExposesStagingRootForTest`.
- Registered `t.Cleanup(func() { _ = plan.Close() })` immediately after successful planning, while retaining the explicit `Close` call and filesystem removal assertion.
- Kept the TUI tests tied to the exact staging path belonging to their restore plan.
- Removed the resolved transient flake entry from the active testing backlog.
- Left the unrelated untracked `.claude/` artifact untouched.

## TDD evidence

After updating callers and tests first, `go test ./internal/manifest ./internal/tui` failed because `RestorePlan.StagingRootForTest` did not yet exist. Adding the narrowed accessor made both affected packages pass.

## Verification

- `go test ./internal/tui -run 'TestRestoreNestedModalsNavigateBackAndFinalCancelClosesStaging|TestRestoreQuitClosesStagingFromEveryNestedModal' -count=20` — pass.
- `go test -race ./...` — pass.
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `git diff --check` — pass.
