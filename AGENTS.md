# AGENTS.md

## Project Overview

`x-skills` is a personal archive and linker for AI agent skills. It stores reusable skills in `~/.x-skills/skills` and links only the ones needed into project or global agent directories (`.agents/skills`, `.claude/skills`, `.codex/skills`).

The current implementation is in Python (>=3.12) using `textual` for the TUI.
**Note:** There is an ongoing experimental Go rewrite planned (see `docs/superpowers/plans/2026-07-04-go-rewrite.md`), which uses Cobra and Bubble Tea.

## Setup Commands

Dependencies:
- `uv` for Python package management.
- `git` for skill cloning.

To install the CLI locally:
```bash
curl -fsSL https://raw.githubusercontent.com/InkyQuill/x-skills/main/install.sh | sh
```
Or set up for development:
```bash
uv sync
```

## Development Workflow

- The project uses `uv` for dependency management.
- The entrypoint is `x_skills.cli:main` (`x-skills`).
- The Python source is in `src/x_skills/`.
- The CLI is working-directory based. Many operations depend on the current working directory (`cwd`) to resolve project roots.

## Testing Instructions

Tests are located in the `tests/` directory and use `pytest`.

- Run all tests: `uv run pytest`

For Go changes, run `go test ./...`, relevant `go test -race` packages, and `go vet ./...`
before pushing. Cross-compile affected test packages for Windows when the host is not Windows.
Because Linux cannot execute native macOS or Windows tests, every pushed Go change must also wait
for both `Go macos-latest` and `Go windows-latest` CI jobs. Do not mark the work ready or merge it
until both jobs pass; fix failures and re-check the replacement run.

## Code Style

Python code style is enforced by `ruff`.

- Check linting: `uv run ruff check .`
- Check formatting: `uv run ruff format --check .`
- Line length limit is 100 characters.

## Architecture and Key Concepts

- **Repo Root**: `~/.x-skills/skills` (Archived skills)
- **Active Roots**: Where skills are linked/copied for use.
  - Project scope: `<cwd>/.agents/skills`, `<cwd>/.claude/skills`, `<cwd>/.codex/skills`
  - Global scope: `~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`
- **Skill States**:
  - `managed`: symlinked to the same-named repo skill.
  - `unmanaged`: a real directory or symlink outside the repo.
  - `broken`: invalid symlink.
- **UI/UX**: 
  - Do not silently decide ambiguous or destructive actions. Prompt the user or fail with actionable commands in non-interactive mode.
  - Interactive mode uses `textual`.

## Gotchas & Notes

- Batch operations run in order and do not roll back earlier successful changes if a later item fails.
- When implementing new CLI features or TUI models, do not duplicate mutation logic. Put domain/filesystem behavior in shared internal modules.
- Ensure cross-platform compatibility when writing paths (especially with tests, use temporary directories).
