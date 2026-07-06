# Go TUI Full Parity Design

Date: 2026-07-06

## Goal

Bring the Go `x-skills tui` experience to design parity with the intended
maintenance workflow. The TUI should be a keyboard-first Charm application for
reviewing active skills, repo skills, doctor issues, and mutation decisions
without hiding filesystem risk.

This design supersedes the quick `2026-07-06-go-tui-parity-design.md` note as
the implementation target. The earlier note remains historical context.

See [Go TUI Views and Mockups Spec](./2026-07-06-go-tui-views-mockups.md) for concrete character-based terminal mockups and visual layouts for the top-level TUI views and critical modal states.

## Scope

In scope:

- List + inspector shell for Active, Repo, and Doctor views.
- Restrained Unicode visual language with color-coded root chips.
- Typed modal system for details, confirmations, previews, diffs, results, and help.
- Fullscreen archive conflict review with file list and full-file unified diff.
- Active/repo/doctor action flows described below.
- Local filtering for Active and Repo.
- Responsive terminal behavior.
- README cleanup to use `x-skills tui` consistently for the Go path.

Out of scope for this pass:

- Managed-agent registry/config beyond the current agents/claude/codex roots.
- Mouse support.
- Fuzzy filtering.
- Theme switching.
- Remote `skills.sh` search/install in the TUI.
- Repo update checks in the TUI.
- Command palette.
- Per-file/per-conflict merge choices in archive conflicts.

Deferred items are tracked in `docs/backlog.md`.

## Product Shape

The Go TUI is a maintenance manager, not a dense table clone.

The primary shell is List + Inspector:

- Header: product mark, view tabs, root coverage.
- Main area: custom-rendered rows on the left, read-only inspector on the right.
- Status bar: one line above footer, persists until the next state-changing
  action.
- Footer: context-sensitive keymap; never replaced by logs or status output.
- Modal layer: Lip Gloss overlays for all action and detail flows.

The default Active scope includes all current roots:

- project agents, claude, codex
- global agents, claude, codex

Wide headers show abbreviated root chips:

- `.Ag`, `.Cl`, `.Cd`
- `~Ag`, `~Cl`, `~Cd`

Narrow headers collapse to a compact label such as `scope: all roots`.

## Visual Language

Use restrained Unicode by default:

- cursor: `›`
- unselected row: `□`
- selected row: `■`
- group count: `×N`
- local/project chips: colored distinctly from global/home chips
- warning/error symbols only for actual risk

The TUI must support:

- `x-skills tui --ascii` for ASCII symbols.
- `NO_COLOR` to disable color styling while preserving layout and symbols.

The current built-in target labels are:

- agents: `Ag`
- claude: `Cl`
- codex: `Cd`

Future managed-agent configuration may define arbitrary short labels, including
emoji or custom glyphs.

## Global Navigation

Top-level views use one global uppercase tab schema:

- `A`: Active
- `R`: Repo
- `D`: Doctor
- `I`: Install, reserved for the planned Install view

Lowercase action keys remain view-specific. Refresh is global `ctrl+r` so `R`
can consistently mean Repo navigation.

## Reference Mockups

Wide shell:

```text
◆ x-skills  A Active  R Repo  D Doctor       scope: .Ag .Cl .Cd ~Ag ~Cl ~Cd
┌ Active skills ───────────────────────────────┐ ┌ Inspector ───────────────┐
│ › □ zen-of-go       ● .Ag ◆ ~Cl  ◆ unmanaged ×2 │ ◇ zen-of-go             │
│   □ opentui-react   ● .Cd       ✓ managed      │ aliases                  │
│   ■ svelte-coder    ◆ ~Ag       ▲ broken       │  zen-of-go               │
│   □ prompt-master   ● .Cl       ◆ local        │  go-style                │
│                                                │ repo    ▲ differs         │
│                                                │ target  ~/.x-skills/...   │
└────────────────────────────────────────────────┘ └─────────────────────────┘
relinked zen-of-go to existing archive
enter details  / filter  p preview  m migrate  u unlink  ^R refresh  ? help  q quit
```

