# Remote skills guide

Remote installation is Git-based. `x-skills search QUERY` discovers skills through skills.sh; `x-skills add` clones the selected repository, discovers directories containing `SKILL.md`, stores selected content in the archive, and links it unless `--no-link` is used.

## Source grammar and transports

- `owner/repo` discovers all skills in a GitHub repository.
- `owner/repo@skill` selects a named/path-matching skill.
- A GitHub tree URL such as `https://github.com/owner/repo/tree/ref/path/to/skill` records the tree ref and path.
- `--git CLONE_URL [--ref REF]` supports generic Git transports, including non-GitHub HTTPS, SSH, and local Git repositories accepted by the installed `git` CLI.

Extra positional names restrict discovery; `--all` selects every discovered skill. Interactive multi-skill discovery asks for a selection. `--archive-as` renames one incoming selection. Search results carry the GitHub owner/repository and path used to construct an `add` command.

Arbitrary URL sources are deliberately unsupported. Zip/tar archives, raw `SKILL.md` URLs, non-GitHub web pages, and direct download URLs are rejected rather than guessed. Use a GitHub source or explicit `--git` clone URL.

## Provenance and identity

Each reproducible archive stores `.x-skills.json` source metadata: source type (`github` or `git`), clone/repository identity, skill path, optional source ref, installed commit, and compatibility metadata. The source ref is used again for update discovery. For older archives, GitHub owner/repo/path fields provide the remote identity fallback when a clone URL is absent; archives with no reproducible metadata remain archive-source manifest entries.

Source identity is transport-aware: GitHub identity compares owner, repository, and normalized skill path; generic Git identity compares clone URL and normalized skill path. A matching name alone is never identity.

## Archive state, conflicts, and updates

Incoming results have one archive state:

- `not archived`: no archive uses the proposed name;
- `archived`: the same source identity and content are already stored;
- `update available`: the same source identity has different incoming content;
- `name conflict`: the name exists but source identity is missing or different.

An update may replace content only after the same-source comparison and confirmation/diff workflow. A name conflict instead offers keeping the archive, renaming the incoming skill, renaming the existing archive, or explicit replacement; it is not mislabeled as an update. Rename preserves archive content identity and rewrites managed usages and manifest names.

During Install discovery, stored provenance lets x-skills compare a discovered skill with its archive at the recorded ref when present. The resulting archive state drives same-source update and conflict handling in Install. The Repo page does not currently check remotes, expose update status, or update archives; those maintenance workflows remain future work.

Search/install audit status is advisory. Safe, warning, or risky summaries and partner details report upstream service data; they are neither a security guarantee nor an install blocker. Compatibility profiles likewise produce warnings against destination consumer metadata rather than silently changing destinations.

See the [CLI guide](cli.md), [TUI guide](tui.md), and the thematic [ADRs](adr/).
