from __future__ import annotations

import argparse
import hashlib
import os
from dataclasses import dataclass
from pathlib import Path

from rich.text import Text
from textual.app import App, ComposeResult
from textual.widgets import DataTable, Footer, Header, Input, Static
from textual.worker import Worker, WorkerState

from x_skills import cli

SCOPE_STYLES = {
    "project": "bold green",
    "global": "bold cyan",
}
TARGET_STYLES = {
    "agents": "magenta",
    "codex": "blue",
    "claude": "yellow",
}
STATUS_STYLES = {
    "managed": "green",
    "unmanaged": "yellow",
    "broken": "red",
    "mixed": "bold yellow",
}


@dataclass(frozen=True)
class ActiveSkillGroup:
    key: str
    display_name: str
    skills: list[cli.ActiveSkill]
    status: str
    locations: str
    details: str
    fingerprint: str


def run_bulk_action(
    args: argparse.Namespace, skills: list[cli.ActiveSkill], action: str
) -> list[str]:
    results: list[str] = []
    for skill in skills:
        action_args = _args_for_skill(args, skill)
        if action == "migrate":
            was_managed = skill.status == "managed"
            cli._cmd_migrate_one(action_args)
            if was_managed:
                results.append(f"migrate skipped: {skill.name}")
            elif skill.path.is_symlink():
                results.append(f"migrated {skill.name}")
            else:
                results.append(f"migrate skipped: {skill.name}")
        elif action == "unlink":
            cli._cmd_unlink_one(action_args)
            if not skill.path.exists() and not skill.path.is_symlink():
                results.append(f"unlinked {skill.name}")
            else:
                results.append(f"unlink skipped: {skill.name}")
        else:
            raise cli.XSkillsError(f"unknown interactive action: {action}")
    return results


def clean_broken_skills(args: argparse.Namespace, skills: list[cli.ActiveSkill]) -> list[str]:
    results: list[str] = []
    for skill in skills:
        if skill.status != "broken":
            continue
        if skill.path.is_symlink():
            skill.path.unlink()
            results.append(f"removed broken {skill.name}")
    return results


def install_search_result(args: argparse.Namespace, result: cli.SearchResult) -> Path:
    install_args = argparse.Namespace(**vars(args))
    install_args.replace_archive = getattr(args, "replace_archive", False)
    return cli.install_search_result_to_repo(install_args, result)


def active_skill_groups(
    args: argparse.Namespace, fingerprint_cache: dict[Path, str] | None = None
) -> list[ActiveSkillGroup]:
    groups: dict[str, list[cli.ActiveSkill]] = {}
    for skill in cli.active_skills(args, cli.active_roots(args, include_all=True)):
        fingerprint = _cached_active_skill_fingerprint(skill, fingerprint_cache)
        key = f"sha:{fingerprint}" if fingerprint else _ungrouped_skill_key(skill)
        groups.setdefault(key, []).append(skill)
    return [_active_skill_group(key, skills) for key, skills in groups.items()]


def _args_for_skill(args: argparse.Namespace, skill: cli.ActiveSkill) -> argparse.Namespace:
    action_args = argparse.Namespace(**vars(args))
    action_args.name = skill.name
    action_args.names = [skill.name]
    action_args.target = skill.root.target
    action_args.project_ = skill.root.scope == "project"
    action_args.global_ = skill.root.scope == "global"
    action_args.yes = True
    action_args.no = False
    action_args.no_input = False
    action_args.replace_archive = getattr(args, "replace_archive", False)
    action_args.archive_as = None
    action_args.delete_unmanaged = False
    return action_args


