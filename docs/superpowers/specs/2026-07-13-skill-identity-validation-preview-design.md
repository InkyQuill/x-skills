# Skill Identity, Validation, and Preview Design

**Date:** 2026-07-13

## Summary

Make filesystem identity explicit throughout x-skills, add deterministic skill and source-metadata
validation, provide a reusable remote preview path for both the CLI and TUI, and correct the bundled
skill workflows that currently assume richer skills.sh data or the wrong compatibility JSON shape.

The work fixes a manifest-reconciliation failure caused by treating the `name` declared in
`SKILL.md` as an archive key. It also makes `list` and `repo` honor `--json`, makes `link`
idempotent when the requested link is already correct, replaces redundant Active/Repo detail
modals with skill previews, and gives remote Search previews an immediate animated loading state.

## Verified Current State

The design is based on the merged `main` branch rather than the earlier report alone.

Confirmed defects and gaps:

- `actions.ActiveSkill.Name` contains the frontmatter name, while manifest and archive identity are
  directory based. `manifest.planLocalReconciliation` uses that declared name to build an archive
  path, so a skill such as `composition-patterns` declaring `vercel-composition-patterns` can break
  every later project reconciliation.
- `list` and `repo` ignore the root `--json` option and always write human text.
- `actions.Link` rejects every existing destination, including a symlink that already resolves to
  the requested archive skill.
- There is no `validate` command.
- Source metadata uses a nested `compatibility` object, but `x-port-skill` instructs agents to write
  `agnostic` or `agents` at the top level. The current JSON decoder silently ignores those unknown
  fields.
- Active/Repo Enter opens a detail modal that repeats inspector data, even though a local
  `SKILL.md` preview already exists behind `p`.
- Search preview performs a checkout asynchronously but does not set a visible preview-loading
  state, so Enter appears unresponsive.
- The live skills.sh search response can omit description, path, and audit and currently provides
  only identity/source/install-count fields for representative queries.

Reclassified or already fixed:

- `x-skills version` now exists and is wired to build information. No new version work belongs in
  this scope.
- `doctor --fix -y` does not execute its suggested `git rm --cached` command. It adds the relevant
  ignore entry and reports the command for manual follow-up. The remaining gap is safety guidance
  in `x-manage-skills`, not an automatic destructive app action.
- Missing search descriptions and audit data are an upstream registry limitation, not a parsing
  defect in `internal/remote`.

## Goals

- Separate directory/archive identity from the name declared in skill frontmatter.
- Prevent a declared-name mismatch from blocking manifest reconciliation or later mutations.
- Make list/repo human and JSON output describe both identities without ambiguity.
- Make an already-correct link a successful, explicitly reported no-op.
- Add portable validation for individual skills, `SKILL.md` paths, and shallow skill collections.
- Strictly reject unknown source-metadata fields so compatibility declarations cannot disappear
  silently.
- Validate compatibility syntax everywhere and configured consumer membership when destinations
  are supplied.
- Provide a non-interactive raw remote preview command for users and agents.
- Reuse one remote resolver for CLI preview and TUI Search preview.
- Give remote TUI previews an immediate animated, cancellable loading modal.
- Make Enter preview local skills in Active and Repo while retaining Doctor details.
- Correct and test the bundled `x-port-skill`, `x-find-skills`, and `x-manage-skills` workflows.

## Non-goals

- Changing the manifest schema or redefining manifest `name` away from archive identity.
- Requiring a declared frontmatter name to match its directory.
- Rejecting vendor-specific `SKILL.md` frontmatter fields.
- Deep recursive validation of arbitrary repositories.
- Rendering Markdown in the new CLI preview command.
- Building a standalone interactive preview application.
- Scraping the skills.sh Next.js site or inventing registry fields absent from its API.
- Redesigning all TUI inspector and modal infrastructure.
- Changing `doctor --fix` behavior.

## Architecture

### Explicit skill identity

Internal skill records must stop using one ambiguous `Name` field for two meanings.

- `Identity` is the basename of the skill directory in its active or archive root. It is the key
  used by paths, mutations, manifests, recommendations, and archive lookup.
- `DeclaredName` is the `name` value parsed from `SKILL.md` frontmatter.
- `Description` remains declared metadata.

`actions.ActiveSkill` and `repo.Skill` expose both fields explicitly. Internal consumers are
updated at compile time rather than retaining an ambiguous compatibility alias. Broken active
skills, whose metadata cannot be read, have an identity and an empty declared name.

For active entries, scanning derives identity from the root entry. For archived entries,
`repo.List` derives identity from the archive directory entry. Frontmatter parsing supplies only
declared metadata.

