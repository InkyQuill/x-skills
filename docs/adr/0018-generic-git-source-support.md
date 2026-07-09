# Support generic Git sources alongside GitHub

`x-skills add` accepts a generic `--git CLONE_URL` source in addition to GitHub shorthand (`owner/repo`) and GitHub tree URLs. Generic Git sources are recorded with `source_type: git` in `.x-skills.json` (clone URL, commit, optional ref, skill path, optional `upstream_name`) rather than being coerced into GitHub-shaped metadata, and update checks use `git ls-remote` against the recorded clone URL and ref. Remote skill discovery via `skills.sh` search stays GitHub-only; advisory audit fetching (ADR 0009) is also GitHub/skills.sh-source only, so generic Git installs show no audit pill and the inspector reports audit as unavailable for that source type.

This keeps the transport (`git` CLI, ADR 0004) provider-neutral while keeping discovery, identity (ADR 0001), and audit signals scoped to sources we can actually reason about.