class XSkillsInteractive(App[None]):
    BINDINGS = [
        ("space", "toggle_selected", "Select"),
        ("m", "migrate_selected", "Migrate"),
        ("u", "unlink_selected", "Unlink"),
        ("x", "clean_broken", "Clean broken"),
        ("s", "search_mode", "Search"),
        ("i", "install_selected", "Install"),
        ("escape", "back", "Back"),
        ("q", "quit", "Quit"),
        ("r", "refresh", "Refresh"),
    ]

    def __init__(self, args: argparse.Namespace) -> None:
        super().__init__()
        self.args = args
        self.table: DataTable[str] | None = None
        self.search_input: Input | None = None
        self.detail: Static | None = None
        self.mode = "active"
        self.active_rows: dict[str, ActiveSkillGroup] = {}
        self.row_positions: dict[str, int] = {}
        self.search_rows: dict[str, cli.SearchResult] = {}
        self.selected_active: set[str] = set()
        self.fingerprint_cache: dict[Path, str] = {}

    def compose(self) -> ComposeResult:
        yield Header()
        self.search_input = Input(placeholder="Search skills.sh", id="search")
        self.search_input.display = False
        yield self.search_input
        table: DataTable[str] = DataTable()
        table.cursor_type = "row"
        table.zebra_stripes = True
        self.table = table
        yield table
        self.detail = Static("Select a skill to inspect it.")
        yield self.detail
        yield Footer()

    def on_mount(self) -> None:
        self.title = "x-skills"
        self._refresh_table()

    def action_refresh(self) -> None:
        if self.mode == "search" and self.search_input is not None:
            self._set_detail("Press Enter to search again, or Esc to return to active skills.")
            return
        self.fingerprint_cache.clear()
        self._refresh_table()

    def action_back(self) -> None:
        if self.mode == "search":
            self.mode = "active"
            self._refresh_table()
            return
        self._set_detail("Already showing active skills.")

    def action_toggle_selected(self) -> None:
        if self.mode != "active" or self.table is None:
            return
        row_key = self._current_row_key()
        if not row_key or row_key not in self.active_rows:
            return
        if row_key in self.selected_active:
            self.selected_active.remove(row_key)
        else:
            self.selected_active.add(row_key)
        self._refresh_table(preserve_row_key=row_key)

    def action_migrate_selected(self) -> None:
        self._run_selected_active_action("migrate")

    def action_unlink_selected(self) -> None:
        self._run_selected_active_action("unlink")

    def action_clean_broken(self) -> None:
        selected = self._selected_active_skills()
        if not selected:
            selected = [
                skill
                for group in active_skill_groups(self.args, self.fingerprint_cache)
                for skill in group.skills
                if skill.status == "broken"
            ]
        if not selected:
            self._set_detail("No broken skills.")
            return
        try:
            results = clean_broken_skills(self.args, selected)
        except (cli.XSkillsError, OSError) as error:
            self._set_detail(str(error))
            return
        self.selected_active.clear()
        self.fingerprint_cache.clear()
        self._refresh_table()
        self._set_detail("\n".join(results) if results else "No selected broken skills.")

    def action_search_mode(self) -> None:
        self.mode = "search"
        if self.search_input is not None:
            self.search_input.display = True
            self.search_input.focus()
        self._render_search_results([])
        self._set_detail("Type a query and press Enter.")

    def action_install_selected(self) -> None:
        if self.mode != "search" or self.table is None:
            self._set_detail("Use search mode first.")
            return
        row_key = self._current_row_key()
        result = self.search_rows.get(row_key)
        if result is None:
            self._set_detail("Select a search result first.")
            return
        try:
            self.run_worker(
                lambda: install_search_result(self.args, result),
                name="install",
                group="io",
                exit_on_error=False,
                exclusive=True,
                thread=True,
            )
        except Exception as error:
            self._set_detail(str(error))
            return
        self._set_detail(f"Installing {result.name}...")

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input is self.search_input:
            self._run_search(event.value)

    def on_worker_state_changed(self, event: Worker.StateChanged) -> None:
        if event.state == WorkerState.SUCCESS:
            if event.worker.name == "search":
                self._render_search_results(event.worker.result)
                return
            if event.worker.name == "install":
                installed = event.worker.result
                self._set_detail(f"installed: {installed.name}")
                return
        if event.state == WorkerState.ERROR:
            self._set_detail(str(event.worker.error))

    def on_data_table_row_highlighted(self, event: DataTable.RowHighlighted) -> None:
        if self.detail is None:
            return
        row = self.table.get_row(event.row_key) if self.table is not None else []
        if not row:
            return
        if len(row) == 4:
            name, package, installs, url = row
            self.detail.update(f"{name}\npackage: {package}\ninstalls: {installs}\n{url}")
        elif len(row) == 6:
            selected, name, locations, status, fingerprint, details = row
            self.detail.update(
                f"{selected} {name}\nlocations: {locations}\nstatus: {status}\n"
                f"sha: {fingerprint}\n\n{details}"
            )
        else:
            self.detail.update("Unexpected row format.")

    def _refresh_table(self, *, preserve_row_key: str | None = None) -> None:
        if self.table is None:
            return
        self.mode = "active"
        if self.search_input is not None:
            self.search_input.display = False
        self.table.clear(columns=True)
        self.active_rows.clear()
        self.row_positions.clear()
        self.table.add_columns("Sel", "Skill", "Locations", "Status", "SHA", "Details")
        for row_index, group in enumerate(active_skill_groups(self.args, self.fingerprint_cache)):
            self.active_rows[group.key] = group
            self.row_positions[group.key] = row_index
            self.table.add_row(
                "*" if group.key in self.selected_active else "",
                Text(group.display_name, style=_group_name_style(group)),
                _locations_text(group.skills),
                Text(group.status, style=STATUS_STYLES.get(group.status, "")),
                group.fingerprint[:8] if group.fingerprint else "",
                group.details,
                key=group.key,
            )
        self.selected_active.intersection_update(self.active_rows)
        self._restore_cursor(preserve_row_key)
        self._set_detail("Space select | m migrate | u unlink | x clean broken | s search")

    def _run_selected_active_action(self, action: str) -> None:
        selected = self._selected_active_skills()
        if not selected:
            self._set_detail("Select skills first.")
            return
        try:
            results = run_bulk_action(self.args, selected, action)
        except (cli.XSkillsError, OSError) as error:
            self._set_detail(str(error))
            return
        self.selected_active.clear()
        self.fingerprint_cache.clear()
        self._refresh_table()
        self._set_detail("\n".join(results))

    def _selected_active_skills(self) -> list[cli.ActiveSkill]:
        selected: list[cli.ActiveSkill] = []
        for key in sorted(self.selected_active):
            group = self.active_rows.get(key)
            if group is not None:
                selected.extend(group.skills)
        return selected

    def _run_search(self, query: str) -> None:
        if len(query.strip()) < 2:
            self._render_search_results([])
            self._set_detail("Search query must be at least 2 characters.")
            return
        try:
            self.run_worker(
                lambda: cli.search_skills(query.strip()),
                name="search",
                group="io",
                exit_on_error=False,
                exclusive=True,
                thread=True,
            )
        except Exception as error:
            self._set_detail(str(error))
            return
        self._set_detail(f'Searching "{query.strip()}"...')

    def _render_search_results(self, results: list[cli.SearchResult]) -> None:
        if self.table is None:
            return
        self.table.clear(columns=True)
        self.search_rows.clear()
        self.table.add_columns("Name", "Package", "Installs", "URL")
        for index, result in enumerate(results, start=1):
            key = f"search-{index}"
            self.search_rows[key] = result
            self.table.add_row(
                result.name,
                cli.search_result_package(result),
                cli.format_installs(result.installs),
                f"https://skills.sh/{result.slug}",
                key=key,
            )
        self._set_detail(
            "Select result and press i to install." if results else "No search results."
        )

    def _set_detail(self, message: str) -> None:
        if self.detail is not None:
            self.detail.update(message)

    def _current_row_key(self) -> str:
        if self.table is None:
            return ""
        try:
            cell_key = self.table.coordinate_to_cell_key(self.table.cursor_coordinate)
        except Exception:
            return ""
        return str(cell_key.row_key.value)

    def _restore_cursor(self, row_key: str | None) -> None:
        if self.table is None or not row_key:
            return
        row = self.row_positions.get(row_key)
        if row is None:
            return
        self.table.move_cursor(row=row)


