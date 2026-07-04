from __future__ import annotations

import os
from pathlib import Path

import pytest

from x_skills.cli import main


def run_cli(
    tmp_path: Path,
    *args: str,
    archive_root: Path | None = None,
    global_root: Path | None = None,
    claude_global_root: Path | None = None,
    codex_global_root: Path | None = None,
    project_root: Path | None = None,
) -> None:
    argv = [
        "--archive-root",
        str(archive_root or tmp_path / "archive"),
        "--global-root",
        str(global_root or tmp_path / "global" / "skills"),
        "--claude-global-root",
        str(claude_global_root or tmp_path / "claude-global" / "skills"),
        "--codex-global-root",
        str(codex_global_root or tmp_path / "codex-global" / "skills"),
        "--project-root",
        str(project_root or tmp_path / "project"),
        *args,
    ]
    main(argv)


def make_skill(root: Path, name: str, description: str = "Use when testing.") -> Path:
    skill = root / name
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n\n# {name}\n",
        encoding="utf-8",
    )
    return skill


def test_links_archive_skill_into_project(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    skill = make_skill(archive / "skills", "opentui-react")

    run_cli(tmp_path, "link", "opentui-react", archive_root=archive, project_root=project)

    target = project / ".agents" / "skills" / "opentui-react"
    assert target.is_symlink()
    assert target.resolve() == skill


def test_links_archive_skill_into_global_set_with_g_flag(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    global_root = tmp_path / "global" / "skills"
    skill = make_skill(archive / "skills", "temporal-js")

    run_cli(tmp_path, "link", "-g", "temporal-js", archive_root=archive, global_root=global_root)

    target = global_root / "temporal-js"
    assert target.is_symlink()
    assert target.resolve() == skill


def test_links_archive_skill_into_project_codex_target(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    skill = make_skill(archive / "skills", "typescript-expert")

    run_cli(
        tmp_path,
        "link",
        "--target",
        "codex",
        "typescript-expert",
        archive_root=archive,
        project_root=project,
    )

    target = project / ".codex" / "skills" / "typescript-expert"
    assert target.is_symlink()
    assert target.resolve() == skill


def test_links_archive_skill_into_global_claude_target(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    claude_global_root = tmp_path / "home" / ".claude" / "skills"
    skill = make_skill(archive / "skills", "ui-ux-designer")

    run_cli(
        tmp_path,
        "link",
        "-g",
        "--target",
        "claude",
        "ui-ux-designer",
        archive_root=archive,
        claude_global_root=claude_global_root,
    )

    target = claude_global_root / "ui-ux-designer"
    assert target.is_symlink()
    assert target.resolve() == skill


def test_status_reports_whether_active_skills_are_archived(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    archived = make_skill(archive / "skills", "opentui")
    active_root = project / ".agents" / "skills"
    active_root.mkdir(parents=True)
    os.symlink(archived, active_root / "opentui")
    make_skill(active_root, "local-only")

    run_cli(tmp_path, "status", archive_root=archive, project_root=project)

    assert capsys.readouterr().out.splitlines() == [
        "local-only missing",
        "opentui archived linked",
    ]


def test_migrate_moves_project_skill_to_archive_and_replaces_it_with_link(
    tmp_path: Path,
) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    active = make_skill(project / ".codex" / "skills", "next-best-practices")

    run_cli(
        tmp_path,
        "migrate",
        "--target",
        "codex",
        "next-best-practices",
        archive_root=archive,
        project_root=project,
    )

    archived = archive / "skills" / "next-best-practices"
    assert archived.is_dir()
    assert (archived / "SKILL.md").exists()
    assert active.is_symlink()
    assert active.resolve() == archived


def test_unlink_global_archives_plain_directory_before_removing(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    global_root = tmp_path / "global" / "skills"
    plain_skill = make_skill(global_root, "ui-ux-pro-max")

    run_cli(
        tmp_path, "unlink", "-g", "ui-ux-pro-max", archive_root=archive, global_root=global_root
    )

    assert not plain_skill.exists()
    archived = archive / "skills" / "ui-ux-pro-max"
    assert archived.is_dir()
    assert (archived / "SKILL.md").exists()


def test_unlink_plain_directory_refuses_to_overwrite_archive(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    global_root = tmp_path / "global" / "skills"
    make_skill(archive / "skills", "ui-ux-pro-max", "Use archived.")
    active = make_skill(global_root, "ui-ux-pro-max", "Use active.")

    with pytest.raises(SystemExit) as error:
        run_cli(
            tmp_path,
            "unlink",
            "-g",
            "ui-ux-pro-max",
            archive_root=archive,
            global_root=global_root,
        )

    assert error.value.code == 2
    assert active.is_dir()
    assert (archive / "skills" / "ui-ux-pro-max").is_dir()


def test_unlink_symlink_removes_only_link(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    global_root = tmp_path / "global" / "skills"
    skill = make_skill(archive / "skills", "pixijs")
    global_root.mkdir(parents=True)
    os.symlink(skill, global_root / "pixijs")

    run_cli(tmp_path, "unlink", "-g", "pixijs", archive_root=archive, global_root=global_root)

    assert not (global_root / "pixijs").exists()
    assert skill.is_dir()


def test_archive_existing_skill_directory(tmp_path: Path) -> None:
    source = make_skill(tmp_path / "source", "control-cli")
    archive = tmp_path / "archive"

    run_cli(tmp_path, "archive", str(source), archive_root=archive)

    assert not source.exists()
    assert (archive / "skills" / "control-cli" / "SKILL.md").exists()


def test_archive_requires_skill_md(tmp_path: Path) -> None:
    source = tmp_path / "not-a-skill"
    source.mkdir()

    with pytest.raises(SystemExit) as error:
        run_cli(tmp_path, "archive", str(source))

    assert error.value.code == 2
