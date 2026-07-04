# Go Rewrite Design

Date: 2026-07-04

## Goal

Build an experimental Go implementation of `x-skills` on a new branch without
removing the current Python implementation. The first Go pass should be a
vertical slice that proves the architecture, CLI behavior, and TUI interaction
model on real local skill data.

The rewrite should target a durable local manager:

- one binary eventually;
- shared operation logic for CLI and TUI;
- explicit filesystem state and prompts;
- no Textual dependency;
- no silent decisions for ambiguous or destructive operations.

## First Slice

The first Go implementation is an operational slice, not full parity.

Included:

- `x-skills list`
- `x-skills repo`
- `x-skills link NAME...`
- `x-skills migrate NAME...`
- `x-skills unlink NAME...`
- `x-skills doctor`
- `x-skills doctor --fix`
- `x-skills tui`

Deferred:

- skills.sh remote search;
- GitHub install and update checks;
- final GitHub Releases installer;
- removal of Python/Textual code.

## Stack

Use Go with:

- Cobra for CLI command wiring, help, subcommands, and shared global flags;
- Bubble Tea for the TUI update loop;
- Bubbles for reusable list, viewport, text input, and related widgets;
- Lipgloss for terminal styling.

Avoid building the core logic inside Cobra commands or Bubble Tea models. CLI
and TUI should both call the same domain/action layer.

## Architecture

Proposed package layout:

```text
cmd/x-skills/          main package, CLI entrypoint
internal/config/       paths, env vars, cwd/project resolution
internal/skills/       SKILL.md parsing, descriptions, validation
internal/roots/        active roots: project/global x agents/claude/codex
internal/repo/         ~/.x-skills/skills listing, repo skill metadata
internal/fingerprint/  directory content fingerprints for grouping
internal/actions/      link, migrate, unlink, doctor fixes
internal/prompt/       yes/no, selection, no-input policy
internal/cli/          command wiring and output formatting
internal/tui/          Bubble Tea screens, wizard, styles
```

`internal/actions` should not write to stdout and should not ask questions. It
should return typed results, required confirmations, and ambiguity cases. CLI
commands turn those into prompts or actionable non-interactive errors. TUI turns
them into wizard steps and previews.

Python remains in the repository as the reference implementation until the Go
version reaches parity for the relevant command surface.

## Paths And Status

The Go implementation keeps the existing cwd-based mental model:

- project roots are resolved from the working directory;
- global roots are resolved from home directory defaults;
- `--project`, `--global`, and `--target agents|claude|codex` narrow active
  operations;
- tests may override roots.

Active roots:

- `./.agents/skills`
- `./.claude/skills`
- `./.codex/skills`
- `~/.agents/skills`
- `~/.claude/skills`
- `~/.codex/skills`

Statuses:

- `managed`: active symlink resolves to the same-named repo skill;
- `unmanaged`: real skill directory or symlink outside the repo;
- `broken`: symlink cannot resolve to a valid skill directory.

Broken diagnostics must explain the reason, including missing target, target not
a directory, and target missing `SKILL.md`.

Directory SHA fingerprints are used to merge identical active skill content in
the TUI. The SHA is internal state and should not appear in normal cards or
default human output. It may appear in inspector/debug output when useful.

## CLI Behavior

Keep the current command mental model and flags:

- `--project`
- `--global`
- `--target agents|claude|codex`
- `-y`, `--yes`
- `-n`, `--no`
- `--no-input`
- `--json`
- `--color auto|always|never`

`link`, `migrate`, and `unlink` accept multiple names and print a concise batch
summary. Earlier successful items are not rolled back if a later item fails.

Ambiguous operations must prompt in interactive terminals. In non-interactive
mode they fail with exact commands or flags that resolve the ambiguity.

`x-skills tui` replaces the Python `interactive` command. No pre-release alias
is required.

## Doctor And Fixes

`x-skills doctor` is the health-check command. It reports:

