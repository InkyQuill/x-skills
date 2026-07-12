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

Parse results for `name`, `description`, `owner`, `repo`, `path`, `installs`, and `audit`. Treat search/API failures as transient unless the same repository/path also fails during checkout.

1. Reconcile remote results with local archive:

- Already archived with matching source metadata: recommend linking, not reinstalling.
- Same name but different source: flag as a conflict; recommend preview/diff before replacing.
- Not archived: recommend `x-skills add`.
- Registry path is stale: still try install; `x-skills add` searches the repo by folder name and `SKILL.md` name when the provided path is wrong.

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

Prefer candidates in this order:

1. Already archived and relevant to the current project.
2. Official or well-known source repos with clear descriptions and strong install counts.
3. Skills whose `SKILL.md` directly matches the user's requested workflow.
4. Lower-install or unknown-source skills only after previewing the contents and explaining the risk.

Do not recommend solely from a registry row. Preview or inspect the skill when the user is likely to install it.

## Response Shape

When presenting options, include:

- Skill name and source (`owner/repo@skill`).
- Local status: archived, linked, conflict, or new.
- Why it matches the request.
- One exact `x-skills add` or `x-skills link` command.

Keep the recommendation short. Install only after the user asks or when the request explicitly asks you to install.

## Parity Notes

- Use `x-skills add` instead of `npx skills add`.
- Agents should use CLI commands and direct API calls, not `x-skills tui`.
- Use `x-skills search --json` for agent-readable discovery output.
- URL/archive/direct `SKILL.md` installs are intentionally deferred; prefer GitHub shorthand or `--git`.
