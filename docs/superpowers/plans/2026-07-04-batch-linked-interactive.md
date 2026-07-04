# Batch, Linked Groups, and Interactive Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe unmanaged unlink choices, batch operations, linked active group handling, and a Textual-backed `x-skills interactive` MVP.

**Architecture:** Keep `src/x_skills/cli.py` as the command router, but extract reusable operation helpers inside the same module before adding the TUI. Add `src/x_skills/interactive.py` for the Textual app so the optional UI is isolated from non-interactive CLI paths while still using shared discovery helpers.

**Tech Stack:** Python 3.12, argparse, pathlib, pytest, ruff, uv, Textual.

---

### Task 1: Safe Unmanaged Unlink Choices

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- `unlink unmanaged --delete-unmanaged -y` removes the active directory without archiving.
- interactive `unlink unmanaged` accepts selection `2` and removes the directory without archiving.
- `unlink unmanaged -y` keeps current safe behavior: migrate first.
- `unlink unmanaged -n` cancels.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because `--delete-unmanaged` and the selection prompt do not exist.

- [ ] **Step 2: Implement unlink action resolution**

Add `--delete-unmanaged` to `unlink`. Replace the unmanaged yes/no prompt with a numbered choice in interactive mode. Keep `-y` as migrate-first and `-n` as cancel.

- [ ] **Step 3: Verify Task 1**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for unmanaged unlink tests.

### Task 2: Batch Skill Arguments

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- `link foo bar --target codex --project` links both names.
- `migrate foo bar --target codex --project -y` migrates both names.
- `unlink foo bar --target codex --project -y` unlinks both names.
- `repo remove foo bar -y` removes both repo skills.
- batch output includes `Summary:` with successful names.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because commands accept one name only.

- [ ] **Step 2: Implement batch command wrapper**

Change relevant parsers from `name` to `names` with `nargs="+"`. Add a small helper that executes one item at a time and prints summary only when more than one name was provided.

- [ ] **Step 3: Verify Task 2**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for batch tests.

### Task 3: Linked Active Group Detection

**Files:**
- Modify: `tests/test_cli.py`
- Modify: `src/x_skills/cli.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- two active locations resolving to the same directory are detected as a linked group.
- same-name separate directories are not grouped and still prompt location choice.
- migrating a linked group with group selection migrates the canonical source to repo and relinks all group entries to repo.

Run: `uv run pytest tests/test_cli.py -q`
Expected: FAIL because matching active skills are treated independently.

- [ ] **Step 2: Implement group analysis**

Add helper types/functions to classify same-name matches by resolved path. For linked groups, prompt whether to act on the group or a single location. Implement group migrate for the one canonical-directory case.

- [ ] **Step 3: Verify Task 3**

Run: `uv run pytest tests/test_cli.py -q`
Expected: PASS for linked group tests.

### Task 4: Textual Dependency and Interactive Command

**Files:**
- Modify: `pyproject.toml`
- Modify: `uv.lock`
- Create: `src/x_skills/interactive.py`
- Modify: `src/x_skills/cli.py`
- Modify: `tests/test_cli.py`
- Modify: `tests/test_install_docs.py`

- [ ] **Step 1: Write failing tests**

Add tests for:
- `pyproject.toml` includes `textual`.
- README says `uv` is required.
- `x-skills interactive --no-input` exits 2 with a clear error.
- `x-skills interactive` calls `x_skills.interactive.run_interactive` when importable.

Run: `uv run pytest tests/test_cli.py tests/test_install_docs.py -q`
Expected: FAIL because dependency and command do not exist.

- [ ] **Step 2: Add Textual dependency**

Run: `uv add textual`.

Expected: `pyproject.toml` and `uv.lock` update successfully.

- [ ] **Step 3: Implement interactive MVP**

Add `src/x_skills/interactive.py` with a simple Textual app that displays active skills in a table and supports quit/refresh. Add `cmd_interactive` in `cli.py` that rejects `--no-input`, imports Textual lazily, and calls the app.

- [ ] **Step 4: Verify Task 4**

Run: `uv run pytest tests/test_cli.py tests/test_install_docs.py -q`
Expected: PASS for interactive and dependency tests.

### Task 5: Docs and Final Verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README**

Document:
- `uv` and `git` prerequisites.
- batch arguments.
- unmanaged unlink action choices.
- linked group behavior.
- `x-skills interactive`.

- [ ] **Step 2: Final verification**

Run:
- `uv run pytest`
- `uv run ruff check .`
- `uv run ruff format --check .`
- `sh -n install.sh`

Expected: all commands exit 0.

- [ ] **Step 3: Commit and push**

Run:
- `git status --short`
- `git add .`
- `git commit -m "feat: add batch and interactive management"`
- `git push`

Expected: working tree clean and `main` synchronized with `origin/main`.
