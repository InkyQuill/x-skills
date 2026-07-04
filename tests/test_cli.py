from __future__ import annotations

import io
import os
import shutil
import subprocess
from pathlib import Path

import pytest

from x_skills.cli import main


class TtyStringIO(io.StringIO):
    def isatty(self) -> bool:
        return True


def run_cli(
    tmp_path: Path,
    *args: str,
    archive_root: Path | None = None,
    global_root: Path | None = None,
    claude_global_root: Path | None = None,
    codex_global_root: Path | None = None,
    project_root: Path | None = None,
    input_text: str = "",
    tty: bool = False,
) -> tuple[int | None, str, str]:
    stdout = TtyStringIO() if tty else io.StringIO()
    stderr = io.StringIO()
    argv = [
        "--archive-root",
        str(archive_root or tmp_path / "archive"),
        "--global-root",
        str(global_root or tmp_path / "global" / ".agents" / "skills"),
        "--claude-global-root",
        str(claude_global_root or tmp_path / "global" / ".claude" / "skills"),
        "--codex-global-root",
        str(codex_global_root or tmp_path / "global" / ".codex" / "skills"),
        "--project-root",
        str(project_root or tmp_path / "project"),
        *args,
    ]
    try:
        main(
            argv,
            input_stream=io.StringIO(input_text),
            output_stream=stdout,
            error_stream=stderr,
        )
    except SystemExit as error:
        return error.code, stdout.getvalue(), stderr.getvalue()
    return None, stdout.getvalue(), stderr.getvalue()


def make_skill(root: Path, name: str, description: str = "Use when testing.") -> Path:
    skill = root / name
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n\n# {name}\n",
        encoding="utf-8",
    )
    return skill


