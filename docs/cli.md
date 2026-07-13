# CLI guide

The Go CLI is cwd-based. Unless `--project-root` is supplied, project Skills Folders and both manifests are resolved from the working directory. `--archive-root` changes the archive location. Run `x-skills list-roots` to see the enabled Skills Folders and their selectors.

## Commands

| Command | Current behavior |
| --- | --- |
| `list [--at ...]` | Lists active managed, unmanaged, and broken skills. |
| `list-roots` | Lists enabled managed roots, labels, paths, and consumer metadata. |
| `repo` | Lists archived skills and descriptions. |
| `search QUERY` | Searches skills.sh; supports `--owner` and `--limit`. |
| `add SOURCE [SKILL_NAME...]` | Discovers Git skills, archives selections, and links by default. Supports `--git`, `--ref`, `--all`, `--no-link`, `--at`, `--replace`, and single-skill `--archive-as`. |
| `link NAME...` | Links archived skills into selected Skills Folders. |
| `migrate NAME...` | Moves unmanaged active skills into the archive and replaces them with managed links. |
| `unlink NAME...` | Removes active occurrences; `--delete-unmanaged` permits confirmed deletion instead of migration. |
| `recommend NAME...` / `unrecommend NAME...` | Adds/removes archived skills in `.x-skills.yaml`; ordinary mutations continue to reconcile `.x-skills.local.yaml`. |
| `restore [--full] --at ...` | Applies the effective manifest to explicit project destinations. Default restore preserves extras; full restore exactly reconciles selected folders only. |
| `sync` | Aggregates project occurrences, then interactively selects variants and destinations. `--all` or repeatable `--skill` provides non-interactive skill selection; `--at` supplies destinations. |
| `doctor [--fix]` | Diagnoses roots, broken links, built-ins, manifests, compatibility, and Git hygiene; safe repair does not stage or untrack files. |
| `validate PATH... [--at LOCATION...] [--json]` | Validates skill documents and strict source metadata, optionally against destination consumers. |
| `preview OWNER/REPO SKILL [--lines N] [--json]` | Resolves a GitHub skill and prints raw `SKILL.md` content. |
| `tui` | Opens the guided manager; `--ascii` replaces Unicode symbols and `--no-input` refuses to open it. |

Archive rename is currently a Repo-page TUI action, not a Cobra command. It atomically renames the archive, rewrites managed links and both manifests, and rolls back the logical change on failure.

## Destination selectors and prompts

Repeatable `--at` accepts canonical selectors (`project:agents`, `global:codex`), compact selectors (`p:Ag`, `g:Cd`), scope-prefixed labels (`.Ag`, `~Cd`), and configured labels. A bare target or label such as `codex` resolves only to a project root; global roots require an explicit global selector such as `global:codex`, `g:Cd`, or `~Cd`. Ambiguity fails with candidates in non-interactive mode and is presented as a choice interactively; `-y` and `-n` never choose a location.

`-y/--yes` and `-n/--no` apply only to yes/no confirmation boundaries such as replacement, unlinking, deletion, or applying a prepared change. They are mutually exclusive. `--no-input` makes any required prompt an actionable error. Interactive `sync` is a selection workflow, not a yes/no prompt: it starts identical candidate groups selected, asks for divergent same-name variants, and then asks for explicit destination Skills Folders.

## Restore and repair boundaries

Default restore adds or repairs desired managed links but preserves extra managed and unmanaged skills. Full restore can remove extra managed links and migrate extra unmanaged skills before removing them, but never touches global or unselected roots and never deletes archives. Source compatibility mismatches are warnings that require acknowledgement where applicable.

Doctor's automatic fixes repair safe link/root problems, archive missing built-in skills, and append literal project ignore rules. Built-ins are linked only to explicitly selected global folders; project destinations are rejected. Git index commands remain suggestions. Mutating logical operations revalidate and use staging/rollback where atomicity is required; independent multi-name batches report partial success.

