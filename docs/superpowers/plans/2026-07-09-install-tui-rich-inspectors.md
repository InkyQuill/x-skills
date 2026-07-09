# Install TUI Rich Inspectors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the `I:Install` page to use rich rows, multi-selection, complete install details, preview access, and reusable Key/Value inspectors across all TUI pages.

**Architecture:** Extract reusable inspector rendering into a small component that accepts sections, Key/Value rows, and rich inline renderers such as pills/chips. Rework Install rows to use the existing `rowSegment` rich-row system, then add Install selection semantics and batch-aware actions. Selectively import the diamond animation worktree after row/inspector structure is stable.

**Tech Stack:** Bubble Tea, Lip Gloss, existing TUI model/views, existing remote install workflow.

---

## Context And Constraints

- ADR 0015: Install is a top-level page.
- ADR 0016: TUI should use snapshots/rich components and keep `Update` non-blocking.
- Grilling Q&A selected restrained Unicode and color, list + inspector shell, and modal actions.
- Current Install rows are plain formatted strings inside one `rowSegment`, not rich rows.
- Current inspector has ad hoc plain lines; it should become a reusable Key/Value renderer.
- Animation worktree exists at `/home/inky/Development/x-skills-tui-animations`.
- Do not run a direct git merge while either checkout is dirty. Use selective patch/cherry-pick after tests isolate the visual behavior.

## Animation Worktree Merge Assessment

The animation worktree currently has uncommitted changes in:

- `internal/tui/animation.go` (new)
- `internal/tui/model.go`
- `internal/tui/model_test.go`
- `internal/tui/rows_test.go`
- `internal/tui/views.go`

Useful pieces:
- `animationTickMsg`, `animationFrame`, and ASCII-disabled animation guard.
- Diamond frame helpers (`pulseDiamond`, `spinDiamond`, `selectedDiamond`, progress helpers).
- Tests proving ASCII mode does not animate.

Do not copy these directly before refactoring Install rows, because `views.go` will be heavily touched by the inspector/rich-row work.

## File Structure

- Create `internal/tui/inspector.go`: reusable inspector sections and Key/Value rendering.
- Create `internal/tui/inspector_test.go`: focused rendering tests.
- Modify `internal/tui/styles.go`: add semantic styles for inspector title/key/value and install row pills.
- Modify `internal/tui/views.go`: route all page inspectors through the reusable inspector, render rich Install rows.
- Modify `internal/tui/model.go`: allow selection on Install page and use selected Install rows for `i`/`a`.
- Modify `internal/tui/install.go`: add helpers for selected install rows and batch action summaries.
- Modify `internal/tui/install_test.go`: row, inspector, selection, preview, and batch tests.
- Import from animation worktree: create/adapt `internal/tui/animation.go` after the row/inspector tests pass.

## Task 1: Reusable Inspector Component

**Files:**
- Create: `internal/tui/inspector.go`
- Create: `internal/tui/inspector_test.go`
- Modify: `internal/tui/styles.go`

- [ ] **Step 1: Write failing inspector tests**

Tests:

```go
func TestInspectorRendersKeyValueHierarchy(t *testing.T)
func TestInspectorRendersRichValue(t *testing.T)
func TestInspectorTruncatesToHeight(t *testing.T)
```

Expected visible text:

```text
Inspector
next-best-practices
Source      vercel-labs/skills
Status      update available
Audit       warn
```

The test should assert ANSI-stripped content and, when colors are enabled, that key/value styles are configured separately.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/tui -run TestInspector -count=1`

Expected: compile failure because `inspectorSection` does not exist.

- [ ] **Step 3: Implement inspector primitives**

Add:

```go
type inspectorValue func(width int) string

type inspectorRow struct {
	Key    string
	Value string
	Render inspectorValue
}

type inspectorSection struct {
	Title string
	Rows  []inspectorRow
}

func textInspectorValue(value string) inspectorValue
func renderInspectorDocument(title string, sections []inspectorSection, width, height int) string
```

Rendering rules:
- Section title uses `accentStyle`.
- Keys use `inspectorKeyStyle`.
- Values use `inspectorValueStyle`.
- Rich values call `Render(width)` and may return chips/pills.
- Every rendered line is explicitly truncated to avoid wrapping.

- [ ] **Step 4: Add styles**

In `styles.go`:

```go
inspectorTitleStyle = accentStyle
inspectorKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
inspectorValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
installSourceStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
installCountStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
```

Unset them in `NO_COLOR` init branch.

- [ ] **Step 5: Run inspector tests**

Run: `go test ./internal/tui -run TestInspector -count=1`

Expected: PASS.

## Task 2: Convert Existing Active/Repo/Doctor Inspectors

**Files:**
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/modal_test.go`

