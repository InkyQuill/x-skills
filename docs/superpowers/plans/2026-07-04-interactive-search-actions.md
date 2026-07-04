# Interactive Search Actions Plan

Date: 2026-07-04

## Steps

1. Add failing tests for `x-skills search`, search install, and TUI action
   helpers.
2. Implement a small `skills.sh` API client compatible with the official Vercel
   client.
3. Add CLI `search` output and `--install <name-or-index>`.
4. Add TUI multi-select, bulk `migrate`/`unlink`, clean broken symlinks, search,
   and install actions.
5. Update README/help text.
6. Run pytest, ruff, shell install checks, and CodeRabbit review.
