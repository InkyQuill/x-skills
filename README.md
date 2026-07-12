# x-skills

`x-skills` keeps reusable agent skills in `~/.x-skills/skills` and links only the
skills you currently need into project or global agent directories.

The CLI is working-directory based: `x-skills list` inspects the current project
plus global skill roots. To inspect another project, `cd` there first.

## Go Implementation

The Go CLI and TUI are the authoritative implementation of `x-skills`.

Install the latest release on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/InkyQuill/x-skills/main/scripts/install.sh | sh
```

Install the latest release on Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/InkyQuill/x-skills/main/scripts/install.ps1 | iex
```

Both installers place the executable in `~/.local/bin` by default. Set
`X_SKILLS_INSTALL_DIR` to choose another directory, or `X_SKILLS_VERSION` to
install a specific tag such as `v1.2.3`. They install `x-skills` and create an
`xs` shortcut only when that command name is available.

Build or run the Go implementation from a source checkout:

```bash
mkdir -p ~/bin
go build -o ~/bin/x-skills ./cmd/x-skills
go run ./cmd/x-skills list
go run ./cmd/x-skills repo
go run ./cmd/x-skills doctor
go run ./cmd/x-skills doctor --fix -y
go run ./cmd/x-skills doctor --fix -y --at global:agents
go run ./cmd/x-skills tui
```

Shipped: cwd-based active scanning, local archive listing, remote search/add,
link/migrate/unlink, recommend/unrecommend, manifest restore, interactive sync,
Doctor repair, and the `x-skills tui` Bubble Tea guided manager (Active, Repo,
Doctor, Install, and Sync workflows). Active rows are merged by
directory SHA fingerprint so identical linked copies appear as one item while
changed copies remain separate; the fingerprint is internal and not shown in
the UI.

Archive removal and rename are available from the Repo TUI page; they are not
separate CLI subcommands. Remote comparisons and same-source updates currently
belong to Install discovery and conflict handling, not Repo maintenance.

Maintained behavior references: [CLI guide](docs/cli.md), [TUI guide](docs/tui.md),
[Remote skills guide](docs/remote-skills.md), and [domain vocabulary](CONTEXT.md).

## Usage

```bash
x-skills list
x-skills list --at project:codex
x-skills list --at .Ag --at '~Cl'
x-skills list-roots --json

x-skills repo

x-skills search next
x-skills search next --owner vercel-labs --json

x-skills link svelte-coder --at project:codex
x-skills link typescript-expert --at global:codex
x-skills link svelte-coder typescript-expert --at project:codex

x-skills migrate next-best-practices --at project:codex
x-skills unlink opentui-react --at global:agents
x-skills unlink supergoal --at global:claude --delete-unmanaged

x-skills add owner/repo@skill
x-skills add owner/repo@skill --no-link -y
x-skills add owner/repo --all --at .Ag --at '~Cl'
x-skills add owner/repo skill-name --at .Cd -y

x-skills tui
x-skills doctor
x-skills doctor --fix -y
x-skills doctor --fix -y --at global:agents
```

Doctor also checks the shipped `x-` Built-In Skills. A non-interactive
`doctor --fix -y` archives missing Built-In Skills but deliberately leaves them
inactive: automation must not guess a Skills Folder. Pass one or more explicit
global destinations, such as `--at global:agents`, to archive and link them.
Project destinations are rejected for Built-In Skill repair. Without `-y`,
`doctor --fix` shows the enabled global Skills Folders with `~Ag` preselected
and an explicit `Archive only` choice.

Doctor also audits project Git hygiene. It reports an untracked recommended
manifest (`.x-skills.yaml`), a tracked local manifest
(`.x-skills.local.yaml`), and tracked files inside configured project Skills
Folders. Both the CLI and the TUI show shell-quoted `git add` or
`git rm --cached` commands for the user to run manually. `doctor --fix` never
changes or stages the Git index; it only appends literal ignore rules for the
local manifest and project Skills Folders to `.gitignore`. An ignored
recommended manifest is reported with an explicit `git add -f` suggestion.

`x-skills list` answers "what am I currently working with?" It shows active
skills from the current project and global managed roots, grouped by scope and
target. Each active skill is marked:

- `managed`: symlinked to the same-named repo skill in `~/.x-skills/skills`;
- `unmanaged`: a real skill directory or symlink outside the repo;
- `broken`: a symlink whose target is not a valid skill directory.

Human output is colorized by default when stdout is a terminal. Use
`--color never` to disable color, or `--color always` to force it through pipes.
Broken skills show the reason, such as a missing symlink target or a target
directory without `SKILL.md`.

`x-skills repo` answers "what do I have saved?" It lists archived skills in
`~/.x-skills/skills` with descriptions from `SKILL.md` frontmatter.

`add`, `link`, `migrate`, and `unlink` accept multiple skill names and
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

Use repeatable `--at` selectors to narrow active commands. Selectors can be
canonical locations such as `project:agents` or `global:codex`, root labels such
as `.Ag` or `~Cl`, or configured custom labels such as `.Oc`. Ambiguous labels
fail with guidance to use the canonical `scope:target` form.

Custom managed roots live in `~/.x-skills/config.yaml`:

```yaml
version: 1
active_roots:
  - scope: project
    target: agents
    path: .agents/skills
    label: .Ag
    consumers: [codex, pi, opencode, crush]
  - scope: global
    target: hermes
    path: ~/.config/hermes/skills
    label: ~Hm
  - scope: global
    target: claude
    enabled: false
```

Config entries add or override roots by `scope` and `target`. `enabled: false`
disables a root and does not require `path`. Relative project paths are resolved
from the current project root; relative global paths and `~/` paths are resolved
from the home directory. Target and consumer ids must match
`^[a-z][a-z0-9-]*$`. Omitted consumers on a custom root mean that its consumer
set is unknown.

Use `x-skills list-roots` to inspect enabled managed roots and discover
available `--at` selectors. Use `x-skills list-roots --json` for
agent-readable root discovery, including each root's canonical `consumers` IDs.

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

`-y` and `-n` do not choose among ambiguous locations. Use explicit selectors
such as `--at project:codex` or answer the interactive selection prompt.

For unmanaged active directories, `unlink` asks whether to migrate first, remove
the active directory without migration, or cancel. For automation, use
`--delete-unmanaged -y` to remove an unmanaged active directory without adding it
to the repo.

## TUI Mode

`x-skills tui` opens the Bubble Tea maintenance manager for longer maintenance
sessions. The TUI has Active, Repo, Doctor, and Install pages plus the Sync
workflow: press `A` for
Active, `R` for Repo, `D` for Doctor, and `I` for Install. Refresh is `ctrl+r`.

Use Install to search `skills.sh`, preview remote `SKILL.md`, archive a skill,
or install and link it into the current project. Audit risk pills (safe/warn/risky)
and source badges are shown alongside each result. Manual generic Git installs
remain CLI-first through `x-skills add --git`.

Use Active to inspect current project/global skills, preview `SKILL.md`, migrate
unmanaged directories into the archive, and unlink active copies. Use Repo to
preview archived skills, link them into a selected destination, unlink visible
current usages, or delete archives after visible usages are removed. Use Doctor
to review and fix current issues. Built-In Skill repairs open a global Skills
Folder checklist with `~Ag` preselected and an `Archive only` choice; the
filesystem work runs in the background so the TUI remains responsive.

## Design Decisions

Significant, non-obvious decisions are recorded as ADRs in `docs/adr/`. When a
choice needs weighing trade-offs (not just implementing an agreed spec), write
or update an ADR rather than only changing code/docs.

## Development

```bash
go build ./...
go test ./...
```
