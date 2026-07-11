# Remote discovery and updates

**Status:** Accepted for discovery and Install archive-state comparison; Repo remote maintenance remains planned

## Context

Remote browsing needs an unauthenticated discovery path and useful safety/provenance signals despite partial upstream failures. Archive state, update identity, and same-name conflicts must remain separate so network results never authorize destructive local changes.

## Decisions

- Discovery uses the legacy unauthenticated `https://skills.sh/api/search` endpoint. Queries start at two characters, accept an owner filter, and request a bounded result set (default 50). The current endpoint has no assumed server pagination; the TUI renders the returned set as a scrollable list. A future authenticated API requires a separate decision.
- Search is discovery only. Search results lead to source/name-based `add` commands or Install-page actions, never index-based CLI installation. Install search is debounced after two characters, Enter may fetch immediately, and query/owner/results have explicit focus.
- Install preview clones into a temporary session cache and uses the same bounded source discovery as installation. Ambiguous name/path matches require selection; search IDs are not treated as paths.
- Every incoming item has an archive state: not archived, archived/current, update available, or name conflict. Same-source identity and content determine current/update; an existing local name with missing or different identity is a conflict. Linking an already-current archive does not reinstall it, and linking never implicitly updates it.
- Name conflicts offer keep/cancel, explicit replacement, rename incoming, or rename existing. Rename suggestions remain editable and validated. Same-source replacement requires confirmation and content diff. Archives are user-owned: missing upstream is reported but never deletes local content.
- Advisory audit data is fetched independently and cached only for the process. Failures never block search, preview, or install. Rows show no pill when unavailable; otherwise worst-signal summarization yields `✓ safe`, `⚠ warn`, or `‼ risky`, with partner details in inspectors/confirmations and no claim of a security guarantee. CLI audit is opt-in; generic Git sources are unaudited.
- Install search, preview, audit, and archive-state results carry generation/source identity so stale responses cannot overwrite a newer query or archive snapshot. Success keeps the user in Install and refreshes badges/status in place.
- The archive metadata model distinguishes installed commit from optional tracked ref. A future Repo check first resolves the recorded ref with `git ls-remote`; unchanged commits need no checkout, while changed commits require a bounded temporary checkout to verify that the recorded skill path still exists before reporting update available rather than missing upstream.
- Planned Repo checks run as non-blocking, bounded background work (four workers by default), refreshing on TUI startup, explicit remote refresh, and after install/update actions. Repo remains usable before results. Mutation stays explicit and selection-aware; update actions process eligible items sequentially, pause for conflicts, and summarize skips/failures without batch rollback.
- The current Repo page intentionally exposes neither remote status nor update mutations. These planned checks and commands must land through the shared source/update engine and documented failure states before being advertised.

## Consequences

Unauthenticated discovery is immediately useful but depends on a legacy upstream contract and bounded, non-paginated results. Audit and network degradation remove enrichment rather than core local functionality. Archive-state labels do not confuse same-name collisions with updates. Accurate missing-upstream detection costs a checkout only after the remote commit changes. Until Repo maintenance ships, users receive Install-time comparison rather than misleading update controls.

## Supersedes

- ADR 0002 — updates versus name conflicts
- ADR 0003 — legacy skills.sh discovery endpoint
- ADR 0009 — advisory audit status
