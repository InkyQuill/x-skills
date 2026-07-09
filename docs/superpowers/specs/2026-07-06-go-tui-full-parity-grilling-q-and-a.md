# Go TUI Full Parity Grilling Q&A

Source session: `/home/inky/.codex/sessions/2026/07/06/rollout-2026-07-06T09-35-02-019f3623-2c46-7720-9dc7-f0d3a3fa4a56.jsonl`

This file preserves the numbered grilling questions, user answers, and short assistant-recorded outcomes from the July 6, 2026 Go TUI parity design session. It is intended as guidance for future spec and implementation work.

Extracted questions: 80

## Q01

Session message ids: assistant `59`, user `60`

**Question**

Open the companion here: http://localhost:65423

It’s showing three actual character-based shell mockups for the primary TUI structure. Question 1: which shell should be the base?

My recommendation is **B: List + Inspector** because it gives us a permanent place for full paths, linked-group details, repo conflict summaries, and action previews without hiding the main list.

Pick A, B, or C, or tell me what to change.

**Answer**

B. Make sure it has colors - so you can color . (local) and ~ (global) locations differently.
Maybe we can add actual unicode symbols to make sure it looks better.

**Recorded Outcome**

B is the base shell. I’ll carry forward two visual requirements: local `./...` and global `~/...` locations need distinct colors, and the UI can use Unicode symbols where they improve scanning without becoming noisy.

## Q02

Session message ids: assistant `63`, user `64`

**Question**

Open the companion: http://localhost:65423

Question 2: choose the color + Unicode density.

My recommendation is **B: Restrained Symbols**:
- local `./...` chips get one color
- global `~/...` chips get another
- a small stable symbol set improves scanning
- no heavy ornamentation or font-risky UI

Pick A, B, or C, or tell me what to adjust.

**Answer**

B. For now it's the best-looking one

**Recorded Outcome**

Restrained symbols it is. I’ll treat that as the visual language: stable glyphs, two location-chip colors, warning/error color reserved for actual risk, and no decorative symbol sprawl.

## Q03

Session message ids: assistant `67`, user `68`

**Question**

Open the companion: http://localhost:65423

Question 3: where should the action/wizard state live?

My recommendation is **A: Action Under List**. It keeps the inspector stable while the action panel changes, and it gives conflict flows more horizontal space than the right column.

Pick A, B, or C, or tell me what to adjust.

**Answer**

D. Use the modal instead. WE are using bubbletea / bubbles / lipgloss. Those CAN ACTUALLY RENDER MODALS.

**Recorded Outcome**

Understood: modal actions are the direction, and the implementation should use Bubble Tea/Bubbles/Lipgloss capabilities rather than treating the terminal as a flat text canvas. I’ll update the design constraint: normal browsing uses list + inspector; mutations and conflicts open a real centered overlay/modal with the background still visible but not active.

## Q04

Session message ids: assistant `71`, user `72`

**Question**

Open the companion: http://localhost:65423

Question 4: what modal pattern should mutations use?

My recommendation is **B: Workbench Modal**:
- real centered modal
- enough room for side-by-side conflict summaries
- options are navigable with `↑↓` and `enter`
- background list + inspector remains visible as context

Pick A, B, or C, or describe the modal variant you want.

**Answer**

If the modal is small, it is A. If it has more data - it's B. For diffs - you should show actual line diffs, allowing user to check the diffs side by side. It should be done in C - fullscreen modal.

**Recorded Outcome**

Modal sizing is now rule-based: compact confirmations use A, richer operation choices use B, and actual diffs use fullscreen C with real side-by-side line diffs. That gives us a modal hierarchy instead of one modal trying to fit every job.

## Q05

Session message ids: assistant `75`, user `76`

**Question**

Question 5: when active and archive content diverge, should we always open the fullscreen line-diff modal, or only when the user asks to inspect?

My recommendation: **always open the fullscreen diff modal for divergent archive conflicts**. The whole point of the conflict is choosing which content survives; a summary-only modal is too easy to approve blindly.

Options:
- **A. Always fullscreen diff for divergent conflicts**: safest, more deliberate.
- **B. Workbench summary first, `d` opens fullscreen diff**: faster, but adds a two-step path for the riskiest decision.
- **C. Summary for small diffs, fullscreen for large diffs**: adaptive, but the threshold can feel arbitrary.

Which one?

**Answer**

A.

**Recorded Outcome**

Divergent archive conflicts will always enter the fullscreen diff modal. I’m showing the next concrete choice visually: how that line diff should be laid out and navigated in a terminal.

## Q06

Session message ids: assistant `78`, user `79`

**Question**

Open the companion: http://localhost:65423

Question 6: how should the fullscreen line-diff modal be structured?

My recommendation is **B: File List + Focused Diff**:
- left pane shows added/removed/changed files
- right pane shows side-by-side line diff for the selected file
- decision still applies to the whole skill directory, not just one file

Pick A, B, or C, or describe the diff layout you want.

**Answer**

B.Maybe we can make it with file list, but the diff may be not side-by-side, but unified for file (git-style, but not only diffs), but showing all text of a file with red - lines and green + lines.

**Recorded Outcome**

Good adjustment: keep the file list, but make the file viewer a full-context unified diff, not side-by-side. So users see the whole selected file with unchanged context plus red removed and green added lines, closer to `git diff --unified` expanded to full file content.

## Q07

Session message ids: assistant `81`, user `82`

**Question**

Open the companion: http://localhost:65423

Question 7: how much unchanged file context should the unified diff show?

My recommendation is **B: Full-File Unified Diff**:
- file list on the left
- selected file on the right
- all text shown
- removed lines red with `-`
- added lines green with `+`
- unchanged lines remain visible

Pick A, B, or C.

**Answer**

B.

**Recorded Outcome**

Full-file unified diff is locked in for conflict review. I’ll keep the decision simple: no compact/full toggle for the first parity design; every changed file can be inspected with full context.

## Q08

Session message ids: assistant `84`, user `85`

**Question**

Question 8: should the conflict decision apply only at the whole-skill level, or should users be able to choose per file?

My recommendation: **whole-skill only for now**. Per-file merge is much more complex: it needs write-back composition, conflict states, partially accepted files, and stronger testing. For a skill manager, “keep archive” vs “save active” is clear and safe.

Options:
- **A. Whole skill only**: choose archive or active for the entire skill directory.
- **B. Per-file choose archive/active**: more flexible, much more implementation and UX complexity.
- **C. Whole skill now, leave explicit design space for per-file later**: same implementation as A, but the spec names the future extension.

I recommend **C** so we don’t overbuild now but don’t paint ourselves into a corner. Which one?

**Answer**

C. Yes, we'll add per-file/per-conflict management later.

**Recorded Outcome**

Decision recorded: first implementation chooses the whole skill directory, but the design keeps space for future per-file or per-conflict selection. The diff modal should make that scope explicit so users don’t think selecting a file changes only that file.

## Q09

Session message ids: assistant `87`, user `88`

**Question**

Question 9: in the main Active list, should rows represent **content groups** or **skill names**?

Current Go groups by content fingerprint, so identical linked copies collapse even if the symlink names differ. But for daily use, users may expect “zen-of-go” to be the stable row identity.

My recommendation: **content groups, but display the primary skill name and surface aliases in the inspector**. That preserves the linked-copy safety model while keeping the list readable.

Options:
- **A. Content groups**: one row per identical folder SHA; safest for linked setups, but can surprise users when names differ.
- **B. Skill-name groups**: one row per name; intuitive, but identical renamed links won’t collapse naturally.
- **C. Hybrid**: group by content fingerprint, display canonical name, show “also linked as …” in inspector.

I recommend **C**. Which one?

**Answer**

C.

**Recorded Outcome**

Hybrid active grouping is set: content fingerprint is the grouping key, the list shows a canonical name, and the inspector explains aliases/locations. I’m going to show the next visual question because canonical names, aliases, and locations need to be scannable.

## Q10

Session message ids: assistant `90`, user `91`

**Question**

Open the companion: http://localhost:65423

Question 10: how much detail belongs in Active rows versus the Inspector?

My recommendation is **B: Balanced Rows**:
- row: canonical name, local/global location chips, status
- inspector: aliases, full paths, repo state, symlink targets, action implications

Pick A, B, or C.

**Answer**

B.

**Recorded Outcome**

Balanced rows are set. Main rows carry only the scanning fields; inspector carries everything that would make rows too wide or ambiguous.

## Q11

Session message ids: assistant `93`, user `94`

**Question**

Question 11: what should the canonical display name be for a content group with multiple names?

Example: the same folder is linked as `zen-of-go` in `.agents` and `go-style` in `.claude`.

My recommendation: **prefer the `name:` from `SKILL.md`; if absent, prefer the repo/archive directory name; otherwise use the shortest active basename and show the rest as aliases**. That makes the skill’s own metadata authoritative while still keeping archive identity stable.

Options:
- **A. `SKILL.md name` first**, then repo name, then shortest active basename.
- **B. Repo/archive directory name first**, then `SKILL.md name`, then active basename.
- **C. Active basename from the selected/current location first**, aliases in inspector.

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Canonical naming is set: `SKILL.md name` wins, repo/archive name is fallback, shortest active basename is last fallback, and all other basenames appear as aliases in the inspector.

## Q12

Session message ids: assistant `96`, user `97`

**Question**

Question 12: should the Active view default scope include **all six roots** every time, or should it default to the current project first with global roots available via filter?

My recommendation: **all six roots by default**, because this tool’s main value is showing cross-agent/cross-scope skill state. But the header should clearly show “scope: all” and filters should be one keystroke away.

Options:
- **A. All roots by default**: project + global, agents + claude + codex.
- **B. Project roots by default**: global roots are opt-in.
- **C. Last-used filter persists**: convenient but can hide state unexpectedly.

I recommend **A**. Which one?

**Answer**

A. We'll later add config section where we'll allow users to pick which agents are managed. for example, pi, opencode, hermes, charm, mimocode, openclaw (it has different structure from all of them) - but it's not for now. Should be written into backlog.

**Recorded Outcome**

All roots by default is set for parity. The future “managed agents registry” is out of scope for this design, but it should be captured as backlog so it doesn’t leak into the current TUI spec. I’ll read the backlog workflow and record it appropriately.

## Q13

Session message ids: assistant `101`, user `102`

**Question**

Question 13: should the header show the root coverage as a compact summary or as explicit chips?

My recommendation: **explicit chips when space allows, compact summary when narrow**.

Example:
- Wide: `scope: ● .agents ● .claude ● .codex ◆ ~/.agents ◆ ~/.claude ◆ ~/.codex`
- Narrow: `scope: all roots`

This reinforces that Active is showing all six roots by default and makes the local/global color language visible.

Options:
- **A. Compact only**: `scope: all roots`
- **B. Explicit chips only**: always show all root chips
- **C. Responsive**: chips on wide terminals, compact on narrow terminals

I recommend **C**. Which one?

**Answer**

C. What about making chips abbreviations?
like `.Ag` `.Cl` `~Cd`? Also, `He`, `Oc`, `Cw`, `Pi`, `Ch`, `Mi` and so on. A lot look like chemical elements)
Inspector can then show them fully.

**Recorded Outcome**

Responsive header is set, with abbreviated root chips as the preferred wide form. The “chemical element” feel is a good fit for dense terminal UI: short, stable, color-coded tokens in rows/header; full labels and paths in inspector.

## Q14

Session message ids: assistant `104`, user `105`

**Question**

Question 14: what exact abbreviation scheme should root chips use?

My recommendation: **prefix scope by first character and target by two-letter title case**:
- project agents: `.Ag`
- project claude: `.Cl`
- project codex: `.Cd`
- global agents: `~Ag`
- global claude: `~Cl`
- global codex: `~Cd`
- future hermes: `.He` / `~He`
- future opencode: `.Oc` / `~Oc`
- future openclaw: `.Ow` or `.Ocw` is the tricky one

The ambiguity is `opencode` vs `openclaw`. We need a rule that avoids collisions.

Options:
- **A. Fixed two-letter aliases with collision overrides**: `Op` for opencode, `Ow` for openclaw, `Ag`, `Cl`, `Cd`, `He`, `Pi`, `Ch`, `Mi`.
- **B. First two letters always, add third on collision**: `Op`, `Ope` or `Opn` when needed.
- **C. User-configured short labels later, built-ins now**: built-ins use hand-picked aliases; future registry requires each agent to define a short label.

I recommend **C**: built-ins get polished labels now, future managed-agent config owns the collision problem. Which one?

**Answer**

C. If user later wants to show an emoji of a cancer for Openclaw, let them.

**Recorded Outcome**

Chip labels will be built-in now and user-configurable later. The spec will note that future agent registry entries can define arbitrary short labels, including emoji, while the current implementation stays conservative.

## Q15

Session message ids: assistant `107`, user `108`

**Question**

Question 15: what should the built-in chip labels be for the current three targets?

My recommendation:
- agents: `Ag`
- claude: `Cl`
- codex: `Cd`

With scope prefix/color:
- local/project: `.Ag`, `.Cl`, `.Cd`
- global/home: `~Ag`, `~Cl`, `~Cd`

These are short, unambiguous, and match your chemical-token direction.

Approve these built-in labels?

**Answer**

yes

## Q16

Session message ids: assistant `109`, user `110`

**Question**

Question 16: should selected rows use checkbox symbols (`□` / `■`) or a more conventional terminal marker (`[ ]` / `[x]`)?

My recommendation: **Unicode checkbox symbols**:
- unselected: `□`
- selected: `■`
- cursor: `›`
- current selected row: `› ■`

They fit the restrained-symbol style and save horizontal space.

Options:
- **A. Unicode**: `□` / `■`
- **B. ASCII**: `[ ]` / `[x]`
- **C. Auto fallback**: Unicode by default, ASCII under `NO_COLOR` or dumb terminals later

I recommend **C** for implementation robustness, but visually target **A**. Which one?

**Answer**

C.

**Recorded Outcome**

Selection markers will target Unicode but have an ASCII fallback path. I’ll carry that same rule for arrows/status symbols where practical: polished by default, safe in constrained terminals.

## Q17

Session message ids: assistant `112`, user `113`

**Question**

Question 17: should the TUI include a visible help modal (`?`) in the parity design?

My recommendation: **yes, but keep it modal and compact**. The footer should show the top shortcuts only; `?` opens the full keymap and symbol legend, including chip meanings.

Options:
- **A. Yes, compact help modal**: keymap + symbol/chip legend.
- **B. No, footer only**: simpler, but discoverability suffers as flows grow.
- **C. Yes, but fullscreen help**: more space, probably excessive.

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Help will be a compact modal with keymap and symbol/chip legend. It should also document the Unicode/ASCII fallback meanings so users aren’t forced to infer symbols.

## Q18

Session message ids: assistant `115`, user `116`

**Question**

Question 18: should focus move between list and inspector, or should the inspector be read-only with actions driven from the list/modals?

My recommendation: **inspector read-only for the first parity pass**. It reduces focus complexity. The list controls selection and actions; modals handle choices. Later, inspector can gain copy/open actions if needed.

Options:
- **A. Read-only inspector**: no focus state; updates with cursor/selection.
- **B. Focusable inspector**: `tab` moves focus, inspector can scroll/copy details.
- **C. Hybrid**: read-only by default, `enter` or `i` opens a detail modal for scrollable full paths/details.

I recommend **C** if full paths can overflow often; otherwise **A**. My lean is **C** because terminal widths vary. Which one?

**Answer**

C

**Recorded Outcome**

Inspector stays lightweight in the main shell, with a detail modal for overflow/full paths. That avoids permanent focus complexity while still giving access to long paths, aliases, symlink targets, and fingerprints.

## Q19

Session message ids: assistant `118`, user `119`

**Question**

Question 19: what key should open the detail modal?

Current footer used `⇥ details`, but Tab in terminal apps can be awkward and may conflict with focus later.

My recommendation: **Enter opens details in browse mode; action keys open action modals**. It is natural: cursor row + Enter = inspect.

Options:
- **A. Enter opens details**
- **B. Tab opens details**
- **C. `i` opens details**, but `i` currently means install in repo view
- **D. `v` opens details/view**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Enter will open the detail modal while browsing. In action modals, Enter applies the currently selected choice; the meaning is modal-local and should be visible in the footer/modal.

## Q20

Session message ids: assistant `121`, user `122`

**Question**

Question 20: should multi-select persist when switching views?

My recommendation: **no, selections are scoped per view and cleared on view switch**. Cross-view selection is hard to reason about: selecting Active rows and then moving to Repo could accidentally make install/migrate semantics confusing.

