# CLI UX Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved cwd-based `x-skills list` and `x-skills repo` UX, prompt policy, and pre-release command surface.

**Architecture:** Keep the CLI in `src/x_skills/cli.py` for this small pre-release codebase, but introduce focused helper functions for active-root discovery, frontmatter parsing, managed-state classification, prompts, and command routing. Preserve the existing standard-library-only dependency profile.

**Tech Stack:** Python 3.12, argparse, pathlib, pytest, ruff, uv.

---

### Task 1: Active List Dashboard

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests that create project/global skills under `.agents`, `.claude`, `.codex`, archive one skill in `~/.x-skills/skills`, and assert `x-skills list` prints grouped active skills with `managed`, `unmanaged`, and `broken` states.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because current `list` still means repo/global listing and does not scan all roots.

- [ ] **Step 2: Implement active root discovery and status classification**

Add helpers for:
- active roots across scopes and targets;
- valid skill detection without resolving broken symlinks away;
- managed/unmanaged/broken classification;
- frontmatter description extraction.

Update `cmd_list` so it lists active project and global roots by default.

- [ ] **Step 3: Verify Task 1**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for active list tests, with any unrelated old command tests updated or removed.

### Task 2: Repo Command

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for `x-skills repo`:
- prints archived skill names and descriptions from `SKILL.md` frontmatter;
- supports `--used` and `--unused` relative to current project/global roots;
- returns empty output when repo has no skills.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because `repo` does not exist.

- [ ] **Step 2: Implement `repo` parser and command**

Add an argparse `repo` subparser with default listing behavior and subcommands added in later tasks. Implement description extraction and active repo usage matching.

- [ ] **Step 3: Verify Task 2**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for list and repo tests.

### Task 3: Prompt Policy

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- `-y` and `-n` are mutually exclusive;
- non-interactive ambiguity fails with actionable commands;
- interactive selection reads from stdin when `input_stream` and `output_stream` are injected in tests.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because global prompt flags and prompt helpers do not exist.

- [ ] **Step 2: Implement prompt helpers**

Add global flags `-y/--yes`, `-n/--no`, `--no-input`, and `--json`. Add helpers for yes/no confirmation, non-interactive detection, and numbered selection. Keep `-y/-n` limited to yes/no confirmations.

- [ ] **Step 3: Verify Task 3**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for prompt policy tests.

### Task 4: Link, Migrate, and Unlink Redesign

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- `link NAME` prompts when scope/target are unspecified;
- `link NAME --target codex --project` links to `.codex/skills`;
- `migrate NAME` prompts when multiple active locations match;
- `migrate NAME --target codex --project -y` moves an unmanaged directory to repo and links it back;
- `unlink NAME --target codex --project -y` removes a managed symlink;
- `unlink NAME --target codex --project -n` cancels unmanaged directory handling.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because current behavior uses `-g` and does not implement the new prompt semantics.

- [ ] **Step 2: Implement redesigned active commands**

Remove the old `linked`, `status`, and `archive` command registrations. Implement `--project` as the explicit project-only scope filter, keep `--global` as global-only, and make them mutually exclusive. Update `link`, `migrate`, and `unlink` to resolve candidates via active roots and prompt on ambiguity.

- [ ] **Step 3: Verify Task 4**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for active command tests.

### Task 5: Repo Add and Remove

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- `x-skills repo add-github owner/repo path/to/skill` uses the existing GitHub install path;
- `x-skills repo add-url URL` uses the existing URL install path;
- `x-skills repo remove NAME --no-input` fails without `-y`;
- `x-skills repo remove NAME -y` removes the archived skill.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because repo subcommands are not registered yet.

- [ ] **Step 2: Implement repo subcommands**

Move current install handlers under `repo add-github` and `repo add-url`. Add `repo remove` with confirmation and current-active usage warning.

- [ ] **Step 3: Verify Task 5**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for repo subcommand tests.

### Task 6: Docs, Cleanup, and Final Verification

**Files:**
- Modify: `README.md`
- Modify: `tests/test_install_docs.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Update README tests**

Update doc tests so they assert the one-liner still exists and the README documents `list`, `repo`, and `repo add-github`.

Run: `uv run pytest tests/test_install_docs.py -q`
Expected: FAIL until README is updated.

- [ ] **Step 2: Update README and doctor output**

Document the new command surface and prompt flags. Update `doctor` to report repo/global/project roots and dependency availability.

- [ ] **Step 3: Final verification**

Run:
- `uv run pytest`
- `uv run ruff check .`
- `uv run ruff format --check .`
- `sh -n install.sh`

Expected: all commands exit 0.

- [ ] **Step 4: Commit implementation**

Run:
- `git status --short`
- `git add .`
- `git commit -m "feat: redesign CLI UX"`

Expected: one implementation commit after the spec and plan commits.
