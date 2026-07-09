# Run Repo update checks in the background

The Go TUI starts update checks for tracked archived skills through Bubble Tea commands and bounded background workers, then applies immutable status snapshots to the Repo view. The cache refreshes on TUI startup, on `^R`, and after install or update actions. Results must carry enough identity to discard stale messages when the repo state or query generation changes.

This accepts hidden network work because update badges are informational, not mutating. Repo remains usable immediately, while `^U` stays the explicit mutating action for updating selected archived skills.