Options:
- **A. Clear selection on view switch**
- **B. Preserve per-view selection independently**
- **C. Preserve globally until explicit clear**

I recommend **A** for safety and predictability. Which one?

**Answer**

A for now. Might change it later if this feels awkward

**Recorded Outcome**

Selections will clear on view switch for the first parity design, with a note that preserving per-view selections can be revisited if it feels awkward in real use.

## Q21

Session message ids: assistant `124`, user `125`

**Question**

Question 21: should action keys operate on selected rows if any exist, otherwise the cursor row?

My recommendation: **yes**. This is the current behavior and it is efficient: select several rows for batch work, or just move the cursor and press `m`/`u`.

Options:
- **A. Selected rows if any, else cursor row**
- **B. Always cursor row unless batch mode is explicitly entered**
- **C. Require selection for actions**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Action target semantics are set: selected rows if non-empty, otherwise cursor row. Modals must state the target count and list names so batch actions are not silent.

## Q22

Session message ids: assistant `127`, user `128`

**Question**

Question 22: for batch actions, should one conflict pause the batch and resume after resolution, or should the modal collect all conflicts first?

My recommendation: **pause on the first conflict, resolve it, then continue the batch**. It matches ordered batch semantics and keeps each decision concrete.

Options:
- **A. Pause per conflict, then continue**
- **B. Pre-scan all conflicts and show a conflict queue**
- **C. Stop batch on first conflict; user reruns after resolving**

I recommend **A**. Which one?

**Answer**

A,

**Recorded Outcome**

Batch actions will process in order and pause per conflict. After resolving one conflict, the modal continues the remaining batch and the final result log summarizes all applied/skipped/failed items.

## Q23

Session message ids: assistant `130`, user `131`

**Question**

Question 23: should the result log be a modal, a status line, or a persistent side/bottom panel?

My recommendation: **compact status for single success, result modal for batch or partial failure**. A status line is enough for “relinked zen-of-go”; batch results need room.

Options:
- **A. Status line only**
- **B. Result modal only**
- **C. Hybrid**: status line for simple success; modal for batch, skips, failures, conflicts resolved

I recommend **C**. Which one?

**Answer**

C.

**Recorded Outcome**

Result reporting will be hybrid: single clean success goes to status, batch/partial/failure outcomes open a result modal with details. Footer shortcuts remain visible after closing it.

## Q24

Session message ids: assistant `133`, user `134`

**Question**

Question 24: should Doctor be a separate top-level view, or should broken/diagnostic issues be folded into Active as filters?

My recommendation: **keep Doctor as a separate top-level view**, and also surface broken status in Active. Doctor has different rows/actions: issues, safe fixes, and remediation reasons. Folding it into Active risks muddying the active skill model.

Options:
- **A. Separate Doctor view**
- **B. Active filters only**
- **C. Both, but Doctor is just a filtered Active facade**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Doctor remains a top-level view. Active still shows broken rows, but remediation workflow and issue-oriented details live in Doctor.

## Q25

Session message ids: assistant `136`, user `137`

**Question**

Question 25: should `f` in Doctor fix selected issues, or fix all issues when nothing is selected?

Current spec says Doctor/Fix does not require selection; selected rows narrow scope, no selection scans visible working set.

My recommendation: **selected issues if any, otherwise all current Doctor issues**. That makes “open Doctor, press f” useful.

Options:
- **A. Selected if any, else all Doctor issues**
- **B. Cursor only unless selected**
- **C. Require explicit selection**

I recommend **A**. Which one?

**Answer**

Doctor is a completely separate view, so all current doctor issues.

**Recorded Outcome**

Doctor `f` will apply to all current Doctor issues, not cursor fallback. If we later add issue selection there, it should be explicit and visually clear; for parity, Doctor is a “review all, fix all safe issues” view.

## Q26

Session message ids: assistant `139`, user `140`

**Question**

Question 26: should Doctor fixes still show a confirmation modal before applying safe fixes?

My recommendation: **yes, compact modal**. Even safe fixes mutate symlinks/removals, so the user should see the count and categories.

Example:
`Fix 4 doctor issues: 3 relink broken symlinks, 1 remove dead symlink`

Options:
- **A. Yes, compact confirmation modal**
- **B. No, pressing `f` applies safe fixes immediately**
- **C. Workbench modal with issue list every time**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Doctor fix will open a compact confirmation modal with issue counts/categories before mutation. That preserves the “no silent mutation” rule without overloading the flow.

## Q27

Session message ids: assistant `142`, user `143`

**Question**

Question 27: should Repo view support search/filter in the first parity design?

My recommendation: **yes, local filtering only**. Repo lists can get large, and filtering is core navigation, not extra package-management scope. Remote `skills.sh` search can remain separate.

Options:
- **A. Yes, local filter in Repo and Active**
- **B. Yes, local filter in Repo only**
- **C. No search/filter in first parity pass**

I recommend **A** if we can keep it simple: `/` opens a filter input modal/inline bar; `esc` clears. Which one?

**Answer**

A.

**Recorded Outcome**

Local filtering is in scope for Active and Repo. I’ll keep remote `skills.sh` search out of the parity shell unless we explicitly add it later; this is navigation filtering over already-loaded local data.

## Q28

Session message ids: assistant `145`, user `146`

**Question**

Question 28: where should the filter input appear?

My recommendation: **inline command bar at the bottom, above the footer**, not a modal. Filtering is transient navigation, not a mutation.

Options:
- **A. Bottom command bar**: `/ filter: react`
- **B. Header search field**
- **C. Compact modal**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Filter input will be a bottom command bar above the persistent footer. It is non-modal and non-mutating; Escape clears/exits filter mode.

## Q29

Session message ids: assistant `148`, user `149`

**Question**

Question 29: should filtering match only names, or names plus descriptions/paths/status?

My recommendation: **names, aliases, descriptions, and location chips; not full absolute paths by default**. Full paths are noisy and can cause surprising matches like `/home/inky`.

Options:
- **A. Name + aliases only**
- **B. Name + aliases + descriptions + location chips/status**
- **C. Everything including full paths**

I recommend **B**. Which one?

**Answer**

B.

**Recorded Outcome**

Filter scope is set: names, aliases, descriptions, status, and root chips; full absolute paths are excluded from normal filtering. Full paths stay inspectable in detail modals.

## Q30

Session message ids: assistant `151`, user `152`

**Question**

Question 30: should Repo view rows show active usage chips?

Example: archived `zen-of-go` is currently linked into `.Ag` and `~Cl`.

My recommendation: **yes**. Repo’s main question is “what do I have saved?” but maintenance needs “where is this used?” in the same scan.

Options:
- **A. Yes, show active usage chips in Repo rows**
- **B. No, keep Repo rows only name + description**
- **C. Show usage only in inspector**

I recommend **A**, with inspector showing full active paths. Which one?

**Answer**

A.

**Recorded Outcome**

Repo rows will show active usage chips using the same abbreviated root-chip language. Inspector shows full active paths and repo metadata.

## Q31

Session message ids: assistant `154`, user `155`

**Question**

Question 31: in Repo view, what should `i` do?

The current Go TUI uses `i` to install/link selected repo skills into a destination. But “install” is ambiguous because repo skills are already installed locally; the action is really “link into active root.”

My recommendation: **rename the visible action to Link, keep key `l` for link, avoid `i` in the TUI**. `i` can later mean inspect/install remote search if needed.

Options:
- **A. Keep `i install`**
- **B. Change to `l link`**
- **C. Support both `i` and `l`, show `l link` in footer**

I recommend **C** for compatibility while moving the language to “link.” Which one?

**Answer**

B. l - link, u - unlink, d - delete, p - preview - here's what actions we should have in Repo view. Preview shows a modal with skill text - no refs for now.

**Recorded Outcome**

Repo action model is updated: `l` link, `u` unlink usages, `d` delete from archive, `p` preview `SKILL.md` text. No `i install` in the parity TUI. Preview is a read-only modal for the main skill text, not references yet.

## Q32

Session message ids: assistant `157`, user `158`

**Question**

Question 32: should Repo `u` unlink **all active usages** of the selected repo skill, or open a chooser for which usages to unlink?

My recommendation: **open a workbench modal listing usages, default all selected**. Repo rows can show multiple chips; unlinking all by one keypress is too destructive even with confirmation.

Options:
- **A. Always unlink all usages after confirmation**
- **B. Open usage chooser modal, default all selected**
- **C. Open usage chooser modal, default none selected**

I recommend **B**. Which one?

**Answer**

B.

**Recorded Outcome**

Repo unlink will open a usage chooser modal with all usages selected by default, then confirmation/apply. That is efficient for cleanup but still explicit.

## Q33

Session message ids: assistant `160`, user `161`

**Question**

Question 33: should Repo `d` delete be allowed when the skill is actively linked?

My recommendation: **yes, but only through a stronger modal that makes active usages explicit and offers safe choices**:
- cancel
- unlink usages then delete archive
- delete archive anyway, leaving broken links should not be offered

So practically: active usages block direct delete; user must unlink then delete in one confirmed flow.

Options:
- **A. Block delete if active usages exist; tell user to unlink first**
- **B. Offer “unlink usages then delete archive” in one modal**
- **C. Allow delete anyway after warning**

I recommend **B**. Which one?

**Answer**

B. But this one is tricky - the app needs to track where the skill is linked, and it goes out of the current project/global usage. Might be fine, though - user will doctor broken projects later.

**Recorded Outcome**

Repo delete will have a combined “unlink usages then delete archive” flow, but the usage scope needs to be explicit. The current app only knows current project roots plus global roots; it cannot see every other project that may contain symlinks to the archive unless we add a project index later.

## Q34

Session message ids: assistant `163`, user `164`

**Question**

Question 34: for Repo `d`, which active usages should the modal track and unlink?

My recommendation: **only current project roots + global roots for now**, and the modal must say that clearly. Other projects may become broken and can be found later by running `doctor` in those projects. A cross-project usage index is a separate feature.

Options:
- **A. Current project + global roots only**: matches current architecture, honest limitation.
- **B. Try to scan known/recent projects too**: useful but needs project history/indexing.
- **C. Block delete when external usage is unknown**: safest, but makes archive cleanup too hard.

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Repo delete scope is current project roots plus global roots only, with explicit copy in the modal. Future cross-project usage indexing stays out of scope.

## Q35

Session message ids: assistant `166`, user `167`

**Question**

Question 35: should Repo `p` preview render Markdown formatting, or show raw `SKILL.md` text?

My recommendation: **raw text with minimal syntax color**. Rendering Markdown inside Bubble Tea is possible but can obscure exact instructions; preview should let users inspect what agents will actually read.

Options:
- **A. Raw text, minimal syntax color**
- **B. Rendered Markdown**
- **C. Toggle raw/rendered**

I recommend **A** for first parity. Which one?

**Answer**

B. there's Glamour - let's use it. Also, make sure you use  Lip Gloss for modals. We might want the whole Charm_ ecosystem.

**Recorded Outcome**

Repo preview will use Glamour-rendered Markdown, and modal styling should lean into the Charm stack: Bubble Tea for state, Bubbles where appropriate, Lip Gloss for layout/borders/styles, Glamour for Markdown rendering. I’ll design that explicitly instead of treating preview as plain text.

## Q36

Session message ids: assistant `173`, user `175`

**Question**

The new URL is ready. I’m re-publishing Question 36 into the fresh session.

**Answer**

B.

**Recorded Outcome**

Repo preview modal is set: Glamour-rendered Markdown with operational metadata, usage chips, path, and a raw toggle. I’ll continue using the fresh companion URL for visual questions: http://localhost:63675.

## Q37

Session message ids: assistant `177`, user `178`

**Question**

Question 37: should the raw toggle in preview be required for parity, or noted as a near-follow-up?

My recommendation: **include it in parity**. Since preview is rendered Markdown, `r` gives users exact source when rendering hides frontmatter, code fences, or unusual formatting.

Options:
- **A. Include `r` raw/rendered toggle now**
- **B. Rendered only now, raw toggle later**
- **C. Raw available only in detail modal, not preview**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Preview parity includes `r` to toggle rendered/raw. Glamour remains the default view; raw source is one key away.

## Q38

Session message ids: assistant `180`, user `181`

**Question**

Question 38: should archive conflict diffs use Glamour for Markdown files, or stay raw unified diff text?

My recommendation: **raw unified diff text**. Conflict review is about exact changed lines; rendered Markdown would hide syntax and make additions/removals harder to reason about.

Options:
- **A. Raw full-file unified diff for all text files**
- **B. Glamour-render Markdown diffs**
- **C. Toggle raw/rendered in diff modal too**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Conflict diffs stay raw full-file unified diffs for all text files. Glamour is for preview/reading, not for deciding which changed lines survive.

## Q39

Session message ids: assistant `183`, user `184`

**Question**

Question 39: how should binary or unreadable files appear in the conflict diff modal?

My recommendation: **file list marks them, viewer shows metadata only, and the whole-skill decision still applies**. We should not attempt binary diffing in the TUI.

Options:
- **A. Metadata only**: size/hash/status, no content diff.
- **B. Hide binary/unreadable files from the viewer**
- **C. Hex preview first bytes**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Binary/unreadable files will appear in the diff file list and show metadata only in the viewer: type/status, size if available, and hash if readable. No hex preview in parity.

## Q40

Session message ids: assistant `186`, user `187`

**Question**

Question 40: should the app support mouse clicks in the TUI design, or keyboard only?

Bubble Tea can support mouse events, but it adds testing and interaction complexity.

My recommendation: **keyboard-first, mouse optional later**. Terminal users will expect reliable keyboard control; mouse can be a later enhancement.

Options:
- **A. Keyboard only for parity**
- **B. Keyboard plus mouse row selection/modals**
- **C. Mouse support only where Bubbles gives it cheaply**

I recommend **A**. Which one?

**Answer**

A. Mouse should be deferred to backlog.

**Recorded Outcome**

Keyboard-only parity is set, and mouse support should be tracked as a deferred enhancement. I’ll add that backlog item now.

## Q41

Session message ids: assistant `189`, user `190`

**Question**

Question 41: should the TUI have a command palette, or only direct shortcut keys and help?

My recommendation: **no command palette in the first parity pass**. We already have direct action keys, filter input, modals, and help. A palette is another navigation model.

Options:
- **A. No command palette**
- **B. Add command palette with `:`**
- **C. Add later only if shortcuts become too many**

I recommend **C**: explicitly defer it, don’t build it now. Which one?

**Answer**

C.

**Recorded Outcome**

Command palette is deferred. The parity design stays direct-key plus help modal, with a note that `:` palette can be added later if the keymap grows too large.

## Q42

Session message ids: assistant `192`, user `193`

**Question**

Question 42: should the footer be global/static, or context-sensitive per view/modal?

My recommendation: **context-sensitive but stable in shape**. Always same location and style, but keys change by view:
- Active: `enter details  / filter  m migrate  u unlink`
- Repo: `enter details  p preview  l link  u unlink  d delete`
- Doctor: `enter details  f fix all`
- Modal: modal-local keys

Options:
- **A. Static global footer**
- **B. Context-sensitive footer**
- **C. Two-line footer: global keys + context keys**

I recommend **B**. Which one?

**Answer**

B.

**Recorded Outcome**

Footer is context-sensitive, with stable placement/style. It should show only relevant keys for the current view or modal, and the help modal covers the full keymap.

## Q43

Session message ids: assistant `195`, user `196`

**Question**

Question 43: should status/result messages appear above the footer or inside the footer area?

My recommendation: **above the footer as a one-line status bar**. Footer remains reserved for controls. Result modals handle longer output.

Options:
- **A. Status bar above footer**
- **B. Status text replaces footer temporarily**
- **C. Status appears in header**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Status bar is separate and sits above the footer. Footer never gets replaced by logs or status messages.

## Q44

Session message ids: assistant `198`, user `199`

**Question**

Question 44: should the status bar auto-clear after an action or persist until the next action?

My recommendation: **persist until the next user action changes state**. Timed auto-clear is awkward in terminal apps and complicates tests.

Options:
- **A. Persist until next state-changing action**
- **B. Clear on any keypress**
- **C. Auto-clear after a timer**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Status persists until the next state-changing action. No timers in parity.

## Q45

Session message ids: assistant `201`, user `202`

**Question**

Question 45: should filtering be fuzzy or simple substring?

My recommendation: **simple case-insensitive substring for parity**. It is predictable, easy to test, and enough for names/descriptions/chips. Fuzzy ranking can come later if needed.

