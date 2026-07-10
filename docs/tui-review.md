# TUI Production-Readiness Review

Scope: `internal/tui/**` (~13.4k lines incl. tests) on branch `go-rewrite-prototype`.
Method: full manual read of every non-generated file, `go vet`, `staticcheck`, `go test -race`,
and a targeted deep-dive on `install.go` (the largest, network/disk-touching file).

## Blocking issues

### 1. A test currently fails on this branch
`go test ./internal/tui/... -race` fails:

```
--- FAIL: TestInspectorRendersBlockValueWithoutTruncatingDescription (internal/tui/inspector_test.go:88)
```

The rendered block-value wrap for the inspector "Description" row leaves trailing
whitespace / uneven padding on wrapped lines instead of the tightly-wrapped text the
test expects (e.g. `"Use when adding, altering, or removing "` with a trailing space,
and `"enough detail to wrap."` padded out with ~17 trailing spaces). `inspector.go`
and `inspector_test.go` are currently **untracked** (new, uncommitted WIP), so this
isn't a regression against `main` — but it must be fixed and the file committed before
this is release-ready. `go vet` and `staticcheck` are otherwise clean on the package.

**Action:** fix `wrapInspectorText`/`blockInspectorValue` in `internal/tui/inspector.go`
so wrapped lines don't carry trailing padding, then `git add` the new files.

## High-severity findings (install.go — the Install/marketplace tab)

### 2. Cancelled/stale batch archive operations still run to completion
`archiveInstallRows` (`install.go:425-467`) and the `cmd().(installArchiveMsg)` unwrap
loop it drives never check `useGeneration.isCurrent(token)` before executing each row's
command — contrast with `installAndUseRowsWithProgress` (lines ~1002-1055), which does
check liveness before every step. If a user multi-selects N skills, presses **a**
(archive), and then immediately starts a new search or leaves the Install tab, the
in-flight batch keeps performing git checkouts and disk writes for every remaining row.
The result is discarded via a token check only *after* all the work is done
(`applyInstallArchiveResult`, line ~1483). Net effect: wasted bandwidth/disk I/O and a
window where the old batch's checkout cache use can race the new search's.

**Fix direction:** thread the batch's capture token through `archiveInstallRows` and
bail out per-row (like the install/use path already does) instead of only filtering the
final message.

### 3. Search results can trigger an unbounded burst of concurrent git checkouts
`applyInstallSearchResult` (`install.go:1384+`) calls `installArchiveStateCheck` for
*every* already-archived result in a page of search results and batches them all with
`tea.Batch`. Each check does a full `checkouts.Checkout` (clone/fetch) +
`FindSkillContext`. A search that returns, say, 20 already-archived results from the
same owner fires ~20 simultaneous git network operations with no concurrency cap and no
de-duplication by repo (multiple skills from the same repo each re-clone/re-fetch
independently). On a slow network or against a rate-limited host this will be visibly
janky and could trip GitHub's abuse-rate limiting.

**Fix direction:** cap concurrency (e.g. a small worker pool) and coalesce checkouts by
`(owner, repo)` before firing state checks.

## Medium-severity findings

### 4. Silent failures in the background "update available" check
Inside `installArchiveStateCheck`'s `tea.Cmd` (`install.go:1432-1465`), every failure
path (checkout error, `FindSkillContext` error, `PlanArchive` error) returns
`installArchiveStateMsg{state: ""}`, and `applyInstallArchiveStateResult` silently
drops empty-state messages (line ~1467). A transient network failure or a rate limit
during this background recheck produces **zero** user-visible feedback — the row just
looks up-to-date, with no way for the user to tell "no update" apart from "check
failed." Worth at minimum surfacing a muted "check failed" pill, or logging.

### 5. Unchecked type assertions rely on an unenforced invariant
`cmd().(installArchiveMsg)` appears three times (`install.go:441, 895, 1009`) unwrapping
a `tea.Cmd` synchronously. It's safe today only because `archiveInstallRow` always
returns a non-nil closure of that exact message type — but `archiveInstallRows` guards
with `if cmd == nil { continue }` (line 429-431), which, if a future code path in
`archiveInstallRowWithConflict` ever legitimately returned `nil`, would silently drop
that row from `commands` while later indexing (`rows[i]` at lines ~446/452/459/463)
assumes `commands` and `rows` stay index-aligned — misattributing results to the wrong
skill name. Not currently triggered; worth a comment or an assertion so it fails loudly
if that invariant is ever broken.

### 6. Reload is always synchronous, and the async path is dead code
`model.go:330` defines `reloadCmd()` — a `tea.Cmd` that runs `loadTUIData` off the
render loop and dispatches `reloadResultMsg` — but **nothing calls it** (verified via
grep, only the definition matches). Both startup (`New()` → `m.reload()`, `model.go:76`)
and the manual refresh key (`Ctrl+R` in `handleKey`, `model.go:154`) call the
**synchronous** `m.reload()`, which blocks the whole event loop — including all
key/redraw handling — until `actions.ScanActive`, `repo.List`, and `doctor.Diagnose`
finish walking the filesystem. On a large skills tree or a slow/networked filesystem
this will freeze the TUI with no spinner or feedback. Either wire up `reloadCmd` for
both paths, or delete it if synchronous loading is an intentional simplification (in
which case at least show a status line while it's mid-scan, since `reloadResultMsg` and
its token-matching plumbing already assume async is the goal).

## Low-severity / polish

