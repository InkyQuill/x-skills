# JSON output starts with read-only remote commands

The Go remote-install slice adds root `--json` and `-j`, but initially supports structured JSON only for read-only commands such as `search` and `repo check`. Mutation JSON for `add`, `repo update`, and `repo update-all` is deferred until result schemas are designed, keeping agent-readable discovery/check data available without freezing incomplete mutation contracts.
