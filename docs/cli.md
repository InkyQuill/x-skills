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
| `tui` | Opens the guided manager; `--ascii` replaces Unicode symbols and `--no-input` refuses to open it. |

Archive rename is currently a Repo-page TUI action, not a Cobra command. It atomically renames the archive, rewrites managed links and both manifests, and rolls back the logical change on failure.

## Destination selectors and prompts

Repeatable `--at` accepts canonical selectors (`project:agents`, `global:codex`), compact selectors (`p:Ag`, `g:Cd`), scope-prefixed labels (`.Ag`, `~Cd`), and configured labels. A bare target/label is accepted only when it resolves uniquely. Ambiguity fails with candidates in non-interactive mode and is presented as a choice interactively; `-y` and `-n` never choose a location.

`-y/--yes` and `-n/--no` apply only to yes/no confirmation boundaries such as replacement, unlinking, deletion, or applying a prepared change. They are mutually exclusive. `--no-input` makes any required prompt an actionable error. Interactive `sync` is a selection workflow, not a yes/no prompt: it starts identical candidate groups selected, asks for divergent same-name variants, and then asks for explicit destination Skills Folders.

## Restore and repair boundaries

Default restore adds or repairs desired managed links but preserves extra managed and unmanaged skills. Full restore can remove extra managed links and migrate extra unmanaged skills before removing them, but never touches global or unselected roots and never deletes archives. Source compatibility mismatches are warnings that require acknowledgement where applicable.

Doctor's automatic fixes repair safe link/root problems, archive missing built-in skills, and append literal project ignore rules. Built-ins are linked only to explicitly selected global folders; project destinations are rejected. Git index commands remain suggestions. Mutating logical operations revalidate and use staging/rollback where atomicity is required; independent multi-name batches report partial success.

## JSON support

`--json` is currently supported by the read-only `list-roots` and `search` commands. `list-roots` exposes canonical locations, labels, paths, consumers, and built-in/enabled flags. `search` returns the query, optional owner, results, and a reproducible `add_command`. Other commands retain human output.

See also the [TUI guide](tui.md), [remote skills guide](remote-skills.md), and [domain vocabulary](../CONTEXT.md).
