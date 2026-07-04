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
from json import JSONDecodeError
from pathlib import Path
from typing import TextIO
from urllib.parse import urlencode, urlparse

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
    reason: str = ""


@dataclass(frozen=True)
class RepoSkill:
    name: str
    path: Path
    description: str
    used: bool


@dataclass(frozen=True)
class SearchResult:
    name: str
    slug: str
    source: str
    installs: int


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
        "--color",
        choices=("auto", "always", "never"),
        default=os.environ.get("X_SKILLS_COLOR", "auto"),
        help="colorize human output; default: auto",
    )
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
    repo_remove.add_argument("names", nargs="+")
    repo_remove.set_defaults(func=cmd_repo_remove)

    search_parser = subparsers.add_parser("search", aliases=["find"], help="search skills.sh")
    search_parser.add_argument("--owner", help="filter by GitHub owner")
    search_parser.add_argument(
        "--limit", type=int, default=10, help="maximum API results to fetch; default: 10"
    )
    search_parser.add_argument(
        "--install",
        metavar="NAME_OR_INDEX",
        help="install a search result into the repo archive",
    )
    search_parser.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy when installing"
    )
    search_parser.add_argument("query", nargs="+")
    search_parser.set_defaults(func=cmd_search)

    link_parser = subparsers.add_parser("link", help="link a repo skill into active skills")
    _add_active_filters(link_parser)
    link_parser.add_argument("names", nargs="+")
    link_parser.set_defaults(func=cmd_link)

    migrate_parser = subparsers.add_parser(
        "migrate", help="move an active skill into the repo and link it back"
    )
    _add_active_filters(migrate_parser)
    migrate_parser.add_argument(
        "--replace-archive", action="store_true", help="replace archived copy"
    )
    migrate_parser.add_argument("--archive-as", help="archive under another name")
    migrate_parser.add_argument("names", nargs="+")
    migrate_parser.set_defaults(func=cmd_migrate)

    unlink_parser = subparsers.add_parser("unlink", help="remove an active skill")
    _add_active_filters(unlink_parser)
    unlink_parser.add_argument(
        "--delete-unmanaged",
        action="store_true",
        help="remove unmanaged active directories instead of migrating them",
    )
    unlink_parser.add_argument("names", nargs="+")
    unlink_parser.set_defaults(func=cmd_unlink)

    interactive = subparsers.add_parser("interactive", help="open interactive skill manager")
    interactive.set_defaults(func=cmd_interactive)

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
        header = f"{root.scope.upper()} {root.target}"
        path = _display_path(args, root.path)
        _print(
            args,
            f"{_style(args, header, 'header')}  {_style(args, path, 'path')}",
        )
        for skill in group:
            details = skill.reason or skill.description
            _print(
                args,
                (
                    f"  {_style_padded(args, skill.name, 22, 'name')} "
                    f"{_style_padded(args, skill.status, 10, skill.status)} "
                    f"{_style(args, details, 'detail')}"
                ),
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


def cmd_search(args: argparse.Namespace) -> None:
    query = " ".join(args.query).strip()
    results = search_skills(query, owner=args.owner, limit=args.limit)
    if args.json_:
        _print_json(args, [_search_result_to_json(result) for result in results])
        return

    if not results:
        owner_suffix = f' from owner "{args.owner}"' if args.owner else ""
        _print(args, f'No skills found for "{query}"{owner_suffix}')
        return

    if args.install:
        result = _resolve_search_install_selection(args.install, results)
        if not _confirm(
            args,
            f'Install "{result.name}" from {result.source or result.slug} into repo? [y/N]: ',
        ):
            if _is_non_interactive(args):
                raise XSkillsError(
                    "refusing to install search result without confirmation; pass -y"
                )
            _print(args, "cancelled")
            return
        installed = install_search_result_to_repo(args, result)
        _print(args, f"installed: {installed.name}")
        return

    _print(args, f"Install with x-skills search {query} --install <name-or-index> -y")
    _print(args, "")
    for index, result in enumerate(results[: min(args.limit, len(results))], start=1):
        package = _search_result_package(result)
        installs = _format_installs(result.installs)
        installs_label = f" {installs}" if installs else ""
        _print(args, f"{index}. {package}{installs_label}")
        _print(args, f"   https://skills.sh/{result.slug}")
        _print(args, "")


def cmd_link(args: argparse.Namespace) -> None:
    _run_name_batch(args, "linked", _cmd_link_one)


def _cmd_link_one(args: argparse.Namespace) -> None:
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
    _run_name_batch(args, "migrated", _cmd_migrate_one)


def _cmd_migrate_one(args: argparse.Namespace) -> None:
    candidates = _matching_active_skills(args, args.name)
    if not candidates:
        raise XSkillsError(f"active skill not found: {args.name}")

    managed = [skill for skill in candidates if skill.status == "managed"]
    unmanaged = [skill for skill in candidates if skill.status == "unmanaged"]
    if not unmanaged and managed:
        _print(args, f"already managed: {args.name}")
        return

    linked_group = _linked_active_group(unmanaged)
    if linked_group and _resolve_linked_group_action(args, args.name, linked_group, "migrate"):
        _migrate_linked_group(args, linked_group)
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
    _run_name_batch(args, "unlinked", _cmd_unlink_one)


def _cmd_unlink_one(args: argparse.Namespace) -> None:
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

    action = _resolve_unmanaged_unlink_action(args, selected)
    if action == "cancel":
        _print(args, "cancelled")
        return
    if action == "delete":
        if args.delete_unmanaged and not _confirm_delete_unmanaged(args, selected):
            _print(args, "cancelled")
            return
        shutil.rmtree(selected.path)
        _print(
            args,
            f"removed unmanaged {selected.root.scope} {selected.root.target}: {selected.name}",
        )
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
        _run_git_clone(clone_url, checkout)
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
    _run_name_batch(args, "removed", _cmd_repo_remove_one)


def _cmd_repo_remove_one(args: argparse.Namespace) -> None:
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


def cmd_interactive(args: argparse.Namespace) -> None:
    if args.no_input:
        raise XSkillsError("interactive cannot run with --no-input")
    _run_interactive_app(args)


def _run_interactive_app(args: argparse.Namespace) -> None:
    try:
        from x_skills.interactive import run_interactive
    except ImportError as error:
        raise XSkillsError(
            "interactive mode requires Textual; reinstall with `uv tool install --upgrade git+https://github.com/InkyQuill/x-skills.git`"
        ) from error
    run_interactive(args)


def _archive_skills_root(args: argparse.Namespace) -> Path:
    return args.archive_root.expanduser() / "skills"


def _archive_skill(args: argparse.Namespace, name: str) -> Path:
    return _archive_skills_root(args) / name


def search_skills(query: str, *, owner: str | None = None, limit: int = 10) -> list[SearchResult]:
    if limit < 1:
        raise XSkillsError("--limit must be greater than zero")
    params = {"q": query, "limit": str(limit)}
    if owner:
        params["owner"] = owner
    url = f"https://skills.sh/api/search?{urlencode(params)}"
    try:
        with urllib.request.urlopen(url, timeout=10) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except OSError as error:
        raise XSkillsError(f"skills.sh search failed: {error}") from error
    except (JSONDecodeError, UnicodeDecodeError) as error:
        raise XSkillsError(f"skills.sh search returned invalid JSON: {error}") from error
    if not isinstance(payload, dict):
        return []
    skills = payload.get("skills", [])
    if not isinstance(skills, list):
        return []
    results: list[SearchResult] = []
    for item in skills:
        if not isinstance(item, dict):
            continue
        name = str(item.get("name") or "").strip()
        slug = str(item.get("id") or "").strip()
        source = str(item.get("source") or "").strip()
        if not name or not slug:
            continue
        try:
            installs = int(item.get("installs") or 0)
        except (TypeError, ValueError):
            installs = 0
        results.append(SearchResult(name=name, slug=slug, source=source, installs=installs))
    return sorted(results, key=lambda result: result.installs, reverse=True)


def install_search_result_to_repo(args: argparse.Namespace, result: SearchResult) -> Path:
    source = result.source or _source_from_slug(result.slug)
    if not source:
        raise XSkillsError(f'search result "{result.name}" has no installable source')
    clone_url, _ = _github_clone_url_and_path(source, None)
    with tempfile.TemporaryDirectory(prefix="x-skills-search-") as tmp:
        checkout = Path(tmp) / "repo"
        _run_git_clone(clone_url, checkout)
        skill = _find_named_skill(checkout, result.name)
        return _install_skill_copy(skill, args)


def active_roots(args: argparse.Namespace, *, include_all: bool = False) -> list[ActiveRoot]:
    return _active_roots(args, include_all=include_all)


def active_skills(
    args: argparse.Namespace, roots: list[ActiveRoot] | None = None
) -> list[ActiveSkill]:
    return _active_skills(args, roots)


def display_path(args: argparse.Namespace, path: Path) -> str:
    return _display_path(args, path)


def search_result_package(result: SearchResult) -> str:
    return _search_result_package(result)


def format_installs(count: int) -> str:
    return _format_installs(count)


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


def _run_name_batch(args: argparse.Namespace, success_label: str, operation: object) -> None:
    names = args.names
    if len(names) == 1:
        args.name = names[0]
        operation(args)
        return

    completed: list[str] = []
    failed: list[str] = []
    for name in names:
        args.name = name
        try:
            operation(args)
        except XSkillsError as error:
            failed.append(f"{name} ({error})")
        else:
            completed.append(name)

    _print(args, "Summary:")
    if completed:
        _print(args, f"  {success_label}: {', '.join(completed)}")
    if failed:
        _print(args, f"  failed: {', '.join(failed)}")


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
            reason = f"target missing: {os.readlink(path)}"
            return ActiveSkill(path.name, root, path, "broken", "", reason)
        if not resolved.is_dir():
            reason = f"target is not a directory: {resolved}"
            return ActiveSkill(path.name, root, path, "broken", "", reason)
        if not (resolved / "SKILL.md").is_file():
            reason = f"target missing SKILL.md: {resolved}"
            return ActiveSkill(path.name, root, path, "broken", "", reason)
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


def _resolve_search_install_selection(selection: str, results: list[SearchResult]) -> SearchResult:
    try:
        index = int(selection)
    except ValueError:
        index = 0
    if index:
        if index < 1 or index > len(results):
            raise XSkillsError("search result index out of range")
        return results[index - 1]
    matches = [
        result
        for result in results
        if result.name == selection or _search_result_package(result) == selection
    ]
    if len(matches) == 1:
        return matches[0]
    if not matches:
        raise XSkillsError(f"search result not found: {selection}")
    raise XSkillsError(f"multiple search results match: {selection}; use an index")


def _search_result_package(result: SearchResult) -> str:
    source = result.source or _source_from_slug(result.slug)
    return f"{source}@{result.name}" if source else result.slug


def _source_from_slug(slug: str) -> str:
    parts = slug.split("/")
    if len(parts) >= 2:
        return "/".join(parts[:2])
    return ""


def _format_installs(count: int) -> str:
    if count <= 0:
        return ""
    if count >= 1_000_000:
        return f"{count / 1_000_000:.1f}".removesuffix(".0") + "M installs"
    if count >= 1_000:
        return f"{count / 1_000:.1f}".removesuffix(".0") + "K installs"
    suffix = "" if count == 1 else "s"
    return f"{count} install{suffix}"


def _find_named_skill(root: Path, name: str) -> Path:
    matches: list[Path] = []
    for skill_md in root.rglob("SKILL.md"):
        if ".git" in skill_md.parts:
            continue
        skill_dir = skill_md.parent
        frontmatter_name = _skill_frontmatter(skill_md).get("name")
        if skill_dir.name == name or frontmatter_name == name:
            matches.append(skill_dir)
    if len(matches) == 1:
        return matches[0]
    if not matches:
        raise XSkillsError(f'GitHub repo does not contain skill "{name}"')
    raise XSkillsError(f'GitHub repo contains multiple skills named "{name}"')


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


def _resolve_unmanaged_unlink_action(args: argparse.Namespace, skill: ActiveSkill) -> str:
    if args.delete_unmanaged:
        return "delete"
    if args.yes:
        return "migrate"
    if args.no:
        return "cancel"
    if _is_non_interactive(args):
        raise XSkillsError(
            f'unmanaged active directory "{skill.name}" requires a choice; '
            "pass -y to migrate first or --delete-unmanaged -y to remove it"
        )

    _print(args, f'"{skill.name}" is an unmanaged directory at {_display_path(args, skill.path)}.')
    _print(args, "")
    _print(args, "Choose action:")
    _print(args, "  1. migrate to repo, then unlink active copy")
    _print(args, "  2. unlink without migration (remove active directory)")
    _print(args, "  3. cancel")
    args.output_stream.write("Select [1-3]: ")
    args.output_stream.flush()
    answer = args.input_stream.readline().strip()
    match answer:
        case "1":
            return "migrate"
        case "2":
            return "delete"
        case "3" | "":
            return "cancel"
        case _:
            raise XSkillsError("invalid selection")


def _linked_active_group(candidates: list[ActiveSkill]) -> list[ActiveSkill] | None:
    if len(candidates) < 2:
        return None
    resolved_paths: set[Path] = set()
    for candidate in candidates:
        try:
            resolved_paths.add(candidate.path.resolve(strict=True))
        except OSError:
            return None
    if len(resolved_paths) == 1:
        return candidates
    return None


def _resolve_linked_group_action(
    args: argparse.Namespace, name: str, group: list[ActiveSkill], action: str
) -> bool:
    if _is_non_interactive(args):
        raise XSkillsError(
            f'linked active setup found for "{name}"; '
            f"run interactively to choose group or selected-location {action}"
        )

    _print(args, f'Found linked setup for "{name}":')
    _print(args, "")
    for index, skill in enumerate(group, start=1):
        target = ""
        if skill.path.is_symlink():
            target = f" -> {os.readlink(skill.path)}"
        location = _display_path(args, skill.path)
        _print(
            args,
            f"  {index}. {skill.root.scope} {skill.root.target}  {location}{target}",
        )
    _print(args, "")
    _print(args, "Apply action to:")
    _print(args, "  1. linked group")
    _print(args, "  2. selected location only")
    _print(args, "  3. cancel")
    args.output_stream.write("Select [1-3]: ")
    args.output_stream.flush()
    answer = args.input_stream.readline().strip()
    match answer:
        case "1":
            return True
        case "2":
            return False
        case "3" | "":
            _print(args, "cancelled")
            return True
        case _:
            raise XSkillsError("invalid selection")


def _migrate_linked_group(args: argparse.Namespace, group: list[ActiveSkill]) -> None:
    canonical = _linked_group_canonical(group)
    archive_name = args.archive_as or canonical.name
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
        f'Migrate linked group "{canonical.name}" into repo? [y/N]: ',
    ):
        _print(args, "cancelled")
        return

    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.move(str(canonical.path.resolve(strict=True)), destination)
    for skill in group:
        if skill.path.exists() or skill.path.is_symlink():
            if skill.path.is_symlink() or skill.path.is_file():
                skill.path.unlink()
            else:
                shutil.rmtree(skill.path)
        os.symlink(destination, skill.path, target_is_directory=True)
    _print(args, f"migrated linked group: {canonical.name}")


