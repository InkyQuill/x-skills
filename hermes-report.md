# Hermes Report: Go TUI Standardization Review

Date: 2026-07-08
Snapshot note: findings describe the pre-Install implementation reviewed on
this date; later changes may have resolved them.
Scope: `internal/tui/**`, `internal/cli/tui.go`, cross-checked against
`docs/adr/*`, `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`,
`2026-07-06-go-tui-views-mockups.md`, and
`2026-07-06-go-tui-install-and-repo-updates-design.md`.

## TL;DR

The shipped TUI (Active/Repo/Doctor) already implements most of the parity
spec correctly: uppercase global tabs, per-view selection-with-cursor-fallback,
typed modals, restrained Unicode + ASCII/NO_COLOR fallback, Glamour preview,
full-file diff conflict modal. But there are real gaps against the spec and
several inconsistencies *within* the shipped code itself — not just "not
built yet" items, but places where one view does something the others don't
for no documented reason. Below is what needs to be standardized, what's
outright missing, and what's actively wrong.

## 1. Standardization gaps (inconsistent between views today)

### 1.1 `enter` (details) only works in Active — spec requires it everywhere

`Model.openDetailModal()` (`internal/tui/model.go:156`) has a `switch m.view`
with **only** a `ViewActive` case. Pressing `enter` in Repo or Doctor does
nothing. The full-parity spec and views-mockups doc both show a Repo detail
modal (archive path, description, usage chips, source metadata) and a Doctor
issue-detail modal (issue kind, path, reason, proposed fix) — see
"Operational Detail Modal" mockup and the Spec Coverage Matrix rows "Repo
details" / "Doctor details". This is the single biggest functional
inconsistency: one view has the promised behavior, two don't.

**Fix:** add `repoDetailModal(skill, usages)` and `doctorDetailModal(issue)`
builders and wire them into `openDetailModal`'s switch, mirroring
`activeDetailModal`.

### 1.2 `p` (preview) is Active/Repo only, but that's actually correct

Spec says preview applies to Active and Repo, not Doctor — `openPreviewModal`
correctly has no Doctor case. No change needed here; flagging only so it's
not confused with 1.1's gap (this one is *intentional* scope, not a bug).

### 1.3 Selection model contradicts ADR 0010

