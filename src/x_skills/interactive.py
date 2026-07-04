from __future__ import annotations

import argparse

from textual.app import App, ComposeResult
from textual.widgets import DataTable, Footer, Header, Static

from x_skills import cli


class XSkillsInteractive(App[None]):
    BINDINGS = [
        ("q", "quit", "Quit"),
        ("r", "refresh", "Refresh"),
    ]

    def __init__(self, args: argparse.Namespace) -> None:
        super().__init__()
        self.args = args
        self.table: DataTable[str] | None = None
        self.detail: Static | None = None

    def compose(self) -> ComposeResult:
        yield Header()
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
        self._refresh_table()

    def on_data_table_row_highlighted(self, event: DataTable.RowHighlighted) -> None:
        if self.detail is None:
            return
        row = self.table.get_row(event.row_key) if self.table is not None else []
        if not row:
            return
        name, scope, target, status, path, details = row
        self.detail.update(
            f"{name}\nscope: {scope}\ntarget: {target}\nstatus: {status}\npath: {path}\n\n{details}"
        )

    def _refresh_table(self) -> None:
        if self.table is None:
            return
        self.table.clear(columns=True)
        self.table.add_columns("Name", "Scope", "Target", "Status", "Path", "Details")
        for skill in cli._active_skills(self.args, cli._active_roots(self.args, include_all=True)):
            self.table.add_row(
                skill.name,
                skill.root.scope,
                skill.root.target,
                skill.status,
                cli._display_path(self.args, skill.path),
                skill.reason or skill.description,
            )


def run_interactive(args: argparse.Namespace) -> None:
    XSkillsInteractive(args).run()
