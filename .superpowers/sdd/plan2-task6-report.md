# Plan 2 Task 6 Report

## Outcome

Implemented built-in skill diagnosis and repair across Doctor core, CLI, and TUI.

- `missing-builtin` reports catalog skills absent from the archive.
- `inactive-builtin` reports archived catalog skills with no managed global link.
- Globally linked built-ins are healthy.
- Fixes archive through `builtin.Archive` and link through `actions.Link`, preserving earlier results when a later archive or link conflicts.
- `doctor --fix -y` without `--at` archives built-ins and reports `archived but inactive`; it does not guess a Skills Folder.
- Explicit global `--at` destinations archive and link. Project destinations are not used for built-in fixes.
- Interactive CLI and TUI flows show enabled global Skills Folders with `~Ag` preselected and an explicit `Archive only` choice.
- The TUI checklist opens only when the user invokes the Doctor fix action, so dismissing it does not cause refresh-time modal reopening and the Doctor list remains usable.

## TDD evidence

Observed failing tests before implementation:

- `go test ./internal/doctor -run BuiltIns -count=1` failed to compile because `KindMissingBuiltIn` and `KindInactiveBuiltIn` did not exist.
- `go test ./internal/doctor -run FixBuiltIns -count=1` failed because no archive/link results were produced and project destinations were accepted.
- `go test ./internal/cli -run 'DoctorFixBuiltIns' -count=1` failed because CLI fixes produced `No fixes applied.`
- `go test ./internal/tui -run 'Doctor.*BuiltIn|DoctorFixModal' -count=1` failed because the old confirmation modal had no built-in destination checklist.

Each group passed after its minimal implementation, followed by the complete verification chain below.

## Exact verification evidence

Executed from `/home/inky/Development/x-skills` after the final edit:

```text
$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ staticcheck ./...
(no output)

$ go test -race -count=1 ./...
all packages passed; internal/tui completed in 1.845s, with no race reports

$ go test ./internal/doctor ./internal/cli ./internal/tui -count=1 -run 'BuiltIn|Builtin'
ok github.com/InkyQuill/x-skills/internal/doctor 0.002s
ok github.com/InkyQuill/x-skills/internal/cli 0.009s
ok github.com/InkyQuill/x-skills/internal/tui 0.009s

$ git diff --check
(no output)
```

## Review

Zen of Go review found no blocking issues after the verification chain. The implementation keeps mutation errors explicit, avoids replacement paths, uses existing feature primitives, and retains partial-success results.

## Scope note

The pre-existing untracked `x-skills` binary was not modified or staged.
