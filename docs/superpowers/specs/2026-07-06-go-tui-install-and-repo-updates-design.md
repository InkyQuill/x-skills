# Go TUI Install And Repo Updates Design

Date: 2026-07-06 (synthesized from the remote install/search grilling session)

## Context

This design closes the gap left by [Go TUI Full Parity Design](./2026-07-06-go-tui-full-parity-design.md),
which originally treated remote `skills.sh` search/install and Repo update
checks as out of scope. A later grilling session (recorded in
[Go TUI Full Parity Grilling Q&A](./2026-07-06-go-tui-full-parity-grilling-q-and-a.md),
items R01–R81) decided to build both, as a top-level `I:Install` page plus a
background Repo update-check pipeline. This file is the synthesized,
implementation-facing spec for that decision. ADRs 0001–0003, 0005–0007,
0009, 0011–0018 record the individual load-bearing decisions; this file
composes them into one design surface with concrete mockups.

Deferred from this design (tracked in `docs/backlog.md`):

- Batch remote installs in the TUI Install view (CLI `add --all` is separate
  and already in scope, ADR 0011).
- Full JSON output for mutation commands (`add`, `repo update`,
  `repo update-all`); JSON is in scope now only for `search` and `repo check`
  (ADR 0014).
- URL/archive installs (ADR 0008).
- Cross-project usage indexing for Repo delete/rename (still current
  project + global roots only).

## Domain Language

- **Search**: read-only discovery via `skills.sh`. Never installs.
- **Add**: the top-level CLI verb (`x-skills add SOURCE [SKILL_NAME...]`) and
  the TUI Install-page action that archives a skill and, by default, links it
  into the current project's Agents root (ADR 0005, 0011).
- **Archive-only**: add without linking (`--no-link` in CLI, `a` in TUI).
- **Use now**: add and link to selected destinations (`i` in TUI).
- **Archive** vs **Incoming remote**: diff-modal labels for update/name
  conflicts, chosen to avoid confusion with "local" active-root paths.
- **Tracked**: an archived skill with `.x-skills.json` source metadata,
  eligible for background update checks.
- **Update states**: `up to date` (✓ green), `update available` (🗘 yellow),
  `missing upstream` (! red), `unknown` (? blue), or a neutral `tracked`
  pending pill before the first check completes.
- **Audit states**: `✓ safe`, `⚠ warn`, `‼ risky`, or no pill at all when
  audit data is unavailable — never a guessed/neutral risk pill.

## CLI Surface

```bash
x-skills search QUERY [--owner OWNER] [--limit N] [--audit] [--json|-j]
x-skills add SOURCE [SKILL_NAME...] [--no-link] [--to DEST...] [--git URL]
                     [--ref REF] [--all] [--replace] [--archive-as NAME]
                     [--rename-existing NAME] [-y|-n] [--no-input]
x-skills repo check [NAME...] [--json|-j]
x-skills repo update NAME... [--replace] [-y|-n] [--no-input]
x-skills repo update-all [-y|-n] [--no-input]
```

Notes:

- `search` never installs; it prints `add`-shaped hints, e.g.:
  - `Add and use: x-skills add vercel-labs/skills find-skills -y`
  - `Archive only: x-skills add vercel-labs/skills find-skills --no-link -y`
- `add` accepts GitHub shorthand (`owner/repo`), GitHub tree URLs
  (`https://github.com/owner/repo/tree/main/skills/foo`), the
  `owner/repo@skill` package shorthand, and generic `--git URL` (ADR 0018).
- `--to` destination grammar matches the TUI root-chip language: scope
  prefixes `global:`/`project:`/`g:`/`p:`/`~`/`.`, full target names, and
  short labels, case-insensitive; unscoped selectors default to project scope
  (ADR 0006).
- Non-interactive conflicts skip with an exact rerun hint
  (`... run x-skills repo update NAME --replace to accept incoming, or
  inspect in tui`); they never silently pick a side.
- `repo update-all` plans first, then prompts for confirmation on a TTY
  unless `-y`; it fails in non-interactive mode without `-y`.
- Command names deliberately do not mirror Python's `repo add-github`/
  `repo add-url`/`repo --check-updates` (ADR 0012): parity is capability
  parity, not command-name parity.

## TUI: Install Page

Install is a top-level page (`I`) with the same global page schema as
Active/Repo/Doctor (ADR 0015). It is search-driven only in this pass — no
manual source entry in the TUI; manual/generic-git sources are CLI-first via
`add --git` (deferred TUI addition, tracked implicitly by scope above).

### Wide layout

```text
◆ x-skills  A Active  R Repo  D Doctor  I Install      scope: skills.sh
┌ Install: search "svelte"  owner: all ──────────────┐ ┌ Inspector ──────────────┐
│ / svelte_                                          │ │ ◇ svelte-coder          │
│───────────────────────────────────────────────────│ │ vercel-labs/skills      │
│ › □ svelte-coder      vercel-labs/skills  ✓ safe   │ │ skill: svelte-coder     │
│   □ svelte-a11y       jane/skills         ⚠ warn   │ │ installs: 812           │
│   □ svelte-testing    jane/skills                  │ │ status: not archived    │
│                                                     │ │ audit: Socket 0 alerts │
│                                                     │ │ Gen: Safe               │
└─────────────────────────────────────────────────────┘ └─────────────────────────┘
found 3 results for "svelte"
enter preview  i install & use  a archive only  o owner filter  ^R refresh  ? help  q quit
```

### Narrow layout