Options:
- **A. Case-insensitive substring**
- **B. Fuzzy match/ranking**
- **C. Prefix/token match only**

I recommend **A**. Which one?

**Answer**

A. for now, fuzzy in backlog

**Recorded Outcome**

Filtering will be case-insensitive substring for parity, with fuzzy matching deferred. I’ll record fuzzy filtering in the backlog.

## Q46

Session message ids: assistant `204`, user `205`

**Question**

Question 46: should filters persist when switching views?

My recommendation: **no, filters are per-view and cleared when switching views**. Like selections, it avoids hidden state causing “where did my rows go?” confusion.

Options:
- **A. Clear filter on view switch**
- **B. Preserve per-view filters**
- **C. One global filter across views**

I recommend **A** for parity. Which one?

**Answer**

A.

**Recorded Outcome**

Filters clear on view switch. View changes reset transient navigation state: cursor to top, selection cleared, filter cleared.

## Q47

Session message ids: assistant `207`, user `208`

**Question**

Question 47: should the TUI include a refresh key?

My recommendation: **yes, `R` refreshes data**. Filesystem state can change outside the app, and users need a clear way to rescan without restarting.

Options:
- **A. Yes, uppercase `R` refresh**
- **B. Yes, lowercase `r` refresh**
- **C. No refresh key; restart app**

I recommend **A** because lowercase `r` is useful for raw/rendered toggle inside preview. Which one?

**Answer**

A.

**Recorded Outcome**

Refresh is `R` globally in browsing views. Lowercase `r` remains available inside preview for rendered/raw toggle.

## Q48

Session message ids: assistant `210`, user `211`

**Question**

Question 48: should destructive actions require typing the skill name, or is modal confirmation enough?

Destructive actions here are delete archive and unlink unmanaged active directories.

My recommendation: **modal confirmation enough for parity**, because the TUI is local and keyboard-driven, but destructive modals must be visually distinct and list exact paths/usages.

Options:
- **A. Modal confirmation only**
- **B. Type skill name for archive delete**
- **C. Type skill name for delete and unmanaged unlink**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Destructive actions use explicit confirmation modals, not type-to-confirm. The modal must list exact scope, affected paths/usages, and default to cancel when applicable.

## Q49

Session message ids: assistant `213`, user `214`

**Question**

Question 49: in confirmation modals, should the default highlighted option be the safe action or cancel?

My recommendation: **safe action for reversible/low-risk, cancel for destructive**:
- Doctor safe fixes: default Apply
- Link: default Link
- Migrate: default Apply
- Delete archive / unlink unmanaged directory: default Cancel

Options:
- **A. Always default Apply**
- **B. Always default Cancel**
- **C. Risk-based default**

I recommend **C**. Which one?

**Answer**

C.

**Recorded Outcome**

Confirmation defaults are risk-based: safe/apply defaults for low-risk actions, cancel defaults for destructive actions.

## Q50

Session message ids: assistant `216`, user `217`

**Question**

Question 50: should modals support `y/n` shortcuts in addition to arrow/enter?

My recommendation: **yes for simple confirmation modals only**. For multi-choice conflict/diff modals, use explicit action keys like `k`/`l` and arrow/enter to avoid accidental destructive choices.

Options:
- **A. No `y/n`; only arrows + enter**
- **B. `y/n` for all modals**
- **C. `y/n` only for simple yes/no confirmations**

I recommend **C**. Which one?

**Answer**

C.

**Recorded Outcome**

`y/n` shortcuts apply only to simple yes/no confirmations. Multi-choice and diff modals require explicit selection/action keys.

## Q51

Session message ids: assistant `219`, user `220`

**Question**

Question 51: should the app have an explicit “dry-run preview” action before applying migrations/unlinks, or are modal previews enough?

My recommendation: **modal previews are enough for parity**. A separate dry-run mode is more useful for CLI than TUI.

Options:
- **A. Modal preview only**
- **B. Add `p preview` for actions before apply in all views**
- **C. Add dry-run result modal for batch actions**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Action modals are the preview. No separate dry-run action in the parity TUI.

## Q52

Session message ids: assistant `222`, user `223`

**Question**

Question 52: should Active `u` unlink unmanaged directories offer both “migrate then unlink” and “delete active copy” in the modal?

My recommendation: **yes**. This was one of the original UX gaps. Default should be “migrate then unlink” because it preserves data; destructive delete option is present but not default.

Options:
- **A. Offer both choices**
- **B. Only migrate then unlink**
- **C. Only delete if a separate flag/config enables it**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Active unlink for unmanaged directories will offer both: migrate then unlink, and delete active copy. Preserve-data action is default; destructive delete is visually marked and not default.

## Q53

Session message ids: assistant `225`, user `226`

**Question**

Question 53: if selected Active rows include a mix of managed symlinks, broken symlinks, and unmanaged directories, should unlink use one combined modal or split into steps by category?

My recommendation: **split by category inside one workbench modal**. It shows a grouped plan:
- remove managed links
- remove broken links
- migrate/delete unmanaged directories

The unmanaged category may require per-item choice if multiple exist.

Options:
- **A. One grouped workbench modal**
- **B. Separate modal per item**
- **C. Reject mixed selection**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Mixed unlink uses one grouped workbench modal. It groups managed, broken, and unmanaged entries, with unmanaged entries exposing preserve/delete choices.

## Q54

Session message ids: assistant `228`, user `229`

**Question**

Question 54: for multiple unmanaged directories in an unlink modal, should the preserve/delete choice be global or per item?

My recommendation: **global default with per-item override later, but parity should use one global choice**. Per-item choices add a mini table editor inside the modal.

Options:
- **A. One global choice for all unmanaged items**
- **B. Per-item preserve/delete choices**
- **C. If mixed unmanaged items exist, process one at a time**

I recommend **A** for parity. Which one?

**Answer**

A.

**Recorded Outcome**

For parity, unmanaged unlink choice is global across all unmanaged items in the modal. Future per-item choices can be a later enhancement if needed.

## Q55

Session message ids: assistant `231`, user `232`

**Question**

Question 55: should linked active groups show a dedicated group badge/count in the row?

Example: `zen-of-go .Ag ~Cl ◆ unmanaged ×2`

My recommendation: **yes, show a small count badge when a row represents multiple active locations**. It prevents hidden multiplicity.

Options:
- **A. Yes, count badge in row**
- **B. No, locations chips are enough**
- **C. Only show count in inspector**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Linked/content groups get a count badge in the row when they contain multiple active locations. Inspector explains exact members and aliases.

## Q56

Session message ids: assistant `234`, user `235`

**Question**

Question 56: what should the group count badge look like?

My recommendation: **`×2` / `×3` after location chips**. It is compact and familiar.

Options:
- **A. `×2`**
- **B. `2x`**
- **C. `group:2`**
- **D. `⧉2`**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Group count badge is `×N`. It appears only when the row represents multiple active entries.

## Q57

Session message ids: assistant `237`, user `238`

**Question**

Question 57: should broken active entries group by content fingerprint?

They usually cannot resolve to content, so current code groups broken by path.

My recommendation: **broken entries are never content-grouped; each broken path is its own row**. Broken issues need precise path remediation.

Options:
- **A. Broken entries are one row per broken path**
- **B. Broken entries group by skill name**
- **C. Broken entries appear only in Doctor, not Active**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Broken entries remain one row per broken path in Active. Doctor also shows issue rows, but Active should still reveal broken active state in context.

## Q58

Session message ids: assistant `240`, user `241`

**Question**

Question 58: should the Inspector show internal fingerprints?

My recommendation: **not by default; show only in detail modal under a debug section**. Fingerprints are useful for trust/debugging but noise for normal maintenance.

Options:
- **A. Never show fingerprints**
- **B. Show in inspector always**
- **C. Show only in detail modal/debug section**

I recommend **C**. Which one?

**Answer**

C.

**Recorded Outcome**

Fingerprints go in the detail modal’s debug section only. Main rows and normal inspector stay human-oriented.

## Q59

Session message ids: assistant `243`, user `244`

**Question**

Question 59: should the detail modal include full resolved symlink targets?

My recommendation: **yes**. It is one of the main reasons to have a detail modal.

Options:
- **A. Yes, show active path + resolved target for each member**
- **B. Show active paths only**
- **C. Show resolved targets only for broken/unmanaged symlinks**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Detail modal includes active paths and resolved targets for every member when available, plus error reason for unresolved/broken links.

## Q60

Session message ids: assistant `246`, user `247`

**Question**

Question 60: should the TUI have a theme switcher, or just one theme plus terminal color fallback?

My recommendation: **one theme for parity**, with semantic styles and color fallback. Theme switching can wait.

Options:
- **A. One theme only**
- **B. Light/dark theme toggle**
- **C. Respect terminal background heuristics**

I recommend **A**. Which one?

**Answer**

A. for now, theme support to the backlog.

**Recorded Outcome**

One theme for parity. I’ll record theme support as a deferred enhancement.

## Q61

Session message ids: assistant `249`, user `250`

**Question**

Question 61: should color be disabled under `NO_COLOR`?

My recommendation: **yes**. Respect `NO_COLOR`; keep symbols/layout but remove color styling. Also keep ASCII fallback as a separate capability.

Options:
- **A. Respect `NO_COLOR`**
- **B. Always color in TUI**
- **C. Add explicit TUI setting only**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

The TUI will respect `NO_COLOR`: semantic styling remains in structure/symbols, but color output is disabled.

## Q62

Session message ids: assistant `252`, user `253`

**Question**

Question 62: should the TUI expose a `--ascii` or `--no-unicode` flag for symbol fallback?

My recommendation: **yes, `--ascii` on `x-skills tui`**, plus automatic fallback for dumb terminals if detectable. That gives users a deterministic escape hatch.

Options:
- **A. Add `--ascii`**
- **B. Auto-detect only**
- **C. No fallback flag**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

`x-skills tui --ascii` is in scope for parity, with optional auto-detection as best effort.

## Q63

Session message ids: assistant `255`, user `256`

**Question**

Question 63: should `x-skills interactive` become an alias for `x-skills tui`?

README and old Python docs mention `interactive`, but Go spec says `tui`.

My recommendation: **yes, add `interactive` as an alias/subcommand to the Go TUI**. It reduces migration friction and aligns docs without forcing users to remember the rename.

Options:
- **A. Add `interactive` alias**
- **B. Keep only `tui`, update docs**
- **C. Keep both but mark `interactive` deprecated**

I recommend **A**. Which one?

**Answer**

B.

**Recorded Outcome**

Command surface is `x-skills tui` only for the Go version. The docs/spec should be updated to stop advertising `interactive` for the Go path rather than adding an alias.

## Q64

Session message ids: assistant `258`, user `259`

**Question**

Question 64: should the parity spec include README cleanup as part of implementation?

My recommendation: **yes**. Since we are choosing `tui` only, README’s Go prototype/usage/interactive sections need to be made consistent.

Options:
- **A. Include README cleanup**
- **B. Spec only TUI behavior, leave docs for later**
- **C. Backlog README cleanup**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

README cleanup is in scope for the parity implementation: use `x-skills tui`, remove/adjust stale `interactive` wording for the Go path, and keep Python reference only where explicitly needed.

## Q65

Session message ids: assistant `261`, user `262`

**Question**

Question 65: should search/install from remote `skills.sh` be part of this TUI parity spec?

Earlier docs mention search, but current Go command set does not implement it yet.

My recommendation: **no, local filtering and repo preview/linking are enough for this parity pass**. Remote search/install is a separate feature because it touches networking, package install semantics, trust warnings, and update metadata.

Options:
- **A. Exclude remote search/install from this TUI parity spec**
- **B. Include remote search UI only, no install**
- **C. Include full remote search/install**

I recommend **A**. Which one?

**Answer**

A. Put it in the backlog

**Recorded Outcome**

Remote `skills.sh` search/install is out of this TUI parity spec and belongs in backlog.

## Q66

Session message ids: assistant `264`, user `265`

**Question**

Question 66: should repo update checks (`--check-updates`) be part of this TUI parity spec?

My recommendation: **no, backlog it**. Like remote search, update checks need source metadata, network calls, and separate status/error handling.

Options:
- **A. Exclude and backlog**
- **B. Include passive update status if already cached**
- **C. Include full update checks in Repo view**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Repo update checks are excluded from this TUI parity spec and deferred.

## Q67

Session message ids: assistant `267`, user `268`

**Question**

Question 67: should the implementation use Bubbles `list`/`viewport` components, or keep custom rendering?

My recommendation: **use Bubbles `viewport` for scrollable panes and text views, but keep custom row rendering for the main list**. Bubbles `list` is useful, but custom rows give us tighter control over chips, symbols, grouping, and responsive columns.

Options:
- **A. Bubbles viewport + custom rows**
- **B. Bubbles list for main panes**
- **C. Fully custom rendering**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Implementation direction is set: Bubbles viewport for scrollable panes/text/diffs, custom row rendering for main lists. Lip Gloss owns modal/layout/styling.

## Q68

Session message ids: assistant `270`, user `271`

**Question**

Question 68: should modal state be one generic modal model or separate modal models per flow?

My recommendation: **separate typed modal models behind a common interface**. Generic modals become fragile when conflict diff, preview, confirmation, detail, and result flows all have different state and key handling.

Options:
- **A. Separate typed modal models with shared interface**
- **B. One generic modal model with variants**
- **C. Keep modal state embedded in root model fields**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Modal architecture will use separate typed modal models behind a shared interface. That keeps preview, diff, confirmation, details, and result logic testable and avoids a giant variant struct.

## Q69

Session message ids: assistant `273`, user `274`

**Question**

Question 69: should data loading stay synchronous in the initial TUI model, or use async Bubble Tea commands for scans/actions?

My recommendation: **async commands for actions and refresh, synchronous initial load is acceptable only if fast**. Filesystem scans can grow; action modals should show pending state instead of freezing.

Options:
- **A. Async for refresh/actions, initial sync okay**
- **B. Async for all loading including initial**
- **C. Keep everything synchronous for simplicity**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Data flow is set: initial synchronous load is acceptable, but refresh and mutations should run through Bubble Tea commands with pending states.

## Q70

Session message ids: assistant `276`, user `277`

**Question**

Question 70: should action modals block all background key handling while open?

My recommendation: **yes**. Modal owns input until close/apply. Background list/selection/view state should not change under an open modal.

Options:
- **A. Modal captures all keys except global Ctrl+C**
- **B. Some global keys still work behind modal**
- **C. Background stays navigable behind modal**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Modal input is exclusive: modal captures all keys except global quit/interrupt handling. Background state is visible context, not interactive.

## Q71

Session message ids: assistant `279`, user `280`

**Question**

Question 71: should `q` close the current modal or quit the whole app?

My recommendation: **inside modals, `esc` closes and `q` does nothing or also closes; `ctrl+c` quits app**. Letting `q` quit from inside a destructive modal is risky.

Options:
- **A. `q` closes modal, second `q` quits from main shell**
- **B. `q` always quits app**
- **C. `q` ignored in modal; `esc` closes modal**

I recommend **A** because it matches many terminal UIs. Which one?

**Answer**

A.

**Recorded Outcome**

Inside modal, `q` closes the modal; from the main shell, `q` quits. `ctrl+c` remains global quit.

## Q72

Session message ids: assistant `282`, user `283`

**Question**

Question 72: should Enter on a main row open details in all views?

My recommendation: **yes**:
- Active: active group detail modal
- Repo: repo skill detail modal
- Doctor: issue detail modal

Repo still has `p` for rendered skill preview; Enter is operational details.

Options:
- **A. Enter details in all views**
- **B. Enter primary action per view**
- **C. Enter only in Active/Doctor, Repo uses preview**

I recommend **A**. Which one?

**Answer**

A

**Recorded Outcome**

Enter opens operational details in all top-level views. Repo preview remains `p`, distinct from details.

## Q73

Session message ids: assistant `285`, user `286`

**Question**

Question 73: should `p` preview also work from Active view when the active skill has readable `SKILL.md`?

My recommendation: **yes, but lower priority than Repo preview**. It is useful for unmanaged skills not yet archived.

Options:
- **A. `p` preview in Active and Repo**
- **B. `p` preview only in Repo**
- **C. Active preview only for unmanaged skills**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

`p` preview works in Active and Repo when a readable `SKILL.md` exists. In Active, it previews the selected/cursor active content; in Repo, it previews archive content.

## Q74

Session message ids: assistant `288`, user `289`

**Question**