def _active_skill_fingerprint(skill: cli.ActiveSkill) -> str:
    if skill.status == "broken":
        return ""
    root = _resolved_skill_root(skill)
    if root is None or not root.is_dir():
        return ""
    return _directory_fingerprint(root)


def _cached_active_skill_fingerprint(
    skill: cli.ActiveSkill, fingerprint_cache: dict[Path, str] | None
) -> str:
    if fingerprint_cache is None or skill.status == "broken":
        return _active_skill_fingerprint(skill)
    root = _resolved_skill_root(skill)
    if root is None:
        return ""
    if root in fingerprint_cache:
        return fingerprint_cache[root]
    fingerprint = _directory_fingerprint(root) if root.is_dir() else ""
    fingerprint_cache[root] = fingerprint
    return fingerprint


def _resolved_skill_root(skill: cli.ActiveSkill) -> Path | None:
    try:
        return skill.path.resolve(strict=True) if skill.path.is_symlink() else skill.path
    except OSError:
        return None


def _directory_fingerprint(root: Path) -> str:
    digest = hashlib.sha256()
    try:
        for current, dirs, files in os.walk(root):
            dirs.sort()
            files.sort()
            current_path = Path(current)
            rel_dir = current_path.relative_to(root).as_posix()
            for directory in dirs:
                path = current_path / directory
                digest.update(b"D\0")
                digest.update(f"{rel_dir}/{directory}".encode())
                digest.update(b"\0")
                if path.is_symlink():
                    digest.update(b"L\0")
                    digest.update(os.readlink(path).encode("utf-8"))
                    digest.update(b"\0")
            for filename in files:
                path = current_path / filename
                rel_path = path.relative_to(root).as_posix()
                digest.update(b"F\0")
                digest.update(rel_path.encode("utf-8"))
                digest.update(b"\0")
                if path.is_symlink():
                    digest.update(b"L\0")
                    digest.update(os.readlink(path).encode("utf-8"))
                else:
                    with path.open("rb") as file:
                        for chunk in iter(lambda: file.read(1024 * 1024), b""):
                            digest.update(chunk)
                digest.update(b"\0")
    except OSError:
        return ""
    return digest.hexdigest()