def _linked_group_canonical(group: list[ActiveSkill]) -> ActiveSkill:
    directories = [skill for skill in group if not skill.path.is_symlink()]
    if len(directories) == 1:
        return directories[0]
    if directories:
        raise XSkillsError("linked group has multiple directory sources")
    return group[0]


def _confirm_delete_unmanaged(args: argparse.Namespace, skill: ActiveSkill) -> bool:
    return _confirm(
        args,
        (
            f"Remove unmanaged {skill.root.scope} {skill.root.target} skill "
            f'"{skill.name}" without migrating? [y/N]: '
        ),
    )


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


def _style(args: argparse.Namespace, value: str, role: str) -> str:
    if not value or not _use_color(args):
        return value
    codes = {
        "header": "1;36",
        "path": "2",
        "name": "1",
        "managed": "32",
        "unmanaged": "33",
        "broken": "31",
        "detail": "2",
    }
    code = codes.get(role)
    if code is None:
        return value
    return f"\x1b[{code}m{value}\x1b[0m"


def _style_padded(args: argparse.Namespace, value: str, width: int, role: str) -> str:
    return _style(args, f"{value:<{width}}", role)


def _use_color(args: argparse.Namespace) -> bool:
    if args.json_ or args.color == "never":
        return False
    if args.color == "always":
        return True
    if "NO_COLOR" in os.environ:
        return False
    return args.output_stream.isatty()


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
        "reason": skill.reason,
    }


def _search_result_to_json(result: SearchResult) -> dict[str, str | int]:
    return {
        "name": result.name,
        "slug": result.slug,
        "source": result.source,
        "installs": result.installs,
        "package": _search_result_package(result),
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


def _run_git_clone(clone_url: str, checkout: Path) -> None:
    try:
        subprocess.run(
            ["git", "clone", "--depth", "1", clone_url, str(checkout)],
            check=True,
            stdout=subprocess.DEVNULL,
            timeout=120,
        )
    except FileNotFoundError as error:
        raise XSkillsError("git not found; install git and try again") from error
    except subprocess.TimeoutExpired as error:
        raise XSkillsError(f"git clone timed out: {clone_url}") from error
    except subprocess.CalledProcessError as error:
        raise XSkillsError(f"git clone failed: {clone_url}") from error


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
