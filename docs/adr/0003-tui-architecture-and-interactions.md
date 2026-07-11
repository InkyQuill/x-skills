# TUI architecture and interactions

**Status:** Accepted

## Context

The TUI manages long lists, grouped filesystem state, remote discovery, conflicts, and destructive mutations. It must stay responsive, explain exact targets, remain usable without color or Unicode, and avoid state leaking between pages or behind modals.

## Decisions

- `x-skills tui` has four durable top-level pages: Active, Repo, Doctor, and Install. Active covers current-project/global occurrences and opens Sync; Repo owns archives and usages; Doctor owns diagnostics/repair; Install owns skills.sh discovery, preview, audit/archive state, and optional archive-plus-link. Manual generic-Git addition stays CLI-first.
- Wide layouts use list plus lightweight inspector. The main row carries scan-critical name/status/root chips; details hold paths, aliases, resolved targets, fingerprints, provenance, and errors. At less than roughly 100x30 the inspector collapses into details and the list remains usable; unsafe full-screen diff sizes show a resize/cancel state.
- Active groups identical content by fingerprint, uses `SKILL.md` name then archive/repo name then shortest active basename as canonical display, and retains aliases/usages in details. Broken paths remain individual rows. Repo shows archive source and usage information. Doctor remains issue-oriented rather than hiding repair in Active.
- Actions target checked rows on the current page, falling back to that page's cursor only when none are checked. They never borrow hidden selections from another page. Switching pages clears selection, filter, and cursor state; every action modal states target count and names.
- Repo exposes preview, link, usage-unlink, remove, and rename around archived content; removing an archive can unlink only managed usages visible in the current project and global roots and must say so. Active unlink preserves unmanaged content by default through migration and makes deletion an explicit destructive alternative. Doctor repair confirms issue counts and categories rather than silently mutating diagnostics.
- Bubble Tea owns the update loop, commands, and cancellation. Filesystem/network work runs outside `Update` and returns immutable snapshots or cache updates. Generation/identity tokens discard stale search, preview, audit, archive-state, and refresh results. Leaving an owning page, replacing work, `Esc`, or quitting cancels its context where offered.
- Background concurrency is bounded. Independent batch items may complete before cancellation and are summarized; transactional Sync/restore/rename flows retain their own rollback guarantees. Rendering and refresh never wait on network work.
- Bubbles viewports/text inputs provide scrolling and input, while typed custom rich rows and Lip Gloss own compact chips, badges, layout, and modal overlays. Row components receive explicit cursor/selection/background state. Glamour renders cached Markdown previews by content/width; raw mode remains available.
- Separate typed modal models own input exclusively behind a shared modal contract. Compact confirmations handle simple choices, workbench modals handle grouped/multi-step operations, and full-screen file-list plus full-context unified diff handles divergent content. Conflict decisions apply to the whole skill; binary/unreadable files show metadata rather than fake text diffs.
- Destructive actions always confirm and default to cancel; low-risk confirmations may default to apply. `y/n` applies only to yes/no modals. `q` closes an open modal, then quits from the shell; global interrupt remains available. All-success feedback uses persistent status, while mixed/error/conflict batches use a result modal.
- Filtering is local, case-insensitive substring matching over human-oriented fields, not absolute paths. Footer shortcuts are contextual but stable; status has a separate persistent line; help documents the complete per-state keymap and glyph legend.
- Color and Unicode improve scanning but never carry meaning alone. Project/global locations have distinct styling and concise configured chips; warning colors are reserved for risk. `NO_COLOR` disables color, and `--ascii` substitutes glyphs while retaining labels, cursor, selection, and status semantics. Keyboard operation is complete without mouse support.
- Mouse input, fuzzy filtering, runtime themes, per-file conflict choices, persistent cross-page selections, and a command palette are deferred. The first architecture keeps one semantic theme, predictable substring filtering, whole-skill conflict decisions, direct keys, and keyboard-complete operation; the backlog owns those extension seams.
- Model/state tests and focused rendered-string assertions are preferred to brittle full-terminal snapshots; responsive/no-color/ASCII and stale-message behavior require explicit tests.

## Consequences

The interface supports discovery and high-context conflict review without freezing or silently changing background state. Typed components and modal ownership add structure but prevent a single root model from becoming an untestable variant bag. Narrow and no-color terminals lose decoration and the persistent inspector, not capability or meaning. Page-local selection and explicit modal scope make batch actions predictable.

## Supersedes

- ADR 0007 — background Repo update checks
- ADR 0010 — current-page selection
- ADR 0015 — Install as a top-level page
- ADR 0016 — Bubble Tea snapshots and rich components
