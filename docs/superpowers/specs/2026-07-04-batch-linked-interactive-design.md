# Batch, Linked Groups, and Interactive Mode Design

## Goal

Improve `x-skills` for real maintenance sessions:

- `unlink` must let users remove an unmanaged active directory without migrating it.
- Skill commands should accept multiple skill names.
- When the same skill is present in several active roots, `x-skills` should detect whether those locations are linked together and offer group management.
- `x-skills interactive` should provide a Textual-based TUI for multi-step maintenance.

## Unlink Semantics

Current problem:

```text
x-skills unlink supergoal
Migrate unmanaged global claude skill "supergoal" before unlinking? [y/N]: N
cancelled
```

For an unmanaged active directory, `N` cancels because the current prompt only asks about migration. That is wrong UX: the user asked to unlink, and migration is just the safe option.

New interactive prompt:

```text
"supergoal" is an unmanaged directory at ~/.claude/skills/supergoal.

Choose action:
  1. migrate to repo, then unlink active copy
  2. unlink without migration (remove active directory)
  3. cancel
Select [1-3]:
```

Rules:

- Managed symlink: confirm and remove symlink.
- Broken symlink: confirm and remove symlink.
- Unmanaged directory: prompt for migrate, remove directory, or cancel.
- `-y`: choose the safe action, migrate first.
- `-n`: cancel.
- `--no-input`: fail unless the action is fully specified by flags.

Add an explicit flag for automation:

```bash
x-skills unlink NAME --delete-unmanaged
```

This removes an unmanaged active directory without migrating it. It still requires `-y` or an interactive confirmation because it deletes local data.

## Batch Commands

Commands that operate on skill names should accept one or more names:

```bash
x-skills link foo bar --target codex --project
x-skills migrate foo bar
x-skills unlink foo bar baz
x-skills repo remove foo bar
```

Behavior:

- Process names in the order given.
- Prompt per skill when ambiguity or confirmation is needed.
- Continue after skipped items.
- Stop on unexpected operational errors that may indicate a bug or filesystem issue.
- Do not attempt rollback after partial success.
- Print a summary at the end for batch runs.

Example summary:

```text
Summary:
  linked: foo, bar
  skipped: baz
  failed: qux (active skill not found)
```

For a single name, keep concise one-off output.

## Linked Active Groups

When the same skill name appears in multiple active roots, `x-skills` should inspect whether locations represent the same physical setup.

Classify matches by comparing:

- `path.resolve(strict=True)` for all valid entries;
- whether an active symlink points to another active root entry;
- whether managed entries point to the same repo skill;
- whether directories are separate copies.

Group categories:

- `linked-group`: locations resolve to the same physical skill or symlink through each other.
- `same-repo`: managed locations all point to `~/.x-skills/skills/NAME`.
- `separate-copies`: same name, different physical directories.
- `mixed`: a combination of linked and separate locations.

For linked groups, prompt:

```text
Found linked setup for "foo":

  1. global agents  ~/.agents/skills/foo
  2. global claude  ~/.claude/skills/foo -> ~/.agents/skills/foo

Apply action to:
  1. linked group
  2. selected location only
  3. cancel
Select [1-3]:
```

Rules:

- `migrate` on a linked group migrates the canonical source into repo and relinks all group members to the repo skill.
- `unlink` on a linked group removes all active entries only when the user selects group action.
- `link` should fail when any destination already exists unless replacement is explicitly supported later.
- Separate copies must not be merged automatically. Prompt the user to choose a location.

Canonical source for linked groups:

1. Prefer the real directory if exactly one group entry is a directory.
2. Otherwise prefer the resolved path target.
3. If the canonical source cannot be identified, fail with a diagnostic.

## Interactive Mode

Add:

```bash
x-skills interactive
```

Use Textual as a runtime dependency. The one-liner installer already uses:

```bash
uv tool install --upgrade git+https://github.com/InkyQuill/x-skills.git
```

Runtime dependencies listed in `pyproject.toml` are installed by `uv tool install`, so Textual will be available to users. Tests should verify that `textual` is declared as a project dependency and that the documented one-liner still uses `uv tool install`.

MVP screens:

- Active table with project/global roots across targets.
- Repo table with archived skills.
- Broken filter/view.
- Detail pane showing path, symlink target, repo state, status reason, and description.
- Multi-select.
- Action commands: migrate, unlink, link, repo remove, refresh, quit.

MVP implementation constraints:

- The TUI should call the same operation helpers used by CLI commands.
- No mutation happens without explicit action confirmation.
- If Textual cannot import, print a clear install diagnostic and exit 2.
- `--no-input` is not meaningful for `interactive`; reject it with an error.

The first implementation may keep the UI simple: a single table with keyboard commands and a detail pane is enough. Full tabs can come later if the operation layer is already shared.

## Testing Requirements

Tests should cover:

- `unlink unmanaged` interactive selection can delete without migration.
- `unlink unmanaged -n` cancels.
- `unlink unmanaged -y` migrates first.
- `unlink unmanaged --delete-unmanaged -y` removes the active directory.
- `link`, `migrate`, `unlink`, and `repo remove` accept multiple names.
- Batch operations print summary output.
- Linked active entries are detected as one group.
- Separate same-name copies are not grouped.
- `migrate` on linked group relinks all entries to repo when group action is selected.
- `pyproject.toml` includes `textual` as a dependency.
- `x-skills interactive --no-input` exits with a clear error.
- `x-skills interactive` dispatches to the Textual app when Textual is importable.
