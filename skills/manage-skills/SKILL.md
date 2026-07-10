---
name: manage-skills
description: Audit, prune, link, unlink, migrate, and reconcile project agent skills with x-skills. Use when the user wants to disable unnecessary project skills while preserving useful skills in the x-skills archive, clean active skill roots, resolve duplicate/conflicting skills, or choose the best local/remote skill version.
---

# Manage Skills

Use `x-skills` as the authority for active roots and the archive. The goal is to keep the project linked only to skills that help the current work, while preserving reusable skills in `~/.x-skills/skills`.

## Safety Rules

- Never delete an unmanaged active skill before either archiving it or getting explicit confirmation that it is disposable.
- Prefer unlinking active symlinks over deleting archived skills.
- Preserve project-specific useful skills in the archive before removing active copies.
- For conflicts, inspect both versions and choose the better maintained, more specific, or newer source. Do not silently overwrite divergent content.
- Use CLI commands only. Agents should not use `x-skills tui`; reserve TUI usage for humans.

## Audit

Start with:

```bash
x-skills list-roots
x-skills list
x-skills repo
x-skills doctor
```

If working from the x-skills repository and the binary is not installed, use `./bin/x-skills`.

Classify active skills:

- `managed`: symlinked to the archive; safe to unlink from active roots when not needed.
- `unmanaged`: real active directory; migrate before unlinking if useful.
- `broken`: stale symlink; usually fix with `x-skills doctor --fix -y` or unlink.
- duplicate active copies: keep only the roots needed for the current project.

## Decide What To Keep

Keep a skill linked when it is directly relevant to the current task, project stack, or recurring workflow. Unlink skills that are unrelated, redundant, too broad, obsolete, or superseded by a better archived skill.

When two skills overlap:

1. Preview both `SKILL.md` files.
2. Prefer the skill with clearer triggers, narrower scope, fresher source metadata, and fewer risky side effects.
3. If one is unmanaged and useful, migrate it into the archive, then link only the chosen one.
4. If both are useful for different scopes, keep both archived but link only the one needed for this project.

## Mutations

Archive an unmanaged active skill and relink it:

```bash
x-skills migrate skill-name --at project:codex -y
```

Link an archived skill into a project root:

```bash
x-skills link skill-name --at project:agents -y
```

Unlink a managed active skill:

```bash
x-skills unlink skill-name --at project:agents -y
```

Unlink an unmanaged skill after archiving it:

```bash
x-skills unlink skill-name --at project:codex -y
```

Delete an unmanaged active copy without archiving only when the user explicitly accepts data loss:

```bash
x-skills unlink skill-name --at project:codex --delete-unmanaged -y
```

For larger cleanups, keep using CLI commands with explicit names, `--at` selectors, and confirmation flags. Use `x-skills list-roots --json` to discover configured custom roots. If a decision is ambiguous and no non-interactive flag can express it safely, stop and ask the user rather than opening the TUI.

## Conflict Policy

When `migrate`, `unlink`, or install flows report an existing archive with different content:

1. Use CLI output, file reads, and repository inspection to compare both versions.
2. Choose incoming/local based on content quality, source freshness, and project relevance.
3. Rename when both versions are useful.
4. Keep the archive unchanged when the active copy is accidental or lower quality.
5. After resolving, continue the queue until all requested skills are processed.

## Output

Report:

- Skills kept linked and why.
- Skills archived but unlinked.
- Skills removed from active roots.
- Conflicts resolved and the chosen source.
- Any commands that failed or require manual follow-up.