Question 74: should preview for a content group with multiple active members choose canonical content automatically?

Since grouped members have identical fingerprint, content should be the same, unless read errors occur.

My recommendation: **yes, preview canonical resolved path automatically and show member count/aliases in metadata**.

Options:
- **A. Preview canonical content automatically**
- **B. Ask which active member to preview**
- **C. Disable preview for multi-member groups**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Active preview for content groups uses canonical resolved content automatically, with aliases/member count in preview metadata.

## Q75

Session message ids: assistant `291`, user `292`

**Question**

Question 75: should the spec include a concrete keymap table?

My recommendation: **yes**. With this many choices, implementation needs a keymap table per state: shell, filter, compact modal, workbench modal, diff modal, preview modal.

Options:
- **A. Include full keymap table**
- **B. Describe keys inline only**
- **C. Leave keymap to implementation plan**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

The spec will include a full keymap table by state: shell views, filter mode, compact/workbench/diff/preview/detail/result/help modals.

## Q76

Session message ids: assistant `294`, user `295`

**Question**

Question 76: should the spec include exact Charm libraries and intended responsibilities?

My recommendation: **yes**. This avoids drifting back into custom flat rendering.

Proposed:
- Bubble Tea: app/update loop, commands, key handling
- Bubbles viewport: scrollable lists/text/diffs/preview
- Bubbles textinput: filter input
- Lip Gloss: layout, modal overlays, borders, colors
- Glamour: Markdown rendering for preview
- Custom renderer: rows/chips/status symbols

Approve this responsibility split?

**Answer**

yes

**Recorded Outcome**

Charm stack responsibilities are approved and will be explicit in the spec.

## Q77

Session message ids: assistant `297`, user `298`

**Question**

Question 77: should the spec require tests for visual rendering snapshots, or only model/state tests plus selected rendered-string assertions?

My recommendation: **model/state tests plus focused rendered-string assertions**. Full snapshots of terminal UI are brittle with Lip Gloss widths/colors. We should test behavior and key content.

Options:
- **A. Model/state tests + focused rendered assertions**
- **B. Full render snapshots**
- **C. Manual smoke only for visual layout**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Testing strategy is model/state tests plus focused rendered assertions, not full terminal snapshots. Visual layout can be smoke-tested manually where needed.

## Q78

Session message ids: assistant `300`, user `301`

**Question**

Question 78: should we design for minimum terminal size?

My recommendation: **yes: minimum 100×30 for full two-column experience; below that, collapse inspector into details modal and use single-list layout**. Terminal sizes vary, and hard breakage is unacceptable.

Options:
- **A. Minimum 100×30; below collapse inspector**
- **B. Minimum 80×24; cram two columns**
- **C. No explicit minimum; best effort**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Responsive threshold is set: full layout targets 100×30 and above; below that, collapse the inspector into details modal and use a single-list layout with footer/status retained.

## Q79

Session message ids: assistant `303`, user `304`

**Question**

Question 79: should fullscreen diff modal require a larger minimum size than the shell?

My recommendation: **yes. If too small, show a compact message asking user to resize, while still allowing cancel.** A full-file diff below a certain size is unusable.

Options:
- **A. Require minimum size for diff modal, show resize prompt if too small**
- **B. Render diff anyway with heavy truncation**
- **C. Fall back to summary-only conflict modal**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

Fullscreen diff modal gets its own minimum-size guard. If terminal is too small, show a resize prompt and allow cancel; no summary-only fallback for divergent conflicts.

## Q80

Session message ids: assistant `306`, user `307`

**Question**

Question 80: should the written spec replace the quick `2026-07-06-go-tui-parity-design.md` or create a new fuller spec?

My recommendation: **create a new fuller spec and leave the quick one as historical context**. The quick file came from the immediate bugfix; this brainstorming output is a proper design.

Options:
- **A. New fuller spec**
- **B. Replace existing quick spec**
- **C. Append to existing quick spec**

I recommend **A**. Which one?

**Answer**

A.

**Recorded Outcome**

New fuller spec is the target. I think we’ve resolved the core design tree: shell, symbols, chips, grouping, modals, diffs, repo actions, doctor flow, filtering, responsiveness, keymap, Charm stack, and testing. I’m moving from grilling into approach synthesis now.

# Remote Install/Search Grilling Q&A

This section preserves the later unnumbered `skills.sh` search, Git/GitHub install, source tracking, update, audit, CLI, and Install-tab grilling session from the same JSONL log. Items are labeled `RNN` because the original prompts were decision prompts rather than numbered questions.

Extracted remote decisions: 81

## R01

Session message ids: assistant `982`, user `983`

**Question / Decision Prompt**

First decision: should remote discovery become a real fourth top-level TUI view, using the reserved `I:Install` tab?

My recommendation is **A**.

A. **Add `I:Install` as a top-level view.**
Best fit for our current design language: Active, Repo, Doctor, Install become the four durable work areas. Search state, pagination, selected result, source metadata, install status, and update checks have enough surface area to deserve their own view.

B. **Keep search/install inside Repo as actions/modals.**
Less UI surface, but Repo becomes overloaded: local archive management, source/update tracking, remote discovery, and install workflows all compete in one view.

C. **Use a transient fullscreen search modal launched from Repo.**
Good for quick search, weaker for paginated browsing and repeated install/update work. It also makes source tracking feel bolted on rather than part of the model.

Which direction do you want: A, B, or C?

**Answer**

A. I meant it this way - I Install tab with search box that searches on skills.sh, shows which skills from there are already in our repo, and allows installing (skills.sh links to github). https://github.com/vercel-labs/skills

the user cases we need:
- user just wants to all the skill into repo - they don't want to install it anywhere
- user already has the skill with the same name. we need them to allow choosing - overwrite, rename archive, rename incoming.
- user wants to immediately install - they need to choose the locations (may be more than one)
- user just wants to view the skill info and text - they don't want to install anywhere at all.

the Repo tab needs the following:
- show badges for skills that can be updated (using info from registered github upstream). Allow updating selection.

Updating needs to check if the skill actually exists - some may be deleted on the upstream - but we don't have to delete them ourselves - it's user's call.

**Recorded Outcome**

That settles the first domain boundary: Install is not “link into active roots”; it is remote discovery plus archive/install decisions, with optional immediate linking after archiving. I’m going to capture that terminology in a root `CONTEXT.md` before asking the next branch question.

## R02

Session message ids: assistant `985`, user `986`

**Question / Decision Prompt**

Next decision: when a remote result from `skills.sh` maps to GitHub, what should we treat as the canonical identity for “already in our repo”?

Recommendation: **A**, with a fallback.

A. **Primary identity is source metadata; fallback is skill name.**
If archived skill has `.x-skills.json` matching `source=owner/repo` and `skill_path`, it is the same remote skill. If metadata is missing, compare by `name`. This is pragmatic because existing local archives may not have metadata yet, but future installs become precise.

B. **Identity is only skill name.**
Simplest UI, but too blunt: two different upstreams can publish `code-review`, and update checks become ambiguous.

C. **Identity is only GitHub source + path.**
Technically cleaner, but it fails for existing archives without metadata and won’t show “already in repo” for older/manual installs.

I’d pick **A**. It lets the Install view badge results as `archived`, `update`, or `name conflict` without over-trusting names when metadata exists.

Do you want A, B, or C?

**Answer**

A.

**Recorded Outcome**

Identity is now: source metadata first, name fallback for legacy/manual archives. That is a real trade-off and future readers will wonder why name fallback exists, so I’m recording it as an ADR and adding the resolved terms to the glossary.

## R03

Session message ids: assistant `989`, user `990`

**Question / Decision Prompt**

Next decision: conflict choices when an incoming remote skill has the same archive name.

Your cases mention “overwrite, rename archive, rename incoming.” We need define the exact names and defaults.

Recommendation: **B**.

A. **Always prompt with three choices and no default apply.**
Choices: `replace archive`, `keep archive as <name>-local and install incoming as <name>`, `install incoming as <name>-remote`. Safe, but repetitive when it is truly the same upstream with an update available.

B. **Branch by identity.**
If source metadata proves it is the same remote skill: show an update/replace confirmation, with preview/diff.
If it is only a name conflict or unknown identity: show the three-way conflict modal: `replace archive`, `rename existing archive`, `rename incoming archive`, `cancel`.
This is my recommendation because “update same source” and “name collision” are different user intents.

C. **Never overwrite automatically; always rename incoming unless user explicitly chooses replace.**
Safest for data preservation, but creates archive clutter and makes updates awkward.

Do you want A, B, or C?

**Answer**

B.

**Recorded Outcome**

Conflict handling is now split between same-source updates and unknown/name conflicts. I’ll add that language to the glossary; this probably also deserves an ADR because it shapes install/update behavior and prevents future simplification into a single overwrite prompt.

## R04

Session message ids: assistant `992`, user `993`

**Question / Decision Prompt**

Next decision: what does “view skill info and text” mean before install, given `skills.sh` search results only expose basic fields and the actual text lives in GitHub?

Recommendation: **B**.

A. **Preview only search metadata until installed.**
Fast and simple, but weak: user cannot inspect the actual `SKILL.md` before trusting/installing it.

B. **Fetch a temporary GitHub checkout for preview, then show info + `SKILL.md` in a Glamour preview modal.**
Best user fit. The preview action does network/git work into a temp dir, finds the named skill, reads `SKILL.md`, and stores nothing unless the user installs. Cache per result during the TUI session so repeated previews don’t reclone.

C. **Use GitHub raw URLs/API to fetch only likely `SKILL.md` paths.**
Less disk work, but brittle because `skills.sh` may only give source/name, and skill path discovery often requires scanning the repo.

I’d choose **B**. It matches install behavior and avoids adding a second GitHub discovery strategy.

Do you want A, B, or C?

**Answer**

B.

**Recorded Outcome**

Preview-before-install will use the same temporary GitHub checkout/discovery path as install, cached only for the session. That decision is more implementation strategy than domain language, so I’m not adding an ADR unless we later choose a long-lived cache.

## R05

Session message ids: assistant `995`, user `996`

**Question / Decision Prompt**

Next decision: pagination for `skills.sh` search.

I need to verify the actual API before freezing this, but the Python currently only sends `q`, `owner`, and `limit`, then renders the first `limit` results. If `skills.sh` supports no page/cursor parameter, “pagination” in Go can still mean incremental fetch with larger limits or local paging over fetched results.

Which UX should we design for first?

A. **Local paging over fetched results.**
CLI gets `--limit`; TUI fetches up to a configured result count, then pages locally with `n/p` or page keys. This is robust even if skills.sh has no server pagination.

B. **Server pagination if API supports it, local fallback otherwise.**
Best if the API has `page`/`offset`/`cursor`, but adds discovery and error cases. I’ll verify the API before writing the spec.

C. **Infinite “load more” by increasing `limit`.**
Works without server pagination, but can duplicate/reorder results and gets inefficient.

Recommendation: **B**, but only if we can confirm the API supports it; otherwise spec should explicitly fall back to **A**.

Pick A, B, or C?

**Answer**

I don't think we need actual pagination - we need to check if server has one, or returns everything. - if it has one, we adapt to it in "load more" (C). it if doesn't, we just show everything in a long list. we'll need at least 2 characters for search, though. Check the https://github.com/vercel-labs/skills.git for how they implement it.

**Recorded Outcome**

I’ll verify the Vercel skills implementation before we lock pagination semantics. Since this is a source-referenced current implementation, I’m checking the upstream repo directly rather than guessing from the Python client.

## R06

Session message ids: assistant `1000`, user `1001`

**Question / Decision Prompt**

Checked `vercel-labs/skills` current source:

- `src/find.ts` uses legacy unauthenticated `https://skills.sh/api/search`.
- It sends only `q`, optional `owner`, and `limit=10`.
- I found no `page`, `cursor`, `offset`, or load-more support in the CLI search path.
- Interactive search waits until query length is at least 2 characters.
- It renders at most 8 visible results from the fetched result set.
- There is a newer `/api/v1` API, but public notes indicate it requires auth via Vercel OIDC/API access, so it is not a drop-in CLI/TUI parity path.

So the design should say: use the legacy `/api/search` endpoint for now; search starts at 2 chars; no assumed server pagination. TUI displays the full returned result set as a scrollable long list. We can keep a `Load more` affordance out of v1 unless we later discover/choose an authenticated paginated API.

Next decision: how many remote results should the Go TUI request per search?

Recommendation: **B**.

A. **Request 10, matching upstream exactly.**
Strict parity, but weak for your “long list” expectation.

B. **Request 50 by default, configurable in CLI with `--limit`, fixed or configurable in TUI later.**
Pragmatic: still bounded, enough for browsing, no fake pagination. CLI can preserve Python’s `--limit` behavior but default can be discussed.

C. **Request a high cap like 200.**
Better discovery, but heavier on an undocumented legacy endpoint and more likely to hit latency/rate issues.

I’d choose **B**, with TUI showing “showing N results” and a retry/error state. Which do you want: A, B, or C?

**Answer**

B.

**Recorded Outcome**

Search behavior is settled: legacy unauthenticated `/api/search`, min two characters, bounded request with a default of 50, then a normal scrollable list. I’ll capture that as an ADR because it records why we are not using the newer authenticated API or inventing pagination.

## R07

Session message ids: assistant `1003`, user `1004`

**Question / Decision Prompt**

Next decision: for GitHub installs, should Go mirror the Python implementation by shelling out to `git clone`, or use GitHub/API/archive downloads?

Recommendation: **A**.

A. **Use `git` CLI as the install/update transport.**
Matches Python parity, supports arbitrary public GitHub repos, lets us record exact commit via `git rev-parse HEAD`, and update via `git ls-remote HEAD`. It also keeps implementation straightforward. Downside: requires `git` and can hit GitHub rate/network issues.

B. **Use GitHub archive ZIP downloads.**
No local git clone needed for install, but commit tracking and subpath discovery get trickier. Private/authenticated later work is harder.

C. **Use the official skills.sh download/API path.**
Cleaner long term if authenticated API is adopted, but not parity and introduces auth/token design now.

I’d choose **A** for this parity slice, with clear errors if `git` is missing and optional future backlog for authenticated API/download support.

Do you want A, B, or C?

**Answer**

A.

**Recorded Outcome**

Git transport is settled: clone for install/preview and `ls-remote HEAD` for updates. That’s a dependency and transport decision future maintainers will question, so I’m adding an ADR and term entries now.

## R08

Session message ids: assistant `1006`, user `1007`

**Question / Decision Prompt**

Next decision: immediate install locations.

Your case says user may want to “immediately install” after archiving and choose more than one location. In our domain language, that means “archive remote skill, then link it into selected active roots.”

What should the TUI default selection be in that flow?

Recommendation: **B**.

A. **No active roots selected by default.**
Safest, but extra friction for the common “install and use now” path.

B. **Current project Agents (`.Ag`) selected by default, with checklist for all six roots.**
Matches Python CLI’s local search install default of project agents, while making multi-root explicit and editable. Good balance.

C. **Preselect roots where this skill is already active, otherwise `.Ag`.**
Smart for updates/reinstalls, but can surprise users by modifying multiple roots after a remote install.

I’d choose **B**. The modal should show chips/checklist: `.Ag .Cl .Cd ~Ag ~Cl ~Cd`, with `.Ag` checked unless the user opened from an existing active context.

A, B, or C?

**Answer**

B.

**Recorded Outcome**

Immediate install means archive first, then optionally link to a selected checklist of active roots, defaulting to `.Ag`. I’m updating the glossary because “install” and “link” are deliberately separate in this project.

## R09

Session message ids: assistant `1009`, user `1010`

**Question / Decision Prompt**

Next decision: CLI parity for “install and immediately link”.

Python has `x-skills search react --install 1 -y`, but it only archives remote results; local repo search results link to project agents. For Go parity-plus, should remote search install get flags to link after archiving?

Recommendation: **B**.

A. **Keep Python parity only: remote `search --install` archives, never links.**
Simple and predictable, but CLI lacks the “immediately install to locations” case.

B. **Add explicit link-after-install flags: `--link`, plus optional `--project/--global --target`.**
Default remote install still archives only. If `--link` is present with no location flags, link to project agents. For multiple locations, repeat something like `--to project:agents --to global:claude` or equivalent. This mirrors TUI while staying explicit.

C. **Always prompt after remote archive install: “Link now?”**
Nice interactive UX, but worse for scripts and adds another prompt layer.

I’d choose **B**, but we need decide flag shape later if you agree.

A, B, or C?

**Answer**

