# Plan 3 Task 4 Report

## Outcome

- Added `x-skills recommend NAME...` and `x-skills unrecommend NAME...`.
- Added manifest operations that move ownership between the committed Recommended Skill Manifest and the Local Skill Manifest.
- Recommendation accepts only archived skills with reproducible Git or GitHub metadata, preserves provenance and compatibility, and preflights the whole batch before either manifest is written.
- Paired manifest updates write recommended first and restore its exact prior contents if the local write fails.
- Unrecommend returns a still-active project skill to the local overlay while removing committed ownership.
- Added Repo-view recommendation toggling on lowercase `r`; uppercase `R` remains the existing Repo navigation key. Mixed promote/remove selections are rejected rather than partially applied.
- Added the Repo footer hint and full Help labels for both `Promote to project recommendations` and `Remove from project recommendations`.

## TDD evidence

- Manifest tests first failed because `Recommend` and `Unrecommend` were undefined, then passed after the ownership operations were implemented.
- CLI tests first failed with `unknown command "recommend"`, then passed after command registration and handlers were added.
- The TUI routing test first failed because lowercase `r` did not update the manifest, then passed after routing and the action handler were added.
- Full verification initially exposed a Repo-footer regression caused by the long action label. The compact hint and updated footer contract now pass while full wording remains in Help.

## Verification

- `go test ./internal/cli ./internal/tui ./internal/manifest -count=1 -run 'Recommendation|Recommend|Unrecommend'` — pass.
- `gofmt -l .` — pass, no files listed.
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `go test -race -count=1 ./...` — pass.
- `git diff --check` — pass.

## Notes

- The pre-existing untracked `x-skills` binary was left untouched and is not part of the commit.

## Review remediation

- Moved all Repo recommendation manifest reads, project scans, paired writes, and snapshot reload work into a typed `tea.Cmd`.
- Added a serialized in-flight guard and generation token. Duplicate key presses do not start overlapping operations, and stale typed results cannot mutate current model state.
- The command returns a composed snapshot of Active, Repo, Doctor, and usage data; the Bubble Tea update loop applies it only for the current generation and reports mutation/reload errors separately.
- Refactored local reconciliation planning into the shared `planLocalReconciliation` path. `Unrecommend` now removes committed ownership in memory, derives the replacement local overlay from actual project occurrences, and performs the paired write only after reconciliation succeeds.
- Added regressions for unmanaged divergent archive content, divergent same-name project roots, no-active removal, deferred TUI command execution, in-flight serialization, and stale-result rejection.

### Remediation verification

- `go test ./internal/cli ./internal/tui ./internal/manifest -count=1 -run 'Recommendation|Recommend|Unrecommend|Reconcile'` — pass.
- `gofmt -l .` — pass, no files listed.
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `go test -race -count=1 ./...` — pass.
