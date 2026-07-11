# Go TUI Views And Mockups Spec

This document provides character-based terminal mockups and behavioral definitions for the `x-skills tui` Go application. It is the visual companion to [Go TUI Full Parity Design](./2026-07-06-go-tui-full-parity-design.md).

These mockups are normative for layout intent, information hierarchy, key visibility, and modal content. They are not pixel-perfect snapshots: implementation may adjust spacing to fit terminal width, Lip Gloss borders, and viewport constraints.

## Active View

The Active view answers: "What skills are currently active across my project and global roots?"

### Wide Layout (`>= 100` columns)

```text
◆ x-skills  A Active  R Repo  D Doctor       scope: .Ag .Cl .Cd ~Ag ~Cl ~Cd
┌ Active skills ─────────────────────────────────┐ ┌ Inspector ──────────────┐
│ › □ zen-of-go       ● .Ag ◆ ~Cl  ◆ unmanaged ×2│ │ ◇ zen-of-go             │
│   □ opentui-react   ● .Cd       ✓ managed      │ │ aliases                 │
│   ■ svelte-coder    ◆ ~Ag       ▲ broken       │ │  go-style               │
│   □ prompt-master   ● .Cl       ◆ local        │ │ repo                    │
│                                                 │ │ ▲ differs from archive  │
│                                                 │ │ next                    │
│                                                 │ │ migrate opens diff      │
└─────────────────────────────────────────────────┘ └─────────────────────────┘
relinked zen-of-go to existing archive
enter details  / filter  p preview  m migrate  u unlink  ^R refresh  ? help  q quit
```

### Narrow Layout (`< 100` columns)

The inspector is collapsed out. Full details remain available via `enter`.

```text
◆ x-skills  A Active  R Repo  D Doctor                         scope: all roots
┌ Active skills ─────────────────────────────────────────────────────┐
│ › □ zen-of-go       ● .Ag ◆ ~Cl  ◆ unmanaged ×2                   │
│   □ opentui-react   ● .Cd       ✓ managed                         │
│   ■ svelte-coder    ◆ ~Ag       ▲ broken                          │
└────────────────────────────────────────────────────────────────────┘
enter details  / filter  p preview  m migrate  u unlink  ^R refresh  ? help  q quit
```

## Repo View

The Repo view answers: "What skills are saved in the archive, and where are they used in the current working set?"

### Wide Layout (`>= 100` columns)

```text
◆ x-skills  A Active  R Repo  D Doctor       scope: .Ag .Cl .Cd ~Ag ~Cl ~Cd
┌ Repo skills ───────────────────────────────────┐ ┌ Inspector ──────────────┐
│ › □ zen-of-go         Go style guide    .Ag ~Cl│ │ ◇ zen-of-go             │
│   □ opentui-react     React components  .Cd    │ │ description             │
│   ■ prompt-master     System prompts    ~Cd    │ │ Go style guide          │
│   □ svelte-coder      Svelte helper            │ │ path                    │
│                                                  │ │ ~/.x-skills/skills/...  │
│                                                  │ │ usages                  │
│                                                  │ │ .Ag  .agents/skills/... │
│                                                  │ │ ~Cl  ~/.claude/skills...│
└──────────────────────────────────────────────────┘ └─────────────────────────┘
enter details  / filter  p preview  l link  u unlink  d delete  ^R refresh  ? help  q quit
```

### Narrow Layout (`< 100` columns)

```text
◆ x-skills  A Active  R Repo  D Doctor                         scope: all roots
┌ Repo skills ───────────────────────────────────────────────────────┐
│ › □ zen-of-go         Go style guide                       .Ag ~Cl │
│   □ opentui-react     React components                     .Cd     │
│   ■ prompt-master     System prompts                       ~Cd     │
└────────────────────────────────────────────────────────────────────┘
enter details  / filter  p preview  l link  u unlink  d delete  ^R refresh  ? help  q quit
```

## Doctor View

Doctor is a separate issue-oriented view. It shows all current Doctor issues and fixes all safe current issues after confirmation.

```text
◆ x-skills  A Active  R Repo  D Doctor       scope: .Ag .Cl .Cd ~Ag ~Cl ~Cd
┌ Doctor issues ───────────────────────────────┐ ┌ Inspector ──────────────┐
│ › ▲ broken-link     zen-of-go in .Ag         │ │ ◇ broken-link           │
│   ▲ dead-link       prompt-master in ~Cl     │ │ path                    │
│                                              │ │ .agents/skills/zen-o... │
│                                              │ │ target                  │
│                                              │ │ ~/.x-skills/skills/...  │
│                                              │ │ reason                  │
│                                              │ │ symlink target missing  │
│                                              │ │ fix                     │
│                                              │ │ relink symlink to repo  │
└──────────────────────────────────────────────┘ └─────────────────────────┘
enter details  f fix all  ^R refresh  ? help  q quit
```