For CLI - if user wants to install the script, it's ~80% they want to install into a project. so instead of --link we should have a --no-link flag for times when they just want to install into the repo, not the current project. B, but with my decision

**Recorded Outcome**

You said “script”; I’m going to normalize that to **skill** in the docs because this project’s glossary is skill/archive/link based, not script based. The CLI decision is: remote install archives and links to project `.Ag` by default, with `--no-link` for archive-only. That differs from Python parity, so I’m recording it as a deliberate parity-plus decision.

## R10

Session message ids: assistant `1012`, user `1013`

**Question / Decision Prompt**

Next decision: CLI flag shape for selecting more than one immediate link location.

Recommendation: **A**.

A. **Use repeatable `--to SCOPE:TARGET`.**
Examples:
`x-skills search react --install 1 -y --to project:agents --to global:claude`
`x-skills repo add-github owner/repo skills/foo -y --to project:codex`
Clear, compact, repeatable, and maps directly to root chips. If omitted, default is `project:agents`. `--no-link` conflicts with `--to`.

B. **Reuse `--project/--global --target`, but only one destination.**
Matches existing commands, but cannot express “more than one” without repeated command runs.

C. **Separate flags: `--project-target agents --global-target claude` repeated.**
More verbose and awkward; not worth it.

I’d choose **A**. Do you want A, B, or C?

**Answer**

A, but as `--to global:agents --to .agents --to ~claude --to ~cd` and so on. Users don't actually want to write a lot of text. So we need to ensure we allow short names, case-insensitive, and markers like . and ~

so we allow:
- global: and project: prefixes, as well as g: and p:
- ~ and . prefixes to match TUI.
- full names like codex, claude, opencode, also mixed case and uppercase, like OpenCode
- short names like Cd, Ag, and so on, also low-case like cl

**Recorded Outcome**

Destination parsing is now an actual user-facing language, not just a flag. I’ll record the accepted forms in the glossary and likely make an ADR because this compact grammar will be surprising unless documented.

## R11

Session message ids: assistant `1015`, user `1016`

**Question / Decision Prompt**

Next decision: what should `--to agents` or `--to Ag` mean when there is no scope marker?

Recommendation: **A**.

A. **Default unscoped selectors to project scope.**
`--to agents`, `--to Ag`, `--to codex` mean `.Ag`, `.Cd`, etc. This matches “install into current project” as the common CLI case. Users write `~Ag`, `global:agents`, or `g:ag` for global.

B. **Reject unscoped selectors as ambiguous.**
Safer, but defeats your goal of short CLI spellings.

C. **Default unscoped selectors to current command flags if present, otherwise project.**
Flexible, but harder to explain and test.

I’d choose **A**. Is that right?

**Answer**

A. default to project scope.

**Recorded Outcome**

Unscoped destination selectors default to project scope. I’m updating the ADR with that explicit rule so the future parser tests have a crisp contract.

## R12

Session message ids: assistant `1018`, user `1019`

**Question / Decision Prompt**

Next decision: source/update metadata format.

Python writes `.x-skills.json` inside the archived skill with:

```json
{
  "version": 1,
  "source_type": "github",
  "source": "owner/repo",
  "clone_url": "https://github.com/owner/repo.git",
  "commit": "abc123",
  "skill_path": "skills/foo"
}
```

Recommendation: **A**.

A. **Keep this exact file name and v1 shape for Go.**
Best Python parity and migration path. Go may add fields later, but v1 must read/write this shape.

B. **Move metadata to central repo index under `~/.x-skills`.**
Better for global queries, but a bigger migration and easier to desync from copied skill folders.

C. **Use both: embedded metadata plus a generated index cache.**
Eventually useful, but not needed for this parity slice.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Metadata stays as embedded `.x-skills.json` v1. This is straightforward Python parity, so I’m just adding it to the glossary, not a new ADR.

## R13

Session message ids: assistant `1021`, user `1022`

**Question / Decision Prompt**

Next decision: update checks in Repo.

Python checks `git ls-remote <clone_url> HEAD` and compares the returned commit with stored `commit`. User says we also need to check if the skill actually exists upstream, because the source repo may still exist but the skill path may be deleted.

What should the Repo view/CLI display states be?

Recommendation: **B**.

A. **Keep Python states only:** `up to date`, `update available`, `unknown`.
Parity, but misses deleted upstream skill path.

B. **Add explicit source availability states:**
`up to date`, `update available`, `missing upstream`, `unknown`.
`missing upstream` means GitHub source was reachable but the stored `skill_path` no longer contains a valid `SKILL.md`. User decides whether to delete local archive.

C. **Split into two dimensions:** update status plus availability status.
Most precise, but too verbose for rows and CLI output.

I’d choose **B** for row badges and summaries. Do you agree?

**Answer**

B. up to date should be just a green pill with a tick. update available is a yellow pill with a good symbol like 🗘 . missing upstream should be red pill with !
unknown - blue pill with ? - but we need to make sure that we have a legend somewhere, or explanation with the same pill shape and colors and also text in inspector.

**Recorded Outcome**

Update states are settled as four visible pill states, with text explanations in the inspector/help rather than relying on symbols alone. I’ll capture the domain terms now; the exact colors/icons will go into the design spec/mockups when we reach the visual section.

## R14

Session message ids: assistant `1024`, user `1025`

**Question / Decision Prompt**

Next decision: when should Repo update checks run in the TUI?

Recommendation: **B**.

A. **Automatically check every tracked repo skill whenever entering Repo.**
Convenient, but surprising network activity and can make Repo feel slow or flaky.

B. **Manual action in Repo: `U` or `^u` checks updates for visible/selected tracked skills.**
Keeps network work explicit. Rows initially show a neutral `tracked`/source badge if metadata exists; after check they show `✓`, `🗘`, `!`, or `?` pills. Selection lets user check/update a batch.

C. **Background check after Repo loads, with a spinner/status.**
Modern UX, but more moving state and harder to reason about in a terminal manager.

I’d choose **B**. It matches our “no hidden network/destructive work” posture. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Repo update checks will be explicit, selection-aware network actions. That matches the existing prompt/network philosophy and does not need another ADR beyond the source/update decisions already recorded.

## R15

Session message ids: assistant `1027`, user `1028`

**Question / Decision Prompt**

Next decision: what key should Repo use for update checks and applying updates?

We already have global uppercase tabs: `A`, `R`, `D`, `I`. Lowercase `u` in Repo currently means unlink visible usages. We need avoid collisions.

Recommendation: **B**.

A. **Use `U` for “check updates” and `enter`/modal action for “apply updates”.**
Problem: uppercase letters are tab schema territory, and we already decided uppercase is global navigation/reserved.

B. **Use `^U` for “check updates”, and `x` for “update selected” only after update status is known.**
Keeps uppercase tabs clean. `x` is currently unused in Repo and reads as execute/apply action, but may be unintuitive.

C. **Use `o` for “check updates” and `x` for “update selected”.**
No control key, but `o` is less semantically obvious.

My recommendation is **B**: `^U check updates`, `x update selected`. Footer can show `^U check` and `x update` only in Repo. Do you prefer B, or another key scheme?

**Answer**

I don't think we need to check updates manually - it should be a hidden goroutine that caches the results. it should start when the app starts but it shouldn't hinder the app in any way. so, update selected is ^U.

Also, somehow now link action in repo doesn't work on selected skills - onle for highlighted (under cursor) - we need to fix this,

**Recorded Outcome**

That supersedes the manual-check choice: update checks should be non-blocking background work, cached in the TUI, and `^U` means “update selected.” I’m also folding the Repo selected-skills action bug into this parity scope: Repo actions must operate on selected rows when selection exists, and cursor row only when nothing is selected.

## R16

Session message ids: assistant `1030`, user `1031`

**Question / Decision Prompt**

Next decision: background update cache lifetime and refresh behavior.

Recommendation: **B**.

A. **Check once per TUI process, refresh only on app restart.**
Simple, but stale if the user keeps the TUI open or installs/updates during the session.

B. **Check on TUI startup, cache during session, refresh on `^R` and after install/update actions.**
Best fit. The checks never block initial rendering; statuses update when results arrive. `^R` still means “rescan filesystem and refresh remote statuses.”

C. **Check periodically on a timer.**
Probably unnecessary network noise for a maintenance tool.

I’d choose **B**. Do you want A, B, or C?

**Answer**

B.

**Recorded Outcome**

Cache lifetime is settled: startup, `^R`, and after install/update actions. I’m updating the ADR with that explicit refresh policy.

## R17

Session message ids: assistant `1033`, user `1034`

**Question / Decision Prompt**

Next decision: batch update conflict handling in Repo.

Suppose user selects 5 archived skills and presses `^U`. Two are clean updates, one is missing upstream, one has local modifications/diverged content, one fails network. What should happen?

Recommendation: **B**.

A. **All-or-nothing batch.**
Abort the whole batch on first non-clean item. Simple, but frustrating.

B. **Per-skill batch with modal summary; clean updates proceed, conflicts pause for each decision.**
For each selected skill:
- `update available` with same upstream path: fetch incoming, compare fingerprints.
- if same as current commit/content somehow: mark up to date.
- if different: show archive/incoming diff modal and ask keep archive vs save incoming.
- `missing upstream` and `unknown`: skip with reason.
- failures: record and continue.
This matches existing migrate conflict design and batch mutation summaries.

C. **Require user to update one skill at a time.**
Simpler implementation, worse for the requested “update selection.”

I’d choose **B**. Is that the behavior you want?

**Answer**

B.

**Recorded Outcome**

Batch update semantics now mirror our existing mutation style: per item, no rollback, conflict diff when needed, and a summary. That fits existing domain/action behavior, so I’ll keep it in the spec rather than another ADR.

## R18

Session message ids: assistant `1036`, user `1037`

**Question / Decision Prompt**

Next decision: rename behavior for install name conflicts.

When user chooses “rename existing archive” or “rename incoming archive,” how should the new name be chosen?

Recommendation: **B**.

A. **Auto-suggest and allow editing.**
Default suggestions:
- existing archive → `<name>-local`
- incoming archive → `<name>-remote`
If that exists, append `-2`, `-3`, etc. User can edit before applying. Best UX.

B. **Require user-entered name, prefilled with a suggested editable value.**
Slightly safer because user acknowledges the name. Good with Huh/text input or Bubble Tea textinput.

C. **Automatically pick name and apply.**
Too magical for archive identity decisions.

I’d choose **B**: prefilled suggestion, but Enter confirms the visible editable name. Do you want A, B, or C?

**Answer**

B. Ask user to rename. Use Huh or Bubbles.

**Recorded Outcome**

Rename conflicts require an editable prompt, not silent auto-naming. We’ll allow Huh or Bubbles implementation, but the behavior contract is the important part: visible prefilled suggestion, user confirmation, validate archive name, no overwrite unless explicitly chosen.

## R19

Session message ids: assistant `1039`, user `1040`

**Question / Decision Prompt**

Next decision: install/update diff view source labels.

For migration conflicts we use Archive vs Incoming active. For remote install/update, the competing sides are different.

Recommendation: **A**.

A. **Use `Archive` vs `Incoming remote`.**
Consistent with existing diff language and clear that local archive is what might be kept/replaced.

B. **Use `Local` vs `Remote`.**
Shorter, but “local” could mean active project root, not archive.

C. **Use Git-like `ours`/`theirs`.**
Avoid; too easy to confuse.

I’d choose **A**. Diff modal title examples:
- `Update conflict: code-review`
- `Name conflict: code-review`
Legend: `Archive` and `Incoming remote`

A, B, or C?

**Answer**

A.

**Recorded Outcome**

Remote conflicts use Archive vs Incoming remote labels, keeping “local” reserved for project/global roots rather than archive content.

## R20

Session message ids: assistant `1042`, user `1043`

**Question / Decision Prompt**

Next decision: how should `repo add-github` behave in Go compared to `search --install`?

Recommendation: **B**.

A. **Strict Python parity: `repo add-github` archives only, no link flags.**
Simple, but inconsistent with the new CLI remote install default.

B. **Same remote install engine and flags as search install.**
`repo add-github owner/repo skills/foo` archives and links to `.Ag` by default; `--no-link` archives only; `--to` selects roots; same conflict/update prompts and metadata. This is consistent for all remote installs.

C. **Keep `repo add-github` as lower-level archive-only, make `search --install` user-friendly.**
Cleaner command separation, but likely surprising: two remote install commands behave differently.

I’d choose **B**. Do you want B?

**Answer**

B.

**Recorded Outcome**

All remote install entry points will share one install engine and one CLI linking policy. That’s important architecture but unsurprising given the previous decision, so it belongs in the spec/plan rather than a separate ADR.

## R21

Session message ids: assistant `1045`, user `1046`

**Question / Decision Prompt**

Next decision: `repo add-url`.

Python supports direct `SKILL.md`, zip, tar, tar.gz URL installs. Your request specifically says “skills.sh search, GitHub skill install, source tracking and updates.” Should URL install be part of this parity slice?

Recommendation: **B**.

A. **Include `repo add-url` now too.**
Full README/Python parity, but expands scope into archive extraction, direct SKILL.md materialization, URL trust/error handling, and no update tracking.

B. **Defer `repo add-url`; include only skills.sh/GitHub/source/update parity.**
Keeps this slice focused on the source-tracked GitHub path and TUI Install view. We can leave URL install in backlog/spec as a future parity item.

C. **CLI-only `repo add-url`, no TUI.**
Partial parity but awkward; no source tracking/update story.

I’d choose **B** for this pass. A, B, or C?

**Answer**

B. url doesn't allow us to track updates as it can only go for CRC/SHA of the skill and has to allow remote directory listing to get references if skill has one. So defer for now until we have a good understanding of how to work with the url installation.

**Recorded Outcome**

URL install is deferred because it lacks a clean update/provenance model for directories/references. That rationale meets the ADR bar: future readers will wonder why Python parity was intentionally incomplete here.

## R22

Session message ids: assistant `1048`, user `1049`

**Question / Decision Prompt**

Next decision: what exactly should the Install view rows show?

Recommendation: **A**, in our existing rich-row language.

A. **Rich rows with source/update/archive badges before description.**
Example fields:
`□ react-helper  owner/repo  1.2K  [archived] [update]  description...`
Inspector shows full slug, source, installs, archive match reason, metadata, and available actions. Row badges should use our pill component.

B. **Table-like rows with columns.**
Readable in wide terminals, but less consistent with current Active/Repo rich rows and badges.

C. **Minimal rows, all details in inspector.**
Clean but too opaque for comparing search results.

I recommend **A**. Does that match what you want?

**Answer**

A. Rich rows. We'll need to extract them into our own ui package so they are reused everywhere - we just define the array of elements they have and they pass backgrounds into elements and so on.

**Recorded Outcome**

Rich rows become a reusable UI primitive, not one-off Install rendering. That is a cross-view code structure decision and will go into the spec/plan; it probably doesn’t need an ADR because it’s a natural continuation of the existing pill/shortcut extraction.

## R23

Session message ids: assistant `1051`, user `1052`

**Question / Decision Prompt**

Next decision: what are the reusable row elements?

Recommendation: **B**.

A. **Generic string segments only.**
Simple but weak: every caller manually manages pill/background behavior.

B. **Typed row elements:** text, muted text, status pill, root/source pill, spacer/fill, description.
Renderer receives row state (`normal`, `cursor`, `selected`, `disabled`) and background, then each element renders itself correctly. This matches your pill refactor and prevents selected row backgrounds from breaking pills.

C. **Full row builder per view with callbacks.**
Flexible, but likely over-engineered now.

I’d choose **B**. It gives Install, Repo, Active, and Doctor the same row behavior while letting each view define its element array.

A, B, or C?

**Answer**

B.

**Recorded Outcome**

Row rendering will be typed elements with row-state-aware backgrounds. I’ll treat this as a supporting UI refactor in the implementation plan, scoped only to rows we touch for Install/Repo update work.

## R24

Session message ids: assistant `1054`, user `1055`

**Question / Decision Prompt**

Next decision: should the TUI Install view support selecting multiple search results and installing them as a batch?

Recommendation: **A** for the first slice.

A. **Single-result install only; multi-location linking for that result.**
Search/install conflict decisions are already complex: preview, same-source updates, name conflicts, rename prompts, optional multi-root linking. Single result keeps the wizard understandable. Batch remote install can go to backlog.

B. **Multi-select search results and batch install.**
Powerful, but conflict/preview/rename/link choices multiply quickly.

C. **Multi-select only for archive-only install, not immediate link.**
Still complex and inconsistent.

