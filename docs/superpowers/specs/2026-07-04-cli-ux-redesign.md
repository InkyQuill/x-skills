# x-skills CLI UX Redesign

## Goal

Make `x-skills` answer two user questions clearly:

- `x-skills list`: What skills am I currently working with in this directory?
- `x-skills repo`: What skills do I have saved in `~/.x-skills`?

The CLI should help the user discover choices, but it must not make ambiguous or destructive decisions silently.

## Mental Model

`x-skills` is cwd-based. The current project is `Path.cwd()` unless `--project-root` is passed for tests or advanced use. To inspect another project, the user should `cd` into it, matching how agent tools resolve local instructions.

There are two kinds of skill locations:

- Active roots: currently usable skills for the current project or global agent configuration.
- Repo root: archived skills in `~/.x-skills/skills`.

Active roots are split by scope and target:

- Project agents: `<cwd>/.agents/skills`
- Project claude: `<cwd>/.claude/skills`
- Project codex: `<cwd>/.codex/skills`
- Global agents: `~/.agents/skills`
- Global claude: `~/.claude/skills`
- Global codex: `~/.codex/skills`

## Managed Status

An active skill is `managed` when it is a symlink to the same-named skill in `~/.x-skills/skills`.

An active skill is `unmanaged` when it is a real skill directory or points somewhere outside the repo.

An active skill is `broken` when it is a symlink whose target cannot be resolved to a valid skill directory.

The CLI should show managed status anywhere it lists active skills.

## Command Surface

### `x-skills list`

Shows the active working set for the current directory plus global roots, across all targets. It replaces the current split between `list`, `linked`, and `status`.

Default human output groups by scope and target:

```text
PROJECT codex  ./.codex/skills
  next-best-practices   managed     Next.js best practices...
  svelte-coder          unmanaged   Implement features in Svelte 5...

GLOBAL agents  ~/.agents/skills
  opentui-react         managed     OpenTUI with React...
```

Empty groups are omitted by default. If no active skills are found, print:

```text
No active skills found for this project or global roots.
```

Useful options:

- `--target agents|claude|codex`: limit active roots by target.
- `--global`: show only global roots.
- `--project`: show only current project roots.
- `--all`: include empty groups.
- `--json`: machine-readable output.

`--global` and `--project` are mutually exclusive. With neither flag, commands that list active skills include both scopes.

### `x-skills repo`

Shows skills archived in `~/.x-skills/skills` with descriptions from `SKILL.md` frontmatter. This command answers "What do I have saved?"

Default output:

```text
next-best-practices   Next.js best practices - file conventions...
opentui-react         OpenTUI with React - components, hooks...
typescript-expert     TypeScript and JavaScript expert...
```

Useful options:

- `--used`: show only repo skills active in the current project or globals.
- `--unused`: show only repo skills not active in the current project or globals.
- `--json`: machine-readable output.

### `x-skills link NAME`

Links a skill from the repo into an active root.

Examples:

```bash
x-skills link svelte-coder
x-skills link svelte-coder --target codex
x-skills link svelte-coder --global --target claude
```

If `--target` or scope is ambiguous, the CLI should prompt in interactive terminals. It should not infer the target from the skill name, description, or installed agents.

`--global` and `--project` are mutually exclusive. With neither flag, `link` should prompt for the destination scope and target unless future configuration defines a default.

If a destination already exists, show the existing location and ask before replacing only when replacement is explicitly supported by the command. The default should be conservative: fail with an explanation unless the user passes a replace flag or confirms an interactive replace prompt.

### `x-skills migrate NAME`

Moves an unmanaged active skill into `~/.x-skills/skills` and links it back in the selected active root.

Resolution rules:

1. Search active roots for `NAME`, respecting explicit `--target`, `--global`, or `--project` filters.
2. If exactly one unmanaged directory is found, ask for confirmation, then migrate it.
3. If multiple matches are found, prompt the user to choose a concrete location.
4. If a matching managed skill is found, report that it is already managed.
5. If the repo already contains `NAME`, ask before replacing. In non-interactive mode, fail and show the explicit command needed to replace.

