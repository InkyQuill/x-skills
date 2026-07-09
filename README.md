# x-skills

`x-skills` keeps reusable agent skills in `~/.x-skills/skills` and links only the
skills you currently need into project or global agent directories.

The CLI is working-directory based: `x-skills list` inspects the current project
plus global skill roots. To inspect another project, `cd` there first.

## Go Rewrite

This branch is the Go implementation of `x-skills`, replacing the earlier
Python/Textual prototype (Python source remains in `src/x_skills` only as a
historical reference; it is not the active implementation).

```bash
go run ./cmd/x-skills list
go run ./cmd/x-skills repo
go run ./cmd/x-skills doctor
go run ./cmd/x-skills doctor --fix -y
go run ./cmd/x-skills tui
```

Shipped today: cwd-based active scanning, local repo listing, `link`,
`migrate`, `unlink`, `doctor` (with `--fix`), and the `x-skills tui` Bubble Tea
guided manager (Active, Repo, and Doctor pages). Active rows are merged by
directory SHA fingerprint so identical linked copies appear as one item while
changed copies remain separate; the fingerprint is internal and not shown in
the UI.

Not yet implemented (designed, tracked in `docs/adr/` and
`docs/superpowers/specs/2026-07-06-go-tui-install-and-repo-updates-design.md`):
remote `skills.sh` search/install (`add`), Repo update checks, and the TUI
Install page. See that design doc and the backlog for current status before
relying on any of it.

## Usage

```bash
x-skills list
x-skills list --target codex
x-skills list --project
x-skills list --global

x-skills repo
x-skills repo --used
x-skills repo --unused

x-skills link svelte-coder --target codex --project
x-skills link typescript-expert --target codex --global
x-skills link svelte-coder typescript-expert --target codex --project

x-skills migrate next-best-practices --target codex --project
x-skills unlink opentui-react --target agents --global
x-skills unlink supergoal --target claude --global --delete-unmanaged

x-skills tui
x-skills doctor
x-skills doctor --fix -y
```

`x-skills list` answers "what am I currently working with?" It shows active
skills from the current project and global roots across `agents`, `claude`, and
`codex`, grouped by scope and target. Each active skill is marked:

- `managed`: symlinked to the same-named repo skill in `~/.x-skills/skills`;
- `unmanaged`: a real skill directory or symlink outside the repo;
- `broken`: a symlink whose target is not a valid skill directory.

Human output is colorized by default when stdout is a terminal. Use
`--color never` to disable color, or `--color always` to force it through pipes.
Broken skills show the reason, such as a missing symlink target or a target
directory without `SKILL.md`.

`x-skills repo` answers "what do I have saved?" It lists archived skills in
`~/.x-skills/skills` with descriptions from `SKILL.md` frontmatter.

`link`, `migrate`, `unlink`, and `repo remove` accept multiple skill names and
print a summary for batch runs. Batch operations run in order and do not roll
back earlier successful changes if a later item fails.

When the same skill appears in multiple active roots, `x-skills` checks whether
those entries resolve to the same physical skill. Linked setups are shown as a
group and can be managed together; separate same-name copies are never merged
automatically.

## Paths

Default repo root:

- `~/.x-skills/skills`

Default project active roots:

- `<cwd>/.agents/skills`
- `<cwd>/.claude/skills`
- `<cwd>/.codex/skills`

Default global active roots:

- `~/.agents/skills`
- `~/.claude/skills`
- `~/.codex/skills`

Use `--target agents`, `--target claude`, or `--target codex` to narrow active
commands. Use `--project` or `--global` to select a scope when needed.

## Prompts

`x-skills` helps you choose, but it does not silently decide ambiguous or
destructive actions.

It prompts for ambiguous active locations, destination choices, replacements,
repo removal, unlinking, and unmanaged directory migration. In non-interactive
mode it fails with actionable commands instead of hanging.

Global prompt flags:

- `-y`, `--yes`: answer yes to yes/no confirmations;
- `-n`, `--no`: answer no to yes/no confirmations;
- `--no-input`: never prompt.

`-y` and `-n` do not choose among ambiguous locations. Use explicit flags such as
`--target codex --project` or answer the interactive selection prompt.

For unmanaged active directories, `unlink` asks whether to migrate first, remove
the active directory without migration, or cancel. For automation, use
`--delete-unmanaged -y` to remove an unmanaged active directory without adding it
to the repo.

## TUI Mode

`x-skills tui` opens the Bubble Tea maintenance manager for longer maintenance
sessions. The TUI has Active, Repo, Doctor, and Install pages: press `A` for
Active, `R` for Repo, `D` for Doctor, and `I` for Install. Refresh is `ctrl+r`.

Use Install to search `skills.sh`, preview remote `SKILL.md`, archive a skill,
or install and link it into the current project. Manual generic Git installs
remain CLI-first through `x-skills add --git`.

Use Active to inspect current project/global skills, preview `SKILL.md`, migrate
unmanaged directories into the archive, and unlink active copies. Use Repo to
preview archived skills, link them into a selected destination, unlink visible
current usages, or delete archives after visible usages are removed. Use Doctor
to review and fix current issues.

## Design Decisions

Significant, non-obvious decisions are recorded as ADRs in `docs/adr/`. When a
choice needs weighing trade-offs (not just implementing an agreed spec), write
or update an ADR rather than only changing code/docs.

## Development

```bash
go build ./...
go test ./...
```