```text
◆ x-skills  A Active  R Repo  D Doctor  I Install               scope: skills.sh
┌ Install: search "svelte" ───────────────────────────────────────────┐
│ / svelte_                                                          │
│ › □ svelte-coder      vercel-labs/skills          ✓ safe            │
│   □ svelte-a11y       jane/skills                 ⚠ warn            │
└──────────────────────────────────────────────────────────────────────┘
enter preview  i install & use  a archive only  ^R refresh  ? help  q quit
```

### Behavior

- Query input is a `/`-style bar, debounced after 2+ characters; Enter
  forces an immediate search. Below 2 characters, show
  "type at least 2 characters"; while in flight, show "searching…" and keep
  the previous result set visible.
- `o` edits an optional owner filter (compact field, not a settings screen).
- Uses the legacy unauthenticated `https://skills.sh/api/search` endpoint,
  requesting up to 50 results by default, rendered as one scrollable list —
  no fake pagination (ADR 0003).
- Advisory audit fetch runs in the background per result set, cached by
  `source + skill` for the process lifetime; rows show a pill only when data
  exists (ADR 0009).
- Row model: name, source (`owner/repo`), install count, audit pill
  (if any), archive-state badge (`not archived` / `archived` /
  `update available`), description last. Built from the shared rich-row
  element types (text, muted text, pill, spacer) so Active/Repo/Doctor/Install
  render consistently (R22/R23).
- `enter` opens a preview modal: clones the source into a session-only temp
  checkout (reused for that result's preview/install for the rest of the TUI
  session), discovers the skill by name/slug match — prompting if the
  discovered path is ambiguous — and renders `SKILL.md` with Glamour, `r`
  toggles raw.
- `i` ("install and use"): archives (if not already archived/current) then
  opens a destination checklist (`.Ag .Cl .Cd ~Ag ~Cl ~Cd`, `.Ag` checked by
  default) to link. If already archived and current, skips straight to the
  checklist. If already archived with an update available, first asks
  "update archive, then link" / "link current archive" / "cancel".
- `a` ("archive only"): archives without linking. No-ops with a status
  message if already archived and current.
- Name conflicts (same name, unproven same source) open the three-way
  conflict modal: replace archive / rename existing archive / rename
  incoming archive / cancel (ADR 0002). Renames use an editable prompt with a
  prefilled suggestion (`<name>-local` / `<name>-remote`, deduped with a
  numeric suffix if taken) — not silent auto-naming (R18). Renaming an
  existing archive relinks its visible current-project/global managed usages
  to the new name (R53). A renamed incoming archive uses its final archive
  name for any immediate link (R54).
- Same-source updates open the standard Archive vs Incoming remote diff
  modal (full-file unified diff, whole-skill decision) when content diverges;
  identical content relinks without a diff.
- After any action, the TUI stays on the Install page and updates the
  affected row/status in place (R63).
- Result reporting matches Active/Repo: status bar for clean single success,
  result modal for batch/partial/conflict outcomes.

## Repo Page: Update Badges

- Repo starts a bounded-concurrency (4 workers) background update-check
  pipeline for all tracked (source-metadata-bearing) archived skills on TUI
  startup, refreshing on `^R` and after install/update actions (ADR 0007).
- Update checks use `git ls-remote` first; only clone (shallow) to verify the
  recorded `skill_path` still contains `SKILL.md` when the remote commit has
  changed, so `missing upstream` is accurate without cloning every unchanged
  source.
- Rows show a neutral `tracked` pill while a check is pending, then upgrade
  to `✓ up to date`, `🗘 update available`, `! missing upstream`, or
  `? unknown`. A symbol+color legend lives in the help modal and inspector
  text (not symbols alone).
- `^U` updates: selected rows on the Repo page if any (current-page selection
  rule, ADR 0010), else the cursor row — but only rows in `update available`
  state; other rows are skipped with an explicit reason in the result
  summary (R37).
- Batch `^U` processes in order, no rollback; a divergent update opens the
  same Archive vs Incoming remote diff modal used by Install; clean updates
  proceed; missing/unknown are skipped with reasons.
- Background check/audit failures never surface as modals — only as
  row-level pills/inspector text and an aggregate status-bar line (e.g.
  "updates checked: 8 ok, 2 unknown").

## Keymap Additions

| Key | Install | Repo (new) |
| --- | --- | --- |
| `A`/`R`/`D`/`I` | switch pages | switch pages |
| `/` | edit search query | — |
| `o` | edit owner filter | — |
| `enter` | preview result | details (unchanged) |
| `i` | install & use (archive + link) | — |
| `a` | archive only | — |
| `^U` | — | update selected/cursor `update available` rows |
| `^R` | refresh search + background checks | refresh + rescan |
| `?` | help | help |
| `q` | quit | quit |

## Testing Requirements

- Search: debounce/min-length behavior, result rendering, owner filter,
  legacy-endpoint request shape.
- Preview: temp-checkout reuse within a session, ambiguous-path prompt.
- Install/`add`: archive-only vs archive+link default, `--to` destination
  parsing (all accepted forms), already-archived short-circuit, update-vs-link
  branch when an update is available.
- Conflict flows: name conflict three-way choice, rename validation/dedupe,
  relink-on-rename for both existing and incoming cases.
- Repo update pipeline: background check triggers (startup/`^R`/post-action),
  bounded concurrency, `ls-remote`-then-clone-on-change verification,
  eligibility filtering for `^U`, batch summary correctness.
- Audit: worst-signal summarization, no-pill-when-unknown, GitHub/skills.sh
  source scoping (no audit for generic `--git`).
- CLI: `repo check`/`repo update`/`repo update-all` name-scoping and
  confirmation/non-interactive behavior; `--json`/`-j` support limited to
  `search` and `repo check`.
