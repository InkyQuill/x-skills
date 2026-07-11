# Documentation Structure

This document defines the maintained documentation set for x-skills. Project
documentation describes current behavior; ADRs preserve the reasoning behind
groups of related architectural decisions. Completed implementation plans,
design-session transcripts, and superseded specifications are not retained in
the main branch.

## Goals

- Make current behavior discoverable without reading implementation history.
- Keep domain terminology and safety invariants in one authoritative place.
- Separate user-facing CLI and TUI behavior from remote-source internals.
- Preserve significant architectural reasoning in a small thematic ADR set.
- Keep the repository Go-only.

## Maintained Documents

`README.md` is the entry point: installation, a concise workflow overview, and
links to deeper documentation.

`CONTEXT.md` owns the product model, terminology, filesystem layout, manifest
semantics, compatibility model, and cross-interface safety invariants.

`docs/cli.md` owns the command surface, destination selector grammar,
interactive behavior, confirmation rules, and machine-readable output.

`docs/tui.md` owns navigation, pages, selection semantics, inspectors, modal
flows, responsive behavior, status presentation, and key bindings.

`docs/remote-skills.md` owns remote discovery, source identity, provenance,
archive state, updates, audits, and supported Git transports.

`docs/backlog.md` contains only unfinished work. Completed implementation notes
must not remain there merely to preserve history.

## Thematic ADRs

The existing individual ADRs are consolidated into four decision families:

1. Skill sources and identity: source metadata, fallback identity, provenance,
   generic Git sources, and deferred unsupported sources.
2. CLI and destination workflows: install defaults, destination selectors,
   capability-oriented commands, explicit repository operations, and JSON
   output boundaries.
3. TUI architecture and interactions: Bubble Tea architecture, top-level pages,
   current-page selection, reusable components, and background operations.
4. Remote discovery and updates: discovery endpoint, name conflicts versus
   updates, audit status, and update-check behavior.

Each consolidated ADR records the current decision, context, consequences, and
the prior ADR numbers it supersedes. It does not reproduce obsolete alternatives
or implementation chronology unless they are necessary to understand the
decision.

## Consolidation Rules

- Preserve a statement only when it describes current behavior, an active
  invariant, an unfinished backlog item, or reasoning still needed to maintain
  the architecture.
- Prefer the implemented code and tests over older specifications when they
  disagree.
- Convert mockups and grilling answers into concise behavioral requirements;
  do not retain the original transcript.
- Delete completed plans after their remaining current facts are represented in
  maintained documentation.
- Update all incoming links and documentation tests in the same change.
- Retain changelog references to the former Python implementation only as
  release history; remove Python source, packaging, tests, and present-tense
  documentation references.

## Verification

The consolidation is complete when:

- no Python implementation or packaging files remain;
- no `docs/superpowers/specs` or `docs/superpowers/plans` files remain;
- only the four thematic ADRs remain under `docs/adr`;
- repository references point to maintained documents;
- the backlog contains only unfinished work;
- documentation tests, Go tests, vet, formatting, and link/reference scans pass.