## Identity, JSON, and idempotent links

Filesystem identity comes from the active entry or archive directory; the declared frontmatter name is display and filter metadata only. It never replaces identity for filesystem behavior. Human `list` and `repo` output annotates a mismatch as `identity (declared: declared-name)` and omits the annotation when the values match.

`list --json` and `repo --json` return typed arrays, including `[]` when empty. Active records contain `identity`, an optional differing `declared_name`, `description`, `status`, active `path`, root `scope`/`target`/`label`/`path`, and an optional broken `reason`. Repo records contain `identity`, an optional differing `declared_name`, `description`, archive `path`, and available `source` metadata. `list-roots --json` exposes canonical locations, labels, paths, consumers, and built-in/enabled flags. `search --json` returns the query, optional owner, results, and a reproducible `add_command`.

`link` is idempotent only when an existing destination symlink resolves to the intended archive path: that item succeeds without mutation and reports `already linked` in human output or the stable `already_linked` status in JSON. A real file, directory, broken link, or link to another target remains untouched and returns a destination-exists error.

## Validation

```text
x-skills validate PATH... [--at LOCATION...] [--json]
```

A `SKILL.md` input validates its parent; a directory containing `SKILL.md` validates one skill; any other directory is a collection whose immediate child skill directories are validated without deeper recursion. Missing paths, unrelated files, unreadable inputs, and empty collections are errors. Repeated and overlapping inputs are deduplicated by canonical path.

Portable `SKILL.md` checks accept LF or CRLF frontmatter delimiters, require YAML mapping frontmatter with non-empty `name` and `description` strings, and require a non-empty body. Names must be lowercase hyphen-case, at most 64 characters, with no leading, trailing, or consecutive hyphens. Descriptions are limited to 1024 characters and cannot contain angle brackets. Unknown frontmatter keys are allowed. A directory identity/declared name mismatch is a warning.

If `.x-skills.json` exists, decoding is strict: unknown or trailing fields and invalid schemas, source identities, or compatibility declarations are errors in validation and ordinary metadata reads. Schema-v1 legacy metadata remains accepted. Schema-v2 `compatibility` must be nested and contain exactly one of `{"agnostic": true}` or a non-empty `{"agents": [...]}`. Agent IDs must be unique and match `^[a-z][a-z0-9-]*$`. Without `--at`, valid IDs are portable; repeated `--at` selectors validate them against the union of configured consumers for the selected roots. Complete remote source provenance is required only for schema v2 when any source identity is present.

This structured `.x-skills.json.compatibility` contract is distinct from a `compatibility` field in `SKILL.md` frontmatter. The latter is free-text vendor metadata and is not interpreted as an x-skills Compatibility Profile.

Validation input and skill diagnostics are aggregated before the report is written. Human output groups them by skill and ends with counts; JSON returns `valid`, `summary` (`skills`, `errors`, and `warnings`), and typed `diagnostics` (`path`, `level`, `code`, `message`, and optional field/related path). Warnings alone exit zero; validation errors exit nonzero after all validatable inputs have been checked. Configuration loading and `--at` selector resolution happen before validation; failures return nonzero without a validation report.

## Remote preview

```text
x-skills preview OWNER/REPO SKILL [--lines N] [--json]
```

The default output is the first 50 raw lines of the resolved `SKILL.md`, including frontmatter, with no heading, Markdown rendering, ANSI decoration, or synthetic truncation marker. `--lines N` requires a positive value. `--json` returns the repository, requested skill, resolved repository-relative skill path, commit, exact returned content, returned line count, requested limit, and `truncated`. Resolution errors produce no partial successful document.

## Other JSON support

Mutation commands retain their existing human or per-item JSON behavior; use each command's `--help` for supported output flags.

See also the [TUI guide](tui.md), [remote skills guide](remote-skills.md), and [domain vocabulary](../CONTEXT.md).