- broken symlinks and reasons;
- active symlinks that point somewhere unexpected;
- active entries whose same-name repo skill exists but is not used;
- real skill directories missing `SKILL.md`;
- invalid repo skills;
- missing or unwritable roots;
- missing external dependencies where relevant.

`x-skills doctor --fix` runs a remediation flow. With `-y`, these fixes are
automatic:

- broken symlink plus same-name repo skill exists: relink to the repo skill;
- broken symlink plus no same-name repo skill: remove the broken symlink.

`doctor --fix -y` should not automatically delete real unmanaged directories.
For symlinks outside the repo, relink automatically only when the entry is
classified as broken or clearly mislinked. Otherwise it remains unmanaged unless
the user explicitly chooses a fix.

In the TUI, Doctor/Fix does not require selection. If rows are selected,
selection narrows the scope; without selection it scans the visible working set.

## TUI UX

The target visual direction is a guided manager, not a dense table-only app.

Core screens:

- Active: current project and global active skills, grouped by content
  fingerprint where identical;
- Repo: local archive under `~/.x-skills/skills`;
- Doctor: diagnostics and suggested fixes;
- Action Wizard: link, migrate, unlink, and doctor fix flows.

The main UI should use cards or card-like rows with:

- skill name;
- status;
- path-like location chips;
- description or diagnostic reason.

Path-like labels are preferred because they are short and exact:

- `./.agents`
- `./.claude`
- `./.codex`
- `~/.agents`
- `~/.claude`
- `~/.codex`

Full paths, symlink targets, and internal fingerprint values belong in the
inspector/detail area, not in the main card.

The TUI should avoid making `p/g/1/2/3` the primary interaction model. Keyboard
shortcuts are acceptable, but the visible UI must show current destination and
action state in readable labels. The wizard must preview filesystem changes
before mutation.

## Prompt Policy

The application helps the user choose, but does not choose silently when the
operation is ambiguous or destructive.

Prompt or wizard-required cases:

- multiple active locations match a name;
- destination scope or target is unspecified for a mutating operation;
- unmanaged directory unlink requires migrate/delete/cancel choice;
- replacing repo or active content;
- destructive fixes not covered by the `doctor --fix -y` safe-fix policy.

`-y` and `-n` answer yes/no confirmations. They do not choose among ambiguous
locations.

## Installation Strategy

Use a hybrid strategy:

1. During the rewrite branch and early testing, support `go install` and
   `go run ./cmd/x-skills`.
2. Design the public one-liner for GitHub Releases and prebuilt binaries.
3. Keep the current Python installer on `main` until the Go binary becomes the
   primary implementation.
4. After parity, replace the one-liner with a release installer and keep
   `go install` as a development fallback.

The final user-facing installer should not require Python, uv, Node, or Go when
prebuilt binaries are available.

## Testing Strategy

Port the existing Python fixture scenarios into Go tests:

- active listing with managed, unmanaged, and broken statuses;
- broken symlink reasons;
- repo listing and used/unused state;
- link/migrate/unlink batch behavior;
- unmanaged unlink semantics;
- linked active group detection;
- same-name separate copies stay separate;
- non-interactive ambiguity errors;
- doctor diagnostics;
- `doctor --fix` safe fixes.

Core actions should be tested against temporary directories without shelling out.
CLI tests may run the compiled binary or `go run ./cmd/x-skills`. Human output
golden tests should focus on semantic content and avoid brittle spacing/color
assertions.

TUI tests should focus on Bubble Tea model/update behavior:

- selection;
- screen switching;
- wizard transitions;
- destination choice state;
- action previews;
- doctor fix flow.

Full-screen terminal rendering can start with smoke tests and be expanded after
the core interaction model settles.

## Migration Plan

1. Add Go module and operation slice beside Python.
2. Keep README clear that the Go branch is experimental.
3. Use Python tests and behavior as reference during porting.
4. Make the Go binary primary locally once the operation slice passes tests.
5. Add remote search, GitHub install, metadata update checks, and release
   installer in later passes.
6. Remove Python/Textual only after Go reaches accepted parity.
