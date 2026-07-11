# Plan 3 Task 6 Report

## Outcome

- Added `x-skills restore --at ... [--full] [-y]` with explicit project-only destination validation.
- The CLI prints grouped available, unavailable, links, migrations, and removals before confirmation.
- Additive restore leaves extras untouched. Full restore requires confirmation and keeps destructive work blocked when desired skills are unavailable.
- `-y` confirms only an unambiguous plan. Preserve-name conflicts still require an explicit interactive archive name and fail under `--no-input`.
- Added a globally discoverable TUI Restore workbench (`S`) with a project-only destination checklist, Full toggle defaulting off, asynchronous plan preview, unavailable warnings, editable preserve names, and removal-only destructive confirmation.
- Restore planning and apply have generation tokens and owned cancellation. Stale plans are closed, discarded previews close staging, apply consumes and closes the plan, and command contexts are canceled on completion or user cancellation.

## TDD evidence

- CLI contract tests first failed because `restore` was not registered.
- TUI contract tests first failed on the absent workbench, async state, plan message, and rename editor.
- Minimal command and TUI state-machine implementations made the focused tests pass.
- A full race run caught footer/help truncation after initial shortcut placement; Restore discovery was moved into the global header and the help copy was compacted without reducing existing action visibility.

## Verification

- `go test ./internal/cli ./internal/tui ./internal/manifest -count=1 -run Restore` — pass.
- `gofmt -l .` — pass, no files listed.
- `go vet ./...` — pass.
- `staticcheck ./...` — pass.
- `go test -race -count=1 ./...` — pass.
- `git diff --check` — pass.

## Scope notes

- No global or unselected Skills Folder is passed to restore planning or apply.
- No archive copy is deleted by the new surfaces; mutation semantics remain owned by the reviewed manifest restore engine at `93f4ab3`.
- The pre-existing untracked `x-skills` binary was left untouched and is not part of the commit.

## Review remediation

- Added real terminal detection for conflict prompts. `-y`, piped input, and EOF can no longer accept a suggested preserve name; conflicts require a TTY or fail.
- Reprint the finalized CLI plan after rename decisions. Destructive lines now include the exact Skills Folder source path and selected archive name.
- Classify and confirm every destructive normalization and removal, including normalization-only plans.
- Reworked the TUI restore workflow into reversible setup, preview, rename, and confirmation stages. Escape returns to the preceding stage while retaining checklist, Full, rename, and conflict-selection state.
- Added selectable multi-conflict navigation. Each conflict can be revisited and edited independently.
- Nested restore modals retain ownership of their plan while navigating backward; final preview cancellation and application quit close staging, while apply consumes the plan through the manifest engine.
- Added regression tests for piped non-TTY conflict input, finalized destructive details, normalization-only confirmation, staging cleanup, back navigation, and multi-conflict selection.
