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
    active_skill_groups,
    clean_broken_skills,
    install_repo_skill,
    install_search_result,
    repo_skill_rows,
    run_bulk_action,
    search_results,
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


def test_active_skill_groups_merge_identical_directories_by_fingerprint(tmp_path: Path) -> None:
    args = make_args(tmp_path)
    project_root = args.project_root / ".codex" / "skills"
    global_root = args.global_root
    project_skill = make_skill(project_root, "local-name", "Shared content.")
    global_skill = global_root / "global-name"
    global_skill.parent.mkdir(parents=True)
    os.symlink(project_skill, global_skill)
    changed = make_skill(args.claude_global_root, "local-name", "Changed content.")

    groups = active_skill_groups(args)

    shared = next(group for group in groups if len(group.skills) == 2)
    assert len(shared.skills) == 2
    assert {skill.name for skill in shared.skills} == {"local-name", "global-name"}
    assert shared.display_name == "local-name + global-name"
    assert shared.locations == "project codex, global agents"
    assert shared.status == "unmanaged"
    assert shared.fingerprint
    changed_group = next(group for group in groups if group.skills[0].path == changed)
    assert len(changed_group.skills) == 1


def test_active_skill_groups_reuses_fingerprint_cache(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    args = make_args(tmp_path)
    make_skill(args.project_root / ".codex" / "skills", "supergoal")
    calls: list[Path] = []

    def fake_directory_fingerprint(root: Path) -> str:
        calls.append(root)
        return "abc123"

    monkeypatch.setattr("x_skills.interactive._directory_fingerprint", fake_directory_fingerprint)
    cache: dict[Path, str] = {}

    first = active_skill_groups(args, cache)
    second = active_skill_groups(args, cache)

    assert first[0].fingerprint == "abc123"
    assert second[0].fingerprint == "abc123"
    assert len(calls) == 1


def test_toggle_selected_preserves_cursor_row_after_refresh(tmp_path: Path) -> None:
    app = XSkillsInteractive(make_args(tmp_path))
    group_key = "sha:abc123"
    moves: list[int] = []

    class FakeRowKey:
        value = group_key

    class FakeCellKey:
        row_key = FakeRowKey()

    class FakeTable:
        cursor_coordinate = (3, 0)

        def coordinate_to_cell_key(self, coordinate: tuple[int, int]) -> FakeCellKey:
            assert coordinate == (3, 0)
            return FakeCellKey()

        def move_cursor(self, *, row: int | None = None, column: int | None = None) -> None:
            assert column is None
            moves.append(row if row is not None else -1)

    app.mode = "active"
    app.table = FakeTable()
    app.active_rows[group_key] = []
    app.row_positions[group_key] = 3
    app._refresh_table = lambda *, preserve_row_key=None: app._restore_cursor(preserve_row_key)

    app.action_toggle_selected()

    assert app.selected_active == {group_key}
    assert moves == [3]


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


def test_clean_broken_action_cleans_all_broken_when_nothing_is_selected(
    tmp_path: Path, monkeypatch
) -> None:
    app = XSkillsInteractive(make_args(tmp_path))
    active_root = app.args.project_root / ".claude" / "skills"
    active_root.mkdir(parents=True)
    broken_path = active_root / "broken-one"
    os.symlink(tmp_path / "missing", broken_path)
    make_skill(active_root, "local-one")
    details: list[str] = []
    monkeypatch.setattr(app, "_set_detail", details.append)
    monkeypatch.setattr(app, "_refresh_table", lambda: None)

    app.action_clean_broken()

    assert not broken_path.exists()
    assert details == ["removed broken broken-one"]


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

    def fake_check_output(command: list[str], text: bool) -> str:
        return "abc123\n"

    monkeypatch.setattr("x_skills.cli.subprocess.run", fake_run)
    monkeypatch.setattr("x_skills.cli.subprocess.check_output", fake_check_output)

    installed = install_search_result(args, result)

    assert installed.name == "github-skill"
    assert (args.archive_root / "skills" / "github-skill" / "SKILL.md").is_file()
    assert not (args.archive_root / "skills" / "other-skill").exists()


def test_install_repo_skill_links_to_selected_destination(tmp_path: Path) -> None:
    args = make_args(tmp_path)
    repo_skill = make_skill(args.archive_root / "skills", "repo-skill", "From repo.")

    installed = install_repo_skill(args, "repo-skill", scope="global", target="codex")

    assert installed == args.codex_global_root / "repo-skill"
    assert installed.is_symlink()
    assert installed.resolve() == repo_skill


def test_repo_skill_rows_include_update_hint(tmp_path: Path) -> None:
    args = make_args(tmp_path)
    skill = make_skill(args.archive_root / "skills", "repo-skill", "From repo.")
    (skill / ".x-skills.json").write_text(
        '{"version":1,"source_type":"github","source":"owner/repo",'
        '"clone_url":"https://github.com/owner/repo.git","commit":"old",'
        '"skill_path":"skills/repo-skill"}',
        encoding="utf-8",
    )

    rows = repo_skill_rows(args, update_statuses={"repo-skill": "update available"})

    assert rows[0].name == "repo-skill"
    assert rows[0].update_status == "update available"
    assert (
        "x-skills repo add-github owner/repo skills/repo-skill --replace-archive" in rows[0].details
    )


def test_search_results_return_local_repo_matches_first(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    args = make_args(tmp_path)
    make_skill(args.archive_root / "skills", "react-local", "Local.")

    def fake_search_skills(
        query: str, *, owner: str | None = None, limit: int = 10
    ) -> list[cli.SearchResult]:
        assert query == "react"
        assert owner is None
        assert limit == 10
        return [
            cli.SearchResult(
                name="react-remote",
                slug="owner/repo/react-remote",
                source="owner/repo",
                installs=10,
            )
        ]

    monkeypatch.setattr("x_skills.interactive.cli.search_skills", fake_search_skills)

    results = search_results(args, "react")

    assert [result.kind for result in results] == ["local", "remote"]
    assert [result.name for result in results] == ["react-local", "react-remote"]


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
    monkeypatch.setattr(app, "_refresh_table", lambda **_: None)

    app.action_toggle_selected()

    assert app.selected_active == {row_key}