I’d choose **A**. Do you agree?

**Answer**

A.

**Recorded Outcome**

Install view remote install is single-result for now; batch remote installs go to backlog.

## R25

Session message ids: assistant `1057`, user `1058`

**Question / Decision Prompt**

Next decision: source preview cache.

You chose temporary GitHub checkout for preview. Should the TUI cache fetched preview/install checkouts only in memory/temp dirs, or persist them under `~/.x-skills`?

Recommendation: **A**.

A. **Session-only temp cache.**
Previewing a result clones once per TUI session, reused for preview/install of that result, deleted when process exits. No cache invalidation or disk growth problem.

B. **Persistent cache under `~/.x-skills/cache`.**
Faster repeated previews, but now we need eviction, commit freshness, and cache corruption handling.

C. **No cache: reclone on every preview/install.**
Simple but slow and wasteful.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Preview/install checkout cache is session-only. That keeps the source-tracking system focused on archived skills, not cache management.

## R26

Session message ids: assistant `1060`, user `1061`

**Question / Decision Prompt**

Next decision: trust/security warning.

Remote skills affect future agent behavior. Where should the warning appear?

Recommendation: **B**.

A. **Only in docs/help.**
Low friction, but easy to miss.

B. **In install confirmation and preview/inspector text, but not as a blocking extra step after user confirms.**
Balanced: user sees “Review SKILL.md before installing remote skills” and the source/commit before mutating. No repeated scary modal.

C. **Always require explicit trust confirmation for every remote source.**
Safer, but too much friction for the common case.

I’d choose **B**. A, B, or C?

**Answer**

B. Skills.sh site has some kind of information about safety, if it is readable through API, we need to show it to users, so they understand what's going on. if not - just B.

**Recorded Outcome**

I’ll check whether the current public search payload exposes any safety/trust fields before we lock that UI contract. If it is only name/source/install counts, the spec will use our own trust copy in confirmation/preview.

## R27

Session message ids: assistant `1064`, user `1065`

**Question / Decision Prompt**

Next decision: should Go implement the upstream security audit API now, separate from search?

Recommendation: **B**.

A. **Implement upstream audit fetching now for GitHub installs/previews.**
Potentially useful, but it is outside Python parity and needs its own API contract/error states. We’d have to inspect `src/telemetry.ts`/`src/add.ts` deeper and decide whether we trust that endpoint.

B. **Do not implement audit fetching in this slice; show trust warning and source metadata only.**
Focused and predictable. We can backlog “remote audit/risk badges” once we understand the API and whether it is stable/public.

C. **Only show a static “unknown safety” pill for all remote skills.**
Useful visually, but could imply we performed a check when we didn’t.

I’d choose **B**. Do you want B, or should we investigate audit fetching as part of this scope?

**Answer**

A.

**Recorded Outcome**

I’ll investigate the upstream audit path before we commit to it, because this is a separate API surface from search and may have auth/stability assumptions. If it looks unsuitable for unauthenticated use, I’ll push back with the exact reason.

## R28

Session message ids: assistant `1068`, user `1069`

**Question / Decision Prompt**

Next decision: how should audit data appear in our TUI/CLI?

Recommendation: **B**.

A. **Rows show partner-specific audit columns.**
Too much for the Install row; also couples our UI to current partners (`ath`, `socket`, `snyk`).

B. **Rows show one compact audit pill, inspector/confirmation shows partner details.**
Install row pill examples: `risk safe`, `risk low`, `risk med`, `risk high`, `risk ?`.
Inspector/preview/confirmation show details like:
`Gen: Safe`, `Socket: 0 alerts`, `Snyk: Low Risk`, plus “advisory only; review SKILL.md before install.”
If audit fetch fails or returns nothing, show `risk ?` or “audit unavailable” depending on space.

C. **Only show audit details in confirmation, not rows.**
Less clutter, but users can’t compare results quickly.

I’d choose **B**. Do you agree?

**Answer**

B.

**Recorded Outcome**

Audit data becomes advisory: compact row pill, partner details in inspector/confirmation, failures are non-blocking. I’m adding that to the glossary and an ADR because we are adopting a non-core upstream API and deliberately not blocking installs on it.

## R29

Session message ids: assistant `1071`, user `1072`

**Question / Decision Prompt**

Next decision: when should audit fetch run in the TUI?

Recommendation: **B**.

A. **Fetch audits only when user previews/installs a result.**
Less network work, but rows cannot show risk pills while browsing.

B. **Fetch audits for current search result set in background after search returns; cache by `source + skill`.**
Best UX. Search results appear immediately with `risk ?`/loading, then risk pills update when audit returns. Preview/install uses cached data if present, otherwise shows unavailable.

C. **Fetch audit before showing search results.**
More complete rows but makes search feel slower and couples two network calls.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Audit fetching will be background and cached per source+skill for the current process. I’ll include this in the Install view data flow.

## R30

Session message ids: assistant `1074`, user `1075`

**Question / Decision Prompt**

Next decision: CLI audit output.

Should `x-skills search` include audit info by default?

Recommendation: **B**.

A. **Yes, fetch audit by default for CLI search output.**
Useful, but makes simple search slower and noisier.

B. **Add `--audit` for CLI search/install details; TUI fetches in background by default.**
CLI remains quick and scriptable. If user wants risk info in terminal output, they opt in. During `search --install`, confirmation can fetch/show audit if `--audit` is set; otherwise show static trust warning.

C. **Never show audit in CLI, TUI only.**
Too inconsistent if audit becomes a first-class signal.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

CLI audit is opt-in with `--audit`; TUI audit is background by default. This keeps script output predictable while making the interactive view richer.

## R31

Session message ids: assistant `1077`, user `1078`

**Question / Decision Prompt**

Next decision: search result source matching for `skills.sh`.

Search results expose `source` as `owner/repo` and `id` as something like `owner/repo/skill-slug`. To install, Python clones `source` and finds a skill by result name. Should Go do the same, or derive a path from `id`?

Recommendation: **B**.

A. **Clone source and find skill by result `name`.**
Python parity, but name collisions inside one repo could be ambiguous.

B. **Clone source and use best-effort path discovery: first match by `name`, then slug tail, then require disambiguation if multiple.**
More robust while staying compatible with current API fields. If no unique match, show a modal/CLI prompt listing discovered skill paths.

C. **Assume `id` maps directly to path.**
Likely wrong; current Python does not rely on that.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Install discovery will not assume the search id is a path. It will clone source, scan for skill directories, match by name/slug, and ask if ambiguous.

## R32

Session message ids: assistant `1080`, user `1081`

**Question / Decision Prompt**

Next decision: if GitHub repo contains multiple matching skill candidates for a search result, how should TUI/CLI resolve it?

Recommendation: **B**.

A. **Fail and tell user to use `repo add-github owner/repo path/to/skill`.**
Simple, but interrupts the TUI flow.

B. **Prompt/select from discovered skill paths.**
TUI shows a compact choice modal with path, parsed name, description. CLI prompts when interactive; `--no-input` fails with exact `repo add-github owner/repo path --no-link` hints.

C. **Pick first sorted path.**
Too risky.

I’d choose **B**. Do you agree?

**Answer**

B.

**Recorded Outcome**

Ambiguous discovered skill paths require user selection; no silent first match. That fits the existing ambiguity policy.

## R33

Session message ids: assistant `1083`, user `1084`

**Question / Decision Prompt**

Next decision: GitHub URL/path parsing scope for this pass.

Python supports:
- `owner/repo`
- `https://github.com/owner/repo/tree/main/path/to/skill`
- install from search result source `owner/repo`
It may not robustly handle branches with slashes because parsing tree URLs is tricky.

For Go, what should we commit to?

Recommendation: **A**.

A. **Support `owner/repo [skill_path]` and GitHub tree URLs with simple branch segment.**
Examples:
`repo add-github owner/repo skills/foo`
`repo add-github https://github.com/owner/repo/tree/main/skills/foo`
Good parity, lower complexity.

B. **Support full GitHub tree URL parsing including branch names with slashes.**
More complete but harder: requires resolving branches/tags or GitHub API.

C. **Support only `owner/repo [skill_path]`; defer tree URLs.**
Too much parity loss.

I’d choose **A**, with explicit error guidance for complex tree URLs: use `owner/repo path`. A, B, or C?

**Answer**

A.

**Recorded Outcome**

GitHub source parsing scope is bounded: owner/repo plus simple tree URLs. Complex branch path resolution is out of this slice.

## R34

Session message ids: assistant `1086`, user `1087`

**Question / Decision Prompt**

Next decision: does `repo add-github` also support `--replace-archive` in Go, or do we route all replacement through the new conflict prompt?

Recommendation: **B**.

A. **Keep `--replace-archive` as a force flag.**
Parity, scriptable, but bypasses the richer conflict decision.

B. **Keep `--replace-archive` for non-interactive/scripted use; interactive default shows update/name-conflict modal/prompt.**
Best of both. `--replace-archive -y` replaces without asking, but normal interactive CLI/TUI gets the same decision flow.

C. **Remove/avoid `--replace-archive`; use explicit conflict choices only.**
Cleaner UX, worse automation and less parity.

I’d choose **B**. A, B, or C?

**Answer**

B. but better to have switch as --replace

**Recorded Outcome**

The force replacement flag will be `--replace`, not Python’s `--replace-archive`, with interactive conflict prompts as the normal path. I’ll capture that as CLI naming in the spec; no ADR needed.

## R35

Session message ids: assistant `1089`, user `1090`

**Question / Decision Prompt**

Next decision: rename flags for CLI conflict resolution.

Interactive CLI can prompt for rename names. But in `--no-input` or scripts, users need one-shot flags.

Recommendation: **A**.

A. **Add explicit non-interactive conflict flags.**
- `--replace`
- `--archive-as NAME` for incoming archive name
- `--rename-existing NAME` for moving current archive out of the way
These conflict with each other where appropriate.
Good scriptability, mirrors the three choices.

B. **Only `--replace` and `--archive-as`; no rename-existing flag.**
Simpler, but cannot script “preserve old archive under new name, install incoming under canonical name.”

C. **No rename scripting; fail in non-interactive conflict.**
Safe but incomplete.

I’d choose **A**. Do you want all three?

**Answer**

A.

**Recorded Outcome**

CLI conflict controls are settled: `--replace`, `--archive-as`, `--rename-existing`, with validation for mutually exclusive choices.

## R36

Session message ids: assistant `1092`, user `1093`

**Question / Decision Prompt**

Next decision: what should Repo row update badges do before the background check completes?

Recommendation: **A**.

A. **Show `tracked` pill first, then replace with status pill when checked.**
Clear that the skill has source metadata, and the app has not yet completed update/audit work.

B. **Show `?` unknown immediately.**
Could imply a failed check rather than pending.

C. **Show no badge until checked.**
Misses useful provenance info.

I’d choose **A**: `tracked` neutral pill while pending, then `✓`, `🗘`, `!`, or `?`.

A, B, or C?

**Answer**

A.

**Recorded Outcome**

Repo rows start with a neutral tracked pill and upgrade to update status when background checks return.

## R37

Session message ids: assistant `1095`, user `1096`

**Question / Decision Prompt**

Next decision: update action eligibility.

When user presses `^U` in Repo, which selected rows are acted on?

Recommendation: **B**.

A. **Any selected tracked skill.**
If status is pending, run fresh check/update pipeline. More forgiving but can trigger unexpected network work.

B. **Only selected rows with `update available`; skip others with reasons in summary.**
Clear and predictable. If status is still `tracked` pending, summary says skipped: update status pending. `missing upstream`, `unknown`, untracked are skipped.

C. **If no selected update-available rows, update the highlighted row if eligible.**
Matches existing selected-or-cursor pattern, but can surprise if user has selections elsewhere.

I’d choose **B**, with the general rule “selected rows if any, otherwise cursor row” applying before eligibility. So no selection + cursor on update-available updates one; selected rows update only eligible ones and summarize skipped.

Does that work?

**Answer**

B.

**Recorded Outcome**

`^U` will use selected rows if any, otherwise cursor row, and only update `update available` items. Everything else is skipped with explicit reasons.

## R38

Session message ids: assistant `1098`, user `1099`

**Question / Decision Prompt**

Next decision: background update/audit error visibility.

Recommendation: **B**.

A. **Surface every background error as a status message/modal.**
Too noisy; network checks are advisory.

B. **Keep row-level `?`/unknown pills and inspector details; status bar only says aggregate like `updates checked: 8 ok, 2 unknown`.**
Best balance. No modal unless user explicitly runs an update action and it fails.

C. **Hide background errors entirely.**
Too opaque.

I’d choose **B**. A, B, or C?

**Answer**

Previous one should be C. not B. The same model - all actions work on selected skills with fallback on highlighted one if nothing is selected. Mark it in ADR. all pages actions work only on the current page - not pulling things from Active while in Repo.

For current one - it's B.

**Recorded Outcome**

Correcting that decision: the action target model is global across pages, not update-specific. Every page action uses selected rows on the current page, or the highlighted row only when there is no current-page selection. I’ll record that as an ADR because it fixes a subtle UI consistency rule and directly covers the Repo link bug you noticed.

## R39

Session message ids: assistant `1101`, user `1102`

**Question / Decision Prompt**

Updated: previous decision is now **C in spirit**, with the stricter global rule:

- Every TUI action targets selected rows on the current page.
- If the current page has no selected rows, it targets the highlighted row.
- Selections from other pages are ignored.
- Repo `link` must be fixed to follow this, not just cursor-only behavior.
- `^U` in Repo follows the same targeting rule; rows that are not eligible to update are skipped with reasons in the result summary.

For background update/audit error visibility, decision is **B**:
- row-level `?`/unknown pills and inspector details;
- aggregate status bar messages;
- no modal for advisory background failures.

Next decision: current-page selection storage.

Right now the TUI uses one `selected map[string]bool` keyed by row IDs across views. To enforce the current-page rule, what storage model should we specify?

Recommendation: **B**.

A. **Keep one selection map, but filter by current page prefixes.**
Smallest change, but easy to keep leaking cross-page assumptions.

B. **Separate selection sets per page.**
`activeSelected`, `repoSelected`, `doctorSelected`, `installSelected`. Clearer, directly encodes the ADR, easier to test.

C. **Clear selection on every view switch.**
Simplest behavior, but frustrating if user switches to inspect details and comes back.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Selection state will become separate per page. That gives the current-page action rule an explicit state model instead of relying on key prefixes.

## R40

Session message ids: assistant `1104`, user `1105`

**Question / Decision Prompt**

Next decision: Install view search query behavior.

Recommendation: **B**.

A. **Search only on Enter.**
Less network, but clunkier than upstream interactive search.

B. **Debounced search after 2+ characters, with Enter forcing immediate search.**
Matches upstream intent. Show “type at least 2 characters” below 2 chars, “searching…” while request in flight, keep previous results until new results arrive.

C. **Search on every keypress immediately after 2 chars.**
Too much network churn.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Install search is debounced after two characters, with Enter as an immediate fetch.

## R41

Session message ids: assistant `1107`, user `1108`

**Question / Decision Prompt**

Next decision: `skills.sh` owner filter in TUI.

CLI will have `--owner`. Should the Install view expose owner filtering?

Recommendation: **B**.

A. **No owner filter in TUI.**
Simpler, but loses parity with search CLI and upstream.

B. **Optional owner filter field in Install inspector/header, not a modal.**
Install view has query input plus an owner filter toggle/input. Keep it compact: `owner: all` by default, `/` edits query, maybe `o` edits owner. Results show current filter.

C. **Owner filter only in a settings/help modal.**
Hidden and awkward.

I’d choose **B**. Does Install need an owner filter field?

**Answer**

B.

**Recorded Outcome**

Install view will include an owner filter, but it should stay compact and not turn into a settings screen.

## R42

Session message ids: assistant `1110`, user `1111`

**Question / Decision Prompt**

Next decision: Install view keymap.

We need integrate `I` tab, search query, owner filter, preview, install/archive, install+link, and no batch.

Recommendation:

- `/`: focus/edit search query
- `o`: focus/edit owner filter
- `enter`: preview selected result
- `i`: install selected result with link-location checklist
- `a`: archive-only install selected result
- `^R`: refresh current query and background checks
- `esc`: leave input mode / close modal
- `q`: quit main app
- `?`: help

This keeps `i` as “install/use now” and `a` as “archive only.” It does mean lowercase `a` is an Install action while uppercase `A` remains Active tab, which matches our global schema.

Does this keymap work, or would you prefer different keys?

