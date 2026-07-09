# Track source refs for git installs

Git-backed installs support an optional source ref from `--ref` or a simple GitHub tree URL, and source metadata records that ref alongside the installed commit. Update checks use the recorded ref when present instead of always checking default `HEAD`, so skills installed from non-default branches or tags are checked against the same upstream line they came from.
