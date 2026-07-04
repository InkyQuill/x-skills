from __future__ import annotations

import argparse
from pathlib import Path

from textual.app import App, ComposeResult
from textual.widgets import DataTable, Footer, Header, Input, Static
from textual.worker import Worker, WorkerState

from x_skills import cli


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
        self.active_rows: dict[str, cli.ActiveSkill] = {}
        self.search_rows: dict[str, cli.SearchResult] = {}
        self.selected_active: set[str] = set()

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
        self._refresh_table()

    def action_migrate_selected(self) -> None:
        self._run_selected_active_action("migrate")

    def action_unlink_selected(self) -> None:
        self._run_selected_active_action("unlink")

    def action_clean_broken(self) -> None:
        selected = self._selected_active_skills()
        if not selected:
            self._set_detail("Select broken skills first.")
            return
        try:
            results = clean_broken_skills(self.args, selected)
        except (cli.XSkillsError, OSError) as error:
            self._set_detail(str(error))
            return
        self.selected_active.clear()
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
        elif len(row) == 7:
            selected, name, scope, target, status, path, details = row
            self.detail.update(
                f"{selected} {name}\nscope: {scope}\ntarget: {target}\n"
                f"status: {status}\npath: {path}\n\n{details}"
            )
        else:
            self.detail.update("Unexpected row format.")

    def _refresh_table(self) -> None:
        if self.table is None:
            return
        self.mode = "active"
        if self.search_input is not None:
            self.search_input.display = False
        self.table.clear(columns=True)
        self.active_rows.clear()
        self.table.add_columns("Sel", "Name", "Scope", "Target", "Status", "Path", "Details")
        for skill in cli.active_skills(self.args, cli.active_roots(self.args, include_all=True)):
            key = _active_row_key(skill)
            self.active_rows[key] = skill
            self.table.add_row(
                "*" if key in self.selected_active else "",
                skill.name,
                skill.root.scope,
                skill.root.target,
                skill.status,
                cli.display_path(self.args, skill.path),
                skill.reason or skill.description,
                key=key,
            )
        self.selected_active.intersection_update(self.active_rows)
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
        self._refresh_table()
        self._set_detail("\n".join(results))

    def _selected_active_skills(self) -> list[cli.ActiveSkill]:
        return [
            self.active_rows[key] for key in sorted(self.selected_active) if key in self.active_rows
        ]

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


def _active_row_key(skill: cli.ActiveSkill) -> str:
    return f"{skill.root.scope}:{skill.root.target}:{skill.path}"


def run_interactive(args: argparse.Namespace) -> None:
    XSkillsInteractive(args).run()