## Operational Detail Modal

`enter` opens details for the cursor row in every top-level view.

```text
╭───────────────────── Detail: zen-of-go (Active) ─────────────────────╮
│ Canonical name: zen-of-go                                            │
│ Status: unmanaged                                                    │
│ Aliases: go-style                                                    │
│                                                                      │
│ Active members                                                       │
│   ● .Ag  .agents/skills/zen-of-go  -> (directory copy)               │
│   ◆ ~Cl  ~/.claude/skills/go-style -> /work/api/.agents/skills/...   │
│                                                                      │
│ Repo                                                                 │
│   ~/.x-skills/skills/zen-of-go exists, content differs               │
│                                                                      │
│ Debug                                                                │
│   fingerprint: 5d7f963a2dd3fdb554de6cd907bbe6f833f23503              │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ scroll   esc close   q close
```

## Active Migrate Modal

Migration starts with a workbench confirmation. Same-SHA archive collisions relink without opening a diff. Divergent collisions open the fullscreen archive conflict modal.

```text
╭──────────────────────────── Migrate active skills ───────────────────╮
│ Targets                                                              │
│   › zen-of-go       ● .Ag ◆ ~Cl  unmanaged ×2                        │
│                                                                      │
│ Plan                                                                 │
│   1. Compare active content with ~/.x-skills/skills/zen-of-go        │
│   2. If archive is identical, relink active copies                   │
│   3. If archive differs, review full-file diff before choosing       │
│                                                                      │
│ [ Apply ]   Cancel                                                   │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ move   enter choose   y/n select   esc cancel
```

## Archive Conflict Diff Modal

Divergent archive conflicts always use this fullscreen modal. The decision applies to the whole skill directory.

```text
╭──────────────────────── Archive conflict: zen-of-go ────────────────────────╮
│ Decision applies to the whole skill directory.                              │
│                                                                              │
│ Files                         │ SKILL.md                         full file   │
│───────────────────────────────┼──────────────────────────────────────────────│
│ › ± SKILL.md                  │  1  ---                                      │
│   + references/rules.md       │  2  name: zen-of-go                          │
│   - references/old.md         │ -3  description: Go style guide              │
│   ! assets/logo.png           │ +3  description: Simple Go rules             │
│                               │  4  ---                                      │
│                               │  5                                           │
│                               │  6  # Zen of Go                              │
│                               │ -7  Prefer minimal APIs.                     │
│                               │ +7  Prefer tiny APIs.                        │
╰──────────────────────────────────────────────────────────────────────────────╯
↑↓ scroll   tab focus   k keep archive   l save active   esc cancel   q close
```

Binary/unreadable file viewer state:

```text
╭──────────────────────── Archive conflict: zen-of-go ────────────────────────╮
│ Files                         │ assets/logo.png                              │
│───────────────────────────────┼──────────────────────────────────────────────│
│   ± SKILL.md                  │ Binary file                                  │
│   + references/rules.md       │ archive: 12.4 KiB  sha256: 4bc1a0d9f2c1     │
│   - references/old.md         │ active:  18.8 KiB  sha256: 8ff9ed13a422     │
│ › ! assets/logo.png           │                                              │
│                               │ No text diff is available.                   │
╰──────────────────────────────────────────────────────────────────────────────╯
↑↓ scroll   tab focus   k keep archive   l save active   esc cancel   q close
```

## Active Unlink Modal

Mixed selections are grouped by operation category. Unmanaged directory handling uses one global choice for all unmanaged entries.

```text
╭──────────────────────────── Unlink active skills ───────────────────╮
│ Managed links                                                       │
│   ✓ opentui-react  ● .Cd  remove symlink only                       │
│                                                                      │
│ Broken links                                                        │
│   ▲ svelte-coder   ◆ ~Ag  remove broken symlink                     │
│                                                                      │
│ Unmanaged directories                                               │
│   ◆ zen-of-go      ● .Ag ◆ ~Cl ×2                                   │
│                                                                      │
│ Unmanaged choice                                                    │
│   › Migrate to repo, then unlink active copies                      │
│     Delete active copies without archiving                          │
│     Cancel                                                          │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ choose   enter apply   esc cancel   q close
```

## Repo Link Modal

Repo link selects a destination root. Existing destinations block the operation and should be shown before apply.