- [ ] **Step 1: Write failing tests for Key/Value inspectors**

Update existing detail/inspector tests or add:

```go
func TestActiveInspectorUsesKeyValueRows(t *testing.T)
func TestRepoInspectorUsesKeyValueRowsAndUsageChips(t *testing.T)
func TestDoctorInspectorUsesKeyValueRows(t *testing.T)
```

Assertions:
- Active inspector includes `Aliases`, `Repo`, `Description`, `Locations`.
- Repo inspector includes `Description`, `Usages`.
- Doctor inspector includes `Path`, `Reason`, `Fix`.
- Usage chips still render as rich values.

- [ ] **Step 2: Replace ad hoc `renderInspector` body**

Keep `renderInspector(m,width,height)` as the public view helper, but build page-specific `[]inspectorSection`:

```go
func activeInspectorSections(m Model) []inspectorSection
func repoInspectorSections(m Model) []inspectorSection
func doctorInspectorSections(m Model) []inspectorSection
func installInspectorSections(m Model) []inspectorSection
```

- [ ] **Step 3: Run TUI inspector tests**

Run: `go test ./internal/tui -run 'Test(Active|Repo|Doctor).*Inspector|TestModal' -count=1`

Expected: PASS.

## Task 3: Install Rich Rows

**Files:**
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/install_test.go`

- [ ] **Step 1: Write failing rich-row tests**

Add:

```go
func TestInstallRowsRenderRichStateSourceAndDescription(t *testing.T)
func TestInstallRowsRenderAuditAndArchiveStatePills(t *testing.T)
```

Expected row content:

```text
› □ svelte-coder  vercel-labs/skills  812 installs  archived  warn  Svelte help.
```

The exact glyphs can be Unicode or ASCII based on options, but assertions must verify separate source/state/count/audit segments rather than one plain string.

- [ ] **Step 2: Implement install row segment helpers**

Add helpers in `views.go` or split to `install_rows.go` if `views.go` grows too much:

```go
func installID(result remote.SearchResult) string
func renderInstallStatePill(state string) string
func renderInstallCount(count int) string
func renderInstallRows(m Model, width int) []string
```

Rules:
- Use `rowPrefix(m, i, installID(result.Result))`, not `cursorPrefix`, so selection is visible.
- Source uses muted/source color.
- Archive state uses semantic pills.
- Audit pill is appended only when present.
- Description is last and muted/truncated.

- [ ] **Step 3: Run install row tests**

Run: `go test ./internal/tui -run TestInstallRows -count=1`

Expected: PASS.

## Task 4: Install Inspector Content

**Files:**
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/install_test.go`

- [ ] **Step 1: Write failing Install inspector tests**

Add:

```go
func TestInstallInspectorShowsDescriptionAndSourceDetails(t *testing.T)
func TestInstallInspectorShowsAvailableActions(t *testing.T)
```

Expected fields:
- Description
- Source
- Repo
- Path
- Installs
- Archive state
- Audit
- Actions: `enter preview`, `i install & use`, `a archive only`

- [ ] **Step 2: Implement `installInspectorSections`**

Build sections:

```go
Overview:
  Description
  Source
  Installs
State:
  Archive
  Audit
Repository:
  Owner
  Repo
  Path
Actions:
  Preview
  Install and use
  Archive only
```

Use rich renderer for audit/state pills.

- [ ] **Step 3: Run install inspector tests**

Run: `go test ./internal/tui -run TestInstallInspector -count=1`

Expected: PASS.

