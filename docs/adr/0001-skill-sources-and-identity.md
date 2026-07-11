# Skill sources and identity

**Status:** Accepted

## Context

x-skills must install a complete skill directory, reproduce Git-backed installs, distinguish an upstream update from an unrelated same-name skill, and retain useful provenance after a local rename. Existing archives may have no source metadata, but a name alone cannot prove remote identity.

## Decisions

- Git is the reproducible remote-source boundary. GitHub shorthand (`owner/repo`), `owner/repo@skill`, and simple GitHub tree URLs are supported alongside explicit generic Git clone URLs through `add --git`. Arbitrary web pages, raw `SKILL.md` files, and zip/tar download URLs remain unsupported until they have a provenance, integrity, and update model.
- The installed `git` CLI owns transport: temporary clones provide discovery and preview content, `git rev-parse HEAD` records the installed commit, and `git ls-remote` supports update discovery. This works with HTTPS, SSH, local repositories, and the user's existing Git credentials without provider-specific download code.
- Discovery checks standard skill locations and a bounded recursive fallback, excluding heavy/internal directories. Installation copies exactly the discovered skill directory, including its internal files and subdirectories; references outside that directory are not chased.
- Each reproducible archive owns an embedded `.x-skills.json`. GitHub identity is repository owner/name plus normalized skill path; generic Git identity is clone URL plus normalized skill path. Metadata also records the installed commit, optional source ref, compatibility data, and optional `upstream_name` when the local archive was renamed.
- A recorded ref is part of reproducibility. `--ref` and simple tree-URL refs constrain checkout and later update discovery instead of silently switching to default `HEAD`.
- Remote equality requires matching source identity. A name is only a collision signal for legacy/manual archives without reproducible metadata, never proof that two skills are the same. Incoming same-source content can enter an update workflow; missing or different identity enters an explicit name-conflict workflow.
- Archive directory names are local ownership. Renaming incoming content preserves `upstream_name`; renaming an existing archive rewrites visible managed links and manifests. Immediate links use the final archive name, preserving the same-name managed-link invariant.
- Network/source discovery belongs in `internal/remote`; action packages orchestrate it with archive and link primitives. Preview checkouts and audit results are process/session caches, not durable archive state.

## Consequences

Installs and update comparisons are reproducible at a commit/ref and remain provider-neutral, at the cost of requiring `git` and temporary checkouts. Legacy archives remain usable but cannot be declared identical to a remote result from their name alone. Direct-download parity is intentionally incomplete. Moving or deleting an upstream skill path is observable as missing upstream rather than grounds for deleting the user's archive.

## Supersedes

- ADR 0001 — remote skill identity
- ADR 0004 — Git CLI transport
- ADR 0008 — deferred arbitrary URL installs
- ADR 0013 — source refs for Git installs
- ADR 0018 — generic Git source support
