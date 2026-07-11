# Plan 3 Task 7 Report

Implemented Git-hygiene diagnostics and conservative Doctor fixes.

## Behavior

- Reports an existing untracked `.x-skills.yaml` with the manual command `git add -- .x-skills.yaml`.
- Reports a tracked `.x-skills.local.yaml` with the manual command `git rm --cached -- .x-skills.local.yaml`.
- Reports tracked files beneath configured project Skills Folders with an exact recursive `git rm -r --cached -- <path>` suggestion.
- Skips Git-hygiene findings outside a Git worktree and ignores untracked local files.
- `doctor --fix` only appends normalized `.gitignore` rules. It never stages files or removes entries from the Git index.
- Existing TUI Doctor fixes remain asynchronous because the new synchronous package work continues to run inside the existing Bubble Tea commands.

## Tests

- Added temporary-Git-repository coverage for every required tracking state.
- Added package coverage proving fixes preserve the Git index and append ignore entries idempotently.
- Added CLI coverage for diagnostic and fix output, including exact manual Git commands.

## Verification

- `go test ./internal/manifest ./internal/doctor ./internal/cli ./internal/tui -count=1`
- `go test ./... -race -count=1`
- `go vet ./...`
- `staticcheck ./...`
- `go build -o /tmp/x-skills ./cmd/x-skills`
- `git diff --check`