Example ambiguous prompt:

```text
Found multiple active skills named "svelte-coder":

  1. project codex   ./.codex/skills/svelte-coder   unmanaged
  2. global agents   ~/.agents/skills/svelte-coder  managed

Select skill to migrate [1-2]:
```

### `x-skills unlink NAME`

Removes an active skill from the selected active root.

Rules:

- Managed symlink: confirm, then remove the symlink.
- Broken symlink: confirm, then remove the symlink.
- Unmanaged directory: ask whether to migrate it into the repo first. Do not silently delete directories.
- Multiple matches: prompt for the concrete location.

For unmanaged directories, `-y` chooses the safer migrate-first path when possible. `-n` cancels.

### `x-skills repo add-github REPO_OR_URL [SKILL_PATH]`

Installs one skill from a GitHub repository into `~/.x-skills/skills`.

Examples:

```bash
x-skills repo add-github owner/repo path/to/skill
x-skills repo add-github https://github.com/owner/repo/tree/main/skills/foo
```

If the repository contains more than one `SKILL.md`, require `SKILL_PATH`.

### `x-skills repo add-url URL`

Installs one skill from a `.zip`, `.tar`, `.tar.gz`, or direct `SKILL.md` URL.

Direct `SKILL.md` files must contain a `name:` frontmatter field.

### `x-skills repo remove NAME`

Removes a skill from `~/.x-skills/skills`.

This always requires confirmation in interactive mode. In non-interactive mode it fails unless `-y` is passed. Before removal, show whether the skill is currently active anywhere in the current project or global roots.

### `x-skills doctor`

Checks configured roots and dependencies:

- repo root path and writability;
- active root paths and writability when they exist;
- `git` availability for GitHub installs;
- `uv` availability for the documented one-liner install flow.

## Prompt Policy

The application should help, but not decide for the user.

Prompt in interactive terminals for:

- ambiguous active locations;
- destination replacement;
- repo replacement;
- repo removal;
- unlinking managed or broken links;
- migrating or removing unmanaged directories.

Do not prompt when:

- stdin or stdout is not a TTY;
- `--no-input` is passed;
- `CI=1` is set.

In non-interactive mode, fail with a clear message and exact commands or flags that would resolve the ambiguity.

Global prompt flags:

- `-y`, `--yes`: answer yes to yes/no confirmation prompts.
- `-n`, `--no`: answer no to yes/no confirmation prompts.
- `--no-input`: never prompt; fail when input would be required.
- `--json`: machine-readable output for commands that produce data.

`-y` and `-n` do not choose among ambiguous locations. They only answer yes/no confirmations. Choosing a location requires an interactive selection or explicit flags such as `--target codex --global`.

`-y` and `-n` are mutually exclusive.

## Removed Pre-Release Commands

The current project is not released yet, so no legacy aliases are required.

Remove or replace these commands:

- `linked`: replaced by `list`.
- `status`: folded into `list`.
- `archive`: replaced by `migrate` for active skills and `repo add-*` for external installs.
- `install-github`: replaced by `repo add-github`.
- `install-url`: replaced by `repo add-url`.

## Error Style

Errors should be actionable and include concrete commands when possible.

Example:

```text
x-skills: multiple active skills named "svelte-coder"; choose one:
  x-skills migrate svelte-coder --target codex --project
  x-skills migrate svelte-coder --target agents --global
```

## Testing Requirements

Tests should cover:

- `list` groups project and global skills across all targets.
- `list` marks skills as managed, unmanaged, and broken.
- `repo` prints descriptions from frontmatter.
- `migrate` prompts on ambiguity.
- `migrate` fails non-interactively on ambiguity.
- `unlink` prompts before unmanaged directory removal and prefers migrate-first under `-y`.
- `repo remove` refuses non-interactive removal without `-y`.
- `-y` and `-n` are mutually exclusive.
- `repo add-github` and `repo add-url` preserve existing install behavior under the new command surface.
