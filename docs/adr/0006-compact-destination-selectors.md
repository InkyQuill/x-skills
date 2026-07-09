# Support compact destination selectors

CLI install and link-after-install flows accept repeatable compact destination selectors via `--to`, including `global:`/`project:`, `g:`/`p:`, `~`/`.` prefixes, full target names, and short target labels case-insensitively. Unscoped selectors such as `agents` or `Ag` default to project scope. This keeps multi-root installs practical from the command line while preserving the same `.Ag`, `~Cl`, and `~Cd` language used by the TUI.