Narrow shell:

```text
◆ x-skills  A Active  R Repo  D Doctor                         scope: all roots
┌ Active skills ─────────────────────────────────────────────────────┐
│ › □ zen-of-go       ● .Ag ◆ ~Cl  ◆ unmanaged ×2                   │
│   □ opentui-react   ● .Cd       ✓ managed                         │
│   ■ svelte-coder    ◆ ~Ag       ▲ broken                          │
└────────────────────────────────────────────────────────────────────┘
enter details  / filter  p preview  m migrate  u unlink  ^R refresh  ? help  q quit
```

Fullscreen archive conflict:

```text
╭──────────────────────── Archive conflict: zen-of-go ────────────────────────╮
│ Decision applies to the whole skill directory.                              │
│                                                                              │
│ Files                         │ SKILL.md                         full file   │
│───────────────────────────────┼──────────────────────────────────────────────│
│ › ± SKILL.md                  │  1  ---                                      │
│   + references/rules.md       │  2  name: zen-of-go                          │
│   - references/old.md         │ -3  description: Go style guide              │
│                               │ +3  description: Simple Go rules             │
│                               │  4  ---                                      │
│                               │  5                                           │
│                               │  6  # Zen of Go                              │
│                               │ -7  Prefer minimal APIs.                     │
│                               │ +7  Prefer tiny APIs.                        │
╰──────────────────────────────────────────────────────────────────────────────╯
↑↓ scroll   tab focus   k keep archive   l save active   esc cancel
```

Glamour preview:

```text
╭──────────────────────── Preview: zen-of-go ───────────────────────╮
│ ~/.x-skills/skills/zen-of-go/SKILL.md                              │
│ used in .Ag ~Cl       rendered with Glamour                        │
│───────────────────────────────────────────────────────────────────│
│                                                                   │
│  Zen of Go                                                        │
│  ─────────                                                        │
│                                                                   │
│  Ten engineering values for writing simple, readable,             │
│  maintainable Go code.                                            │
│                                                                   │
│  • Keep package boundaries small                                  │
│  • Prefer explicit errors                                         │
│  • Avoid clever abstractions                                      │
╰───────────────────────────────────────────────────────────────────╯
↑↓ scroll   r raw   esc close
```

## Active Rows And Grouping

Active rows use hybrid content grouping:

- Valid readable content groups by directory fingerprint.
- Broken entries do not group by content; each broken path is its own row.
- The displayed canonical name is selected by:
  1. `name:` from `SKILL.md`
  2. repo/archive directory name
  3. shortest active basename
- Other basenames appear as aliases in the inspector and detail modal.

Balanced active rows show:

- canonical name
- abbreviated root chips
- status
- `×N` count badge when multiple active entries are represented

The row should not include full paths, symlink targets, fingerprints, or long
diagnostics.

## Inspector And Details

The inspector is read-only in the main shell. It updates from cursor/selection
and shows concise operational context:

- aliases
- member locations
- repo state
- symlink target summary
- next likely action implication

`enter` opens an operational detail modal in every top-level view:

- Active details: active paths, resolved symlink targets, broken reasons,
  aliases, repo path/state, and fingerprint/debug information in a debug
  section.
- Repo details: archive path, description, usage chips, current project/global
  usages, and source metadata if available.
- Doctor details: issue kind, affected path, reason, and proposed safe fix.

Fingerprints are not shown in normal rows or inspector. They may appear only in
the detail modal debug section.

## Responsive Layout

The full two-column shell targets terminals at least `100x30`.

Below that size:

- collapse the inspector out of the main shell;
- keep list, status bar, and footer visible;
- keep details available through `enter`.

