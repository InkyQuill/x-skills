# Cross-Platform Filesystem Portability Design

## Goal

Make `x-skills` pass and behave correctly on Linux, macOS, and Windows by treating filesystem identity semantically instead of as raw path strings, and by supporting archive rename on all three platforms.

## Current Evidence

The `CI` workflow for commit `d557f35` passed on Ubuntu and failed on macOS and Windows.

macOS failures cluster around two causes:

- `filepath.EvalSymlinks` resolves `/var/...` to `/private/var/...`, while tests and drift checks compare the resolved path to the original temp path string.
- `internal/actions/rename_noreplace_other.go` returns `atomic no-replace rename is unsupported on darwin`, so archive rename workflows cannot run.

Windows failures cluster around two causes:

- The runner exposes equivalent paths as both `C:\Users\RUNNER~1\...` and `C:\Users\runneradmin\...`; path equality checks treat those short and long forms as different.
- `internal/actions/rename_noreplace_other.go` returns `atomic no-replace rename is unsupported on windows`, so archive rename workflows cannot run.

Ubuntu passes because Linux temp paths do not expose these aliases in CI and `renameat2(..., RENAME_NOREPLACE)` is implemented.

The codebase currently has inconsistent path identity helpers. `internal/actions/scan.go` has `samePath`, which only applies `filepath.Abs` and `filepath.Clean`; `internal/syncer/plan.go` has `sameCanonicalPath`, which also uses `filepath.EvalSymlinks`. Consolidating these into one helper avoids platform-specific behavior depending on which package performed the comparison.

## Requirements

- Path identity checks must work across common platform aliases:
  - macOS `/var` and `/private/var`.
  - Windows short and long path forms.
  - Different path separator spellings where Go accepts them.
- Drift checks must still reject real mutations, target changes, and content changes.
- Archive rename must be no-replace on Linux, macOS, and Windows.
- Tests must assert semantic path identity, not incidental string spelling.
- CI must run `go test ./...` and `go build ./cmd/x-skills` on Ubuntu, macOS, and Windows without skips for core filesystem behavior.
- The design should leave room for future BSD support, preferably by sharing the macOS Unix fallback when appropriate.

## Architecture

Add a focused `internal/pathidentity` package. It owns platform-aware path canonicalization and equality so production code and tests do not grow scattered normalization rules.

The package exposes:

```go
func Canonical(path string) (string, error)
func CanonicalEntry(path string) (string, error)
func Equivalent(a, b string) bool
func EquivalentE(a, b string) (bool, error)
```

`Canonical` returns a stable absolute spelling for an existing path. It should resolve symlinks when possible and apply platform-specific canonicalization.

`CanonicalEntry` canonicalizes the parent directory and rejoins the base name. This handles planned paths that do not exist yet, such as archive destinations and active links.

`EquivalentE` answers whether two paths refer to the same filesystem location and surfaces unexpected filesystem errors. It first tries filesystem-backed checks (`os.Stat` plus `os.SameFile`) when both paths exist. If either path does not exist, it falls back to `CanonicalEntry` on both paths, then compares canonical strings. Permission errors and other non-existence errors should be returned instead of silently degrading.

`Equivalent` is a convenience wrapper for call sites where a conservative `false` on error is acceptable. Critical drift checks should use `EquivalentE` so permission and stat failures remain visible.

Platform-specific details:

- Linux and macOS use `filepath.Abs`, `filepath.EvalSymlinks` where possible, and `os.SameFile` for existing paths.
- macOS inherits the generic Unix behavior; resolving symlinks handles `/var` to `/private/var`.
- macOS default APFS is case-insensitive. `os.SameFile` handles differently cased existing paths, while fallback canonical string comparison may not. This is acceptable for non-existent entries because `x-skills` skill names are case-sensitive identities and destination creation should preserve the requested name.
- Windows should prefer standard library behavior if it resolves the observed short-name issue. Verify whether `filepath.EvalSymlinks` already strips `\\?\` prefixes and resolves short names on the supported Go version. Use `golang.org/x/sys/windows` final-path APIs only if the standard library is insufficient for the CI failures. Case-insensitive fallback comparison is allowed after OS-backed canonicalization or absolute cleanup.

## Rename Support

Keep the existing Linux implementation using `unix.Renameat2` with `unix.RENAME_NOREPLACE`.

Add platform implementations:

- Build constraints must be updated before adding platform files. Either change `internal/actions/rename_noreplace_other.go` to `//go:build !linux && !darwin && !windows`, or remove that fallback in favor of explicit per-platform files. Do not add `rename_noreplace_darwin.go` or `rename_noreplace_windows.go` while the fallback still matches those platforms.
- macOS, with a path to future BSD support: use a no-replace algorithm that first creates a destination placeholder with exclusive creation, removes that placeholder, then performs the rename while holding mutation-level locking. If a stronger platform syscall is available in the Go/x/sys version in use, prefer it. If the placeholder fallback is used, document the residual time-of-check/time-of-use race: another process can create the destination between placeholder removal and `os.Rename`. The process-wide mutation lock only prevents in-process races. This residual cross-process race is an accepted macOS portability trade-off for this pass, mitigated by preflight, revalidation, and the destination-exists check immediately before mutation.
- Windows: use a Windows-specific no-replace strategy that fails if the destination exists and does not overwrite. Prefer native Windows file move APIs with no replace semantics if exposed by `golang.org/x/sys/windows`; otherwise use an exclusive destination placeholder plus checked rename under the mutation lock.

The rename abstraction remains `renameNoReplace(oldPath, newPath) error`, so higher-level archive rename rollback behavior does not change.

## Integration Points

Update these areas to use path identity semantics:

- `internal/actions/scan.go`: replace `samePath` string comparison with `pathidentity.Equivalent` or `EquivalentE`.
- `internal/actions/migrate.go`: managed-link detection should use `pathidentity.EquivalentE` when an error should be surfaced.
- `internal/actions/rename.go`: usage discovery and drift revalidation should use `pathidentity.EquivalentE` and canonical entry paths.
- `internal/symlinkcheck`: return canonical resolved paths for stable downstream classification.
- `internal/syncer`: replace local canonical helpers and drift comparisons with `pathidentity.Canonical`, `CanonicalEntry`, and `EquivalentE`.
- Tests that currently compare `filepath.EvalSymlinks(...)` directly to raw expected strings should use path identity assertions.
- TUI modal tests that assert rendered paths should either canonicalize expected strings or assert the meaningful path suffix/context rather than platform-specific temp root spelling.

## Testing

Add unit tests for `internal/pathidentity`:

- Existing same directory returns equivalent.
- Existing symlink target returns equivalent to its resolved target.
- Missing entry canonicalization preserves the base name while canonicalizing the parent.
- `EquivalentE` falls back to `CanonicalEntry` when one or both paths do not exist.
- `EquivalentE` returns permission and stat errors that are not `os.ErrNotExist`.
- Case and separator handling is correct on Windows.
- Windows short and long user profile paths compare equivalent.
- macOS `/var` alias behavior is covered when running on Darwin.

Add rename tests that run on all supported OSes:

- Renaming an archive succeeds when the destination is absent.
- Renaming fails without replacing when the destination exists.
- Rollback behavior still preserves externally modified links and manifests.

Run verification:

```bash
go test ./...
go build ./cmd/x-skills
```

CI must pass the same commands on `ubuntu-latest`, `macos-latest`, and `windows-latest`.

## Non-Goals

- Do not add BSD CI in this pass.
- Do not redesign archive rename UX.
- Do not change the release workflow beyond what is necessary to keep cross-platform CI honest.
- Do not weaken drift checks to hide platform differences; normalize identity while preserving mutation detection.