**Answer**

maybe i to install (use now) and ^I to install to the repo?

**Recorded Outcome**

`^I` is risky: in terminals Ctrl+I is usually indistinguishable from `Tab`, so Bubble Tea will commonly receive it as tab. I would not use it for archive-only install.

Recommendation:

- `i`: install and use now, meaning archive + link to selected destinations
- `a`: archive only, meaning install into repo without linking
- `enter`: preview
- `/`: edit query
- `o`: edit owner
- `tab`: move focus between query/owner/results only if we need focus cycling later, not as a command

This keeps `i` as the primary action and avoids a terminal key collision. Does `a` for archive-only work, or do you want another non-control key?

## R43

Session message ids: assistant `1115`, user `1116`

**Question / Decision Prompt**

Next decision: search results that are already archived.

What should `i` do on a result already archived and up to date?

Recommendation: **B**.

A. **Open the same install flow anyway.**
Allows linking to active roots, but “install” wording is odd.

B. **Treat `i` as “use now”: if archive is already present/up to date, skip archive mutation and show destination checklist to link it.**
Best UX. If already linked to some destinations, preselect none or show linked badges; user picks additional roots. `a` on already archived/up-to-date should show “already archived” and do nothing.

C. **Disable `i`; require user to go to Repo tab to link.**
Too much friction.

I’d choose **B**. Do you agree?

**Answer**

B. skills already in repo should be just linked.

**Recorded Outcome**

Already-archived search results will not reinstall on `i`; they go straight to link destination selection. `a` becomes a no-op/result message for those rows.

## R44

Session message ids: assistant `1118`, user `1119`

**Question / Decision Prompt**

Next decision: if already archived but update is available, what does `i` do from Install?

Recommendation: **B**.

A. **Link existing archive only, ignore update.**
Fast, but may surprise user who came from Install and sees update available.

B. **Ask update vs link existing.**
Modal choices:
- `Update archive, then link`
- `Link current archive`
- `Cancel`
Then destination checklist if linking. Best clarity.

C. **Always update first.**
Too aggressive; remote updates can change behavior.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Install view will distinguish “already archived and current” from “archived but update available”; update is never implicit before linking.

## R45

Session message ids: assistant `1121`, user `1122`

**Question / Decision Prompt**

Next decision: where should source metadata show in Repo?

Recommendation: **A**.

A. **Rows show compact source/update badges; inspector shows full metadata.**
Row: name, usage chips, source pill like `gh`, update pill, description last.
Inspector: source `owner/repo`, skill path, commit short SHA, clone URL, update status, update suggestion/action.

B. **Rows include source text directly.**
Too wide and pushes descriptions off-screen.

C. **Metadata only in details modal.**
Too hidden for update workflow.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Repo rows will get compact source/update badges; full source metadata lives in inspector/detail.

## R46

Session message ids: assistant `1124`, user `1125`

**Question / Decision Prompt**

Next decision: CLI update surface.

Python only has `repo --check-updates` plus a suggestion to rerun `repo add-github ... --replace-archive`. Since Go TUI will support updating selected repo skills, should CLI get a first-class update command too?

Recommendation: **A**.

A. **Add `x-skills repo update NAME...` plus `repo --check-updates`.**
Best parity-plus. `repo --check-updates` lists statuses; `repo update code-review` performs source-tracked update with the same conflict/diff behavior as TUI. `repo update --all` can update all `update available` tracked skills later or now if we want.

B. **Only keep `repo --check-updates`; updates happen via `repo add-github ... --replace`.**
Closer Python parity, but worse UX and duplicates install flags/conflict behavior.

C. **No CLI update command yet; TUI only.**
Inconsistent and harder to test the domain layer.

I’d choose **A**. Do you want `repo update NAME...` in this slice?

**Answer**

A.

**Recorded Outcome**

CLI gets first-class `repo update NAME...`, using the same update engine as TUI `^U`.

## R47

Session message ids: assistant `1127`, user `1128`

**Question / Decision Prompt**

Next decision: should `repo update --all` be in the first slice?

Recommendation: **B**.

A. **Include `repo update --all`.**
Convenient, but conflict handling, skips, and network errors become broader.

B. **Defer `--all`; require explicit names for CLI batch update.**
Focused and safer. TUI can still select multiple visible rows. We can add `--all` later after update behavior stabilizes.

C. **Include `--all` but only for clean no-conflict updates.**
Subtle and probably confusing.

I’d choose **B**. A, B, or C?

**Answer**

CLI:
repo check - checks updates
repo update NAME NAME2 NAME3 ... - updates selected skills
repo update-all - updates everything

this is more logical to me.

**Recorded Outcome**

CLI update surface is now explicit subcommands: `repo check`, `repo update NAME...`, and `repo update-all`. I’ll record this in the spec as a deliberate Go command shape rather than mirroring Python’s `repo --check-updates`.

## R48

Session message ids: assistant `1130`, user `1131`

**Question / Decision Prompt**

Next decision: `repo update-all` safety.

Recommendation: **B**.

A. **Update every tracked skill regardless of status, checking as it goes.**
Simple command, but broad network/mutation surface.

B. **`repo update-all` checks all tracked skills, updates only clean `update available` items, pauses/prompts on conflicts, skips missing/unknown with summary.**
Matches batch semantics and avoids deleting/guessing. With `--no-input`, conflicts are skipped with actionable hints; no partial rollback.

C. **`repo update-all` refuses if any conflict/missing/unknown exists.**
Too brittle.

I’d choose **B**. Does that match your intent?

**Answer**

B.

**Recorded Outcome**

`repo update-all` will run the same batch update pipeline across all tracked archived skills, with clean updates proceeding and conflicts/skips summarized.

## R49

Session message ids: assistant `1133`, user `1134`

**Question / Decision Prompt**

Next decision: non-interactive conflicts for `repo update` / `repo update-all`.

Recommendation: **A**.

A. **Skip conflicts in `--no-input`; print exact rerun hints.**
Example: `skipped: code-review (archive differs from incoming; run x-skills repo update code-review --replace to accept incoming, or inspect in tui)`
Safe and scriptable.

B. **Fail the whole command on first conflict.**
Strict, but poor batch behavior.

C. **Require `--replace` globally to update any conflicting item.**
Useful, but dangerous for `update-all`.

I’d choose **A**, with `--replace` allowed for named `repo update NAME...`, and for `update-all` only if explicitly passed and confirmed. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Non-interactive update conflicts skip with exact hints. Named update can use `--replace`; update-all remains conservative unless explicitly confirmed.

## R50

Session message ids: assistant `1136`, user `1137`

**Question / Decision Prompt**

Next decision: do we need source metadata migration for existing Python-installed archives?

Recommendation: **A**.

A. **Read existing Python `.x-skills.json` as-is; no migration command.**
Since Go keeps v1 shape, existing metadata just works. Bad or missing metadata means untracked/legacy.

B. **Add `repo metadata repair` now.**
Useful eventually, but outside the parity slice.

C. **Auto-infer metadata from directory names.**
Too risky.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Existing Python v1 metadata is read as-is; no migration or inference in this slice.

## R51

Session message ids: assistant `1139`, user `1140`

**Question / Decision Prompt**

Next decision: should source metadata include audit data?

Recommendation: **B**.

A. **Persist last audit status in `.x-skills.json`.**
Faster display, but risk data goes stale and metadata becomes more than provenance.

B. **Do not persist audit data; fetch/cache per process.**
Keeps `.x-skills.json` as provenance only. Audit is advisory and time-sensitive.

C. **Persist only timestamped audit summary.**
More complexity without clear value.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Audit data is not persisted; source metadata remains provenance only.

## R52

Session message ids: assistant `1142`, user `1143`

**Question / Decision Prompt**

Next decision: should source metadata include installed display name if the user renames incoming archive?

Example: upstream skill `code-review`, user installs incoming as `code-review-vercel`.

Recommendation: **B**.

A. **Only store upstream source/path/commit; archive directory name is local name.**
Simple, but hard to show upstream name later if it differs.

B. **Store optional `upstream_name` in v1-compatible metadata when archive name differs.**
Existing v1 readers ignore extra fields; Go can show “upstream: code-review” in inspector. Useful for renamed incoming archives and updates.

C. **Do not allow renaming incoming for source-tracked skills.**
Conflicts with earlier decision.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Metadata will remain v1-compatible but may add optional `upstream_name` for renamed incoming archives. Existing Python-style required fields still define source identity.

## R53

Session message ids: assistant `1145`, user `1146`

**Question / Decision Prompt**

Next decision: rename existing archive and active links.

If user chooses “rename existing archive” during name conflict, and the existing archive is currently linked in active roots, what happens to those active links?

Recommendation: **B**.

A. **Rename archive only; leave active links broken, then Doctor can fix.**
Bad user experience.

B. **Rename archive and relink visible current project/global managed usages to the renamed archive.**
Best within current scope. Warn that other projects are outside visibility and may need Doctor later, same as Repo delete.

C. **Refuse rename existing if archive has any visible active usages.**
Safe but blocks a useful conflict resolution.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Renaming an existing archive must preserve visible managed usages by relinking them to the renamed archive, with the same current-project/global visibility caveat we use for repo delete.

## R54

Session message ids: assistant `1148`, user `1149`

**Question / Decision Prompt**

Next decision: if user renames incoming archive, what should immediate link names be?

Example: incoming upstream name is `code-review`, archived as `code-review-vercel`, then user chooses `.Ag` and `~Cl`.

Recommendation: **A**.

A. **Active link name equals final archive name.**
Links become `.agents/skills/code-review-vercel` and point to archive `code-review-vercel`. This keeps active managed status same-name and avoids aliases.

B. **Active link name equals upstream/original skill name.**
Human-friendly, but managed same-name logic breaks and creates alias complexity.

C. **Ask user separately for active link name.**
Too many prompts.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Final archive name becomes the active link name for immediate linking. That keeps the managed same-name invariant intact.

## R55

Session message ids: assistant `1151`, user `1152`

**Question / Decision Prompt**

Next decision: install/update implementation package boundaries.

Recommendation: **A**.

A. **Add source-focused packages under `internal/`:**
- `internal/sources` or `internal/remote`: skills.sh search, audit fetch, GitHub clone/discovery/update checks.
- `internal/actions`: install/update orchestration using repo/link primitives.
- `internal/repo`: metadata read/write and archive rename/delete/copy helpers.
CLI/TUI stay thin.

B. **Put all remote logic in `internal/actions`.**
Simpler at first, but actions will become too large and hard to test.

C. **Put remote logic in `internal/repo`.**
Wrong boundary: repo should manage local archive, not network discovery.

I’d choose **A**. Package name preference: `internal/sources` reads better for “GitHub/source metadata/audit” than `remote`. Do you agree with A and `internal/sources`?

**Answer**

A.

**Recorded Outcome**

We’ll use an `internal/sources` boundary for network/source discovery and keep actions as orchestration.

## R56

Session message ids: assistant `1154`, user `1155`

**Question / Decision Prompt**

Next decision: do we need a visual mockup now for the Install view and Repo update badges?

This is the point where visuals could help because row layout, pills, search box, owner filter, and inspector content are presentation-heavy.

Recommendation: **yes, use a text/character mockup**, but not a browser companion unless you want more than terminal-style mockups. We can produce actual colored-ish terminal mockups in the spec using the same character style as existing mockups.

Options:

A. **Text mockups in chat/spec only.**
Fast, enough for terminal UI.

B. **Use visual companion for richer mockup comparisons.**
More work/token cost; useful if you want to compare alternative layouts side by side.

C. **Skip mockups; describe layout prose only.**
I don’t recommend this because Install view is new.

I’d choose **A** unless you want the browser. A, B, or C?

**Answer**

B. Make sure to design correctly - not just wireframe.

**Recorded Outcome**

I’ll use the visual companion for the Install/Repo update UI because this is now a real presentation decision, not just key behavior. I’m reading the companion instructions, then I’ll build a concrete terminal-style mockup page rather than a wireframe.

## R57

Session message ids: assistant `1165`, user `1166`

**Question / Decision Prompt**

Next decision: audit/status pill symbols.

For update pills you already chose:
- up to date: green pill with tick
- update available: yellow pill with `🗘`
- missing upstream: red pill with `!`
- unknown: blue pill with `?`

For audit pills, recommendation:

A. **Text-first pills:** `risk safe`, `risk low`, `risk med`, `risk high`, `risk ?`
Best clarity. Color helps but text carries meaning, and inspector gives partner details.

B. **Symbol-first pills:** `✓ safe`, `! med`, `!! high`, `?`
More compact, but less consistent and more alarming.

C. **No audit pill on rows; inspector only.**
We already chose row pill, so this would reverse the visual direction.

I recommend **A**. Do you agree?

**Answer**

B: ✓ safe, ⚠ warn, ‼ risky
if unknown, do not show a pill.

**Recorded Outcome**

Audit rows will only show a pill when audit data has a meaningful result: `✓ safe`, `⚠ warn`, or `‼ risky`. Unknown/unavailable audit data stays out of the row and is explained in inspector when relevant.

## R58

Session message ids: assistant `1168`, user `1169`

**Question / Decision Prompt**

Next decision: how should partner audit risks map to those three row pills?

Upstream risks are `safe`, `low`, `medium`, `high`, `critical`, `unknown`, plus Socket alerts.

Recommendation: **A**.

A. **Worst-partner summary:**
- `✓ safe`: all available partner signals are safe/low and Socket has 0 alerts.
- `⚠ warn`: any medium risk, unknown partner among otherwise available data, or Socket alerts > 0.
- `‼ risky`: any high or critical risk.
- no pill: no audit data returned.
This is conservative and easy to explain.

B. **Provider priority: Gen/Athena decides main risk; Socket/Snyk details only.**
Less noisy, but arbitrary.

C. **Numeric scoring.**
Too much for this slice.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Audit summarization is worst-signal wins, with no pill when there is no audit data.

## R59

Session message ids: assistant `1171`, user `1172`

**Question / Decision Prompt**

Next decision: source-tracked update check implementation should verify upstream skill path existence. With `git` transport, there are two ways:

Recommendation: **A**.

A. **Clone/fetch lightweight temp checkout for update checks that report `update available`, then verify `skill_path/SKILL.md`.**
`ls-remote HEAD` first; if commit unchanged, no checkout. If commit changed, clone depth 1 and verify recorded path exists before marking `update available`; otherwise `missing upstream`. Accurate with acceptable cost.

B. **Use `git ls-remote` only; verify path only during actual update.**
Cheaper, but Repo can show update available for a deleted/moved skill.

C. **Use GitHub API tree lookup.**
Avoids clone for verification, but adds GitHub API/rate/auth complexity separate from git transport.

I recommend **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Update checks will use `ls-remote` first and only clone when the commit changed, so `missing upstream` is accurate without cloning every unchanged source.

## R60

Session message ids: assistant `1174`, user `1175`

**Question / Decision Prompt**

Next decision: background update concurrency.

Recommendation: **B**.

A. **Unlimited goroutines per tracked skill.**
Simple but can hammer GitHub and the machine.

B. **Bounded worker pool, e.g. 4 concurrent checks.**
Responsive without uncontrolled clones/network calls. Status can update as each result returns.

C. **Serial checks.**
Safest but slow for many tracked skills.

I’d choose **B**. Does 4 concurrent checks sound right for the spec?

**Answer**

B.

**Recorded Outcome**

Background update checks use bounded concurrency, defaulting to four workers.

## R61

Session message ids: assistant `1177`, user `1178`

**Question / Decision Prompt**

Next decision: remote install/update concurrency in CLI.

Recommendation: **A**.

A. **Sequential batch operations.**
Matches existing mutation behavior, keeps prompts/diffs deterministic, easier to reason about.

B. **Parallel clean updates, serial conflicts.**
Faster but harder to present and test.

C. **Bounded parallel all the way.**
Too much complexity.

I’d choose **A** for `repo update NAME...` and `repo update-all`. A, B, or C?

**Answer**

A.

**Recorded Outcome**

CLI update batches stay sequential like other mutation batches.

## R62

Session message ids: assistant `1180`, user `1181`

**Question / Decision Prompt**

Next decision: where should installed/updated result summaries appear in TUI?

Recommendation: **A**.

A. **Status bar for all-success, result modal for partial/error/conflict outcomes.**
Matches recent unlink/migrate behavior: successful simple mutations close modal and refresh; failures or mixed outcomes show a result modal.

B. **Always show result modal.**
More explicit, but you already disliked success popups for unlink/migrate.