The fullscreen diff modal has its own minimum-size guard. If the terminal is too
small to review a diff, show a resize prompt and allow cancel. Do not fall back
to summary-only conflict approval.

## Modal System

Use separate typed modal models behind a shared modal interface. Do not build a
single giant variant struct.

Modal types:

- compact confirmation modal
- workbench choice modal
- fullscreen conflict diff modal
- Glamour preview modal
- operational detail modal
- result modal
- help modal

Rules:

- Modal captures all keys except global interrupt.
- `esc` cancels/closes the modal.
- `q` closes the modal; in the main shell, `q` quits the app.
- `ctrl+c` quits globally.
- Enter applies the highlighted modal choice.
- `y/n` shortcuts exist only for simple yes/no confirmation modals.
- Safe actions default to apply.
- Destructive actions default to cancel.

Use modal sizes by task:

- Compact modal: simple confirmations.
- Workbench modal: grouped choices and richer operation plans.
- Fullscreen modal: actual diffs.
- Result modal: batch results, partial failures, skips, or conflict-resolved
  summaries.
- Status bar only: single clean success.

## Archive Conflict Diff

Divergent archive conflicts always open the fullscreen diff modal.

Layout:

- Left pane: changed file list with added/removed/changed markers.
- Right pane: selected file as a full-file unified diff.
- Unchanged lines remain visible.
- Removed lines are red and prefixed `-`.
- Added lines are green and prefixed `+`.
- Binary or unreadable files show metadata only: type/status, size if
  available, and hash if readable.

Markdown files are shown as raw diff text. Glamour is not used for conflict
review.

Decision scope:

- First parity implementation chooses at whole-skill level:
  - keep archive, discard active;
  - save active over archive;
  - cancel.
- The modal must state that the decision applies to the whole skill directory.
- Per-file/per-conflict management is future work.

Batch behavior:

- Batch actions process in order.
- A conflict pauses the batch.
- Resolving the conflict continues the batch.
- Final batch/partial results open a result modal.

## Active View Actions

Active view keys:

- `enter`: details
- `/`: filter
- `p`: preview readable `SKILL.md`
- `m`: migrate selected rows, or cursor row if none selected
- `u`: unlink selected rows, or cursor row if none selected
- `ctrl+r`: refresh
- `?`: help
- `q`: quit

Active preview:

- Uses Glamour-rendered Markdown.
- For content groups, preview canonical resolved content automatically.
- Metadata shows member count and aliases.
- `r` toggles raw/rendered in the preview modal.

Active migrate:

- Copies the selected active skill directory (or cursor row if none selected) to the repo archive and links it back.
- If the archive destination already exists, compares the whole-folder fingerprint (SHA).
- **Fingerprint Match (Same SHA)**: Removes the duplicate active directory and relinks it to the existing archive without error. Displays `relinked <name> to existing archive` in the status bar.
- **Divergent Content (Different SHA)**: Pauses the migration (and any surrounding batch actions) and opens the fullscreen archive conflict diff modal (see "Archive Conflict Diff" below) showing the changes.

Active unlink:

- Mixed selections use one grouped workbench modal.
- Managed links remove symlinks.
- Broken links remove broken symlinks.
- Unmanaged directories offer a global choice:
  - migrate then unlink, default;
  - delete active copy, destructive and not default.

## Repo View Actions

Repo rows show:

- archived skill name
- description
- active usage chips for current project roots and global roots

Repo view keys:

- `enter`: details
- `/`: filter
- `p`: preview archived `SKILL.md`
- `l`: link selected repo skills into an active root
- `u`: unlink active usages through a usage chooser modal
- `d`: delete archive
- `ctrl+r`: refresh
- `?`: help
- `q`: quit

Repo preview:

- Uses Glamour-rendered Markdown by default.
- Shows archive path, usage chips, and render mode.
- `r` toggles raw/rendered.
- References are not previewed in this pass.

Repo unlink:

- Opens a usage chooser modal.
- Current project/global usages are selected by default.
- User confirms before unlinking.

