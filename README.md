# x-skills

`x-skills` keeps niche agent skills in `~/.x-skills/skills` and links only the
skills you currently need into a project or global active skill directory.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/InkyQuill/x-skills/main/install.sh | sh
```

The installer checks for `git` and `uv`, then installs the CLI from
`https://github.com/InkyQuill/x-skills.git` with `uv tool install`.

## Usage

```bash
x-skills list                  # archived skills in ~/.x-skills/skills
x-skills list -g               # active global skills in ~/.agents/skills
x-skills list -g --target codex
x-skills linked                # active project skills in ./.agents/skills
x-skills linked -g             # active global skills
x-skills status                # check active project skills against ~/.x-skills

x-skills link opentui-react    # link into current project
x-skills unlink opentui-react  # unlink from current project

x-skills link -g opentui-react    # link into ~/.agents/skills
x-skills unlink -g opentui-react  # unlink from ~/.agents/skills
x-skills link --target claude ui-ux-designer
x-skills link -g --target codex typescript-expert

x-skills archive ~/.agents/skills/ui-ux-pro-max
x-skills migrate --target codex next-best-practices
x-skills install-github owner/repo path/to/skill
x-skills install-github https://github.com/owner/repo/tree/main/skills/foo
x-skills install-url https://example.com/skill.zip
x-skills doctor
```

Default paths:

- Archive: `~/.x-skills/skills`
- Global active skills: `~/.agents/skills`
- Project active skills: `<cwd>/.agents/skills`

Use `--target agents`, `--target claude`, or `--target codex` with active-skill
commands. Project roots are `<cwd>/.agents/skills`, `<cwd>/.claude/skills`, and
`<cwd>/.codex/skills`. Global roots are `~/.agents/skills`, `~/.claude/skills`,
and `~/.codex/skills`.

`unlink` is conservative. If the active skill is a real directory and the archive
does not already contain that skill, `x-skills` moves it into the archive before
removing it from the active set.

If the archive already contains a skill with that name, `unlink` and `archive`
refuse to overwrite it. Use `--archive-as <name>` or `--replace-archive`
explicitly.

`migrate` is the one-command version for local active skills: it moves the skill
directory into `~/.x-skills/skills` and leaves a symlink in the selected project
or global active root.

## Install Sources

`install-github` clones a GitHub repository and copies one skill into the archive.
Pass `path/to/skill` when the repository contains more than one `SKILL.md`.

`install-url` accepts:

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