C. **Never show result modal, status only.**
Too little feedback for batch update skips/errors.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

TUI install/update result behavior will match unlink/migrate: quiet success in the status bar, modal for mixed/error/conflict summaries.

## R63

Session message ids: assistant `1183`, user `1184`

**Question / Decision Prompt**

Next decision: after installing a remote skill with `i` and linking it to destinations, which view should the TUI remain on?

Recommendation: **A**.

A. **Stay in Install view, refresh result row badges and status.**
Good for browsing/installing another skill. User can switch to Active/Repo if desired.

B. **Switch to Repo view.**
Shows archive result, but interrupts discovery flow.

C. **Switch to Active view if linked.**
Confirms usage, but also interrupts discovery.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Install actions keep the user in Install view and update row badges/status in place.

## R64

Session message ids: assistant `1186`, user `1187`

**Question / Decision Prompt**

Next decision: CLI `search` default output after your changed install behavior.

Because `search --install` now links by default unless `--no-link`, the non-install search hint should change.

Recommendation: **A**.

A. **Show both use-now and archive-only examples.**
`Install and use: x-skills search go --install 1 -y`
`Archive only: x-skills search go --install 1 --no-link -y`
This is explicit about the new default.

B. **Only show default install command.**
Shorter but hides `--no-link`.

C. **No hint.**
Not helpful.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

CLI search output will explicitly show both install-and-use and archive-only forms.

## R65

Session message ids: assistant `1189`, user `1190`

**Question / Decision Prompt**

Next decision: how should `repo add-github` infer a skill path when not provided?

Python `_find_single_skill` requires exactly one skill directory in the repo. For parity-plus with search discovery, we may have path selection.

Recommendation: **B**.

A. **Strict Python parity: no path means exactly one skill or error.**
Simple, but less friendly.

B. **Interactive CLI/TUI can prompt when multiple skills are found; `--no-input` fails with listed paths.**
Consistent with our ambiguity policy. For `repo add-github owner/repo` in interactive mode, show discovered skills and ask one.

C. **Install all skills when no path is provided.**
Too broad.

I’d choose **B**. A, B, or C?

**Answer**

Previous question looks strange. This is two-step process - search and get names, then install by names. Not by numbers in search results. so search command searches - it doesn't install.

for current one B. might want parity with npx skills which uses `npx skills add owner/repo --all` seems better for our case, so we can make `add` install from github by default and `add --git GIT_PATH` for the different git provider.

**Recorded Outcome**

Correcting the CLI model: `search` is discovery only. Remote installation belongs to an `add` command, selected by skill name/source rather than search result number. That means the earlier CLI default-link ADR needs tightening so it talks about `add`, not `search --install`.

## R66

Session message ids: assistant `1192`, user `1193`

**Question / Decision Prompt**

That correction is important. Revised CLI shape so far:

- `x-skills search QUERY` only searches.
- It does not install by number.
- Install is a separate `add` flow.
- `add` links to project `.Ag` by default unless `--no-link`.
- `--to` can override/add destinations.
- `repo add-github` may be replaced or aliased by this clearer `add` command.

Next decision: exact `add` command grammar.

Recommendation: **B**, matching what you just suggested.

A. **Keep existing Python-style `repo add-github owner/repo [path]`.**
Parity, but wordy and too repo-internal.

B. **Add top-level `add SOURCE [SKILL_NAME...]`.**
Default source is GitHub shorthand:
`x-skills add owner/repo code-review`
`x-skills add owner/repo code-review commit-context --to ~cl`
`x-skills add owner/repo --all`
`x-skills add --git https://gitlab.com/owner/repo.git skill-name`
This is closer to `npx skills add`, easier to type, and keeps `repo` focused on local archive management.

C. **Use `install` as the top-level command.**
Clear word, but conflicts with our glossary nuance because install archives and may link; `add` is shorter and matches upstream.

I recommend **B**. Is `x-skills add SOURCE [SKILL_NAME...]` the command surface you want?

**Answer**

B. yes. this should be the ADR. We need to make sure our commands are in line with npx skills

**Recorded Outcome**

I’ll add an ADR for aligning the Go remote install command with `npx skills add`, while preserving x-skills archive/link semantics.

## R67

Session message ids: assistant `1195`, user `1196`

**Question / Decision Prompt**

Next decision: what should `search` output tell users to run?

Recommendation: **A**.

A. **Show `add` commands by name/source, not search result index.**
Example:
`Add and use: x-skills add vercel-labs/skills find-skills -y`
`Archive only: x-skills add vercel-labs/skills find-skills --no-link -y`
This matches the corrected model.

B. **Show package-like `owner/repo@skill` and let `add` parse it.**
Compact, but introduces another grammar.

C. **Show no command hints.**
Less useful.

I recommend **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Search output will point to `x-skills add SOURCE NAME`, not index-based install.

## R68

Session message ids: assistant `1198`, user `1199`

**Question / Decision Prompt**

Next decision: should `add SOURCE [SKILL_NAME...]` support the package shorthand `owner/repo@skill` too?

Recommendation: **A**.

A. **Yes, as convenience input.**
Support:
`x-skills add owner/repo@code-review`
and treat it as source `owner/repo`, skill `code-review`. This aligns with upstream and makes search result package strings copyable.

B. **No, only positional source + names.**
Simpler but less upstream-compatible.

C. **Only in TUI/internal, not CLI.**
No reason to split.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

`owner/repo@skill` shorthand is in scope for `add`.

## R69

Session message ids: assistant `1201`, user `1202`

**Question / Decision Prompt**

Next decision: `add --all`.

You mentioned `npx skills add owner/repo --all`.

Recommendation: **B**.

A. **Include `--all` in first slice.**
Installs every discovered skill from source, then links all of them to destinations by default. This can be a lot of mutations/conflicts.

B. **Include `--all`, but require confirmation unless `-y`; conflicts handled per skill; `--no-link` available.**
Still first-slice compatible if implemented through the same batch pipeline. `--no-input` without `--all` names fails with discovered-skill hints.

C. **Defer `--all`; require names.**
Safer, but less aligned with upstream.

Since you explicitly called out upstream parity, I recommend **B**. Do you want `--all` now?

**Answer**

B.

**Recorded Outcome**

`add --all` is in scope, confirmation-gated, using the same per-skill batch pipeline and destination rules.

## R70

Session message ids: assistant `1204`, user `1205`

**Question / Decision Prompt**

Next decision: top-level `add` support for Git providers.

You said `add` installs from GitHub by default and `add --git GIT_PATH` for different git provider.

Recommendation: **B**.

A. **Only GitHub shorthand/URLs in this slice; generic `--git` deferred.**
Focused, but not what you asked.

B. **Support GitHub shorthand/URLs plus `--git CLONE_URL` with explicit source label.**
Examples:
`x-skills add owner/repo code-review`
`x-skills add https://github.com/owner/repo/tree/main/skills/foo`
`x-skills add --git https://gitlab.com/owner/repo.git code-review`
For `--git`, update metadata source type can still be `git`, but update checks use `git ls-remote HEAD`. This is useful and mostly transport-compatible.

C. **Support many provider-specific parsers now: GitHub, GitLab, Bitbucket.**
Too much.

I recommend **B**. Do you want generic `--git` in this slice?

**Answer**

B.

**Recorded Outcome**

Generic `--git` source is in scope as a transport-compatible path, while shorthand parsing remains GitHub-first.

## R71

Session message ids: assistant `1207`, user `1208`

**Question / Decision Prompt**

Next decision: source metadata for generic `--git`.

Recommendation: **A**.

A. **Use source_type `git`, with source set to the clone URL or user-provided label, clone_url, commit, skill_path, optional upstream_name.**
Update checks work with `git ls-remote HEAD`; Install view search still only discovers `skills.sh`/GitHub.

B. **Pretend all generic git sources are GitHub metadata.**
Wrong.

C. **Do not track generic git updates.**
Less useful and inconsistent.

I’d choose **A**. Do you agree?

**Answer**

A.

**Recorded Outcome**

Generic git installs will be source-tracked as `source_type=git`, not forced into GitHub metadata.

## R72

Session message ids: assistant `1210`, user `1211`

**Question / Decision Prompt**

Next decision: should audit data run for generic `--git` sources?

Recommendation: **B**.

A. **Try audit for any source string.**
Might work for GitHub-like owner/repo only, but generic URLs likely won’t map.

B. **Audit only for GitHub `owner/repo` sources and `skills.sh` results.**
Cleaner. Generic git gets no audit pill and inspector says audit unavailable for non-GitHub source.

C. **Never audit `add`; only Install view search.**
Inconsistent.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Audit fetch is GitHub/skills.sh-source only; generic git sources remain unaudited.

## R73

Session message ids: assistant `1213`, user `1214`

**Question / Decision Prompt**

Next decision: TUI Install view add flow source.

Install view is driven by `skills.sh` search results, so it naturally installs GitHub-backed results. Should it also allow manually entering `owner/repo` or `--git` source inside TUI?

Recommendation: **B**.

A. **Yes, Install view has a manual source field too.**
More complete but adds source parser, name discovery, and manual Git errors into the first UI.

B. **No manual source in first TUI slice; Install view is `skills.sh` search only.**
CLI `add` handles manual sources. TUI can add manual source later if needed.

C. **Only allow GitHub owner/repo manual field, not generic git.**
Middle ground, still extra UI.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

TUI Install remains search-driven only; manual source adds are CLI-first for now.

## R74

Session message ids: assistant `1216`, user `1217`

**Question / Decision Prompt**

Next decision: `repo` command surface after adding top-level `add`.

What happens to Python names `repo add-github` and `repo add-url`?

Recommendation: **B**.

A. **Do not implement them in Go.**
Clean command surface, but README/Python parity may surprise users.

B. **Implement `repo add-github` as a deprecated alias/wrapper to `add`; do not implement `repo add-url`.**
Good migration path. Help text points users to `x-skills add`. URL remains deferred.

C. **Keep both `repo add-github` and top-level `add` forever.**
Too much surface.

I’d choose **B**. A, B, or C?

**Answer**

A. no need to follow the whole command names from python. Only logic - we need to rethink python commands in a more usable way that's easier for the user. Parity means ability to do what python version and npx skills do, but make the command surface easier for user.

**Recorded Outcome**

Understood: parity means capability parity, not command-name parity. We’ll not implement `repo add-github` or `repo add-url` in Go; `add` is the remote install surface, URL remains deferred. I’ll avoid calling omitted Python command names “missing” in the spec.

## R75

Session message ids: assistant `1219`, user `1220`

**Question / Decision Prompt**

Next decision: should `repo check` be a subcommand or should update statuses appear under `repo` list only when requested?

You specified `repo check`, so I’ll shape around that. Details:

Recommendation: **A**.

A. **`repo check [NAME...]` with optional names.**
No names checks all tracked archived skills. Names limit scope. Human output shows status pills/words and update suggestions. JSON output returns structured statuses.

B. **`repo check` always checks all; no names.**
Simpler, less flexible.

C. **`repo --check` flag.**
You already moved away from flags.

I’d choose **A**. Does `repo check [NAME...]` work?

**Answer**

A. if skill names are set, check only them. if no names set - check everything.

**Recorded Outcome**

`repo check [NAME...]` is confirmed: names narrow scope; no names checks all tracked archived skills.

## R76

Session message ids: assistant `1222`, user `1223`

**Question / Decision Prompt**

Next decision: `repo update-all` confirmation.

Since it can mutate many archives, should it always require confirmation unless `-y`?

Recommendation: **A**.

A. **Yes.**
`repo update-all` first checks tracked skills, prints planned updates/skips/conflicts, then asks confirmation. `-y` confirms clean updates but still skips conflicts unless `--replace` is explicitly present.

B. **No confirmation for clean updates.**
Faster but too broad.

C. **Confirmation only if more than one update.**
Inconsistent.

I recommend **A**. A, B, or C?

**Answer**

A. if -y not set, ask interactively for confirmation if TTY

**Recorded Outcome**

`repo update-all` will plan first and prompt interactively unless `-y`; in non-interactive mode without `-y`, it fails with a rerun hint.

## R77

Session message ids: assistant `1225`, user `1226`

**Question / Decision Prompt**

Next decision: JSON support for new commands.

Existing root has `--json` in Python, but Go currently may not have JSON wired broadly. For new remote commands, should JSON be in scope?

Recommendation: **B**.

A. **Full JSON output for search/check/update/add now.**
Useful but adds response schema work to every path.

B. **JSON for read-only commands now: `search` and `repo check`; mutation JSON deferred.**
Reasonable split. Search/check are data commands; add/update human summaries are enough for first slice.

C. **No JSON in this slice.**
Less parity.

I’d choose **B**. A, B, or C?

**Answer**

B. We want to allow agents to use the commands as well, and get data in a way that's readable for them. Ful JSON output goes into backlog.

**Recorded Outcome**

JSON is in scope for read-only `search` and `repo check`; full mutation JSON goes to backlog.

## R78

Session message ids: assistant `1228`, user `1229`

**Question / Decision Prompt**

Next decision: global JSON flag in Go.

For agents to use `search` and `repo check`, should Go add a root `--json` now, scoped to those read-only commands?

Recommendation: **A**.

A. **Add root `--json`; unsupported commands return a clear error for now.**
Consistent future surface: `x-skills --json search go`, `x-skills --json repo check`. Later commands can opt in.

B. **Add command-local `--json` only to search/check.**
Simpler implementation, less consistent.

C. **Use `--format json`.**
Flexible, but Python already had `--json`.

I’d choose **A**. A, B, or C?

**Answer**

A. --json and -j shorthand

**Recorded Outcome**

Go will add root `--json` / `-j`, supported initially for `search` and `repo check`; unsupported commands error clearly.

## R79

Session message ids: assistant `1231`, user `1232`

**Question / Decision Prompt**

Next decision: source discovery depth for cloned repos.

Python finds skills in common locations and can recursively search. Vercel `skills` scans standard paths and fallback recursion. For our Go `add`, how broad should discovery be?

Recommendation: **B**.

A. **Only root and `skills/*/SKILL.md`.**
Fast but misses many real repos.

B. **Standard locations plus bounded recursive fallback.**
Search root `SKILL.md`, `skills/*/SKILL.md`, `.agents/skills/*/SKILL.md`, and fallback recursive `SKILL.md` up to depth 5. Exclude `.git`, `node_modules`, vendor/build dirs. This matches practical upstream behavior without going unbounded.

C. **Unbounded recursive search.**
Too slow/risky.

I’d choose **B**. A, B, or C?

**Answer**

B.

**Recorded Outcome**

Skill discovery in cloned sources will use standard paths plus bounded recursive fallback, excluding common heavy/internal directories.

## R80

Session message ids: assistant `1234`, user `1235`

**Question / Decision Prompt**

Next decision: handling referenced files during install.

A skill directory may contain `SKILL.md` plus references/assets subdirs. Our archive copy should copy the whole skill directory. But if `SKILL.md` references files outside the skill directory, we currently won’t know.

Recommendation: **A**.

A. **Copy only the discovered skill directory.**
Matches current local archive model and Python behavior for repo paths. External references outside that directory are not included; preview can reveal broken references but no special resolver.

B. **Parse SKILL.md references and copy external files too.**
Complex and format-dependent.

C. **Reject skills with references.**
Wrong; most references live inside the skill dir.

I’d choose **A**. A, B, or C?

**Answer**

A.

**Recorded Outcome**

Remote installs copy exactly the discovered skill directory, including its internal files/subdirs, without chasing external references.

## R81

Session message ids: assistant `1237`, user `1238`

**Question / Decision Prompt**

Next decision: branch/ref support.

For GitHub tree URLs, the URL includes a branch/ref. For `owner/repo`, default branch is used by `git clone`. For `--git`, user may need a branch.

Recommendation: **B**.

A. **Ignore branch/ref and always clone default HEAD.**
Bad for tree URLs.

B. **Support `--ref REF` and parse simple tree URL ref.**
`add owner/repo skill --ref main` or tree URL uses its simple ref segment. Metadata stores installed commit, not ref. Update checks compare default HEAD unless we also store ref.

C. **Store and update against ref.**
More correct if user installs non-default branch.

Actually, if we support refs, update checks should use that ref too. So recommendation is **C**:
- support optional `--ref REF`;
- parse simple tree URL ref;
- metadata stores `ref` optionally;
- update check uses `git ls-remote <url> <ref-or-HEAD>`;
- clone uses `--branch <ref>` for branches/tags where possible.

Do you want C?

**Answer**

C. and let's stop grilling for now. Write ADRs, please.