ADR 0010 says: "Selection sets are stored per page, so Active, Repo, Doctor,
and Install can preserve local context without leaking actions across
pages." The actual implementation (`Model.selected map[string]bool`,
`model.go:31`) is **one shared map** across all views, with IDs
namespaced by prefix (`active:`, `repo:`, `doctor:`). `setView` clears the
whole map on every switch (`model.go:216-224`), which happens to produce the
*visible* behavior ADR 0010 and the grilling answer (A20: "Clear selection on
view switch") describe — but it does it by nuking everything, not by keeping
per-page sets. It works today only because selections are cleared on every
switch anyway. If a future change (R20's "might revisit... if it feels
awkward") turns off clear-on-switch, this single shared map will immediately
leak selections between pages, contradicting ADR 0010's explicit "selection
sets are stored per page" text.

**Fix:** either (a) update ADR 0010 to say selection uses one map cleared on
view switch (matches current code, simplest), or (b) change `Model.selected`
to `map[ViewName]map[string]bool` so the ADR text is accurate and the
architecture survives if clear-on-switch is ever relaxed. I'd recommend (b)
since R39 explicitly chose per-page storage as the fix for the real Repo-link
cursor-only bug — collapsing that back to a single map in code while the ADR
still claims otherwise is a doc/code mismatch waiting to bite the next
person who touches selection.

### 1.4 Historical: footer/help called Install "reserved"

`internal/tui/modal_help.go:28`: `"I", Label: "reserved for Install view"`.
This is now wrong per ADR 0015 (Install is a decided top-level page, not
"reserved"), and the `I` key isn't even wired in `model.go`'s key switch at
all — `keyActive`/`keyRepo`/`keyDoctor` exist in `keys.go` but there's no
`keyInstall` constant and no `case keyInstall:` in `handleKey`. Right now `I`
does nothing in the running app while the help text calls it "reserved,"
which is stale relative to both the design decision and (once Install ships)
the actual behavior.

**Fix (near-term, doc-only):** until Install is implemented, change the help
text to "Install (design in progress, not yet available)" rather than
"reserved" — reads as more accurate framing given ADR 0015 already exists.
**Fix (when Install ships):** add `keyInstall = "I"` to `keys.go`, a `ViewInstall`
case, and wire it exactly like the other three tabs for standardization.

### 1.5 Row-selection/cursor markers vs. Doctor's lack of `space`

Active and Repo rows render a selection checkbox (`m.symbols.Unchecked` /
`Checked`) via `rowPrefix`, and `toggleSelection()`/`" "` is globally wired in
`handleKey`. Doctor rows also render the same checkbox via `rowPrefix` (see
`renderDoctorRows`), and the global `" "` handler will happily toggle
`doctor:` IDs into `m.selected` — but there is **no Doctor action that ever
reads selections** (`openDoctorFixModal` always operates on `m.issues`, not
`m.selectedIDsForView()`). This matches the spec's explicit choice ("Doctor
`f` operates on all current Doctor issues, not cursor/selection fallback" —
R25), but the row rendering doesn't communicate that: Doctor rows show a
checkbox that looks selectable and toggles state, yet selecting rows has
*zero effect* on `f`. That's confusing UI, not just a missing feature.

**Fix:** either hide the checkbox/selection affordance in Doctor rows (since
selection there is inert), or explicitly grey it out with a note, so the UI
doesn't imply an interaction that doesn't exist. The parity spec's own keymap
table (`| space | ... | no selection in parity |` for Doctor) already says
Doctor shouldn't have selection — the row renderer just wasn't updated to
match.

### 1.6 `c` (clear selection) is undocumented in the parity spec's keymap table but shipped everywhere

`model.go` wires `case "c":` globally, and it's in every view's footer
(`views.go` `commandPalette`) and in the help modal. This is a *good*,
harmless addition, but it's not in the full-parity-design spec's keymap table
at all (checked: `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`
"## Keymap" section has no `c` row). Since it's shipped consistently across
all three views already, this is a documentation gap, not a code
inconsistency — but it should be added to the spec's keymap table so future
contributors don't "discover" it only by reading code, and so nothing removes
it thinking it's an accidental leftover.

### 1.7 Inconsistent conflict-diff legend text vs. established glossary

`modal_diff.go`'s `diffLegend()` renders `"Legend: Archive  Incoming active"`.
The synthesized Install/Repo-updates design doc (and R19, explicitly) settled
on the label pair **"Archive" vs "Incoming remote"** specifically to avoid
confusing "local" with "active root" paths — but that decision was about the
*remote install/update* conflict flow. The existing *active-migrate* conflict
flow (this file) predates that decision and correctly uses "Incoming active"
because the other side of that diff really is an active-root copy, not a
remote fetch. This is **not a bug** — but it is a subtle enough distinction
that a future implementer wiring up the Repo-update conflict diff (which
reuses `conflictDiffModal`) could easily forget to parameterize the legend
label and ship "Incoming active" for a remote update, which would be wrong
and inconsistent with the Install design doc.

**Fix (preventative):** parameterize `newConflictDiffModal` with an explicit
"incoming label" string now (default `"Incoming active"` for today's only
caller), so when Repo-update wiring lands it's forced to pass
`"Incoming remote"` instead of silently inheriting the wrong label.

## 2. Missing pieces relative to the accepted spec (not yet built)

These are legitimate scope gaps, not bugs — flagging for completeness since
the ask was "what needs to be changed/fixed."

- **No `viewport`/`textinput` from Bubbles.** ADR 0016 and R67 explicitly
  decided "Bubbles viewport for scrollable panes/text/diffs, custom rows for
  main lists." Today, `previewModal` and `conflictDiffModal` hand-roll their
  own scroll offset math (`clampScroll`, manual slicing) instead of using
  `bubbles/viewport`. `go.mod` even carries `bubbles` only as an *indirect*
  dependency (pulled in transitively, not `require`d directly), and
  `filterState` hand-rolls text input instead of using `bubbles/textinput`.
  This isn't broken, but it's a direct deviation from the accepted Charm-stack
  responsibility split, and it's why the hand-rolled scroll/input code is the
  most bug-prone part of the modal layer (see 3.2 below).
- **No async/Bubble Tea commands for reload.** ADR 0016 says `Update` should
  stay non-blocking, routing expensive work through commands + bounded
  workers. Today `m.reload()` runs synchronously inside `handleKey` on every
  `^R` and after every mutation — fine for local filesystem scans today, but
  this is the seam the future Repo-update background pipeline (ADR 0007) is
  supposed to hook into, and no scaffolding for that (tea.Cmd-based async
  loading, stale-result discarding) exists yet anywhere in `internal/tui`.
- **Install page (historical snapshot):** did not exist in the reviewed
  2026-07-08 snapshot; it is now implemented in `internal/tui/install.go`.
- **Repo rows have no source/update/audit badges.** Expected per the above;
  `repo.Skill` and the row renderer have no fields for tracked/update/audit
  state yet.
- **No `--no-unicode`/terminal-size responsive collapse guard** beyond the
  single `width < 100` check in `renderBody`. The full-parity spec's
  "Responsive Layout" section specifies a `100×30` minimum for the full
  shell and a *separate, stricter* minimum-size guard specifically for the
  fullscreen diff modal ("resize prompt, allow cancel, no summary fallback").
  `conflictDiffModal.View` has width/height clamping logic but no explicit
  "too small, please resize" state — it just squeezes the layout down to
  `minInnerWidth`/`minBodyHeight` and renders truncated/garbled content
  instead of the resize prompt the spec calls for.

## 3. Bugs / things that are just wrong today

### 3.1 `containsString` in `rows.go` looks unused outside its own package but has a near-duplicate

Not a functional bug, but worth a quick cleanup pass: `appendUnique` (used
constantly) calls `containsString`, which is fine — no duplicate found on
closer check. Retracting this one; false lead during review. (Kept in report
transparently rather than silently dropping it — no action needed.)

### 3.2 Preview modal scroll can be pushed negative-adjacent / no bounds re-check on resize

`previewModal.Update`'s `"down"` case does `p.scroll++` with **no upper bound
check at all** — `clampScroll` is only applied in `View()`, and `View` is
re-derived from `p.scroll` each render, so this self-corrects visually, but
it means `p.scroll` as *state* can grow unboundedly if a user holds `down`
past the end of a short file, then the value is silently re-clamped next
render. Harmless today, but it's exactly the kind of ad-hoc scroll math ADR
0016 wants replaced by `bubbles/viewport`, which manages this invariant
properly. Low priority, but a good first candidate to migrate.

### 3.3 `repoLinkModal.Update` double-applies on Enter

```go
case "enter":
    r.apply(m)
}
if msg.Type == tea.KeyEnter {
    r.apply(m)
}
```

(`internal/tui/actions.go:271-277`). `msg.String() == "enter"` and
`msg.Type == tea.KeyEnter` are the same condition reached two different ways,
so **`r.apply(m)` runs twice** for a single Enter keypress. `apply` calls
`actions.Link` and then sets `m.modal = newResultModal(...)`. Because the
first call already sets `m.modal` to a `resultModal` and returns, the second
`apply` call re-runs `actions.Link` against a *model already mutated by the
first call* (`m.reload()` already happened), then overwrites `m.modal` again
with a fresh result. Net visible effect is probably "link succeeds, no
visible double-link" because `actions.Link` is presumably idempotent-ish or
errors cleanly the second time — but this is fragile: if `Link` doesn't error
on an already-existing identical symlink, this silently performs a redundant
filesystem operation on every single link action, and if it *does* error the
second time, the result modal the user actually sees is the second
(failure) message, potentially reporting a link failure for an operation that
actually succeeded on the first call.

**Fix:** delete the redundant `if msg.Type == tea.KeyEnter { r.apply(m) }`
block; the `case "enter":` inside the switch above already covers it.

### 3.4 `chipStyle` is dead code

`internal/tui/styles.go` defines `chipStyle` (line 21) and NO_COLOR-resets it
(line 46), but nothing in `internal/tui` ever calls `chipStyle.Render(...)` —
verified via grep, the only two hits are the definition and its own reset.
Not a bug, but dead styling code that should either be wired up (if it was
meant to replace the ad-hoc `renderRootChip`/`Pill` path) or removed.

### 3.5 `rootChip()` symbol-mapping duplicated string-switch logic lives in two files with the same name/shape but different purposes

`internal/tui/symbols.go:45` has `rootChip(scope, target string) string`
which is the canonical `.Ag`/`~Cl` mapper used everywhere. This is fine as a
single source of truth — just flagging that anyone adding a 4th target
(hermes/opencode/etc., per the backlog's managed-agent-registry item) needs
to touch exactly this one function, and its `default: return prefix + target`
fallback will silently produce ugly 8+ character chips (`.opencode`) instead
of failing loudly, which will look broken in the fixed-width row layout the
moment that backlog item ships. Worth a defensive width-cap or a TODO
comment pointing at the backlog item so it isn't forgotten.

### 3.6 `renderInspector` for Active never shows the `reason`/broken diagnostics that the row's `activeDetail()` shows

The Active row (`renderActiveRows`) calls `activeDetail(group)` which shows
`group.Reason` (styled danger red) when status is broken. But
`renderInspector`'s `ViewActive` case (`views.go:100-105`) only ever prints
`"repo"` + `group.Status` — never `group.Reason`. So a broken active skill's
*reason* ("symlink target missing", etc.) is visible in the row but
disappears from the inspector panel that's supposed to be the "richer detail"
surface. This inverts the intended information hierarchy (rows = compact,
inspector = detailed) described in the spec's "Inspector And Details"
section, which explicitly lists "next likely action implication" and
diagnostic context as inspector content.

**Fix:** add a conditional line to the Active inspector case:
`if group.Status == actions.StatusBroken { lines = append(lines, "reason", "  "+group.Reason) }`.

## 4. Priority ordering (if only fixing a few things)

1. **3.3** — double-apply on repo link Enter (real bug, silent double
   filesystem op / possible spurious failure report).
2. **1.1** — missing Repo/Doctor detail modals (spec says all three views get
   `enter`; only one does).
3. **3.6** — Active inspector missing broken reason (inverts intended
   row/inspector information hierarchy).
4. **1.3** — selection storage model vs. ADR 0010 text mismatch (pick one:
   fix code or fix ADR, don't leave them contradicting).
5. **1.4/1.5** — Install "reserved" wording + Doctor's inert selection
   checkbox (both are "the UI says something that isn't true" issues).
6. **1.7** — parameterize conflict-diff legend label now, before Repo-update
   wiring copies the wrong string by default.
7. Everything in section 2 — expected/planned, not urgent, but worth linking
   from the Install/Repo-updates design doc's testing section so the
   Bubbles-viewport migration isn't forgotten once that work starts.

No build/test changes were made as part of this review — it is analysis
only. `go build ./cmd/... ./internal/...` and `go test ./cmd/... ./internal/...`
both currently pass; none of the above are currently caught by existing
tests (confirming the review found real gaps, not already-covered
regressions).