### 7. No explicit cancellation on navigating away from Install
All network/checkout operations rely purely on `context.WithTimeout(context.Background(), ...)`
(60s-ish timeouts throughout `install.go`) with no `context.WithCancel` tied to view
changes, closing the app, or `ctrl+c`. Leaving the Install tab or quitting doesn't
proactively cancel outstanding HTTP/git work — it just runs to timeout with the result
discarded via token checks. Combined with findings #2/#3, background resource usage can
outlive user intent by up to a minute per operation. Low severity because it's bounded
and self-cleans, but worth a `context.WithCancel` wired to `tea.Quit`/view-change if this
ships as a long-lived TUI people leave running.

### 8. Large multi-select batches have no aggregate timeout or progress detail
`archiveInstallRows` and `installAndUseRowsWithProgress` run strictly sequentially, each
row with its own ~60s timeout but no cap on the whole batch. A 30-skill multi-select with
a few slow network calls can make the UI appear to hang for minutes with only a generic
"archiving N skills..." status, no per-item progress or the ability to cancel mid-batch.

### 9. `git-status`-visible security check: passed
Verified (not just assumed) that path-traversal protection for archive/skill names is
solid: `internal/remote/add.go:209` (`validateArchiveName`) rejects absolute paths, `.`,
`..`, `filepath.Clean` mismatches, and path separators before any `filepath.Join`.
`Owner`/`Repo` from search results are never joined into filesystem paths — only
interpolated into a `CloneURL` string passed to `git clone` as a CLI argument, into a
`os.MkdirTemp`-generated temp directory. No injection or traversal vector found here.

## Everything else reviewed (model.go, views.go, actions.go, rows.go, filter.go,
diff.go, inspector.go, modal*.go, animation.go, keys.go, symbols.go, styles.go,
options.go, ui/components.go)

No correctness bugs found. Notably solid:
- `NO_COLOR` handling is thorough and centralized (`styles.go` `init()`).
- Modal stack (`modal.go`) has consistent scroll-clamping and small-terminal fallbacks
  (e.g. `conflictDiffModal` refuses to render below a minimum size instead of corrupting
  layout).
- `ArchiveConflictError` handling threads success/failure accumulators correctly through
  recursive conflict-resolution continuations in `actions.go` (migrate/unlink), including
  when a conflict interrupts a multi-select batch partway through.
- Cursor/selection state resets correctly on view switch (`setView`, `model.go:360`).

Two very minor observations, not worth separate fix tickets:
- `conflictDiffModal.Update`'s "down"/"pgdown" increment `scroll` with no upper clamp in
  `Update` itself (only clamped later, per-render, in `View`) — harmless (an `int` that
  grows unboundedly under repeated key-mashing, re-clamped every frame) but inconsistent
  with `resultModal`/`detailModal`, which clamp in `Update`.
- `symbols.go`'s `Managed`/`Unmanaged`/`Broken` all render the same glyph (`●`) in the
  non-ASCII symbol set, distinguished only by color — fine for color terminals, but
  worth double-checking against the `NO_COLOR` path (falls back to plain `mutedStyle`
  with no differentiation at all between unmanaged/broken there beyond text elsewhere).

---

## Standardization opportunities (moving shared code into `internal/tui/ui`)

The package already started this (`ui/components.go` has `Pill` and `Shortcut`/
`ToolHints`), but a lot of near-identical boilerplate remains duplicated across the
~10 modal files. Concrete candidates to promote:

1. **Footer/command-palette line.** The exact expression
   `mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{...}))`
   is repeated verbatim in every modal (`modal_confirm.go`, `modal_choice.go`,
   `modal_detail.go`, `modal_help.go`, `modal_preview.go`, `modal_result.go`,
   `modal_diff.go`, plus the ad-hoc modals in `actions.go`). A single
   `tuiui.FooterLine(ascii bool, keyStyle, mutedStyle lipgloss.Style, shortcuts []Shortcut) string`
   would collapse ~10 call sites to one line each and make future footer styling changes
   a one-place edit.

2. **Scroll-clamp-on-move boilerplate.** `detailModal.Update`, `helpModal.Update`, and
   `resultModal.Update` all contain the identical pattern:
   ```go
   if delta := modalMoveDelta(msg); delta != 0 {
       x.scroll += delta
       if x.scroll < 0 { x.scroll = 0 }
       m.modal = x
   }
   ```
   Worth a small embeddable `scrollState` helper (`HandleScroll(msg) (newScroll int, handled bool)`)
   in `modal.go` or `ui`, since three of the ~10 modal types are otherwise just
   "title + scrollable body + standard footer" with nothing else going on
   (`detailModal`, `helpModal`, `resultModal` are structurally the same modal).

3. **`renderWithOptionalBackground`** (`styles.go:536`) is a fully generic
   "render this style, optionally overriding its background" helper with zero
   `Model`/`tui`-package coupling — a good fit for `ui/components.go` alongside `Pill`,
   which already takes the same `Color`/`Background` shape.

4. **`renderRootChip`/`renderRootChips`** (`views.go:547-570`) is really just
   "pick a Pill color based on scope, then call `tuiui.Pill`" — the scope→color
   decision could stay in `tui`, but the chip-list-joining part (`renderRootChips`)
   is generic enough to live next to `ToolHints` in `ui`.

5. **`truncate`** (`styles.go:130`) is a pure ANSI-aware string truncator with no
   `Model` dependency — same story, natural fit for `ui`.

6. **`clampModalIndex`/`clampScroll`/`visibleModalBody`** (`modal.go`) are pure
   pagination/windowing math, already dependency-free — could move to `ui` so any
   future non-modal list view (or a different TUI reusing this package) gets them for
   free.

None of these are urgent — they're internal duplication, not bugs — but given the
`ui` package is clearly meant to be the shared-component layer (that's the whole
reason `Pill`/`Shortcut`/`ToolHints` already live there), it's worth sweeping the
rest of this boilerplate over in the same pass rather than letting the modal files
keep re-deriving it ten different times.