Repo delete:

- If active usages exist, offer a combined flow: unlink current project/global
  usages, then delete archive.
- Direct “delete anyway and leave broken links” is not offered.
- The modal must state the visibility limit: only current project roots and
  global roots are known. Other projects may need `doctor` later.

## Doctor View Actions

Doctor is a separate top-level view. Active may still show broken rows, but
Doctor owns issue-oriented remediation.

Doctor keys:

- `enter`: issue details
- `f`: fix all current Doctor issues
- `ctrl+r`: refresh
- `?`: help
- `q`: quit

`f` opens a compact confirmation modal listing issue counts/categories before
mutating. Doctor fix operates on all current Doctor issues for parity.

## Filtering

`/` opens an inline command bar above the footer.

Filtering rules:

- case-insensitive substring matching for parity;
- Active/Repo match names, aliases, descriptions, statuses, and root chips;
- full absolute paths are excluded from normal filtering;
- filters clear on view switch;
- selections clear on view switch.

Action keys operate on selected rows if any exist; otherwise they operate on the
cursor row. Action modals must state the target count and target names.

## Help

`?` opens a compact help modal.

Help includes:

- current view keymap;
- modal key behavior;
- symbol legend;
- root chip legend;
- Unicode/ASCII fallback explanation.

## Keymap

Main shell:

| Key | Active | Repo | Doctor |
| --- | --- | --- | --- |
| `A` | switch Active | switch Active | switch Active |
| `R` | switch Repo | switch Repo | switch Repo |
| `D` | switch Doctor | switch Doctor | switch Doctor |
| `I` | reserved for Install | reserved for Install | reserved for Install |
| `enter` | details | details | details |
| `/` | filter | filter | no filter in parity |
| `space` | toggle selection | toggle selection | no selection in parity |
| `p` | preview | preview | none |
| `m` | migrate | none | none |
| `u` | unlink | unlink usages | none |
| `l` | none | link | none |
| `d` | none | delete archive | none |
| `f` | none | none | fix all |
| `ctrl+r` | refresh | refresh | refresh |
| `?` | help | help | help |
| `q` | quit | quit | quit |

Filter mode:

| Key | Behavior |
| --- | --- |
| text | update filter |
| `esc` | clear/exit filter |
| `enter` | accept filter and return to browse |

Modals:

| Modal | Keys |
| --- | --- |
| compact confirmation | `↑↓`, `enter`, `y/n` for yes/no, `esc`, `q` |
| workbench choice | `↑↓`, `space` where selection applies, `enter`, `esc`, `q` |
| conflict diff | `↑↓` scroll, `tab` focus file list/viewer, `k` keep archive, `l` save active, `esc`, `q` |
| preview | `↑↓` scroll, `r` raw/rendered, `esc`, `q` |
| details | `↑↓` scroll, `esc`, `q` |
| result | `↑↓` scroll if needed, `enter`/`esc`/`q` close |
| help | `↑↓` scroll if needed, `esc`/`q` close |

## Spec Coverage Matrix

This matrix is the implementation contract. A row is not ready to implement if
the required data, modal behavior, or verification path is missing.

