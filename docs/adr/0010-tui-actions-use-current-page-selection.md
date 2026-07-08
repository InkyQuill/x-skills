# TUI actions use current-page selection with cursor fallback

All TUI actions operate on selected rows in the current page, falling back to the highlighted row only when that page has no selection.

Active and Repo support row selection. Selection sets are keyed by view so actions only read the current view, and selections are cleared on view switches to match the accepted full-parity spec. Doctor intentionally does not keep row selection: Doctor fix operates on all current Doctor issues after confirmation.

Actions never pull selections from another page. This keeps workflows independent and fixes cases where Repo actions such as link acted only on the cursor despite selected Repo rows.
