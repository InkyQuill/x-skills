# CLI and destination workflows

**Status:** Accepted, with source-tracked Repo update commands planned

## Context

The Go CLI is cwd-based and must serve interactive humans and automation without guessing consequential choices. Remote add, archive management, active-folder linking, and multi-destination mutations need one vocabulary even though the Python predecessor exposed different command names.

## Decisions

- Capability parity takes precedence over legacy spelling. `search QUERY` is read-only discovery. Remote installation is the top-level `add SOURCE [SKILL_NAME...]`, aligned with the `npx skills add` mental model; obsolete source-specific `repo` add commands are not part of the Go surface.
- `add` archives first and links to the current project's Agents Skills Folder by default. `--no-link` requests archive-only behavior. Explicit repeated `--at` selectors replace the default and can target multiple enabled Skills Folders.
- Accepted source forms include GitHub shorthand, `owner/repo@skill`, simple tree URLs, and `--git CLONE_URL`; `--ref`, positional names, and `--all` refine discovery. Interactive ambiguity presents discovered skills for selection; non-interactive ambiguity fails with actionable choices. `--all` is confirmation-gated.
- `--at` is the canonical destination language. It accepts canonical selectors such as `project:agents`, compact forms such as `p:Ag`, scope-prefixed labels such as `.Ag` and `~Cd`, and configured labels case-insensitively. Bare target/label input is accepted only when it resolves uniquely; otherwise the CLI prompts or fails. The earlier proposed `--to` spelling and unconditional project default are superseded.
- `-y` and `-n` answer only yes/no confirmation boundaries. They never select a skill, location, conflict resolution, or rename. `--no-input` converts every required prompt into an actionable error.
- Conflict controls are explicit (`--replace` and single-skill `--archive-as` where supported). Interactive conflict workflows distinguish same-source replacement from unrelated name collisions and show the content/scope before mutation.
- Operations that logically span one change—archive plus selected links, archive rename plus links/manifests, restore, and Sync—preflight inputs and use staging/rollback to avoid half-applied destinations. Independent multi-name batches are sequential, do not roll back earlier successful items, and report successes, skips, and failures.
- JSON begins with stable read-only contracts. The currently implemented commands are `search` and `list-roots`; unsupported mutation commands keep human output rather than freezing an incomplete schema. Source-tracked `repo check [NAME...]`, `repo update NAME...`, and `repo update-all` remain the planned update surface, including read-only JSON for `repo check`, but are not advertised as implemented.
- When implemented, `repo check` will narrow by names or check all tracked archives; named updates will share the TUI update engine; `repo update-all` will plan before applying, prompt unless `-y`, and fail in non-interactive mode without explicit approval. Conflicts and missing/unknown sources are skipped with reasons rather than guessed through.

## Consequences

Common adds are short and immediately useful while archive-only and multi-folder intent remain explicit. Scripts get deterministic failure instead of hidden prompt choices. Transactional logical operations protect archive/destination consistency, while batch commands retain partial-progress semantics. The Go CLI does not preserve obsolete Python command names, and planned Repo update commands must not be mistaken for current functionality.

## Supersedes

- ADR 0005 — link-by-default remote installs
- ADR 0006 — compact destination selectors
- ADR 0011 — `npx skills add`-aligned command shape
- ADR 0012 — capability parity over legacy names
- ADR 0014 — JSON scope
- ADR 0017 — explicit Repo check/update commands