def test_list_shows_active_project_and_global_skills_with_management_status(
    tmp_path: Path,
) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    agents_global = tmp_path / "global" / ".agents" / "skills"
    managed = make_skill(archive / "skills", "managed-codex", "Managed codex skill.")
    codex_root = project / ".codex" / "skills"
    codex_root.mkdir(parents=True)
    os.symlink(managed, codex_root / "managed-codex")
    make_skill(project / ".claude" / "skills", "local-claude", "Local claude skill.")
    agents_global.mkdir(parents=True)
    os.symlink(tmp_path / "missing-skill", agents_global / "broken-agents")

    code, stdout, stderr = run_cli(
        tmp_path,
        "list",
        archive_root=archive,
        global_root=agents_global,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "PROJECT codex" in stdout
    assert "managed-codex" in stdout
    assert "managed" in stdout
    assert "Managed codex skill." in stdout
    assert "PROJECT claude" in stdout
    assert "local-claude" in stdout
    assert "unmanaged" in stdout
    assert "GLOBAL agents" in stdout
    assert "broken-agents" in stdout
    assert "broken" in stdout


def test_list_can_colorize_human_output(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    managed = make_skill(archive / "skills", "managed-codex", "Managed codex skill.")
    active_root = project / ".codex" / "skills"
    active_root.mkdir(parents=True)
    os.symlink(managed, active_root / "managed-codex")

    code, stdout, stderr = run_cli(
        tmp_path,
        "--color",
        "always",
        "list",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "\x1b[1;36mPROJECT codex" in stdout
    assert "\x1b[32mmanaged   \x1b[0m" in stdout


def test_list_colorizes_tty_output_by_default(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.delenv("NO_COLOR", raising=False)
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    managed = make_skill(archive / "skills", "managed-codex", "Managed codex skill.")
    active_root = project / ".codex" / "skills"
    active_root.mkdir(parents=True)
    os.symlink(managed, active_root / "managed-codex")

    code, stdout, stderr = run_cli(
        tmp_path,
        "list",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
        tty=True,
    )

    assert code is None
    assert stderr == ""
    assert "\x1b[1;36mPROJECT codex" in stdout


def test_list_explains_broken_symlink_reasons(tmp_path: Path) -> None:
    project = tmp_path / "project"
    active_root = project / ".claude" / "skills"
    active_root.mkdir(parents=True)
    os.symlink(tmp_path / "missing-target", active_root / "missing-link")
    not_skill = tmp_path / "not-a-skill"
    not_skill.mkdir()
    os.symlink(not_skill, active_root / "missing-skill-md")

    code, stdout, stderr = run_cli(
        tmp_path,
        "list",
        "--target",
        "claude",
        "--project",
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "missing-link" in stdout
    assert "broken" in stdout
    assert "target missing:" in stdout
    assert "missing-target" in stdout
    assert "missing-skill-md" in stdout
    assert "target missing SKILL.md:" in stdout


def test_repo_lists_archived_skills_with_descriptions_and_usage_filters(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    used = make_skill(archive / "skills", "used-skill", "Already linked.")
    make_skill(archive / "skills", "unused-skill", "Not linked.")
    active_root = project / ".agents" / "skills"
    active_root.mkdir(parents=True)
    os.symlink(used, active_root / "used-skill")

    code, stdout, _ = run_cli(tmp_path, "repo", archive_root=archive, project_root=project)
    assert code is None
    assert "used-skill" in stdout
    assert "Already linked." in stdout
    assert "unused-skill" in stdout
    assert "Not linked." in stdout

    code, stdout, _ = run_cli(
        tmp_path, "repo", "--used", archive_root=archive, project_root=project
    )
    assert code is None
    assert "used-skill" in stdout
    assert "unused-skill" not in stdout

    code, stdout, _ = run_cli(
        tmp_path, "repo", "--unused", archive_root=archive, project_root=project
    )
    assert code is None
    lines = stdout.splitlines()
    assert not any(line.startswith("used-skill") for line in lines)
    assert any(line.startswith("unused-skill") for line in lines)


def test_yes_and_no_flags_are_mutually_exclusive(tmp_path: Path) -> None:
    code, _, stderr = run_cli(tmp_path, "-y", "-n", "list")

    assert code == 2
    assert "not allowed with argument" in stderr


def test_migrate_fails_non_interactively_when_active_skill_name_is_ambiguous(
    tmp_path: Path,
) -> None:
    project = tmp_path / "project"
    agents_global = tmp_path / "global" / ".agents" / "skills"
    make_skill(project / ".codex" / "skills", "svelte-coder")
    make_skill(agents_global, "svelte-coder")

    code, _, stderr = run_cli(
        tmp_path,
        "--no-input",
        "migrate",
        "svelte-coder",
        project_root=project,
        global_root=agents_global,
    )

    assert code == 2
    assert 'multiple active skills named "svelte-coder"' in stderr
    assert "x-skills migrate svelte-coder --target codex --project" in stderr
    assert "x-skills migrate svelte-coder --target agents --global" in stderr


def test_migrate_prompts_for_ambiguous_active_skill_and_confirmation(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    agents_global = tmp_path / "global" / ".agents" / "skills"
    project_skill = make_skill(project / ".codex" / "skills", "svelte-coder")
    make_skill(agents_global, "svelte-coder")

    code, stdout, stderr = run_cli(
        tmp_path,
        "migrate",
        "svelte-coder",
        archive_root=archive,
        project_root=project,
        global_root=agents_global,
        input_text="1\ny\n",
    )

    assert code is None
    assert stderr == ""
    assert "Select skill to migrate [1-2]:" in stdout
    assert 'Migrate project codex skill "svelte-coder" into repo? [y/N]:' in stdout
    archived = archive / "skills" / "svelte-coder"
    assert archived.is_dir()
    assert project_skill.is_symlink()
    assert project_skill.resolve() == archived


def test_migrate_explicit_project_skill_with_yes_flag(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    active = make_skill(project / ".codex" / "skills", "next-best-practices")

    code, _, stderr = run_cli(
        tmp_path,
        "-y",
        "migrate",
        "next-best-practices",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    archived = archive / "skills" / "next-best-practices"
    assert archived.is_dir()
    assert active.is_symlink()
    assert active.resolve() == archived


def test_migrate_accepts_multiple_skill_names_and_prints_summary(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    first = make_skill(project / ".codex" / "skills", "first-skill")
    second = make_skill(project / ".codex" / "skills", "second-skill")

    code, stdout, stderr = run_cli(
        tmp_path,
        "-y",
        "migrate",
        "first-skill",
        "second-skill",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "Summary:" in stdout
    assert "migrated: first-skill, second-skill" in stdout
    assert first.is_symlink()
    assert second.is_symlink()
    assert (archive / "skills" / "first-skill").is_dir()
    assert (archive / "skills" / "second-skill").is_dir()


def test_migrate_linked_active_group_relinks_all_entries_to_repo(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    agents_global = tmp_path / "global" / ".agents" / "skills"
    claude_global = tmp_path / "global" / ".claude" / "skills"
    agents_skill = make_skill(agents_global, "shared-skill")
    claude_global.mkdir(parents=True)
    claude_skill = claude_global / "shared-skill"
    os.symlink(agents_skill, claude_skill)

    code, stdout, stderr = run_cli(
        tmp_path,
        "migrate",
        "shared-skill",
        "--global",
        archive_root=archive,
        global_root=agents_global,
        claude_global_root=claude_global,
        input_text="1\ny\n",
    )

    archived = archive / "skills" / "shared-skill"
    assert code is None
    assert stderr == ""
    assert "Found linked setup" in stdout
    assert archived.is_dir()
    assert agents_skill.is_symlink()
    assert claude_skill.is_symlink()
    assert agents_skill.resolve() == archived
    assert claude_skill.resolve() == archived


def test_migrate_same_name_separate_copies_are_not_grouped(tmp_path: Path) -> None:
    project = tmp_path / "project"
    agents_global = tmp_path / "global" / ".agents" / "skills"
    make_skill(project / ".codex" / "skills", "shared-skill")
    make_skill(agents_global, "shared-skill")

    code, _, stderr = run_cli(
        tmp_path,
        "--no-input",
        "migrate",
        "shared-skill",
        project_root=project,
        global_root=agents_global,
    )

    assert code == 2
    assert "multiple active skills" in stderr
    assert "linked setup" not in stderr


def test_link_explicit_project_target(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    skill = make_skill(archive / "skills", "typescript-expert")

    code, _, stderr = run_cli(
        tmp_path,
        "link",
        "typescript-expert",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    target = project / ".codex" / "skills" / "typescript-expert"
    assert target.is_symlink()
    assert target.resolve() == skill


def test_link_accepts_multiple_skill_names_and_prints_summary(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    first = make_skill(archive / "skills", "first-skill")
    second = make_skill(archive / "skills", "second-skill")

    code, stdout, stderr = run_cli(
        tmp_path,
        "link",
        "first-skill",
        "second-skill",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "Summary:" in stdout
    assert "linked: first-skill, second-skill" in stdout
    first_target = project / ".codex" / "skills" / "first-skill"
    second_target = project / ".codex" / "skills" / "second-skill"
    assert first_target.resolve() == first
    assert second_target.resolve() == second


def test_link_fails_non_interactively_when_destination_is_ambiguous(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    make_skill(archive / "skills", "typescript-expert")

    code, _, stderr = run_cli(
        tmp_path,
        "--no-input",
        "link",
        "typescript-expert",
        archive_root=archive,
    )

    assert code == 2
    assert "choose a destination" in stderr
    assert "x-skills link typescript-expert --target codex --project" in stderr


def test_unlink_managed_symlink_with_yes_flag(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    skill = make_skill(archive / "skills", "opentui-react")
    active_root = project / ".codex" / "skills"
    active_root.mkdir(parents=True)
    target = active_root / "opentui-react"
    os.symlink(skill, target)

    code, _, stderr = run_cli(
        tmp_path,
        "-y",
        "unlink",
        "opentui-react",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert not target.exists()
    assert skill.is_dir()


def test_unlink_accepts_multiple_skill_names_and_prints_summary(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    project = tmp_path / "project"
    first = make_skill(archive / "skills", "first-skill")
    second = make_skill(archive / "skills", "second-skill")
    active_root = project / ".codex" / "skills"
    active_root.mkdir(parents=True)
    first_target = active_root / "first-skill"
    second_target = active_root / "second-skill"
    os.symlink(first, first_target)
    os.symlink(second, second_target)

    code, stdout, stderr = run_cli(
        tmp_path,
        "-y",
        "unlink",
        "first-skill",
        "second-skill",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "Summary:" in stdout
    assert "unlinked: first-skill, second-skill" in stdout
    assert not first_target.exists()
    assert not second_target.exists()


def test_unlink_unmanaged_directory_with_no_flag_cancels(tmp_path: Path) -> None:
    project = tmp_path / "project"
    active = make_skill(project / ".codex" / "skills", "local-only")

    code, stdout, stderr = run_cli(
        tmp_path,
        "-n",
        "unlink",
        "local-only",
        "--target",
        "codex",
        "--project",
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "cancelled" in stdout
    assert active.is_dir()


def test_unlink_unmanaged_directory_with_delete_flag_and_yes_removes_active_copy(
    tmp_path: Path,
) -> None:
    project = tmp_path / "project"
    archive = tmp_path / "archive"
    active = make_skill(project / ".codex" / "skills", "local-only")

    code, stdout, stderr = run_cli(
        tmp_path,
        "-y",
        "unlink",
        "local-only",
        "--target",
        "codex",
        "--project",
        "--delete-unmanaged",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "removed unmanaged" in stdout
    assert not active.exists()
    assert not (archive / "skills" / "local-only").exists()


def test_unlink_unmanaged_directory_interactive_delete_choice_removes_active_copy(
    tmp_path: Path,
) -> None:
    project = tmp_path / "project"
    archive = tmp_path / "archive"
    active = make_skill(project / ".codex" / "skills", "local-only")

    code, stdout, stderr = run_cli(
        tmp_path,
        "unlink",
        "local-only",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
        input_text="2\n",
    )

    assert code is None
    assert stderr == ""
    assert "Choose action" in stdout
    assert "unlink without migration" in stdout
    assert "removed unmanaged" in stdout
    assert not active.exists()
    assert not (archive / "skills" / "local-only").exists()


def test_unlink_unmanaged_directory_with_yes_migrates_first(tmp_path: Path) -> None:
    project = tmp_path / "project"
    archive = tmp_path / "archive"
    active = make_skill(project / ".codex" / "skills", "local-only")

    code, stdout, stderr = run_cli(
        tmp_path,
        "-y",
        "unlink",
        "local-only",
        "--target",
        "codex",
        "--project",
        archive_root=archive,
        project_root=project,
    )

    assert code is None
    assert stderr == ""
    assert "migrated and unlinked" in stdout
    assert not active.exists()
    assert (archive / "skills" / "local-only").is_dir()


def test_repo_remove_requires_yes_in_non_interactive_mode(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    make_skill(archive / "skills", "old-skill")

    code, _, stderr = run_cli(
        tmp_path,
        "--no-input",
        "repo",
        "remove",
        "old-skill",
        archive_root=archive,
    )

    assert code == 2
    assert "refusing to remove repo skill without confirmation" in stderr
    assert (archive / "skills" / "old-skill").is_dir()


def test_repo_remove_with_yes_flag_removes_archived_skill(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    make_skill(archive / "skills", "old-skill")

    code, _, stderr = run_cli(tmp_path, "-y", "repo", "remove", "old-skill", archive_root=archive)

    assert code is None
    assert stderr == ""
    assert not (archive / "skills" / "old-skill").exists()


def test_repo_remove_accepts_multiple_skill_names_and_prints_summary(tmp_path: Path) -> None:
    archive = tmp_path / "archive"
    make_skill(archive / "skills", "first-skill")
    make_skill(archive / "skills", "second-skill")

    code, stdout, stderr = run_cli(
        tmp_path,
        "-y",
        "repo",
        "remove",
        "first-skill",
        "second-skill",
        archive_root=archive,
    )

    assert code is None
    assert stderr == ""
    assert "Summary:" in stdout
    assert "removed: first-skill, second-skill" in stdout
    assert not (archive / "skills" / "first-skill").exists()
    assert not (archive / "skills" / "second-skill").exists()


def test_repo_add_url_installs_direct_skill_md(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    source = tmp_path / "remote-SKILL.md"
    source.write_text(
        "---\nname: remote-skill\ndescription: From URL.\n---\n\n# Remote\n",
        encoding="utf-8",
    )
    archive = tmp_path / "archive"

    def fake_urlretrieve(url: str, filename: Path) -> tuple[Path, None]:
        shutil.copy2(source, filename)
        return filename, None

    monkeypatch.setattr("x_skills.cli.urllib.request.urlretrieve", fake_urlretrieve)

    code, _, stderr = run_cli(
        tmp_path,
        "repo",
        "add-url",
        "https://example.com/SKILL.md",
        archive_root=archive,
    )

    assert code is None
    assert stderr == ""
    assert (archive / "skills" / "remote-skill" / "SKILL.md").is_file()


def test_repo_add_github_installs_skill_from_repo_path(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    archive = tmp_path / "archive"

    def fake_run(command: list[str], check: bool, stdout: object) -> subprocess.CompletedProcess:
        checkout = Path(command[-1])
        make_skill(checkout / "skills", "github-skill", "From GitHub.")
        return subprocess.CompletedProcess(command, 0)

    monkeypatch.setattr("x_skills.cli.subprocess.run", fake_run)

    code, _, stderr = run_cli(
        tmp_path,
        "repo",
        "add-github",
        "owner/repo",
        "skills/github-skill",
        archive_root=archive,
    )

    assert code is None
    assert stderr == ""
    assert (archive / "skills" / "github-skill" / "SKILL.md").is_file()


def test_interactive_rejects_no_input(tmp_path: Path) -> None:
    code, _, stderr = run_cli(tmp_path, "--no-input", "interactive")

    assert code == 2
    assert "interactive cannot run with --no-input" in stderr


def test_interactive_dispatches_to_textual_app(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    called: list[bool] = []

    def fake_run_interactive(args: object) -> None:
        called.append(True)

    monkeypatch.setattr("x_skills.cli._run_interactive_app", fake_run_interactive, raising=False)

    code, _, stderr = run_cli(tmp_path, "interactive")

    assert code is None
    assert stderr == ""
    assert called == [True]
