from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path
from urllib.parse import urlparse

TARGETS = ("agents", "claude", "codex")


class XSkillsError(RuntimeError):
    """Raised for user-facing command errors."""


def main(argv: list[str] | None = None) -> None:
    parser = _build_parser()
    args = parser.parse_args(argv)
    try:
        args.func(args)
    except XSkillsError as error:
        parser.exit(2, f"x-skills: {error}\n")


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="x-skills")
    parser.add_argument(
        "--archive-root",
        type=Path,
        default=Path(os.environ.get("X_SKILLS_HOME", "~/.x-skills")).expanduser(),
        help="archive root; default: ~/.x-skills",
    )
    parser.add_argument(
        "--global-root",
        type=Path,
        default=Path(os.environ.get("X_SKILLS_GLOBAL_ROOT", "~/.agents/skills")).expanduser(),
        help="global active skills root for --target agents; default: ~/.agents/skills",
    )
    parser.add_argument(
        "--claude-global-root",
        type=Path,
        default=Path(
            os.environ.get("X_SKILLS_CLAUDE_GLOBAL_ROOT", "~/.claude/skills")
        ).expanduser(),
        help="global active skills root for --target claude; default: ~/.claude/skills",
    )
    parser.add_argument(
        "--codex-global-root",
        type=Path,
        default=Path(os.environ.get("X_SKILLS_CODEX_GLOBAL_ROOT", "~/.codex/skills")).expanduser(),
        help="global active skills root for --target codex; default: ~/.codex/skills",
    )
    parser.add_argument(
        "--project-root",
        type=Path,
        default=Path.cwd(),
        help="project root for project links; default: current directory",
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    list_parser = subparsers.add_parser(
        "list", help="list archived skills, or global active skills with -g"
    )
    list_parser.add_argument(
        "-g",
        "--global",
        action="store_true",
        dest="global_",
        help="list global active skills",
    )
    _add_target_arg(list_parser)
    list_parser.set_defaults(func=cmd_list)

    linked_parser = subparsers.add_parser("linked", help="list project/global active skills")
    _add_scope_args(linked_parser, "use global active root")
    linked_parser.set_defaults(func=cmd_linked)

    status_parser = subparsers.add_parser(
        "status", help="show whether active skills are present in the archive"
    )
    _add_scope_args(status_parser, "check global active root")
    status_parser.set_defaults(func=cmd_status)

    link_parser = subparsers.add_parser("link", help="link an archived skill into active skills")
    _add_scope_args(link_parser, "link into global active root")
    link_parser.add_argument("name")
    link_parser.set_defaults(func=cmd_link)

    unlink_parser = subparsers.add_parser(
        "unlink", help="remove active skill link, archiving copies first"
    )
    _add_scope_args(unlink_parser, "unlink from global active root")
    unlink_parser.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    unlink_parser.add_argument("--archive-as", help="archive active copy under another name")
    unlink_parser.add_argument("name")
    unlink_parser.set_defaults(func=cmd_unlink)

    archive_parser = subparsers.add_parser(
        "archive", help="move a skill directory into the archive"
    )
    archive_parser.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    archive_parser.add_argument("--archive-as", help="archive under another name")
    archive_parser.add_argument("source")
    archive_parser.set_defaults(func=cmd_archive)

    migrate_parser = subparsers.add_parser(
        "migrate", help="move an active skill into the archive and link it back"
    )
    _add_scope_args(migrate_parser, "migrate from and link into global active root")
    migrate_parser.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    migrate_parser.add_argument("--archive-as", help="archive under another name")
    migrate_parser.add_argument("source", help="active skill name or skill directory path")
    migrate_parser.set_defaults(func=cmd_migrate)

    install_github = subparsers.add_parser(
        "install-github", help="install a skill from a GitHub repo"
    )
    install_github.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    install_github.add_argument("repo_or_url")
    install_github.add_argument("skill_path", nargs="?")
    install_github.set_defaults(func=cmd_install_github)

    install_url = subparsers.add_parser(
        "install-url", help="install a skill from an archive or SKILL.md URL"
    )
    install_url.add_argument("--replace-archive", action="store_true", help="replace archived copy")
    install_url.add_argument("url")
    install_url.set_defaults(func=cmd_install_url)

    doctor = subparsers.add_parser("doctor", help="check configured roots")
    doctor.set_defaults(func=cmd_doctor)

    return parser


def _add_target_arg(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        "--target",
        choices=TARGETS,
        default="agents",
        help="active skill target; default: agents",
    )


def _add_scope_args(parser: argparse.ArgumentParser, global_help: str) -> None:
    parser.add_argument(
        "-g",
        "--global",
        action="store_true",
        dest="global_",
        help=global_help,
    )
    _add_target_arg(parser)


def cmd_list(args: argparse.Namespace) -> None:
    root = _global_skills_root(args) if args.global_ else _archive_skills_root(args)
    for name in _skill_names(root):
        print(name)


def cmd_linked(args: argparse.Namespace) -> None:
    root = _active_root(args)
    for name in _skill_names(root):
        print(name)


def cmd_status(args: argparse.Namespace) -> None:
    archive_root = _archive_skills_root(args)
    for active in _skill_paths(_active_root(args)):
        archived = archive_root / active.name
        state = "archived" if _is_skill_dir(archived) else "missing"
        if active.is_symlink() and _same_resolved_path(active, archived):
            state = f"{state} linked"
        print(f"{active.name} {state}")


def cmd_link(args: argparse.Namespace) -> None:
    source = _archive_skill(args, args.name)
    if not _is_skill_dir(source):
        raise XSkillsError(f"archived skill not found: {args.name}")

    target = _active_root(args) / args.name
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.exists() or target.is_symlink():
        if target.is_symlink() and target.resolve() == source.resolve():
            print(f"already linked: {args.name}")
            return
        raise XSkillsError(f"active skill already exists: {target}")

    os.symlink(source, target, target_is_directory=True)
    print(f"linked: {args.name} -> {target}")


def cmd_unlink(args: argparse.Namespace) -> None:
    active = _active_root(args) / args.name
    if not active.exists() and not active.is_symlink():
        raise XSkillsError(f"active skill not found: {active}")

    if active.is_symlink():
        if not _archive_skill(args, args.name).exists():
            _archive_copy(active.resolve(), args, args.name)
        active.unlink()
        print(f"unlinked: {args.name}")
        return

    archive_name = args.archive_as or args.name
    _archive_move(active, args, archive_name)
    print(f"archived and unlinked: {args.name}")


def cmd_archive(args: argparse.Namespace) -> None:
    source = Path(args.source).expanduser()
    if not _is_skill_dir(source):
        raise XSkillsError(f"not a skill directory: {source}")
    archive_name = args.archive_as or source.name
    _archive_move(source, args, archive_name)
    print(f"archived: {archive_name}")


def cmd_migrate(args: argparse.Namespace) -> None:
    source, link_path, link_name = _migration_paths(args)
    archive_name = args.archive_as or link_name

    if source.is_symlink():
        archived = _archive_skill(args, archive_name)
        if _same_resolved_path(source, archived):
            print(f"already migrated: {link_name}")
            return
        raise XSkillsError(f"active skill is already a symlink: {source}")

    if not _is_skill_dir(source):
        raise XSkillsError(f"not a skill directory: {source}")

    archived = _archive_move(source, args, archive_name)
    _link_skill(archived, link_path, link_name)
    print(f"migrated: {link_name} -> {archived}")


def cmd_install_github(args: argparse.Namespace) -> None:
    clone_url, skill_path = _github_clone_url_and_path(args.repo_or_url, args.skill_path)
    with tempfile.TemporaryDirectory(prefix="x-skills-github-") as tmp:
        checkout = Path(tmp) / "repo"
        subprocess.run(
            ["git", "clone", "--depth", "1", clone_url, str(checkout)],
            check=True,
            stdout=subprocess.DEVNULL,
        )
        source = checkout / skill_path if skill_path else _find_single_skill(checkout)
        _install_skill_copy(source, args)
        print(f"installed: {source.name}")


def cmd_install_url(args: argparse.Namespace) -> None:
    with tempfile.TemporaryDirectory(prefix="x-skills-url-") as tmp:
        tmp_path = Path(tmp)
        downloaded = tmp_path / "download"
        urllib.request.urlretrieve(args.url, downloaded)
        source = _extract_or_materialize_url(downloaded, args.url, tmp_path)
        _install_skill_copy(source, args)
        print(f"installed: {source.name}")


def cmd_doctor(args: argparse.Namespace) -> None:
    print(f"archive: {_archive_skills_root(args)}")
    for target in TARGETS:
        print(f"global {target}:  {_global_skills_root(args, target)}")
    for target in TARGETS:
        print(f"project {target}: {_project_skills_root(args, target)}")


def _archive_skills_root(args: argparse.Namespace) -> Path:
    return args.archive_root.expanduser() / "skills"


def _archive_skill(args: argparse.Namespace, name: str) -> Path:
    return _archive_skills_root(args) / name


def _project_skills_root(args: argparse.Namespace, target: str | None = None) -> Path:
    return args.project_root.expanduser() / f".{target or args.target}" / "skills"


def _global_skills_root(args: argparse.Namespace, target: str | None = None) -> Path:
    match target or args.target:
        case "agents":
            return args.global_root.expanduser()
        case "claude":
            return args.claude_global_root.expanduser()
        case "codex":
            return args.codex_global_root.expanduser()
        case unknown:
            raise XSkillsError(f"unknown target: {unknown}")


def _active_root(args: argparse.Namespace) -> Path:
    return _global_skills_root(args) if args.global_ else _project_skills_root(args)


def _skill_names(root: Path) -> list[str]:
    return [path.name for path in _skill_paths(root)]


def _skill_paths(root: Path) -> list[Path]:
    if not root.exists():
        return []
    return sorted(
        (path for path in root.iterdir() if _is_skill_dir_or_link(path)),
        key=lambda path: path.name,
    )


def _is_skill_dir(path: Path) -> bool:
    return path.is_dir() and (path / "SKILL.md").is_file()


def _is_skill_dir_or_link(path: Path) -> bool:
    if path.is_symlink():
        try:
            return _is_skill_dir(path.resolve())
        except OSError:
            return False
    return _is_skill_dir(path)


def _same_resolved_path(left: Path, right: Path) -> bool:
    if not right.exists() and not right.is_symlink():
        return False
    try:
        return left.resolve() == right.resolve()
    except OSError:
        return False


def _archive_move(source: Path, args: argparse.Namespace, archive_name: str) -> Path:
    destination = _archive_skill(args, archive_name)
    _prepare_archive_destination(destination, replace=args.replace_archive)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.move(str(source), destination)
    return destination


def _archive_copy(source: Path, args: argparse.Namespace, archive_name: str) -> Path:
    if not _is_skill_dir(source):
        raise XSkillsError(f"symlink target is not a skill directory: {source}")
    destination = _archive_skill(args, archive_name)
    _prepare_archive_destination(destination, replace=args.replace_archive)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(source, destination, symlinks=True)
    return destination


def _migration_paths(args: argparse.Namespace) -> tuple[Path, Path, str]:
    source_arg = Path(args.source).expanduser()
    if source_arg.exists() or source_arg.is_absolute() or len(source_arg.parts) > 1:
        link_name = args.archive_as or source_arg.name
        return source_arg, _active_root(args) / link_name, link_name

    link_name = args.source
    source = _active_root(args) / link_name
    return source, source, link_name


def _link_skill(source: Path, target: Path, name: str) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.exists() or target.is_symlink():
        if target.is_symlink() and target.resolve() == source.resolve():
            return
        raise XSkillsError(f"active skill already exists: {target}")
    os.symlink(source, target, target_is_directory=True)


def _install_skill_copy(source: Path, args: argparse.Namespace) -> Path:
    if not _is_skill_dir(source):
        raise XSkillsError(f"not a skill directory: {source}")
    destination = _archive_skill(args, source.name)
    _prepare_archive_destination(destination, replace=args.replace_archive)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(source, destination, symlinks=True)
    return destination


def _prepare_archive_destination(destination: Path, *, replace: bool) -> None:
    if not destination.exists() and not destination.is_symlink():
        return
    if not replace:
        raise XSkillsError(
            f"archive already contains {destination.name}; use --replace-archive or --archive-as",
        )
    if destination.is_symlink() or destination.is_file():
        destination.unlink()
    else:
        shutil.rmtree(destination)


def _github_clone_url_and_path(
    repo_or_url: str, explicit_path: str | None
) -> tuple[str, Path | None]:
    if repo_or_url.startswith(("http://", "https://", "git@")):
        parsed = urlparse(repo_or_url)
        if parsed.netloc == "github.com" and "/tree/" in parsed.path and explicit_path is None:
            repo_path, _, skill_path = parsed.path.strip("/").partition("/tree/")
            parts = skill_path.split("/", 1)
            path = Path(parts[1]) if len(parts) == 2 else None
            return f"https://github.com/{repo_path}.git", path
        return repo_or_url, Path(explicit_path) if explicit_path else None
    return f"https://github.com/{repo_or_url}.git", Path(explicit_path) if explicit_path else None


def _find_single_skill(root: Path) -> Path:
    if _is_skill_dir(root):
        return root
    matches = [path for path in root.rglob("SKILL.md") if ".git" not in path.parts]
    if len(matches) != 1:
        raise XSkillsError("GitHub repo must contain exactly one SKILL.md or pass skill_path")
    return matches[0].parent


def _extract_or_materialize_url(downloaded: Path, url: str, tmp_path: Path) -> Path:
    if zipfile.is_zipfile(downloaded):
        extract_root = tmp_path / "extract"
        with zipfile.ZipFile(downloaded) as archive:
            archive.extractall(extract_root)
        return _find_single_skill(extract_root)

    if tarfile.is_tarfile(downloaded):
        extract_root = tmp_path / "extract"
        with tarfile.open(downloaded) as archive:
            archive.extractall(extract_root, filter="data")
        return _find_single_skill(extract_root)

    filename = Path(urlparse(url).path).name
    if filename == "SKILL.md":
        skill_name = _skill_name_from_frontmatter(downloaded)
        skill_dir = tmp_path / skill_name
        skill_dir.mkdir()
        shutil.copy2(downloaded, skill_dir / "SKILL.md")
        return skill_dir

    raise XSkillsError("URL must point to a .zip, .tar archive, or direct SKILL.md file")


def _skill_name_from_frontmatter(skill_md: Path) -> str:
    for line in skill_md.read_text(encoding="utf-8").splitlines():
        if line.startswith("name:"):
            name = line.split(":", 1)[1].strip().strip("'\"")
            if name:
                return name
    raise XSkillsError("direct SKILL.md URL must include a name in frontmatter")


if __name__ == "__main__":
    main(sys.argv[1:])