## Task 5: Install Multi-Selection

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`

- [ ] **Step 1: Write failing selection tests**

Add:

```go
func TestInstallSpaceTogglesResultSelection(t *testing.T)
func TestInstallArchiveUsesSelectedRows(t *testing.T)
func TestInstallUseUsesSelectedRows(t *testing.T)
```

Behavior:
- Space toggles selected install result.
- If no selected results, `a`/`i` uses cursor row.
- If selected results exist, `a` archives all selected rows sequentially.
- If selected results exist, `i` opens destination modal for the selected batch.

- [ ] **Step 2: Add selection helpers**

In `install.go`:

```go
func (m Model) selectedInstallRows() []installResultView
func (m Model) installActionRows() []installResultView
```

In key handling:
- Reuse existing space selection path for `ViewInstall`.
- Store ids in `m.selected[ViewInstall]`.

- [ ] **Step 3: Batch archive-only implementation**

First slice can archive selected rows sequentially and show:
- quiet status on all success: `archived 3 skills`
- result modal on mixed failures.

Do not implement batch conflict resolution beyond stopping at first conflict modal unless current install conflict workflow already supports continuation.

- [ ] **Step 4: Run selection tests**

Run: `go test ./internal/tui -run 'TestInstall.*Select|TestInstall.*Selected|TestInstallArchiveUsesSelected|TestInstallUseUsesSelected' -count=1`

Expected: PASS.

## Task 6: Preview Text Quality

**Files:**
- Modify: `internal/tui/modal_preview.go`
- Modify: `internal/tui/modal_test.go`
- Modify: `internal/tui/install_test.go`

- [ ] **Step 1: Write failing preview tests**

Add:

```go
func TestInstallPreviewShowsSkillMarkdownText(t *testing.T)
func TestInstallPreviewShowsDescriptionNearTop(t *testing.T)
```

Expected:
- Preview modal title is `Preview: <name>`.
- Body includes raw `SKILL.md` content or rendered markdown content.
- Description from frontmatter is visible near the top.

- [ ] **Step 2: Ensure preview reads `SKILL.md` from found skill dir**

If `newPreviewModal` already reads the file, add only the missing description/title formatting. If it does not, update it to use `skills.Read` plus `SKILL.md` content from `path`.

- [ ] **Step 3: Run preview tests**

Run: `go test ./internal/tui -run 'TestInstallPreview|TestPreviewModal' -count=1`

Expected: PASS.

## Task 7: Selectively Import Diamond Animations

**Files:**
- Create: `internal/tui/animation.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/rows_test.go`

- [ ] **Step 1: Copy animation helper file**

Use `/home/inky/Development/x-skills-tui-animations/internal/tui/animation.go` as source, but keep this branch's imports and model shape.

Do not run `git merge tui-diamond-animations` while worktrees are dirty.

- [ ] **Step 2: Add model frame field and tick handling**

Add to `Model`:

```go
animationFrame int
```

Update:

```go
func (m Model) Init() tea.Cmd {
	if !m.animationsEnabled() {
		return nil
	}
	return animationTick()
}
```

Handle `animationTickMsg` in `Update`, increment frame, and return next tick.

- [ ] **Step 3: Use animations carefully**

Allowed:
- Header product mark pulse.
- Cursor glyph spin.
- Selected row diamond trail.
- Selection progress in status.

Avoid:
- Animated glyphs inside dense Key/Value inspector values.
- Extra status lines that push content out of small terminals.
- Emoji-width-risky frames if tests show layout instability.

- [ ] **Step 4: Add/port animation tests**

Port and adapt:

```go
func TestUnicodeModelAnimatesDiamondFrame(t *testing.T)
func TestASCIIModelDoesNotAnimate(t *testing.T)
func TestDiamondProgressFrames(t *testing.T)
func TestSelectedRowsUseDiamondRipple(t *testing.T)
```

- [ ] **Step 5: Run animation and layout tests**

Run:

```bash
go test ./internal/tui -run 'Test(UnicodeModelAnimates|ASCIIModelDoesNotAnimate|Diamond|SelectedRows|Install|Inspector|Rows)' -count=1
```

Expected: PASS.

## Task 8: Verification And Commit

- [ ] **Step 1: Run focused TUI tests**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS.

- [ ] **Step 2: Run supported package suite**

Run:

```bash
go test ./cmd/x-skills ./internal/... -count=1
go build -o bin/x-skills ./cmd/x-skills
```

Expected: PASS.

- [ ] **Step 3: Manual smoke**

Run:

```bash
./bin/x-skills tui
```

Smoke checklist:
- Install rows have color hierarchy and rich pills.
- Inspector shows description/source/path/audit/state/actions.
- Space selection works on Install.
- Enter preview shows actual skill text.
- Active/Repo/Doctor inspectors still use readable Key/Value layout.
- ASCII mode does not animate.

- [ ] **Step 4: Commit scoped files**

Do not stage unrelated CLI add work or prior remote stale-data work unless intentionally included in the same branch milestone.

```bash
git add internal/tui/animation.go internal/tui/inspector.go internal/tui/inspector_test.go \
  internal/tui/styles.go internal/tui/views.go internal/tui/model.go \
  internal/tui/install.go internal/tui/install_test.go internal/tui/model_test.go \
  internal/tui/rows_test.go internal/tui/modal_preview.go internal/tui/modal_test.go
git commit -m "feat: enrich install tui rows and inspectors"
```

## Self-Review Checklist

- Install rows are no longer plain one-string rows.
- Install inspector explains what the user is about to install.
- Preview shows real skill text.
- Install multi-select exists and does not break single-row workflows.
- All inspectors share Key/Value structure.
- Rich elements such as root chips, audit pills, and state pills render inside inspector values.
- Diamond animation worktree is merged selectively, not as a blind merge over dirty state.
