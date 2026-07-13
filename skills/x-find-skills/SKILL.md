---
name: x-find-skills
description: Discover, compare, and install agent skills with x-skills and skills.sh. Use when the user asks to find a skill, asks whether a skill exists, wants help extending agent capabilities, wants skills.sh search results checked against the local x-skills archive/repo, or wants an install command compatible with the current x-skills workflow.
---

# X Skills: Find Skills

Discover skills repo-first, then registry-first. Prefer `x-skills` over `npx skills` in this environment because it preserves the local archive in `~/.x-skills/skills`, links only the active roots needed for the current project, and records Git metadata for update checks.

## Workflow

1. Clarify the capability in 2-5 keywords: domain, framework/tool, and task type.
1. Inspect local state before recommending remote installs:

```bash
x-skills repo
x-skills list
```

If `x-skills` is unavailable but `./bin/x-skills` exists in the current repo, use `./bin/x-skills`.

1. Search remote candidates from the CLI/API only. Agents should not use the TUI:

```bash
x-skills search '<multi-word query>'
x-skills search '<multi-word query>' --owner <owner> --json
```

If `x-skills search` is unavailable in an older checkout, call the skills.sh API directly:

```bash
curl -fsSLG 'https://skills.sh/api/search' \
  --data-urlencode 'q=<multi-keyword query>' \
  --data-urlencode 'limit=20'
```

Registry results may contain only `name`, `source`, and `installs`; `description`, `path`, and
`audit` are optional. Build the initial ranking from the available name, source, and install count.
Do not fill in absent fields. For every candidate still under consequential consideration,
inspect it with:

```bash
x-skills preview owner/repo skill
```

Base final relevance on successfully returned `SKILL.md` content; running or listing the command
is not enough. A failed preview leaves that candidate provisional and unverified: exclude it from
recommendations and installs, but continue comparing candidates whose previews succeeded. If none
succeeded, present only a provisional shortlist, label it as not a recommendation, and omit install
or link commands.

1. Reconcile remote results with local archive:

- Already archived with matching source metadata: recommend linking, not reinstalling.
- Same name but different source: flag as a conflict; recommend preview/diff before replacing.
- Successfully previewed and not archived: recommend `x-skills add`.
- A supplied registry path is stale: still try install; `x-skills add` searches the repo by folder name and `SKILL.md` name when the provided path is wrong.

## Install Commands

Use archive-first installs:

```bash
x-skills add owner/repo@skill-name
x-skills add owner/repo skill-name --at .Ag -y
x-skills add owner/repo@skill-name --no-link -y
x-skills add --git https://example.com/repo.git skill-name --ref main --no-link -y
```

Default behavior links installed skills into the current project Agents root. Use `--no-link` when the user only wants to save the skill in the archive. Use repeated `--at` selectors for exact active roots: `.Ag`, `.Cl`, `.Cd`, `~Ag`, `~Cl`, `~Cd`, `project:agents`, `global:codex`, etc. Use `x-skills list-roots --json` to discover configured custom roots before choosing a selector.

Use `--all` only when the user explicitly wants every discovered skill from a repository:

```bash
x-skills add owner/repo --all --no-link -y
```

## Ranking

Use this order:

1. Already archived and relevant to the current project.
2. Initially shortlist remote candidates using only available names, sources, and install counts.
3. Run `x-skills preview owner/repo skill` for candidates that could lead to a recommendation or
   install.
4. Rank final candidates by how directly their actual `SKILL.md` content matches the requested
   workflow; explain source or content risks.

Do not make a consequential recommendation or install solely from a registry row.

## Response Shape

For final or recommended options whose preview output was successfully read, include:

- Skill name and source (`owner/repo@skill`).
- Local status: archived, linked, conflict, or new.
- Why it matches the request.
- One exact `x-skills add` or `x-skills link` command.

For a registry-only shortlist, say it is provisional and not a recommendation. Include available
names, sources, install counts, and local status, but no install or link commands. A candidate whose
preview failed may remain in that provisional list as unverified; do not present it as a final or
recommended option.

Keep the recommendation short. Install only after the user asks or when the request explicitly asks you to install.

## Parity Notes

- Use `x-skills add` instead of `npx skills add`.
- Agents should use CLI commands and direct API calls, not `x-skills tui`.
- Use `x-skills search --json` for agent-readable discovery output.
- URL/archive/direct `SKILL.md` installs are intentionally deferred; prefer GitHub shorthand or `--git`.