| Area | User task | Required presentation | Required data | Modal/state | Verification |
| --- | --- | --- | --- | --- | --- |
| Active browse | See active skills across current project and global roots | List + inspector, root chips, status, `×N` group count | active scan, repo state, aliases, fingerprints, root labels | browse state | grouping tests, rendered row assertions |
| Active details | Inspect exact paths and symlink targets | Detail modal with full paths, targets, aliases, debug fingerprint | selected active group members, resolved targets, broken reasons | detail modal | modal content tests |
| Active preview | Read selected skill instructions | Glamour preview with path/member metadata and raw toggle | canonical readable `SKILL.md`, aliases/member count | preview modal | rendered/raw toggle tests |
| Active migrate | Archive unmanaged active content and link back | Workbench confirmation, status/result, conflict diff if needed | active path, archive path, fingerprints, batch queue | confirmation, conflict diff, result | same-SHA relink, divergent conflict, batch resume tests |
| Active unlink | Remove links or remove/migrate unmanaged active dirs | Grouped workbench modal by managed/broken/unmanaged categories | active members by status, root labels, archive paths | workbench choice/result | mixed unlink plan tests |
| Repo browse | See archived skills and current usage | List + inspector, usage chips, description | repo list, active usages in current project/global roots | browse state | usage chip/rendered row tests |
| Repo details | Inspect archive metadata and usages | Detail modal with archive path and current usages | repo skill, source metadata if present, usage paths | detail modal | modal content tests |
| Repo preview | Read archived `SKILL.md` | Glamour preview, archive path, usage chips, raw toggle | archived `SKILL.md`, usage chips | preview modal | rendered/raw toggle tests |
| Repo link | Link archive skill into an active root | Workbench modal with destination scope/target and exact destination path | selected repo skills, destination root, existing destination state | workbench choice/result | destination selection and existing-path tests |
| Repo unlink | Remove current usages of archived skill | Usage chooser modal, all current usages selected by default | active usages in current project/global roots | workbench choice/result | usage chooser tests |
| Repo delete | Delete archive safely | Destructive modal; active usages must be unlinked first; scope limitation visible | archive path, current project/global usages | compact/workbench confirmation/result | active-usage block/delete tests |
| Doctor browse | Review current issues | Doctor list + inspector with issue reason and safe fix | doctor issues, affected paths, fix categories | browse state | issue row/rendered assertions |
| Doctor fix | Apply safe fixes | Compact confirmation with issue counts/categories | current doctor issues and safe-fix actions | compact confirmation/result | fix-all confirmation tests |
| Filtering | Narrow Active/Repo rows | Inline command bar above footer | filter text, searchable row fields | filter state | matching/reset tests |
| Help | Discover keys and symbols | Compact help modal with keymap and chip/symbol legend | current view keymap, symbol mode | help modal | content assertions |
| Responsive shell | Use app in narrow terminals | Inspector collapsed below `100x30`; details via Enter | width/height, active view state | layout state | responsive render tests |

## Charm Stack Responsibilities

- Bubble Tea: application model, update loop, commands, key routing, async
  refresh/actions.
- Bubbles viewport: scrollable panes, preview text, detail text, diff viewer.
- Bubbles textinput: filter command bar.
- Lip Gloss: shell layout, modal overlays, borders, chips, colors, status/footer
  styling.
- Glamour: rendered Markdown preview.
- Custom renderer: active/repo/doctor rows, abbreviated chips, statuses, and
  grouping.

Initial load may remain synchronous if fast. Refresh and mutations should use
Bubble Tea commands with pending states.

## Implementation Approach

Use a modal-first refactor:

1. Introduce typed modal infrastructure and modal key routing.
2. Move existing action previews/conflicts/results into modals.
3. Add preview/detail/help/result modal models.
4. Add filter command bar.
5. Refactor the shell into list + inspector with responsive collapse.
6. Update README and tests.

This approach addresses mutation correctness and decision clarity before visual
shell polish.

## Testing Strategy

Use model/state tests plus focused rendered-string assertions.

Cover:

- key routing by state;
- modal open/close/apply/cancel behavior;
- compact/workbench/fullscreen modal transitions;
- archive conflict pause/resume in batch actions;
- full-file unified diff model behavior;
- preview raw/rendered toggle;
- filtering behavior and filter reset on view switch;
- selection reset on view switch;
- active grouping, aliases, and `×N` badges;
- responsive inspector collapse;
- `NO_COLOR` and `--ascii` behavior;
- README command wording for `x-skills tui`.

Avoid full terminal snapshots because Lip Gloss width/color output is brittle.
Manual smoke checks should verify visual polish in realistic terminal sizes.