TUI Active groups continue to merge physically identical occurrences. A group chooses its primary
identity deterministically: prefer a managed member's archive identity, otherwise choose the
lexicographically first occurrence identity. Other occurrence identities remain aliases. A
different declared name is metadata, not an alias or a replacement identity.

Argument matching for active mutations continues to accept either identity or declared name. When
multiple occurrences match a declared name, existing ambiguity handling remains responsible for
requiring a precise choice.

### Manifest reconciliation

Project reconciliation uses `occurrence.Identity` for all of these operations:

- recommended-manifest exclusion;
- deduplication and divergent-identity detection;
- local manifest `Skill.Name`;
- managed archive-path construction;
- error messages that identify the affected filesystem skill.

The declared name never participates in an archive path. A mismatch may be displayed or diagnosed
but cannot make reconciliation fail by referencing a nonexistent declared-name directory.

This behavior is tested through the real post-mutation reconciliation paths for link, unlink,
migrate, sync, and restore rather than only through an isolated reconciliation unit test.

### Shared validation boundary

Add a focused internal validation package that accepts a configuration, paths, and optional managed
root locations and returns typed diagnostics. Cobra owns argument parsing and presentation; source
metadata owns strict decoding; neither human strings nor exit behavior leak into the validator.

A diagnostic contains:

- skill or input path;
- level (`error` or `warning`);
- stable machine code;
- concise actionable message;
- optional field or related path when useful.

Validation collects independent diagnostics instead of stopping at the first invalid child.

### Shared remote preview boundary

Add a remote skill resolver above the existing Git checkout cache and checkout skill lookup. Its
input is a parsed repository source plus a requested skill selector. Its result contains the
resolved skill directory, `SKILL.md` path/content, resolved repository skill path, and checked-out
commit/source information.

The resolver:

- accepts a context and performs no UI or output work;
- reuses cached checkouts;
- retains existing exact/path/frontmatter lookup and ambiguity behavior;
- returns typed or distinguishable errors for checkout failure, missing skill, ambiguity,
  cancellation, and timeout;
- is exercised with local Git fixtures rather than live GitHub or skills.sh traffic.

Cobra preview and Bubble Tea Search preview are consumers of this one boundary.

## CLI Behavior

### `list --json` and `repo --json`

Human list output uses filesystem identity as the primary name. When the declared name differs, it
renders:

```text
composition-patterns (declared: vercel-composition-patterns)  managed  ...
```

The same rule applies to Repo output. A matching declared name is not repeated.

JSON output is an array, and an empty result is `[]`. Active records include identity,
`declared_name` when different, description, status, active path, root scope/target/label/path, and
an optional broken reason. Repo records include identity, differing `declared_name`, description,
archive path, and available source metadata. JSON uses typed fields and never embeds formatted
human rows or ANSI styling.

### Idempotent `link`

When the destination exists, Link inspects it before returning an error.

1. A non-symlink destination remains an error and is not changed.
2. A symlink target is read with `os.Readlink`.
3. A relative target is resolved against the destination directory.
4. The resolved target and intended archive path are compared with the existing cross-platform
   path-identity boundary.
5. If they identify the same filesystem object, Link returns success with status
   `already_linked` and performs no filesystem mutation.
6. A symlink to another target remains a destination-exists error and is not changed.

Post-command project reconciliation still runs after the no-op. Batch output separates the states:

```text
linked: first-skill
already linked: second-skill
```

JSON uses the stable status `already_linked`.

### `x-skills validate`

Command syntax:

```text
x-skills validate PATH... [--at LOCATION...] [--json]
```

Path classification is deterministic:

- a `SKILL.md` file validates its parent skill directory;
- a directory containing `SKILL.md` validates one skill;
- a directory without `SKILL.md` is a collection whose immediate child skill directories are
  validated;
- traversal does not recurse beyond those immediate children;
- a missing path, unrelated regular file, empty collection, or unreadable input is an error.

Repeated or overlapping inputs are deduplicated by canonical path so a skill is reported once.

`SKILL.md` validation follows the portable core of `skill-creator`'s `quick_validate.py` while
remaining compatible with the real multi-agent archive:

- recognize LF and CRLF frontmatter delimiters;
- require a YAML mapping;
- require non-empty string `name` and `description` fields;
- enforce lowercase hyphen-case name shape, no leading/trailing/consecutive hyphens, and the
  64-character limit;
- enforce the 1024-character description limit and reject angle brackets;
- require non-empty body content after frontmatter;
- allow unknown/vendor-specific frontmatter keys;
- emit a warning, not an error, when directory identity and declared name differ.

If `.x-skills.json` exists, validation uses the production source-metadata parser. The parser uses
strict JSON decoding and rejects unknown fields, including mistaken top-level `agnostic` and
`agents`. It preserves accepted schema-v1 legacy behavior while validating schema version and
schema-v2 compatibility rules.

