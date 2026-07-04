# Interactive Search Actions Design

Date: 2026-07-04

## Goal

Make `x-skills interactive` useful for maintenance sessions, not just inspection.
Add a matching CLI search command for `skills.sh` so users can discover and
install remote skills without leaving `x-skills`.

## UX

- `x-skills search <query>` lists results from the official `skills.sh` search
  API, matching the Vercel client fields: `name`, `id` as slug, `source`, and
  `installs`.
- `x-skills search <query> --install <name-or-index> -y` installs one search
  result into `~/.x-skills/skills`.
- TUI active view supports multi-select and bulk actions:
  - Space: toggle current row.
  - `m`: migrate selected active skills.
  - `u`: unlink selected active skills.
  - `x`: clean selected broken symlinks.
  - `s`: open skills.sh search mode.
  - `i`: install selected search result.
- TUI actions reuse CLI operation logic and still report failures per item.

## Non-Goals

- Do not silently link installed search results into project/global roots. The
  user can link after reviewing the repo copy.
- Do not implement a full package manager for updates/removes from skills.sh in
  this pass.
