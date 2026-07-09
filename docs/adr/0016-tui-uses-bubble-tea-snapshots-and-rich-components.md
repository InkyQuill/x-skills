# Use Bubble Tea snapshots and reusable rich components

The Go TUI should keep `Update` non-blocking and route expensive work through Bubble Tea commands, bounded background workers, and stale-result checks. Search results, audit states, update checks, previews, and rendered markdown should be applied as immutable snapshots or cache updates so `View` can render without filesystem, network, or lock-heavy work.

The visual layer should use reusable Lip Gloss components for pills, keyboard shortcuts, hint lines, rich rows, modals, and source/update/audit badges. Rich rows receive explicit row background state so cursor highlight, selected state, and embedded pills render consistently. Markdown previews use cached Glamour renderers keyed by content and width. Multi-step forms and rename/conflict prompts may use Huh when it gives clearer focus and validation than hand-rolled modal state.