Schema-v2 metadata may contain only a compatibility declaration; locally authored or ported skills
do not need to invent remote provenance. If any remote source identity is present, `source_type`
and the fields required by that GitHub or generic-Git identity become mandatory and internally
consistent. Unknown source types and partially populated remote identities are errors.

Compatibility rules are:

- compatibility is a nested object;
- exactly one of `agnostic: true` or a non-empty `agents` list is present;
- agent IDs match `^[a-z][a-z0-9-]*$`;
- IDs are unique; the writer continues to emit them in sorted order;
- without `--at`, structurally valid agent IDs remain portable and need not exist in this machine's
  configuration;
- with repeated `--at`, each declared agent must appear in the union of consumer IDs for the
  selected enabled roots.

Unknown fields become errors in ordinary `ReadSourceMetadata` calls too. Schema versioning, not
silent field dropping, is the extension mechanism.

Human output groups diagnostics per skill and ends with a count summary. JSON output is one object:

```json
{
  "valid": false,
  "summary": {"skills": 3, "errors": 1, "warnings": 1},
  "diagnostics": [
    {"path": "skills/example", "level": "error", "code": "metadata.unknown_field", "message": "..."}
  ]
}
```

Warnings alone exit successfully. Any validation error produces a nonzero command result after all
inputs have been checked. CLI/configuration failures that prevent validation also remain nonzero.

### `x-skills preview`

Command syntax:

```text
x-skills preview OWNER/REPO SKILL
x-skills preview OWNER/REPO SKILL --lines 100
x-skills preview OWNER/REPO SKILL --json
```

The command resolves a GitHub repository through the shared remote preview boundary. The default
line limit is 50; `--lines` must be positive.

Plain stdout is only the first requested number of raw `SKILL.md` lines, including frontmatter. It
does not contain a heading, Glamour rendering, ANSI decoration, or a synthetic truncation marker.
The command preserves line ordering and terminates the output cleanly without inventing content.

JSON includes repository, requested skill, resolved repository skill path, commit, returned raw
content, returned line count, requested limit, and `truncated`. Missing, ambiguous, timed-out, or
unavailable repositories return actionable errors and no partial successful document.

## TUI Behavior

### Local Active and Repo preview

In Active and Repo, both Enter and `p` open the existing local `SKILL.md` preview. Enter no longer
opens a detail modal in those views. The redundant Active/Repo detail-modal constructors and tests
are removed.

The preview keeps the current behavior:

- rendered Markdown through Glamour by default;
- raw/rendered toggle;
- scrolling and constrained layout;
- local read errors shown in the preview.

The side inspector remains the home for status, paths, usages, source information, descriptions,
and a differing declared name. Doctor Enter continues to open the issue detail modal.

### Remote Search preview loading modal

Pressing Enter on a Search result performs an immediate state transition before starting checkout:

1. Increment the preview token and create a dedicated cancellable preview context.
2. Open a preview modal in loading state in the same Bubble Tea update.
3. Render an indeterminate animated indicator and repository/skill label using the model's existing
   animation ticks and ASCII fallback.
4. Run the shared resolver command.
5. On a matching success message, replace the loading body in the same modal with the normal
   rendered preview.
6. On a matching error, retain the modal and show an actionable error state.

Escape during loading cancels the preview context and closes the modal. Cancellation is not shown
as an error. Token and view/modal checks ignore late results after Escape, cursor changes, a new
preview, or leaving Search. A second preview of the same repository benefits from the checkout
cache.

Preview cancellation is separate from archive/install mutation cancellation so closing a preview
cannot accidentally alter another operation's lifecycle.

## Bundled Skill Updates

The canonical files are the tracked repository copies under `skills/`; installed archive copies are
not the source of truth for the change.

### `x-port-skill`

- Correct examples and instructions to write either
  `"compatibility": {"agnostic": true}` or
  `"compatibility": {"agents": [...]}`.
- Preserve every other source/provenance field and schema version 2 behavior.
- Replace the vague validator reference with
  `x-skills validate <staged-skill> --at <destination>... --json`.
- Repeat the same validation after applying the approved diff.
- Preserve the rule that bundled or source-provided scripts are not executed without explicit
  approval and sandboxing.

### `x-find-skills`

- Treat registry description, path, and audit as optional rather than guaranteed.
- Degrade initial ranking to available name, source, and install count.
- Use `x-skills preview owner/repo skill` before a consequential recommendation or install.
- Rank final relevance from actual `SKILL.md` content instead of inventing missing registry data.
- Keep exact archive-first add/link commands in the recommendation.

### `x-manage-skills`

- State accurately that `doctor --fix -y` adds ignores and may print a manual Git follow-up; it
  does not run `git rm --cached` itself.
