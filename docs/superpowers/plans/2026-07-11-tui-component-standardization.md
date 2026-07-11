# TUI Component Standardization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate repeated modal/list rendering helpers into `internal/tui/ui`, fix color-independent status differentiation, and leave stable seams for later theme, mouse, fuzzy-filter, and command-palette work.

**Architecture:** Move only pure rendering and viewport math into the leaf `ui` package; model-specific choices remain in `tui`. Migrate one helper family at a time with snapshot tests so this refactor cannot mask behavior changes. Add semantic status markers that remain distinct under `NO_COLOR` before introducing future theming.

**Tech Stack:** Go 1.26.5, Lip Gloss, Bubble Tea, existing rendered snapshot tests.

## Global Constraints

- Execute after the production-readiness plan so concurrency fixes and UI refactors do not overlap.
- Preserve key bindings, modal sequencing, layout breakpoints, and current text unless a task explicitly changes accessibility copy.
- `internal/tui/ui` must not import `internal/tui` or depend on `Model`.
- Every migrated helper requires focused unit tests plus existing snapshot coverage.
- Do not add mouse, fuzzy matching, themes, or a command palette in this plan; create seams only.

---

## File Structure

- Modify `internal/tui/ui/components.go`; create `layout.go`, `text.go`, and tests.
- Modify all `internal/tui/modal_*.go`, `actions.go`, `styles.go`, and `views.go` call sites.
- Modify `internal/tui/symbols.go` and render tests for color-independent status.
- Modify `docs/tui-review.md` and `docs/backlog.md`.

### Task 1: Standardize Modal Footer Lines

**Files:**
- Modify: `internal/tui/ui/components.go`
- Create: `internal/tui/ui/components_test.go`
- Modify: `internal/tui/modal_confirm.go`
- Modify: `internal/tui/modal_choice.go`
- Modify: `internal/tui/modal_detail.go`
- Modify: `internal/tui/modal_help.go`
- Modify: `internal/tui/modal_preview.go`
- Modify: `internal/tui/modal_result.go`
- Modify: `internal/tui/modal_diff.go`
- Modify: `internal/tui/actions.go`

**Interfaces:**
- Produces: `ui.FooterLine(ascii bool, keyStyle, mutedStyle lipgloss.Style, shortcuts []ui.Shortcut) string`.

- [ ] **Step 1: Write exact footer tests**

```go
func TestFooterLineRendersAndMutesToolHints(t *testing.T) {
	got := FooterLine(true, lipgloss.NewStyle(), lipgloss.NewStyle(), []Shortcut{{ASCII: "enter", Label: "apply"}, {ASCII: "esc", Label: "cancel"}})
	if got != "enter apply  esc cancel" {
		t.Fatalf("FooterLine() = %q", got)
	}
}
```

- [ ] **Step 2: Implement the helper**

```go
func FooterLine(ascii bool, keyStyle, mutedStyle lipgloss.Style, shortcuts []Shortcut) string {
	return mutedStyle.Render(ToolHints(ascii, keyStyle, shortcuts))
}
```

- [ ] **Step 3: Replace every repeated footer expression**

Use `rg -n 'mutedStyle\.Render\(renderCommandPalette' internal/tui` as the work list. After migration, remove `renderCommandPalette` if no caller remains.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/tui/ui ./internal/tui -count=1
git add internal/tui
git commit -m "refactor(tui): standardize modal footers"
```

### Task 2: Centralize Modal Scroll And Visibility Math

**Files:**
- Create: `internal/tui/ui/layout.go`
- Create: `internal/tui/ui/layout_test.go`
- Modify: `internal/tui/modal.go`
- Modify: `internal/tui/modal_detail.go`
- Modify: `internal/tui/modal_help.go`
- Modify: `internal/tui/modal_result.go`
- Modify: `internal/tui/modal_diff.go`

**Interfaces:**
- Produces: `ui.ClampIndex(index, count int) int`, `ui.ClampScroll(scroll, bodyHeight, viewportHeight int) int`, and `ui.VisibleLines(lines []string, scroll, height int) []string`.

- [ ] **Step 1: Write boundary tests**

Cover negative values, empty lists, exact fits, excessive scroll, and a huge repeated page-down count. Assert helpers return new values without mutating inputs.

- [ ] **Step 2: Implement pure helpers**

Use integer comparisons only; no Bubble Tea or styles enter this package. `VisibleLines` clamps before slicing and returns an empty slice for non-positive height.

- [ ] **Step 3: Migrate modal call sites**

Replace `clampModalIndex`, `clampScroll`, and `visibleModalBody`; add a small `scrollState.Handle(msg tea.KeyMsg, bodyHeight, viewportHeight int) bool` in `modal.go` for Bubble Tea key translation. Clamp `conflictDiffModal` in Update as well as View.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/tui/ui ./internal/tui -count=1 -run 'Clamp|Scroll|Modal'
git add internal/tui
git commit -m "refactor(tui): centralize modal viewport math"
```