```text
╭──────────────────────────── Link repo skill ────────────────────────╮
│ Skill                                                                │
│   zen-of-go                                                          │
│                                                                      │
│ Destination                                                          │
│   scope   › project    global                                        │
│   target  › .Ag        .Cl        .Cd                                │
│                                                                      │
│ Will create                                                          │
│   .agents/skills/zen-of-go -> ~/.x-skills/skills/zen-of-go           │
│                                                                      │
│ [ Link ]   Cancel                                                    │
╰──────────────────────────────────────────────────────────────────────╯
←→ change option   tab field   enter choose   esc cancel
```

## Repo Unlink Usages Modal

All current project/global usages are selected by default.

```text
╭─────────────────────── Unlink usages: zen-of-go ─────────────────────╮
│ Select current usages to remove.                                     │
│                                                                      │
│   › ■ .Ag  .agents/skills/zen-of-go                                  │
│     ■ ~Cl  ~/.claude/skills/zen-of-go                                │
│                                                                      │
│ [ Unlink selected ]   Cancel                                         │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ move   space toggle   enter choose   esc cancel
```

## Repo Delete Modal

Deleting an archive with active usages requires unlinking the visible current usages first. The modal must state the current-project/global visibility limit.

```text
╭────────────────────────── Delete archive: zen-of-go ─────────────────╮
│ This archive is used in the current working set.                     │
│                                                                      │
│ Visible usages                                                       │
│   ■ .Ag  .agents/skills/zen-of-go                                    │
│   ■ ~Cl  ~/.claude/skills/zen-of-go                                  │
│                                                                      │
│ Scope limit                                                          │
│   Only current project roots and global roots are known. Other       │
│   projects may need `x-skills doctor` later.                         │
│                                                                      │
│   › Cancel                                                           │
│     Unlink visible usages, then delete archive                       │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ choose   enter apply   esc cancel   q close
```

## Doctor Fix Confirmation

```text
╭─────────────────────────────── Confirm ──────────────────────────────╮
│ Apply 4 Doctor fixes?                                                │
│                                                                      │
│   - 3 relink broken symlinks                                         │
│   - 1 remove dead symlink                                            │
│                                                                      │
│ [ Apply ]   Cancel                                                   │
╰──────────────────────────────────────────────────────────────────────╯
←→ move cursor   enter choose   y/n select   esc cancel
```

## Preview Modal

Preview uses Glamour-rendered Markdown by default. `r` toggles raw/rendered.

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
↑↓ scroll   r raw   esc close   q close
```

## Result Modal

```text
╭─────────────────────────── Migration Results ────────────────────────╮
│ Batch migration completed.                                           │
│                                                                      │
│   ✓ svelte-coder   migrated successfully                             │
│   ✓ opentui-react  relinked to existing identical archive            │
│   ⚠ zen-of-go      skipped, conflict cancelled                       │
│                                                                      │
│ Total: 2 succeeded, 1 skipped, 0 failed.                             │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ scroll   enter close   esc close   q close
```

## Help Modal

```text
╭─────────────────────────────── Help ─────────────────────────────────╮
│ Keyboard Shortcuts (Active View)                                     │
│   A        switch to Active view                                     │
│   R        switch to Repo view                                       │
│   D        switch to Doctor view                                     │
│   I        switch to Install view                                    │
│   enter    view row details                                          │
│   /        enter local filter mode                                   │
│   space    toggle Active/Repo row selection                          │
│   p        preview SKILL.md                                          │
│   m        migrate active skill                                      │
│   u        unlink active skill                                       │
│   ^R       rescan filesystem                                         │
│   ?        show this help screen                                     │
│   q        quit application                                          │
│                                                                      │
│ Symbol Legend                                                        │
│   ›  cursor position                                                 │
│   □  unselected item                                                 │
│   ■  selected item                                                   │
│   ×N group count badge                                               │
│                                                                      │
│ Root Chip Legend                                                     │
│   .Ag  project agents                                                │
│   .Cl  project claude                                                │
│   .Cd  project codex                                                 │
│   ~Ag  global agents                                                 │
│   ~Cl  global claude                                                 │
│   ~Cd  global codex                                                  │
╰──────────────────────────────────────────────────────────────────────╯
↑↓ scroll   esc close   q close
```

## Filter Input State

```text
◆ x-skills  A Active  R Repo  D Doctor       scope: .Ag .Cl .Cd ~Ag ~Cl ~Cd
┌ Active skills ──────────────────────────────────┐ ┌ Inspector ──────────────┐
│ › □ zen-of-go       ● .Ag ◆ ~Cl  ◆ unmanaged ×2 │ │ ◇ zen-of-go             │
│   □ opentui-react   ● .Cd       ✓ managed       │ │ aliases                 │
│                                                 │ │ zen-of-go               │
└─────────────────────────────────────────────────┘ └─────────────────────────┘
/ filter: go_
enter accept   esc clear/exit
```