def _ungrouped_skill_key(skill: cli.ActiveSkill) -> str:
    return f"{skill.root.scope}:{skill.root.target}:{skill.path}"


def _active_skill_group(key: str, skills: list[cli.ActiveSkill]) -> ActiveSkillGroup:
    ordered = sorted(skills, key=_active_skill_sort_key)
    names = list(dict.fromkeys(skill.name for skill in ordered))
    statuses = sorted({skill.status for skill in ordered})
    details = sorted(
        {
            skill.reason or skill.description
            for skill in ordered
            if skill.reason or skill.description
        }
    )
    fingerprint = key.removeprefix("sha:") if key.startswith("sha:") else ""
    return ActiveSkillGroup(
        key=key,
        display_name=" + ".join(names),
        skills=ordered,
        status=statuses[0] if len(statuses) == 1 else "mixed",
        locations=", ".join(_location_label(skill) for skill in ordered),
        details="; ".join(details),
        fingerprint=fingerprint,
    )


def _location_label(skill: cli.ActiveSkill) -> str:
    return f"{skill.root.scope} {skill.root.target}"


def _active_skill_sort_key(skill: cli.ActiveSkill) -> tuple[int, int, str]:
    scope_order = {"project": 0, "global": 1}
    target_order = {"agents": 0, "claude": 1, "codex": 2}
    return (
        scope_order.get(skill.root.scope, 99),
        target_order.get(skill.root.target, 99),
        skill.name,
    )


def _locations_text(skills: list[cli.ActiveSkill]) -> Text:
    text = Text()
    for index, skill in enumerate(skills):
        if index:
            text.append(", ")
        text.append(skill.root.scope, style=SCOPE_STYLES.get(skill.root.scope, ""))
        text.append(" ")
        text.append(skill.root.target, style=TARGET_STYLES.get(skill.root.target, ""))
    return text


def _group_name_style(group: ActiveSkillGroup) -> str:
    return STATUS_STYLES.get(group.status, "")


def run_interactive(args: argparse.Namespace) -> None:
    XSkillsInteractive(args).run()
