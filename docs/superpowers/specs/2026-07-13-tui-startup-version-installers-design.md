# TUI Startup, Version, and Installer Design

**Date:** 2026-07-13

## Summary

Restore semantic colors and unambiguous status markers in TUI rows, render the TUI before
potentially slow skill discovery completes, display build and update information beside the app
name, and make release and development installation safe to repeat.

The implementation introduces a single build-information boundary, moves initial filesystem work
into Bubble Tea commands, preserves safe ANSI styling in canonical row components, and gives both
release and development installers explicit overwrite behavior.

## Goals

- Restore row foreground colors without reintroducing unsafe terminal control sequences.
- Use status markers that cannot be confused with the selection marker.
- Render the application shell immediately and show progress while skills data loads.
- Show the running version beside `x-skills` and indicate when a newer stable release exists.
- Keep update checks best-effort, asynchronous, and silent when unavailable.
- Make reinstalling over an existing binary an expected, clearly reported operation.
- Provide development installers that overwrite the normal installation with a binary whose
  version is `dev`.

## Non-goals

- Automatic application updates from inside the TUI.
- Update notifications for prereleases or development builds.
- A persistent update-check cache in this pass.
- Redesigning the broader TUI layout, navigation, or color palette.
- Making every filesystem scan operation interruptible when its current lower-level API does not
  accept a context.

## Current-state findings

### Row-color regression

Commit `fdacbe4` added security hardening to selectable rows by applying `ansi.Strip` to all row
segments. That removes both unsafe terminal controls and safe SGR foreground styling. Consequently,
ordinary, focused, and selected rows lose their semantic colors and resemble `NO_COLOR` output.

The codebase now has `ui.SanitizeANSI`, which preserves plain SGR styling while dropping OSC, DCS,
non-SGR CSI sequences, and unsafe raw control bytes. The selectable-row component should use that
boundary instead of stripping all ANSI sequences.

### Status-marker regression

Commit `56a2bd4` changed the status markers to `✓`, `◇`, and `×` to distinguish states without
color. The unmanaged diamond conflicts visually with the unchecked selection diamond. The earlier
all-circle experiment also depended entirely on color and was ambiguous under `NO_COLOR`.

### Slow startup

`tui.New` currently calls `reloadSynchronously()`. Skill scanning and doctor checks therefore finish
before Bubble Tea starts, leaving the user with a silent terminal during a large scan. The model
already has command-based reload and token-based stale-result handling that can be adapted for
initial loading.

### Version and release wiring

The binary currently has no runtime build-version source. The release workflow also builds snapshot
artifacts before semantic-release determines the next version. Version injection must therefore be
connected to the semantic-release result, rather than inferred from the previous tag or snapshot
version.

### Installer behavior

The installers copy directly onto the destination and do not explicitly tell the user when an
existing installation is being replaced. Installation should stage a complete binary first and
make replacement an intentional, idempotent path.

## Architecture

### Build information

Add a small `internal/buildinfo` package as the single source of application-version behavior.

It owns:

- a linker-injected version variable whose source default is `dev`;
- normalization for display (`dev` or `v<semver>`);
- validation and stable semantic-version comparison;
- a best-effort latest-release checker behind a narrow interface.

The CLI reads the package-level build information and passes an immutable value plus an injectable
update checker into the TUI through `tui.Options`. The TUI renders supplied state; it does not parse
versions or implement GitHub HTTP behavior.

The same build information powers a conventional `x-skills version` command so scripts, installer
tests, and users can verify the installed build without opening the TUI.

### Latest-release check

The production checker resolves GitHub's stable `releases/latest` endpoint with a short request
deadline and derives the latest tag from the redirect or final release URL. It does not require a
GitHub token and does not select prereleases.

Checks run only for a valid release build. `dev`, malformed, equal, and newer local versions do not
produce an update badge. Network failures, timeouts, invalid responses, and invalid remote versions
are silently ignored in the TUI.

The checker is injected so tests use deterministic local HTTP servers or fakes rather than GitHub.

### Bubble Tea startup

`tui.New` becomes fast and performs no filesystem or network I/O. It initializes:

- empty skill collections;
- an explicit initial-loading state;
- a token for the initial data request;
- build information and the update-check dependency.

`Model.Init` batches the independently useful commands:

1. initial skills-data load;
2. latest-release check when eligible;
3. animation tick when animations are enabled.

The normal application header, tabs, panels, and footer render immediately. While data is pending,
the list area shows an animated `Loading skills data…` state. Quit and help remain responsive.

When the load finishes, a token-matched result replaces the loading state with populated data. A
failure uses the existing actionable error presentation. A refresh started after initial loading
invalidates any older result so stale data cannot overwrite newer state.

Quitting cancels work that accepts cancellation. Lower-level synchronous filesystem calls that do
not currently accept a context may finish in their command goroutine, but their results cannot be
applied after the program exits. Adding context support throughout unrelated filesystem packages is
outside this pass.

## TUI presentation

### Header badges

The wide-form header is conceptually:

```text
◆ x-skills  v1.0.4  update v1.0.5  A:Active  R:Repo  D:Doctor  I:Install  s:Restore  S:Sync
```

The version is always shown as `dev` or `v<semver>`. The update badge appears only when the latest
stable release is newer than the running release. It uses semantic accent styling, but retains
explicit text under `NO_COLOR`.

Header composition remains ANSI-aware and is truncated to terminal width. The title and running
version take priority; lower-priority material may truncate at narrow widths without breaking
escape sequences.

### Status and selection markers

Unicode mode uses:

