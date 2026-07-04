from __future__ import annotations

import argparse
import io
import os
import subprocess
from pathlib import Path

import pytest

from x_skills import cli
from x_skills.interactive import (
    XSkillsInteractive,
    clean_broken_skills,
    install_search_result,
    run_bulk_action,
)


def make_args(tmp_path: Path) -> argparse.Namespace:
    return argparse.Namespace(
        archive_root=tmp_path / "archive",
        global_root=tmp_path / "global" / ".agents" / "skills",
        claude_global_root=tmp_path / "global" / ".claude" / "skills",
        codex_global_root=tmp_path / "global" / ".codex" / "skills",
        project_root=tmp_path / "project",
        global_=False,
        project_=False,
        target=None,
        yes=True,
        no=False,
        no_input=False,
        json_=False,
        color="never",
        output_stream=io.StringIO(),
        error_stream=io.StringIO(),
        input_stream=io.StringIO(),
    )


def make_skill(root: Path, name: str, description: str = "Use when testing.") -> Path:
    skill = root / name
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n\n# {name}\n",
        encoding="utf-8",
    )
    return skill


def test_run_bulk_action_unlinks_selected_exact_locations(tmp_path: Path) -> None:
    args = make_args(tmp_path)
    archived = make_skill(args.archive_root / "skills", "supergoal")
    active_root = args.project_root / ".codex" / "skills"
    active_root.mkdir(parents=True)
    active = active_root / "supergoal"
    os.symlink(archived, active)
    skill = cli._active_skills(args, [cli.ActiveRoot("project", "codex", active_root)])[0]

    results = run_bulk_action(args, [skill], "unlink")

    assert results == ["unlinked supergoal"]
    assert not active.exists()
    assert archived.is_dir()


def test_run_bulk_action_reports_skipped_migrate_for_managed_skill(tmp_path: Path) -> None:
    args = make_args(tmp_path)
    archived = make_skill(args.archive_root / "skills", "supergoal")
    active_root = args.project_root / ".codex" / "skills"
    active_root.mkdir(parents=True)
    active = active_root / "supergoal"
    os.symlink(archived, active)
    skill = cli._active_skills(args, [cli.ActiveRoot("project", "codex", active_root)])[0]

    results = run_bulk_action(args, [skill], "migrate")

    assert results == ["migrate skipped: supergoal"]
    assert active.is_symlink()


def test_clean_broken_skills_removes_only_selected_broken_symlinks(tmp_path: Path) -> None:
    args = make_args(tmp_path)
    active_root = args.project_root / ".claude" / "skills"
    active_root.mkdir(parents=True)
    broken_path = active_root / "broken-one"
    os.symlink(tmp_path / "missing", broken_path)
    unmanaged = make_skill(active_root, "local-one")
    skills = cli._active_skills(args, [cli.ActiveRoot("project", "claude", active_root)])

    results = clean_broken_skills(args, skills)

    assert results == ["removed broken broken-one"]
    assert not broken_path.exists()
    assert unmanaged.is_dir()


def test_install_search_result_archives_selected_skill(tmp_path: Path, monkeypatch) -> None:
    args = make_args(tmp_path)
    result = cli.SearchResult(
        name="github-skill",
        slug="owner/repo/github-skill",
        source="owner/repo",
        installs=5,
    )

    def fake_run(
        command: list[str], check: bool, stdout: object, timeout: int
    ) -> subprocess.CompletedProcess:
        checkout = Path(command[-1])
        make_skill(checkout / "skills", "github-skill", "From skills.sh.")
        make_skill(checkout / "skills", "other-skill", "Other.")
        return subprocess.CompletedProcess(command, 0)

    monkeypatch.setattr("x_skills.cli.subprocess.run", fake_run)

    installed = install_search_result(args, result)

    assert installed.name == "github-skill"
    assert (args.archive_root / "skills" / "github-skill" / "SKILL.md").is_file()
    assert not (args.archive_root / "skills" / "other-skill").exists()


def test_install_search_result_fails_when_selected_skill_is_missing(
    tmp_path: Path, monkeypatch
) -> None:
    args = make_args(tmp_path)
    result = cli.SearchResult(
        name="missing-skill",
        slug="owner/repo/missing-skill",
        source="owner/repo",
        installs=5,
    )

    def fake_run(
        command: list[str], check: bool, stdout: object, timeout: int
    ) -> subprocess.CompletedProcess:
        checkout = Path(command[-1])
        make_skill(checkout / "skills", "other-skill", "Other.")
        return subprocess.CompletedProcess(command, 0)

    monkeypatch.setattr("x_skills.cli.subprocess.run", fake_run)

    with pytest.raises(cli.XSkillsError, match='does not contain skill "missing-skill"'):
        install_search_result(args, result)


def test_back_action_returns_from_search_to_active_mode(tmp_path: Path, monkeypatch) -> None:
    app = XSkillsInteractive(make_args(tmp_path))
    app.mode = "search"
    refreshed: list[bool] = []

    def fake_refresh_table() -> None:
        assert app.mode == "active"
        refreshed.append(True)

    monkeypatch.setattr(app, "_refresh_table", fake_refresh_table)

    app.action_back()

    assert refreshed == [True]
    assert app.mode == "active"


def test_toggle_selected_uses_coordinate_cell_key(tmp_path: Path, monkeypatch) -> None:
    app = XSkillsInteractive(make_args(tmp_path))
    active_root = app.args.project_root / ".codex" / "skills"
    skill_path = make_skill(active_root, "supergoal")
    skill = cli.ActiveSkill(
        "supergoal",
        cli.ActiveRoot("project", "codex", active_root),
        skill_path,
        "unmanaged",
        "Use when testing.",
    )
    row_key = f"project:codex:{skill_path}"

    class FakeRowKey:
        value = row_key

    class FakeCellKey:
        row_key = FakeRowKey()

    class FakeTable:
        cursor_coordinate = (0, 0)

        def coordinate_to_cell_key(self, coordinate: tuple[int, int]) -> FakeCellKey:
            assert coordinate == (0, 0)
            return FakeCellKey()

    app.mode = "active"
    app.table = FakeTable()
    app.active_rows[row_key] = skill
    monkeypatch.setattr(app, "_refresh_table", lambda: None)

    app.action_toggle_selected()

    assert app.selected_active == {row_key}
