# TUI guide

Run `x-skills tui`. The Bubble Tea interface works from the current project and has Active, Repo, Doctor, and Install top-level pages, plus a Sync workbench opened from Active. Page switching uses `A`, `R`, `D`, and `I`; `ctrl+r` refreshes, `/` filters the current page, `?` opens help, `Esc` cancels the active modal or input, and `q` quits from the top level. Arrow keys or `j`/`k` move, `space` toggles a row, and `enter` applies the focused modal action. Footers show the keys valid in the current context.

## Pages and selection

- Active merges identical physical occurrences while keeping divergent same-name copies separate. Its inspector shows status, scope/root usages, path, compatibility, and description. Actions preview, migrate, unlink, recommend/unrecommend, and open Sync.
- Repo lists archived skills. Its inspector includes source identity and source ref/commit where known, compatibility, and active usages. Actions preview, link/unlink usages, remove, rename, and recommend/unrecommend. Remote check/update actions and update status are not implemented on this page.
- Doctor groups current diagnostics and shows issue details and suggested commands. Doctor rows are not selectable: `f` confirms counts/categories and applies safe repair to all current issues. Built-in repair additionally asks for global destination choices or Archive only.
- Install searches skills.sh, filters results, previews remote `SKILL.md`, displays source/audit/archive state, and archives or installs-and-links one or many results. Generic Git additions remain CLI-first.
- Sync collects non-destination project Skills Folders, groups candidates by name and fingerprint, resolves divergent variants, selects destinations, previews the plan, and applies it transactionally.

On selection-capable pages, actions use current-page selection: all checked rows on the visible page are the target; when none are checked, the cursor row is the fallback. Hidden selections are not borrowed from another page. Filtering narrows visible rows and actions operate on the page's resulting selection semantics. Doctor is the exception: it has no row selection, and its fix action covers all current issues after confirmation.

## Inspectors, modals, and layout

Wide terminals show the list and a contextual inspector side by side. Narrow layouts hide the inspector rather than squeezing it; modals render a terminal-too-small message when safe interaction is impossible. Lists, detail/preview panes, and long diffs scroll within their bounds.

Confirmation modals guard destructive steps. Choice modals resolve destinations and conflicts. Text modals collect rename values. Preview and detail modals render skill content and provenance. Diff modals compare active/archive or incoming/archive files before replacement. Sync and destination modals use checklists. Progress modals report cancellable batch work, and result modals retain per-item successes, skips, and failures.

Color is decorative. In no-color mode textual labels, badges, symbols, and explanations still distinguish managed, unmanaged, broken, safe/warn/risky, archive state, selection, and cursor state. `--ascii` substitutes ASCII glyphs without changing meaning.

## Background work and cancellation

Search, previews, Install archive-state comparisons, audits, archive/install batches, rename, doctor repair, restore, and sync filesystem work run as Bubble Tea commands so rendering stays responsive. A refresh reloads local active skills, archives, usages, and diagnostics; it does not contact remote sources. Install discovery and advisory audit data arrive independently. Generation tokens prevent late results from replacing newer searches, rows, or selections.

`Esc` cancels a modal or owned background operation where cancellation is offered. Starting a replacement operation cancels obsolete work; leaving Install or quitting cancels its outstanding contexts. A cancellation does not imply rollback of earlier independent batch items, while transactional sync/restore/rename changes use their own preservation and rollback rules.

See the [CLI guide](cli.md) and [remote skills guide](remote-skills.md).
