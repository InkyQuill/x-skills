# Defer URL installs until provenance is designed

The Go remote-install parity slice excludes direct-URL installs (`.zip`/`.tar`/`.tar.gz`/direct `SKILL.md` links) even though the Python implementation supports them. URL installs do not provide a reliable update source for skill directories and referenced files, so they need a separate provenance and checksum design before being added to the Go `add` command or TUI. See ADR 0011/0012: the Go remote install surface is the top-level `add SOURCE [SKILL_NAME...]` command, not Python's `repo add-github`/`repo add-url` names.