- Before manually running a suggested recursive untrack command, inspect `git status` and count the
  tracked files beneath the target root.
- Require explicit user confirmation before staging a large untrack operation.
- Call out orchestration repositories and submodules where a Skills Folder may contain a large
  historically tracked tree.

### Skill verification discipline

Each skill edit follows `superpowers:writing-skills` separately:

1. Run a fresh baseline scenario against the current skill and record the observable failure.
2. Make the minimum instruction change for that failure.
3. Re-run the scenario with the revised skill.
4. Run `x-skills validate` on that skill.
5. Review the diff before proceeding to the next skill.

The three skills are not edited as one untested documentation batch.

## Error Handling

- Declared-name divergence is a validation warning and never blocks scanning or reconciliation.
- Invalid source metadata identifies the metadata path and offending field/rule.
- Collection validation reports every independently discoverable issue before returning failure.
- Correct existing links are successful no-ops; incorrect destinations are never replaced.
- Remote preview distinguishes checkout, lookup, ambiguity, timeout, and cancellation.
- TUI cancellation closes quietly, and stale results cannot reopen a modal.
- CLI preview writes no partial success document after a resolver error.
- Network failures do not affect local Active/Repo previews or other TUI views.

## Testing

### Identity and reconciliation

- Scan managed and unmanaged skills whose identity differs from their declared name.
- Preserve the identity/declared-name distinction in Repo records.
- Reconcile a project containing such a skill without looking up a declared-name archive path.
- Exercise post-link, post-unlink, post-migrate, post-sync, and post-restore reconciliation.
- Verify deterministic Active grouping, primary identity, aliases, and declared-name inspector data.
- Preserve argument matching by identity and declared name, including ambiguity errors.

### CLI output and idempotent linking

- Parse `list --json` and `repo --json` as typed JSON and assert empty arrays.
- Assert human divergence annotations appear only when names differ.
- Cover correct absolute and relative symlink targets and platform-equivalent paths.
- Cover regular-file, directory, wrong-target, and link-inspection failures without mutation.
- Assert `already linked` human summaries and `already_linked` JSON status in single and batch runs.

### Validation

- Validate a directory, direct `SKILL.md`, shallow collection, multiple inputs, duplicates, and
  mixed valid/invalid children.
- Cover LF/CRLF, malformed YAML, non-mapping frontmatter, missing/wrong-typed fields, invalid name,
  long description, empty body, and identity mismatch warning.
- Prove vendor-specific frontmatter keys remain accepted.
- Reject unknown source-metadata fields and mistaken top-level compatibility keys.
- Preserve valid schema-v1 metadata and compatibility-only schema-v2 metadata; validate complete
  schema-v2 remote source identities when source fields are present.
- Cover compatibility exclusivity, ID shape, duplicates, portability without `--at`, and consumer
  membership with one or more `--at` selectors.
- Assert warning-only exit success, aggregated error failure, human grouping, and JSON schema.

### Preview

- Use local Git repositories for resolver success, missing skill, ambiguity, checkout error,
  timeout/cancellation, and cached repeat lookup.
- Assert exact first-50-line raw output, custom positive limits, short files, final newline behavior,
  JSON content/counts, and invalid limits.
- Assert Active/Repo Enter and `p` open preview while Doctor Enter keeps details.
- Assert Search Enter opens the loading modal immediately and its indicator changes on animation
  ticks.
- Cover success transition, error transition, Escape cancellation, stale result suppression,
  cursor/view changes, and cached second preview.

### Skill workflows and integrated verification

- Forward-test each bundled skill with a minimal realistic scenario before and after its edit.
- Validate all three changed skills with the newly built command.
- Update `docs/cli.md` and `docs/tui.md` for commands, output, keys, and loading behavior.
- Run the full Go suite, focused race tests for TUI/remote/validation, and `go vet`.
- Require Linux, macOS, and Windows CI because link identity, CRLF parsing, and cancellation paths
  are cross-platform contracts.
- Do not make live GitHub or skills.sh calls from automated tests.

## Completion Criteria

- A frontmatter/directory name mismatch cannot break project reconciliation or later mutations.
- Users and JSON consumers can distinguish filesystem identity from declared metadata.
- `list --json` and `repo --json` produce machine-readable results.
- Re-running `link` against the correct link succeeds and reports `already linked`.
- `x-skills validate` checks individual skills and shallow collections with stable diagnostics.
- Unknown `.x-skills.json` fields fail loudly in validation and normal metadata reads.
- `x-skills preview owner/repo skill` prints the first 50 raw lines by default.
- Active/Repo Enter opens skill text, and Doctor details remain available.
- Search preview acknowledges Enter immediately with an animated cancellable modal.
- The three bundled skills describe commands and data that actually exist.
- All local verification and cross-platform CI gates pass.
