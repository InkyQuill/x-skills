# Use explicit repo check/update/update-all commands

The CLI exposes source-tracked update workflows as `repo check [NAME...]`, `repo update NAME...`, and `repo update-all`, instead of mirroring Python's single `repo --check-updates` flag with a suggested `--replace-archive` rerun. `repo check` reports status without omitted names checking everything; `repo update` mutates only the named skills through the same conflict/diff engine used by the TUI's `^U`; `repo update-all` plans first, applies clean updates, and skips conflicts or missing/unknown sources with actionable summaries, prompting for confirmation unless `-y` (and failing outright without `-y` in non-interactive mode).

This is a deliberate command-shape choice, not Python parity: the goal is capability parity with an easier surface (see ADR 0012), and named vs. all-skills update intent deserves distinct commands rather than one flag plus a manual rerun hint.
