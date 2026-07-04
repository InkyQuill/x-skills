from __future__ import annotations

import argparse
import contextlib
import json
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import TextIO
from urllib.parse import urlparse

TARGETS = ("agents", "claude", "codex")
SCOPES = ("project", "global")


class XSkillsError(RuntimeError):
    """Raised for user-facing command errors."""


@dataclass(frozen=True)
class ActiveRoot:
    scope: str
    target: str
    path: Path


@dataclass(frozen=True)
class ActiveSkill:
    name: str
    root: ActiveRoot
    path: Path
    status: str
    description: str


@dataclass(frozen=True)
class RepoSkill:
    name: str
    path: Path
    description: str
    used: bool


def main(
    argv: list[str] | None = None,
    *,
    input_stream: TextIO | None = None,
    output_stream: TextIO | None = None,
    error_stream: TextIO | None = None,
) -> None:
    input_stream = input_stream or sys.stdin
    output_stream = output_stream or sys.stdout
    error_stream = error_stream or sys.stderr
    parser = _build_parser()
    with contextlib.redirect_stdout(output_stream), contextlib.redirect_stderr(error_stream):
        args = parser.parse_args(argv)
        args.input_stream = input_stream
        args.output_stream = output_stream
        args.error_stream = error_stream
        try:
            args.func(args)
        except XSkillsError as error:
            parser.exit(2, f"x-skills: {error}\n")


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="x-skills")
    prompt = parser.add_mutually_exclusive_group()
    prompt.add_argument("-y", "--yes", action="store_true", help="answer yes to confirmations")
    prompt.add_argument("-n", "--no", action="store_true", help="answer no to confirmations")
    parser.add_argument("--no-input", action="store_true", help="fail instead of prompting")
    parser.add_argument("--json", action="store_true", dest="json_", help="write JSON output")
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
        help="global active skills root for agents; default: ~/.agents/skills",
    )
    parser.add_argument(
        "--claude-global-root",
        type=Path,
        default=Path(
            os.environ.get("X_SKILLS_CLAUDE_GLOBAL_ROOT", "~/.claude/skills")
        ).expanduser(),
        help="global active skills root for claude; default: ~/.claude/skills",
    )
    parser.add_argument(
        "--codex-global-root",
        type=Path,
        default=Path(os.environ.get("X_SKILLS_CODEX_GLOBAL_ROOT", "~/.codex/skills")).expanduser(),
        help="global active skills root for codex; default: ~/.codex/skills",
    )
    parser.add_argument(
        "--project-root",
        type=Path,
        default=Path.cwd(),
        help="project root for project links; default: current directory",
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    list_parser = subparsers.add_parser("list", help="list active skills for this project")
    _add_active_filters(list_parser)
    list_parser.add_argument("--all", action="store_true", help="include empty groups")
    list_parser.set_defaults(func=cmd_list)

    repo_parser = subparsers.add_parser("repo", help="list or manage archived skills")
    repo_parser.add_argument("--used", action="store_true", help="show only active repo skills")
    repo_parser.add_argument("--unused", action="store_true", help="show only inactive repo skills")
    repo_subparsers = repo_parser.add_subparsers(dest="repo_command")
    repo_parser.set_defaults(func=cmd_repo)

    repo_add_github = repo_subparsers.add_parser(
        "add-github", help="install a skill from a GitHub repo"
    )
    repo_add_github.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    repo_add_github.add_argument("repo_or_url")
    repo_add_github.add_argument("skill_path", nargs="?")
    repo_add_github.set_defaults(func=cmd_repo_add_github)

    repo_add_url = repo_subparsers.add_parser(
        "add-url", help="install a skill from an archive or SKILL.md URL"
    )
    repo_add_url.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    repo_add_url.add_argument("url")
    repo_add_url.set_defaults(func=cmd_repo_add_url)

    repo_remove = repo_subparsers.add_parser("remove", help="remove an archived skill")
    repo_remove.add_argument("name")
    repo_remove.set_defaults(func=cmd_repo_remove)

    link_parser = subparsers.add_parser("link", help="link a repo skill into active skills")
    _add_active_filters(link_parser)
    link_parser.add_argument("name")
    link_parser.set_defaults(func=cmd_link)

    migrate_parser = subparsers.add_parser(
        "migrate", help="move an active skill into the repo and link it back"
    )
    _add_active_filters(migrate_parser)
    migrate_parser.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    migrate_parser.add_argument("--archive-as", help="archive under another name")
    migrate_parser.add_argument("name")
    migrate_parser.set_defaults(func=cmd_migrate)

    unlink_parser = subparsers.add_parser("unlink", help="remove an active skill")
    _add_active_filters(unlink_parser)
    unlink_parser.add_argument("name")
    unlink_parser.set_defaults(func=cmd_unlink)

    doctor = subparsers.add_parser("doctor", help="check configured roots and dependencies")
    doctor.set_defaults(func=cmd_doctor)

    return parser


def _add_active_filters(parser: argparse.ArgumentParser) -> None:
    scope = parser.add_mutually_exclusive_group()
    scope.add_argument("--global", action="store_true", dest="global_", help="use global roots")
    scope.add_argument(
        "--project", action="store_true", dest="project_", help="use current project roots"
    )
    parser.add_argument("--target", choices=TARGETS, help="active skill target")


def cmd_list(args: argparse.Namespace) -> None:
    roots = _active_roots(args)
    skills = _active_skills(args, roots)
    if args.json_:
        _print_json(args, [_active_skill_to_json(skill) for skill in skills])
        return

    printed = False
    for root in roots:
        group = [skill for skill in skills if skill.root == root]
        if not group and not args.all:
            continue
        printed = True
        _print(args, f"{root.scope.upper()} {root.target}  {_display_path(args, root.path)}")
        for skill in group:
            _print(
                args,
                f"  {skill.name:<22} {skill.status:<10} {skill.description}",
            )
        _print(args, "")

    if not printed:
        _print(args, "No active skills found for this project or global roots.")


def cmd_repo(args: argparse.Namespace) -> None:
    if args.used and args.unused:
        raise XSkillsError("--used and --unused are mutually exclusive")
    skills = _repo_skills(args)
    if args.used:
        skills = [skill for skill in skills if skill.used]
    if args.unused:
        skills = [skill for skill in skills if not skill.used]

    if args.json_:
        _print_json(
            args,
            [
                {
                    "name": skill.name,
                    "path": str(skill.path),
                    "description": skill.description,
                    "used": skill.used,
                }
                for skill in skills
            ],
        )
        return

    for skill in skills:
        _print(args, f"{skill.name:<22} {skill.description}")


def cmd_link(args: argparse.Namespace) -> None:
    source = _archive_skill(args, args.name)
    if not _is_skill_dir(source):
        raise XSkillsError(f"repo skill not found: {args.name}")
    root = _resolve_destination_root(args, "link")
    target = root.path / args.name
    if target.exists() or target.is_symlink():
        raise XSkillsError(f"active skill already exists: {target}")
    target.parent.mkdir(parents=True, exist_ok=True)
    os.symlink(source, target, target_is_directory=True)
    _print(args, f"linked {root.scope} {root.target}: {args.name}")


def cmd_migrate(args: argparse.Namespace) -> None:
    candidates = _matching_active_skills(args, args.name)
    if not candidates:
        raise XSkillsError(f"active skill not found: {args.name}")

    managed = [skill for skill in candidates if skill.status == "managed"]
    unmanaged = [skill for skill in candidates if skill.status == "unmanaged"]
    if not unmanaged and managed:
        _print(args, f"already managed: {args.name}")
        return

    selected = _resolve_active_candidate(args, args.name, unmanaged, "migrate")
    archive_name = args.archive_as or selected.name
    destination = _archive_skill(args, archive_name)
    if destination.exists() or destination.is_symlink():
        if not args.replace_archive:
            if not _confirm(
                args,
                f'Repo already contains "{archive_name}". Replace it? [y/N]: ',
            ):
                _print(args, "cancelled")
                return
            _prepare_archive_destination(destination, replace=True)
        else:
            _prepare_archive_destination(destination, replace=True)

    if not _confirm(
        args,
        (
            f"Migrate {selected.root.scope} {selected.root.target} skill "
            f'"{selected.name}" into repo? [y/N]: '
        ),
    ):
        _print(args, "cancelled")
        return

    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.move(str(selected.path), destination)
    os.symlink(destination, selected.path, target_is_directory=True)
    _print(args, f"migrated {selected.root.scope} {selected.root.target}: {selected.name}")


def cmd_unlink(args: argparse.Namespace) -> None:
    candidates = _matching_active_skills(args, args.name)
    if not candidates:
        raise XSkillsError(f"active skill not found: {args.name}")
    selected = _resolve_active_candidate(args, args.name, candidates, "unlink")

    if selected.status in {"managed", "broken"}:
        if not _confirm(
            args,
            f'Unlink {selected.root.scope} {selected.root.target} skill "{selected.name}"? [y/N]: ',
        ):
            _print(args, "cancelled")
            return
        selected.path.unlink()
        _print(args, f"unlinked {selected.root.scope} {selected.root.target}: {selected.name}")
        return

    if not _confirm(
        args,
        (
            f"Migrate unmanaged {selected.root.scope} {selected.root.target} skill "
            f'"{selected.name}" before unlinking? [y/N]: '
        ),
    ):
        _print(args, "cancelled")
        return

    destination = _archive_skill(args, selected.name)
    _prepare_archive_destination(destination, replace=False)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.move(str(selected.path), destination)
    _print(
        args, f"migrated and unlinked {selected.root.scope} {selected.root.target}: {selected.name}"
    )


def cmd_repo_add_github(args: argparse.Namespace) -> None:
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
        _print(args, f"installed: {source.name}")


def cmd_repo_add_url(args: argparse.Namespace) -> None:
    with tempfile.TemporaryDirectory(prefix="x-skills-url-") as tmp:
        tmp_path = Path(tmp)
        downloaded = tmp_path / "download"
        urllib.request.urlretrieve(args.url, downloaded)
        source = _extract_or_materialize_url(downloaded, args.url, tmp_path)
        _install_skill_copy(source, args)
        _print(args, f"installed: {source.name}")


def cmd_repo_remove(args: argparse.Namespace) -> None:
    target = _archive_skill(args, args.name)
    if not _is_skill_dir(target):
        raise XSkillsError(f"repo skill not found: {args.name}")

    active = _matching_active_skills(args, args.name)
    if active:
        locations = ", ".join(f"{skill.root.scope} {skill.root.target}" for skill in active)
        _print(args, f"active now: {locations}")

    if not _confirm(args, f'Remove repo skill "{args.name}"? [y/N]: '):
        if _is_non_interactive(args):
            raise XSkillsError("refusing to remove repo skill without confirmation; pass -y")
        _print(args, "cancelled")
        return
    shutil.rmtree(target)
    _print(args, f"removed repo skill: {args.name}")


def cmd_doctor(args: argparse.Namespace) -> None:
    _print(args, f"repo: {_archive_skills_root(args)}")
    for root in _active_roots(args, include_all=True):
        exists = "exists" if root.path.exists() else "missing"
        writable = "writable" if _is_writable_or_creatable(root.path) else "not writable"
        _print(args, f"{root.scope} {root.target}: {root.path} ({exists}, {writable})")
    for command in ("git", "uv"):
        state = "found" if shutil.which(command) else "missing"
        _print(args, f"{command}: {state}")


def _archive_skills_root(args: argparse.Namespace) -> Path:
    return args.archive_root.expanduser() / "skills"


def _archive_skill(args: argparse.Namespace, name: str) -> Path:
    return _archive_skills_root(args) / name


def _active_roots(args: argparse.Namespace, *, include_all: bool = False) -> list[ActiveRoot]:
    scopes = list(SCOPES)
    if getattr(args, "global_", False):
        scopes = ["global"]
    if getattr(args, "project_", False):
        scopes = ["project"]

    targets = list(TARGETS)
    if getattr(args, "target", None) and not include_all:
        targets = [args.target]

    roots: list[ActiveRoot] = []
    for scope in scopes:
        for target in targets:
            roots.append(ActiveRoot(scope, target, _active_root_path(args, scope, target)))
    return roots


def _active_root_path(args: argparse.Namespace, scope: str, target: str) -> Path:
    if scope == "project":
        return args.project_root.expanduser() / f".{target}" / "skills"
    match target:
        case "agents":
            return args.global_root.expanduser()
        case "claude":
            return args.claude_global_root.expanduser()
        case "codex":
            return args.codex_global_root.expanduser()
        case unknown:
            raise XSkillsError(f"unknown target: {unknown}")


def _active_skills(
    args: argparse.Namespace, roots: list[ActiveRoot] | None = None
) -> list[ActiveSkill]:
    skills: list[ActiveSkill] = []
    for root in roots or _active_roots(args):
        if not root.path.exists():
            continue
        for path in sorted(root.path.iterdir(), key=lambda item: item.name):
            skill = _active_skill(args, root, path)
            if skill is not None:
                skills.append(skill)
    return skills


def _active_skill(args: argparse.Namespace, root: ActiveRoot, path: Path) -> ActiveSkill | None:
    if path.is_symlink():
        try:
            resolved = path.resolve(strict=True)
        except OSError:
            return ActiveSkill(path.name, root, path, "broken", "")
        if not _is_skill_dir(resolved):
            return ActiveSkill(path.name, root, path, "broken", "")
        status = (
            "managed" if _same_resolved_path(path, _archive_skill(args, path.name)) else "unmanaged"
        )
        return ActiveSkill(path.name, root, path, status, _skill_description(resolved))

    if _is_skill_dir(path):
        return ActiveSkill(path.name, root, path, "unmanaged", _skill_description(path))
    return None


def _matching_active_skills(args: argparse.Namespace, name: str) -> list[ActiveSkill]:
    return [skill for skill in _active_skills(args) if skill.name == name]


def _repo_skills(args: argparse.Namespace) -> list[RepoSkill]:
    active_names = {
        skill.name
        for skill in _active_skills(args, _active_roots(args, include_all=True))
        if skill.status == "managed"
    }
    root = _archive_skills_root(args)
    if not root.exists():
        return []
    return [
        RepoSkill(path.name, path, _skill_description(path), path.name in active_names)
        for path in sorted(root.iterdir(), key=lambda item: item.name)
        if _is_skill_dir(path)
    ]


def _resolve_destination_root(args: argparse.Namespace, action: str) -> ActiveRoot:
    roots = _active_roots(args)
    if len(roots) == 1:
        return roots[0]
    commands = [
        f"x-skills {action} {args.name} --target {root.target} --{root.scope}" for root in roots
    ]
    if _is_non_interactive(args):
        raise XSkillsError("choose a destination:\n  " + "\n  ".join(commands))
    return _select(args, f"Select destination for {action}", roots)


def _resolve_active_candidate(
    args: argparse.Namespace, name: str, candidates: list[ActiveSkill], action: str
) -> ActiveSkill:
    if len(candidates) == 1:
        return candidates[0]
    commands = [
        f"x-skills {action} {name} --target {skill.root.target} --{skill.root.scope}"
        for skill in candidates
    ]
    if _is_non_interactive(args):
        raise XSkillsError(
            f'multiple active skills named "{name}"; choose one:\n  ' + "\n  ".join(commands)
        )
    return _select(args, f"Select skill to {action}", candidates)


def _select(
    args: argparse.Namespace, label: str, options: list[ActiveRoot] | list[ActiveSkill]
) -> ActiveRoot | ActiveSkill:
    _print(args, f"{label}:")
    for index, option in enumerate(options, start=1):
        if isinstance(option, ActiveRoot):
            _print(
                args,
                f"  {index}. {option.scope} {option.target}  {_display_path(args, option.path)}",
            )
        else:
            location = _display_path(args, option.path)
            _print(
                args,
                f"  {index}. {option.root.scope} {option.root.target}  {location}  {option.status}",
            )
    if isinstance(options[0], ActiveRoot):
        prompt_subject = "destination"
    else:
        prompt_subject = f"skill to {label.rsplit(' ', 1)[-1]}"
    prompt = f"Select {prompt_subject} [1-{len(options)}]: "
    args.output_stream.write(prompt)
    args.output_stream.flush()
    answer = args.input_stream.readline().strip()
    try:
        selected = int(answer)
    except ValueError as error:
        raise XSkillsError("invalid selection") from error
    if selected < 1 or selected > len(options):
        raise XSkillsError("selection out of range")
    return options[selected - 1]


def _confirm(args: argparse.Namespace, prompt: str) -> bool:
    if args.yes:
        return True
    if args.no:
        return False
    if _is_non_interactive(args):
        return False
    args.output_stream.write(prompt)
    args.output_stream.flush()
    return args.input_stream.readline().strip().lower() in {"y", "yes"}


def _is_non_interactive(args: argparse.Namespace) -> bool:
    if args.no_input or os.environ.get("CI") == "1":
        return True
    if args.input_stream is not sys.stdin:
        return False
    return not (args.input_stream.isatty() and args.output_stream.isatty())


def _print(args: argparse.Namespace, message: str = "") -> None:
    print(message, file=args.output_stream)


def _print_json(args: argparse.Namespace, value: object) -> None:
    print(json.dumps(value, indent=2, sort_keys=True), file=args.output_stream)


def _active_skill_to_json(skill: ActiveSkill) -> dict[str, str]:
    return {
        "name": skill.name,
        "scope": skill.root.scope,
        "target": skill.root.target,
        "path": str(skill.path),
        "status": skill.status,
        "description": skill.description,
    }


def _display_path(args: argparse.Namespace, path: Path) -> str:
    expanded = path.expanduser()
    try:
        return str(expanded.relative_to(args.project_root.expanduser()).as_posix()).join(("./", ""))
    except ValueError:
        home = Path.home()
        try:
            return "~/" + expanded.relative_to(home).as_posix()
        except ValueError:
            return str(expanded)


def _skill_description(path: Path) -> str:
    frontmatter = _skill_frontmatter(path / "SKILL.md")
    return frontmatter.get("description", "")


def _skill_frontmatter(skill_md: Path) -> dict[str, str]:
    if not skill_md.is_file():
        return {}
    lines = skill_md.read_text(encoding="utf-8").splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    values: dict[str, str] = {}
    for line in lines[1:]:
        if line.strip() == "---":
            break
        key, separator, value = line.partition(":")
        if separator:
            values[key.strip()] = value.strip().strip("'\"")
    return values


def _is_skill_dir(path: Path) -> bool:
    return path.is_dir() and (path / "SKILL.md").is_file()


def _same_resolved_path(left: Path, right: Path) -> bool:
    if not right.exists() and not right.is_symlink():
        return False
    try:
        return left.resolve(strict=True) == right.resolve(strict=True)
    except OSError:
        return False


def _prepare_archive_destination(destination: Path, *, replace: bool) -> None:
    if not destination.exists() and not destination.is_symlink():
        return
    if not replace:
        raise XSkillsError(
            (
                f"repo already contains {destination.name}; "
                "use --replace-archive or confirm replacement"
            ),
        )
    if destination.is_symlink() or destination.is_file():
        destination.unlink()
    else:
        shutil.rmtree(destination)


def _install_skill_copy(source: Path, args: argparse.Namespace) -> Path:
    if not _is_skill_dir(source):
        raise XSkillsError(f"not a skill directory: {source}")
    destination = _archive_skill(args, source.name)
    _prepare_archive_destination(destination, replace=args.replace_archive)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(source, destination, symlinks=True)
    return destination


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
    name = _skill_frontmatter(skill_md).get("name")
    if name:
        return name
    raise XSkillsError("direct SKILL.md URL must include a name in frontmatter")


def _is_writable_or_creatable(path: Path) -> bool:
    current = path if path.exists() else path.parent
    while not current.exists() and current != current.parent:
        current = current.parent
    return os.access(current, os.W_OK)


if __name__ == "__main__":
    main(sys.argv[1:])
