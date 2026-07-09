# Show advisory audit status for remote skills

Remote search and install flows fetch advisory audit data from the upstream audit service when available, but audit failures never block preview or install. The Go TUI fetches audit data in the background for the current search result set and updates rows when the cache is ready. CLI commands fetch audit data only when the user asks for it with `--audit`.

Rows show one compact risk pill when data exists: `✓ safe`, `⚠ warn`, or `‼ risky`. Unknown or unavailable audit data shows no pill, while inspectors and confirmations explain any available partner details without treating the signal as a security guarantee.
