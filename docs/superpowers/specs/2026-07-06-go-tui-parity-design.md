# Go TUI Parity Design

Date: 2026-07-06

## Context

The Go rewrite currently implements a Bubble Tea TUI at `x-skills tui`. Earlier
interactive specs describe a Python/Textual `x-skills interactive` manager and a
Go vertical slice. Those specs are useful references, but the current Go app is
not yet parity:

- command name is `tui`, while README still references `interactive` in places;
- the Go UI uses custom Bubble Tea rendering rather than Bubbles list/viewport
  components;
- mutation prompts are shallow wizards, not full guided conflict flows;
- linked-group migrate is only partially modeled through content fingerprint
  grouping;
- remote search/install and repo update checks are still absent from the Go TUI.

This file is the saved target design for Go TUI parity.

## TUI Shape

The Go TUI should be a guided maintenance manager with four persistent regions:

1. Header: product name, active view tabs, current scope summary.
2. Main list: scrollable active/repo/doctor rows, with cursor always visible.
3. Inspector/action panel: selected item details, symlink target, full paths,
   fingerprint/debug identifiers only when explicitly useful.
4. Footer: persistent shortcuts that never disappear behind wizards, logs, or
   status messages.

Shortcuts remain available as keyboard accelerators, but destination and action
state must be visible in readable labels rather than only `p/g/1/2/3`.

## Archive Conflict Flow

Migrate must handle an existing archive destination by content fingerprint:

- If active and archive directory fingerprints are identical, do not replace the
  archive. Remove the duplicate active directory and link the active location to
  the archive when migration needs link-back behavior.
- If fingerprints differ, do not fail with a raw filesystem error. Show a
  side-by-side summary:

```text
Archive conflict for zen-of-go

archive                              active
SKILL.md file f678670f184d           SKILL.md file 81ff127a050b

k keep archive, discard active
l save active over archive
esc cancel
```

Choices:

- Keep archive: discard active copy and link active location to archive.
- Save active: replace archive with active copy and link active location to the
  new archive.
- Cancel: no mutation.

For CLI non-interactive mode, divergent conflicts should return an actionable
error with the same summary. Replacement flags or an interactive CLI prompt can
be added later; the TUI is the primary conflict-resolution surface for now.

## Linked Groups

Active rows group identical content by directory fingerprint. Group operations
must preserve data:

- Managed duplicate links should not be migrated.
- Unmanaged linked groups should migrate the canonical source once, then relink
  each group member.
- Same-name separate copies must stay separate and require explicit selection.
- Divergent same-name content must enter the archive conflict flow when the repo
  already contains that skill.

## Parity Checklist

- Active/repo/doctor list scrolling with visible cursor.
- Persistent footer shortcuts.
- Wizard previews for every mutation before filesystem changes.
- Archive conflict side-by-side resolution.
- Linked-group migrate and unlink semantics.
- Detail inspector with full paths and symlink targets.
- Search/install flow for local and remote skills.
- Command/documentation alignment: either make `interactive` an alias for `tui`
  or update docs to use `tui` consistently.
