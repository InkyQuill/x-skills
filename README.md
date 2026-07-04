# x-skills

`x-skills` keeps reusable agent skills in `~/.x-skills/skills` and links only the
skills you currently need into project or global agent directories.

The CLI is working-directory based: `x-skills list` inspects the current project
plus global skill roots. To inspect another project, `cd` there first.

## Install

Requires `uv` and `git`. Install `uv` from
<https://docs.astral.sh/uv/> before running the one-liner.

```bash
curl -fsSL https://raw.githubusercontent.com/InkyQuill/x-skills/main/install.sh | sh
```

The installer checks for `git` and `uv`, then installs the CLI from
`https://github.com/InkyQuill/x-skills.git` with `uv tool install`.

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

x-skills repo add-github owner/repo path/to/skill
x-skills repo add-github https://github.com/owner/repo/tree/main/skills/foo
x-skills repo add-url https://example.com/skill.zip
x-skills repo remove old-skill

x-skills interactive
x-skills doctor
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
- `--no-input`: never prompt;
- `--json`: machine-readable output for data commands.

`-y` and `-n` do not choose among ambiguous locations. Use explicit flags such as
`--target codex --project` or answer the interactive selection prompt.

For unmanaged active directories, `unlink` asks whether to migrate first, remove
the active directory without migration, or cancel. For automation, use
`--delete-unmanaged -y` to remove an unmanaged active directory without adding it
to the repo.

## Interactive Mode

`x-skills interactive` opens a Textual-based manager for longer maintenance
sessions. The first version shows active skills with status, path, and details,
and supports refresh and quit. It uses the same discovery logic as the CLI
commands; mutation actions will continue to require explicit confirmation as the
TUI grows.

## Install Sources

`x-skills repo add-github` clones a GitHub repository and copies one skill into
the repo. Pass `path/to/skill` when the repository contains more than one
`SKILL.md`.

`x-skills repo add-url` accepts:

- a `.zip` archive containing exactly one skill;
- a `.tar`/`.tar.gz` archive containing exactly one skill;
- a direct `SKILL.md` URL with a `name:` frontmatter field.

Install community skills only from trusted sources. Skills can contain scripts
and instructions that affect future agent behavior.

## Development

```bash
uv run pytest
uv run ruff check .
uv run ruff format --check .
```