| Meaning | Marker | Color |
| --- | --- | --- |
| Managed | `●` | Green |
| Unmanaged | `○` | Amber |
| Broken | `×` | Red |
| Unchecked selection | `◇` | Existing selection style |
| Checked selection | `◆` | Existing selection style |

The filled and empty circles remain distinguishable without color, and neither is a diamond.
Broken remains the current multiplication sign.

ASCII mode remains explicit and distinct:

| Meaning | Marker |
| --- | --- |
| Managed | `+` |
| Unmanaged | `?` |
| Broken | `x` |
| Unchecked selection | `[ ]` |
| Checked selection | `[x]` |

### Safe canonical row rendering

Selectable-row rendering must preserve semantic styles emitted by trusted Lip Gloss renderers while
sanitizing all content at the shared row boundary.

- Use `ui.SanitizeANSI` rather than `ansi.Strip` for complete row segments.
- Preserve SGR foreground and background styling.
- Drop OSC hyperlinks, DCS, cursor movement, screen clearing, and unsafe control bytes.
- Keep ANSI-aware display-width calculation and truncation.
- Apply focused or selected backgrounds without erasing segment foreground colors.
- Keep the same security behavior for ordinary, focused, and selected rows.

This restores colors without trusting skill names, descriptions, paths, or remote metadata as raw
terminal output.

## Release pipeline

Release artifacts must be built after semantic-release has calculated the next version. The release
workflow should expose that exact version to GoReleaser, and GoReleaser should inject it into the
`internal/buildinfo` linker variable for every platform artifact.

The implementation must not derive a release version from the previous tag, commit distance, or the
current snapshot template. Development builds retain the source default `dev` unless a caller
explicitly injects another value.

The release sequence remains responsible for tests, cross-platform archives, checksums, installers,
GitHub release notes, and uploaded assets. Its ordering changes so versioned artifacts and the
release metadata agree.

## Installer behavior

### Release installers

Both `scripts/install.sh` and `scripts/install.ps1` follow the same observable sequence:

1. Detect the platform and choose the requested or latest release asset.
2. Download and extract into a temporary location.
3. Verify that the expected executable exists.
4. Ensure the destination directory exists.
5. If the destination binary exists, print
   `existing x-skills found at <path>; replacing it`.
6. Stage the complete executable on the destination filesystem and replace the destination.
7. Preserve the current non-destructive behavior when the `xs` shortcut name is already occupied.
8. Print the final installed path.

Running the installer repeatedly is successful and replaces the installed binary with the selected
version. Staging on the destination filesystem prevents a partial executable and permits an atomic
rename where the platform supports it.

Windows may refuse replacement only when the executable is actively locked. That case reports an
actionable instruction to close the running application and retry; merely having x-skills already
installed is not an error.

### Development installers

Add shell and PowerShell development installers for platform parity. They:

- require a local Go toolchain and run from a source checkout;
- build `./cmd/x-skills` with the version explicitly set to `dev`;
- use `X_SKILLS_INSTALL_DIR` or the same `~/.local/bin` default as release installation;
- report and replace an existing `x-skills` binary;
- maintain the same safe `xs` shortcut behavior;
- leave a binary for which both `x-skills version` and the TUI display `dev`.

The development path intentionally overwrites the normal installation instead of installing a
second command such as `x-skills-dev`.

## Error handling

- Initial data errors replace the loading state with the existing user-visible error state.
- Update-check errors never replace status text, interrupt interaction, or fail startup.
- Stale asynchronous messages are ignored using request tokens.
- Installer download, extraction, validation, permission, and replacement errors identify the
  operation and destination involved.
- A locked Windows binary has a dedicated close-and-retry message.
- Development installation fails early with an actionable message when Go or the source entrypoint
  is unavailable.

## Testing

### Build information and updates

Use table-driven tests for:

- default `dev` display;
- valid release normalization;
- malformed local or remote versions;
- equal, older, and newer remote versions;
- GitHub latest-release redirect success;
- offline failure, timeout, and invalid redirect handling;
- prerelease exclusion.

### TUI startup and rendering

- Inject a blocked loader and assert that construction returns immediately.
- Assert that the first view contains `Loading skills data…` before releasing the loader.
- Assert transition to populated data and to actionable load failure.
- Assert that stale initial or refresh messages cannot overwrite newer data.
- Assert that quit and help work during initial loading.
- Cover header badges at wide and narrow terminal sizes.
- Assert the Unicode and ASCII marker contracts.
- Assert meaningful status shapes under `NO_COLOR`.
- Force a color-capable profile and assert that row SGR foreground styling survives.
- In ordinary, focused, and selected rows, assert that unsafe OSC, non-SGR CSI, and control bytes
  are removed.

### Installers and release wiring

- Install twice into a temporary directory and assert that the second run succeeds, reports
  replacement, and contains the newer test binary.
- Replace a release-labeled binary through the development installer and assert that
  `x-skills version` returns `dev`.
- Exercise Unix and PowerShell behavior in their respective existing CI matrix jobs.
- Assert that GoReleaser contains the expected build-info linker flag and that release artifacts use
  the semantic-release-selected version.

## Completion criteria

- The TUI frame appears before skill scanning completes and visibly reports loading.
- Row state colors render normally while `NO_COLOR` remains respected.
- Managed, unmanaged, broken, and selection markers match the approved contract.
- Release and development versions appear correctly in both the CLI and TUI.
- A newer stable GitHub release produces a non-blocking update badge.
- Offline startup remains quiet and functional.
- Re-running either installer intentionally replaces the existing binary.
- `go test ./...` passes on Linux, macOS, and Windows CI.
- `go test -race ./internal/tui/...` and `go vet ./...` pass.
- Representative release and development builds report their injected versions correctly.
