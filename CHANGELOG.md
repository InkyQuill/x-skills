# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Go rewrite of x-skills CLI with Cobra command tree and Bubble Tea TUI.
- `list`, `repo`, `link`, `migrate`, `unlink`, `doctor`, and `tui` commands.
- TUI with Active, Repo, Doctor, and Install pages.
- ADR directory for design decision records (`docs/adr/`).
- CLI destination selectors (compact spellings for active roots).
- Background repo update checks in Repo view.
- Advisory audit status display for remote skills.
- Git source ref tracking for manual installs.

### Changed

- Replaced Python/Textual prototype with Go implementation.
- Active rows merged by directory SHA fingerprint in TUI.
