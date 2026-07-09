# Backlog

## TUI And Agent Roots

- Add a managed-agent registry configuration for selecting which agent skill roots x-skills manages. Context: current Go TUI parity should default to the existing six roots, but later support should cover additional agents such as pi, opencode, hermes, charm, mimocode, and openclaw, including agents with non-standard directory layouts. Evidence: design discussion for Go TUI parity.
- Add optional mouse support for the Go TUI. Context: parity design is keyboard-only to keep interactions reliable and testable, but later Bubble Tea mouse handling could support row selection, modal option selection, and scrolling. Evidence: design discussion for Go TUI parity.
- Add fuzzy filtering and ranking to the Go TUI. Context: parity filtering should start with predictable case-insensitive substring matching across names, aliases, descriptions, statuses, and root chips; fuzzy matching can improve navigation later. Evidence: design discussion for Go TUI parity.
- Add theme support for the Go TUI. Context: parity design should ship one semantic color theme with fallbacks, while future work can add theme switching or terminal-background-aware palettes. Evidence: design discussion for Go TUI parity.
- Design URL install/update semantics for the Go rewrite. Context: direct `SKILL.md` or archive URL installs are deferred because they do not provide a reliable source update model, and directory/reference support would need remote listing or a checksum/provenance strategy. Evidence: remote install parity design discussion.
- Add batch remote installs in the TUI Install view. Context: first Go remote-install slice installs one selected search result at a time because preview, conflict resolution, rename prompts, and multi-root linking make batch remote install decisions too complex for the initial workflow. CLI `add --all` is a separate, already-decided batch path (ADR 0011) and is not blocked by this item. Evidence: remote install parity design discussion.
- Add full JSON output for remote mutation commands. Context: first Go remote-install slice provides JSON for read-only `search` and `repo check`, while `add`, `repo update`, and `repo update-all` keep human summaries until mutation result schemas are designed. Evidence: remote install parity design discussion.
- Add per-file and per-conflict merge choices for archive conflict resolution. Context: first parity implementation resolves conflicts at the whole-skill level; granular control over which changed files are copied or discarded should be designed and implemented later. Evidence: design discussion for Go TUI parity.
- Add option to persist multi-selection across view switches. Context: parity design resets selections when changing tabs for safety, but if users find this workflow awkward, we can introduce configuration or state tracking to keep selections per-tab. Evidence: design discussion for Go TUI parity.
- Add command palette (triggered by `:`). Context: direct shortcuts and a help modal are sufficient for parity, but a command palette can be introduced if the keymap grows too large to manage. Evidence: design discussion for Go TUI parity.

## TUI Visual Inspiration

- Use [superfile](https://github.com/yorukot/superfile) as the strongest visual reference for a later UI polish pass. Context: superfile is a modern Bubble Tea terminal file manager with a highly finished visual language; borrow region composition and popup/modal treatment from it, including dense but legible panels, polished borders, strong color hierarchy, and theme-ready styling without copying its file-manager layout directly. Evidence: user review noted it is the best-looking reference and specifically called out regions and popups.
- Review [circumflex](https://github.com/bensadeh/circumflex) for simple keyboard-first reading workflows and compact pills. Context: circumflex is a terminal Hacker News browser with compact read/favorite/history states, discoverable keymaps, and theme configuration; borrow its pill treatment for tab headers, key labels, and other small state badges where they fit x-skills maintenance workflows. Evidence: user review specifically mapped pills in tab headers and similar UI elements to circumflex.
- Review [gh-dash](https://github.com/dlvhdr/gh-dash) for configurable dashboard composition and rich rows. Context: gh-dash is a rich GitHub terminal UI with user-defined sections, overridable vim-style hotkeys, custom actions, and YAML-controlled settings; borrow ideas for dense rich rows, future configurable views, command/action organization, and workflow-specific sections once x-skills grows beyond Active/Repo/Doctor/Install. Evidence: user review specifically mapped rich row treatment to gh-dash.

## Testing

- Investigate a mock or virtual filesystem layer for mutation and error-path tests. Context: current tests use real syscalls in `t.TempDir`, which is simple and reliable, but permission failures and platform-specific filesystem errors remain hard to exercise directly. Evidence: code quality review of filesystem-heavy packages such as `internal/repo`, `internal/actions`, and `internal/symlinkcheck`.

## Strange things to investigate

If migration asks you to overwrite skill, then after the migration finishes - the list of active skills doesn't autoupdate.
