# Support compact destination selectors

**Status:** Superseded by the `--at` location-selector contract.

This ADR records the original `--to` spelling. The shipped CLI uses repeatable
`--at` selectors with the same compact grammar: `global:`/`project:`, `g:`/`p:`,
`~`/`.` prefixes, full target names, and short target labels case-insensitively.
Unscoped selectors such as `agents` or `Ag` default to project scope. `--to` is
not supported.
