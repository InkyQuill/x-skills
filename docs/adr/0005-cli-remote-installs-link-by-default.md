# CLI remote installs link by default

CLI remote installs through the `add` command archive the selected skill and link it into the current project's Agents root by default, with `--no-link` for archive-only installs. Search remains discovery-only. The default optimizes for the common command-line case: adding a skill for immediate project use, while preserving an explicit escape hatch for users who only want to populate the archive.
