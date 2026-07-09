# x-skills

`x-skills` manages reusable agent skills by keeping durable archived copies and linking selected skills into project or global agent roots.

## Language

**Archived Skill**:
A skill stored in the local x-skills repo under `~/.x-skills/skills`; it is the durable copy that active roots link to.
_Avoid_: Repo install, local install

**Active Skill**:
A skill visible to an agent from a project or global active root such as `.agents`, `.claude`, or `.codex`.
_Avoid_: Installed skill

**Install**:
The act of adding a remote skill to the local archive, optionally followed by linking it into one or more active roots.
_Avoid_: Search result linking, repo add, script install

**Link**:
The act of creating an active-root entry that points to an archived skill.
_Avoid_: Install

**Destination Selector**:
A compact CLI spelling for an active root, such as `global:agents`, `g:Ag`, `~claude`, `.Cd`, or `.agents`.
_Avoid_: Location string, target argument

**Archive-Only Install**:
An install that stores a remote skill in the local archive without linking it into any active root.
_Avoid_: Download only

**Install View**:
The TUI workspace for searching remote skill sources, previewing results, and installing selected skills into the local archive.
_Avoid_: Search modal, repo search

**Current Page**:
The active top-level TUI view: Active, Repo, Doctor, or Install. Selections and actions are scoped to the current page.
_Avoid_: Current tab

**Selection Set**:
The per-page set of checked rows that an action operates on before falling back to the highlighted row.
_Avoid_: Global selection

**Rich Row**:
A reusable TUI row composed from typed segments such as name, badges, status pills, and trailing description, rendered with explicit cursor and selection backgrounds.
_Avoid_: List string

**Source Badge**:
A compact TUI badge that identifies a remote or active-root source, such as `gh`, `.Ag`, `~Cl`, or `git`.
_Avoid_: Label

**Async Snapshot**:
An immutable result from background work, such as search results, update status, or audit data, applied to the Bubble Tea model only if it still matches the current generation.
_Avoid_: Shared mutable cache

**Search Result**:
A remote or local match shown in the Install view or `search` CLI output before the user decides whether to archive or link it.
_Avoid_: Page item, package row

**Audit Status**:
Advisory risk information fetched for a remote skill from the upstream audit service, summarized as a compact risk pill and explained with partner details when available.
_Avoid_: Security guarantee, verification

**Remote Skill**:
A skill discovered from a remote source such as `skills.sh`, backed by a GitHub repository and skill path when installable.
_Avoid_: Package, plugin

**Source Metadata**:
Stored provenance for an archived skill, including its GitHub source, skill path, and installed commit.
_Avoid_: Lockfile, package manifest

**Metadata File**:
The `.x-skills.json` file stored inside an archived skill that contains v1 source metadata.
_Avoid_: Index entry, sidecar database

**Upstream**:
The GitHub repository and skill path from which an archived skill was installed.
_Avoid_: Origin, remote package

**Source Ref**:
The optional Git branch, tag, or ref used when adding a skill from a Git source.
_Avoid_: Version, channel

**Name Conflict**:
A condition where an incoming remote skill wants to archive under a name already present locally, but source metadata does not prove it is the same remote skill.
_Avoid_: Duplicate install

**Update**:
Replacing an archived skill with a newer copy from the same proven remote source.
_Avoid_: Reinstall, overwrite

**Update Status**:
The Repo view state derived from source metadata and upstream checks: up to date, update available, missing upstream, or unknown.
_Avoid_: Version status

**Missing Upstream**:
An update status where the GitHub source is reachable but the archived skill's recorded upstream path no longer contains a valid `SKILL.md`.
_Avoid_: Deleted skill, broken repo

**Replace**:
Discarding an existing archived skill and storing incoming content at the same archive name.
_Avoid_: Update
