# Backlog

## TUI And Agent Roots

- Add a managed-agent registry configuration for selecting which agent skill roots x-skills manages. Context: current Go TUI parity should default to the existing six roots, but later support should cover additional agents such as pi, opencode, hermes, charm, mimocode, and openclaw, including agents with non-standard directory layouts. Evidence: design discussion for Go TUI parity.
- Add optional mouse support for the Go TUI. Context: parity design is keyboard-only to keep interactions reliable and testable, but later Bubble Tea mouse handling could support row selection, modal option selection, and scrolling. Evidence: design discussion for Go TUI parity.
- Add fuzzy filtering and ranking to the Go TUI. Context: parity filtering should start with predictable case-insensitive substring matching across names, aliases, descriptions, statuses, and root chips; fuzzy matching can improve navigation later. Evidence: design discussion for Go TUI parity.
- Add theme support for the Go TUI. Context: parity design should ship one semantic color theme with fallbacks, while future work can add theme switching or terminal-background-aware palettes. Evidence: design discussion for Go TUI parity.
- Add remote `skills.sh` search and install flows to the Go TUI. Context: parity design should focus on local active/repo/doctor management; remote search touches networking, trust warnings, install semantics, and update metadata and should be designed separately. Evidence: design discussion for Go TUI parity.
- Add repo update checks to the Go TUI. Context: parity design should exclude network-backed update checks; exposing `--check-updates` behavior in Repo view needs source metadata, network status, and error handling designed separately. Evidence: design discussion for Go TUI parity.