### Task 3: Move Pure ANSI Text Helpers

**Files:**
- Create: `internal/tui/ui/text.go`
- Create: `internal/tui/ui/text_test.go`
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/inspector.go`

**Interfaces:**
- Produces: `ui.TruncateANSI(value string, width int) string` and `ui.RenderWithBackground(style lipgloss.Style, background lipgloss.TerminalColor, value string) string`.

- [ ] **Step 1: Move existing tests before implementation**

Copy current truncation assertions into `ui/text_test.go`, including ANSI sequences, wide runes, zero width, and exact width. Add optional-background tests with `lipgloss.NoColor{}`.

- [ ] **Step 2: Move implementations without semantic changes**

Rename call sites to `tuiui.TruncateANSI` and `tuiui.RenderWithBackground`. Do not retain wrapper functions in `tui`; compilation should identify every caller.

- [ ] **Step 3: Verify and commit**

```bash
go test ./internal/tui/ui ./internal/tui -count=1
git add internal/tui
git commit -m "refactor(tui): share ANSI text helpers"
```

### Task 4: Standardize Pill Lists Without Moving Domain Decisions

**Files:**
- Modify: `internal/tui/ui/components.go`
- Modify: `internal/tui/ui/components_test.go`
- Modify: `internal/tui/views.go`

**Interfaces:**
- Produces: `ui.JoinPills(pills []string, separator string) string`.

- [ ] **Step 1: Write join tests**

Assert empty, single, and multiple rendered pill strings; do not trim ANSI or pill contents.

- [ ] **Step 2: Implement and migrate**

Keep scope-to-color and `roots.ActiveRoot` decisions in `tui.renderRootChip`; move only joining rendered chips to `ui.JoinPills`.

- [ ] **Step 3: Verify and commit**

```bash
go test ./internal/tui/ui ./internal/tui -count=1 -run 'Pill|RootChip'
git add internal/tui
git commit -m "refactor(tui): share pill list rendering"
```

### Task 5: Differentiate Status Without Color

**Files:**
- Modify: `internal/tui/symbols.go`
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/rows_test.go`
- Modify: `internal/tui/render_test.go`

**Interfaces:**
- Managed, unmanaged, and broken statuses receive distinct ASCII and Unicode glyph/text pairs.

- [ ] **Step 1: Add `NO_COLOR` render tests**

Render one row of each status with `NO_COLOR=1` and assert the plain output differs without relying on descriptions or paths.

- [ ] **Step 2: Assign semantic symbols**

Use `✓ managed`, `◇ unmanaged`, and `× broken` in Unicode; use `+ managed`, `? unmanaged`, and `x broken` in ASCII. Keep selection/cursor markers separate from status markers.

- [ ] **Step 3: Verify snapshots and commit**

```bash
NO_COLOR=1 go test ./internal/tui -count=1 -run 'NoColor|Status'
go test ./internal/tui -count=1
git add internal/tui
git commit -m "fix(tui): distinguish statuses without color"
```

### Task 6: Final Standardization Audit And Backlog Seams

**Files:**
- Modify: `docs/tui-review.md`
- Modify: `docs/backlog.md`

**Interfaces:**
- None; documentation and verification only.

- [ ] **Step 1: Search for duplicated helpers**

Run:

```bash
rg -n 'renderCommandPalette|clampModalIndex|visibleModalBody|func truncate\(|renderWithOptionalBackground' internal/tui
```

Expected: no old helper definitions or call sites.

- [ ] **Step 2: Run full verification**

```bash
gofmt -w internal/tui
go test ./... -count=1
go test ./internal/tui/... -race -count=1
go vet ./...
staticcheck ./...
```

Expected: PASS.

- [ ] **Step 3: Record deferred enhancement seams**

In `docs/backlog.md`, keep mouse support, fuzzy ranking, themes, command palette, and persistent per-page selection as separate future features. Note their new seams: semantic components in `ui`, pure filter ranking boundary, and centralized key/action descriptions. Do not mark those features implemented.

- [ ] **Step 4: Update the review and commit**

Mark the standardization section of `docs/tui-review.md` complete with the verification commands.

```bash
git add internal/tui docs/tui-review.md docs/backlog.md
git commit -m "refactor(tui): finish component standardization"
```
