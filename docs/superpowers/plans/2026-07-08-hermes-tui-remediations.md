# Hermes TUI Remediations Implementation Plan

> **For Antigravity:** REQUIRED SUB-SKILL: Load executing-plans to implement this plan task-by-task.

**Goal:** Remediate the concrete defects and standardization gaps from `hermes-report.md` for the Go Bubble Tea TUI.

**Architecture:** Keep the existing `internal/tui` structure and fix behavior through small typed modal/view/model changes. Defer broad component migrations to isolated tasks after correctness and spec/doc alignment are restored.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, existing `internal/tui` modal system, existing `go test ./...` suite.

---

## Scope And Order

This plan intentionally separates immediate defects from larger accepted-spec gaps:

1. Fix real behavior bugs first: duplicate Repo link application, missing detail modals, missing broken reason in the inspector.
2. Standardize UI truthfulness: Doctor should not render/toggle inert selection, help should not call Install "reserved", docs should match shipped keys and view-switch selection behavior.
3. Harden future extension points: conflict diff labels must be explicit before remote-update flows reuse the modal.
4. Clean small dead/fragile code.
5. Plan larger Bubble Tea component migrations as their own tasks because viewport/textinput/async reload affect shared modal/input state.

## Files To Modify

- `internal/tui/actions.go`: remove duplicate Enter handling in `repoLinkModal.Update`.
- `internal/tui/actions_test.go`: assert Repo link Enter shows the success result rather than a second failure.
- `internal/tui/modal_detail.go`: add Repo and Doctor detail modal builders.
- `internal/tui/modal_test.go`: add Repo and Doctor detail modal coverage.
- `internal/tui/model.go`: wire Repo/Doctor details; make Doctor ignore row-selection toggles and selection fallback.
- `internal/tui/views.go`: show Active broken reason in inspector; remove Doctor checkbox affordance; remove Doctor clear shortcut.
- `internal/tui/rows_test.go`: update Doctor row expectation to no checkbox.
- `internal/tui/render_test.go`: add inspector broken-reason coverage.
- `internal/tui/modal_help.go`: clarify Install status and selection shortcut scope.
- `internal/tui/modal_diff.go`: parameterize incoming diff labels.
- `internal/tui/modal_test.go`: cover custom incoming labels and the small-terminal resize prompt.
- `internal/tui/styles.go`: remove dead `chipStyle`.
- `internal/tui/symbols.go`: cap unknown root chips defensively.
- `docs/adr/0010-tui-actions-use-current-page-selection.md`: align selection text with the accepted clear-on-view-switch behavior.
- `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`: add shipped `c` key and clarify Doctor has no selection.
- `go.mod`: make `github.com/charmbracelet/bubbles` direct when viewport/textinput tasks start.
- `internal/tui/modal_preview.go`: migrate preview scroll state to `bubbles/viewport`.
- `internal/tui/filter.go`: migrate filter query editing to `bubbles/textinput`.
- `internal/tui/model.go`: add async reload command scaffolding after behavior fixes.

## Validation Commands

Run these after each task unless the task gives a narrower command:

```bash
go test ./internal/tui
go test ./...
```

Expected final result:

All packages pass, including `github.com/InkyQuill/x-skills/internal/tui`.

---

### Task 1: Fix Repo Link Enter Double-Apply

**Files:**
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`

- [ ] **Step 1: Write the failing test**

Add this assertion to the end of `TestRepoLinkModalShowsDestinationAndCreatesLink` in `internal/tui/actions_test.go`, immediately after the `os.Lstat` check:

```go
	view = plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "✓ zen-of-go linked") {
		t.Fatalf("link result should report first successful apply, not a second failure:\n%s", view)
	}
	if strings.Contains(view, "already exists") {
		t.Fatalf("link result reports duplicate second apply:\n%s", view)
	}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/tui -run TestRepoLinkModalShowsDestinationAndCreatesLink -count=1 -v
```

Expected before the fix:

```text
FAIL: TestRepoLinkModalShowsDestinationAndCreatesLink
link result reports duplicate second apply
```

- [ ] **Step 3: Remove duplicate Enter handling**

In `internal/tui/actions.go`, replace `repoLinkModal.Update` with:

```go
func (r repoLinkModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "tab":
		if r.field == 0 {
			r.field = 1
		} else {
			r.field = 0
		}
		r.destination = r.destinationPath(m)
		m.modal = r
	case "left", "right":
		r.move(msg.String())
		r.destination = r.destinationPath(m)
		m.modal = r
	case "enter":
		r.apply(m)
	}
	return false, nil
}
```

- [ ] **Step 4: Verify the targeted test passes**

Run:

```bash
go test ./internal/tui -run TestRepoLinkModalShowsDestinationAndCreatesLink -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/actions.go internal/tui/actions_test.go
git commit -m "fix: prevent duplicate repo link apply"
```

---

### Task 2: Add Repo And Doctor Detail Modals

**Files:**
- Modify: `internal/tui/modal_detail.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/modal_test.go`

- [ ] **Step 1: Write Repo detail modal test**

Add this test to `internal/tui/modal_test.go` after `TestEnterOpensActiveDetailModal`:

```go
func TestEnterOpensRepoDetailModal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	active := makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), "zen-of-go"), active); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"Detail: zen-of-go (Repo)", "Archive path", cfg.ArchiveSkillsRoot(), "Description", "Go style.", "Usages", ".Ag"} {
		if !strings.Contains(view, want) {
			t.Fatalf("repo detail modal missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: Write Doctor detail modal test**

Add this test to `internal/tui/modal_test.go` after the Repo detail test:

```go
func TestEnterOpensDoctorDetailModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(root, "zen-of-go")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), brokenPath); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	for _, want := range []string{"Detail: zen-of-go (Doctor)", "Issue kind", "broken-symlink", "Affected path", brokenPath, "Reason", "Safe fix"} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor detail modal missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
go test ./internal/tui -run 'TestEnterOpens(Repo|Doctor)DetailModal' -count=1 -v
```

Expected before implementation:

```text
FAIL: TestEnterOpensRepoDetailModal
modal is nil
FAIL: TestEnterOpensDoctorDetailModal
modal is nil
```

- [ ] **Step 4: Add detail modal builders**

In `internal/tui/modal_detail.go`, add imports for `github.com/InkyQuill/x-skills/internal/doctor` and `github.com/InkyQuill/x-skills/internal/repo`, then add:

```go
func repoDetailModal(skill repo.Skill, usages []string, symbols symbols) modal {
	usageText := "none"
	if len(usages) > 0 {
		usageText = renderRootChips(symbols, usages, lipgloss.NoColor{})
	}
	lines := []string{
		"Archive path",
		"  " + skill.Path,
		"Description",
		"  " + skill.Description,
		"Usages",
		"  " + usageText,
	}
	return newDetailModal("Detail: "+skill.Name+" (Repo)", lines)
}

func doctorDetailModal(issue doctor.Issue) modal {
	lines := []string{
		"Issue kind",
		"  " + issue.Kind,
		"Affected path",
		"  " + issue.Path,
		"Reason",
		"  " + issue.Reason,
		"Safe fix",
		"  " + issue.SafeFix,
	}
	return newDetailModal("Detail: "+issue.Name+" (Doctor)", lines)
}
```

- [ ] **Step 5: Wire detail modal switch cases**

In `internal/tui/model.go`, replace `openDetailModal` with:

```go
func (m *Model) openDetailModal() {
	switch m.view {
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor >= 0 && m.cursor < len(groups) {
			m.modal = activeDetailModal(groups[m.cursor], m.symbols)
		}
	case ViewRepo:
		skills := m.visibleRepoSkills()
		if m.cursor >= 0 && m.cursor < len(skills) {
			m.modal = repoDetailModal(skills[m.cursor], m.repoUsage[skills[m.cursor].Name], m.symbols)
		}
	case ViewDoctor:
		if m.cursor >= 0 && m.cursor < len(m.issues) {
			m.modal = doctorDetailModal(m.issues[m.cursor])
		}
	}
}
```

- [ ] **Step 6: Verify detail tests pass**

Run:

```bash
go test ./internal/tui -run 'TestEnterOpens(Active|Repo|Doctor)DetailModal' -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit**

```bash
git add internal/tui/modal_detail.go internal/tui/model.go internal/tui/modal_test.go
git commit -m "feat: add repo and doctor detail modals"
```

---

### Task 3: Show Broken Active Reason In Inspector

**Files:**
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/render_test.go`

- [ ] **Step 1: Write inspector regression test**

Add this test to `internal/tui/render_test.go`:

```go
func TestActiveInspectorShowsBrokenReason(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(root, "broken-skill")); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.width = 120
	m.height = 30

	view := plain(m.View())
	for _, want := range []string{"Inspector", "broken-skill", "reason", "symlink target"} {
		if !strings.Contains(view, want) {
			t.Fatalf("active inspector missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestActiveInspectorShowsBrokenReason -count=1 -v
```

Expected before implementation:

```text
FAIL: TestActiveInspectorShowsBrokenReason
active inspector missing "reason"
```

- [ ] **Step 3: Add reason line**

In `internal/tui/views.go`, change the `ViewActive` branch in `renderInspector` to:

```go
	case ViewActive:
		groups := m.visibleActiveGroups()
		if m.cursor >= 0 && m.cursor < len(groups) {
			group := groups[m.cursor]
			lines = append(lines, "◇ "+group.Name, "aliases", "  "+strings.Join(group.Aliases, ", "), "repo", "  "+group.Status)
			if group.Status == actions.StatusBroken && group.Reason != "" {
				lines = append(lines, "reason", "  "+group.Reason)
			}
		}
```

- [ ] **Step 4: Verify test passes**

Run:

```bash
go test ./internal/tui -run TestActiveInspectorShowsBrokenReason -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/views.go internal/tui/render_test.go
git commit -m "fix: show broken active reason in inspector"
```

---

### Task 4: Remove Doctor's Inert Selection UI

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/rows_test.go`
- Modify: `internal/tui/filter_test.go`

- [ ] **Step 1: Write Doctor no-selection behavior test**

Add this test to `internal/tui/filter_test.go`:

```go
func TestDoctorSpaceDoesNotToggleSelection(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(root, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if len(m.selected) != 0 {
		t.Fatalf("doctor selection = %#v, want empty", m.selected)
	}
	view := plain(m.View())
	if strings.Contains(view, "□") || strings.Contains(view, "■") {
		t.Fatalf("doctor view should not render selection checkbox:\n%s", view)
	}
	if strings.Contains(view, "c clear") {
		t.Fatalf("doctor footer should not advertise clear selection:\n%s", view)
	}
}
```

- [ ] **Step 2: Update Doctor row test expectation**

In `internal/tui/rows_test.go`, update `TestDoctorRowsShowIssueReasonAndLocation` to assert no checkbox:

```go
	got := strings.Join(renderDoctorRows(m, 100), "\n")
	for _, want := range []string{"›", "▲", "broken-symlink", "zen-of-go", ".Ag", "symlink target missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor row missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, m.symbols.Unchecked) || strings.Contains(got, m.symbols.Checked) {
		t.Fatalf("doctor row should not render selection checkbox:\n%s", got)
	}
```

- [ ] **Step 3: Run failing tests**

Run:

```bash
go test ./internal/tui -run 'TestDoctor(SpaceDoesNotToggleSelection|RowsShowIssueReasonAndLocation)' -count=1 -v
```

Expected before implementation:

```text
FAIL
```

- [ ] **Step 4: Make selection toggle ignore Doctor**

In `internal/tui/model.go`, replace `toggleSelection` with:

```go
func (m *Model) toggleSelection() {
	if m.view == ViewDoctor {
		return
	}
	id, ok := m.currentID()
	if !ok {
		return
	}
	m.selected[id] = !m.selected[id]
}
```

Also remove the `ViewDoctor` case from `selectedIDsForView`:

```go
func (m *Model) selectedIDsForView() []string {
	var ids []string
	switch m.view {
	case ViewActive:
		for _, group := range m.visibleActiveGroups() {
			if m.selected[group.ID] {
				ids = append(ids, group.ID)
			}
		}
	case ViewRepo:
		for _, skill := range m.visibleRepoSkills() {
			id := repoID(skill.Name)
			if m.selected[id] {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		return ids
	}
	id, ok := m.currentID()
	if !ok {
		return nil
	}
	return []string{id}
}
```

- [ ] **Step 5: Add a cursor-only Doctor prefix**

In `internal/tui/views.go`, add this helper after `rowPrefix`:

```go
func cursorPrefix(m Model, index int) string {
	cursor := " "
	if index == m.cursor {
		cursor = m.symbols.Cursor
	}
	return cursorStyle.Render(cursor)
}
```

Then change `renderDoctorRows` to use `cursorPrefix`:

```go
func renderDoctorRows(m Model, width int) []string {
	var rows []string
	for i, issue := range m.issues {
		prefix := cursorPrefix(m, i)
		rows = append(rows, selectableRow(
			[]rowSegment{
				{text: fmt.Sprintf("%s %s %s ", prefix, dangerStyle.Render(m.symbols.Broken), issue.Kind)},
				{render: func(background lipgloss.TerminalColor) string {
					return renderRootChip(m.symbols, issue.Location, background)
				}},
				{text: fmt.Sprintf("  %s %s", issue.Name, issue.Reason)},
			},
			i == m.cursor,
			false,
			width-6,
		))
	}
	return rows
}
```

- [ ] **Step 6: Remove Doctor clear shortcut**

In `internal/tui/views.go`, change the `ViewDoctor` branch of `commandPalette` to:

```go
	case ViewDoctor:
		return renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "details"},
			{ASCII: "f", Label: "fix"},
			{ASCII: "^R", Label: "refresh"},
			{ASCII: "?", Label: "help"},
			{ASCII: "q", Label: "quit"},
		})
```

- [ ] **Step 7: Verify tests pass**

Run:

```bash
go test ./internal/tui -run 'TestDoctor(SpaceDoesNotToggleSelection|RowsShowIssueReasonAndLocation)' -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/views.go internal/tui/rows_test.go internal/tui/filter_test.go
git commit -m "fix: remove inert doctor selection affordance"
```

---

### Task 5: Refactor Selection Model and Align Help/Documentation

**Files:**
- Modify: `internal/tui/model.go:23-57`
- Modify: `internal/tui/model.go:115-125`
- Modify: `internal/tui/model.go:215-285`
- Modify: `internal/tui/views.go:160-310`
- Modify: `internal/tui/modal_help.go:25-35`
- Modify: `internal/tui/rows_test.go:58-165`
- Modify: `internal/tui/filter_test.go:57-148`
- Modify: `internal/tui/actions_test.go:95-108`
- Modify: `internal/tui/modal_test.go:50-80`
- Modify: `docs/adr/0010-tui-actions-use-current-page-selection.md`
- Modify: `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`

- [ ] **Step 1: Update selection data type and initialization**

In `internal/tui/model.go`, change `selected` field of `Model` struct to:
```go
	selected map[ViewName]map[string]bool
```

Update `New` function to initialize this map for each view:
```go
	m := Model{
		cfg:      cfg,
		opts:     options,
		symbols:  symbolsFor(options),
		view:     ViewActive,
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {},
			ViewDoctor: {},
		},
	}
```

Update `setView` to nuke and reinitialize maps per view rather than a single shared map:
```go
func (m *Model) setView(view ViewName) {
	if m.view == view {
		return
	}
	m.view = view
	m.cursor = 0
	m.selected = map[ViewName]map[string]bool{
		ViewActive: {},
		ViewRepo:   {},
		ViewDoctor: {},
	}
	m.filter = filterState{}
}
```

Update `handleKey` case `"c"` to clear current view's selection:
```go
	case "c":
		m.selected[m.view] = map[string]bool{}
		m.status = "selection cleared"
```

- [ ] **Step 2: Update toggle and lookup methods**

In `internal/tui/model.go`, update `toggleSelection` and `selectedIDsForView` to index by view name:
```go
func (m *Model) toggleSelection() {
	if m.view == ViewDoctor {
		return
	}
	id, ok := m.currentID()
	if !ok {
		return
	}
	if m.selected[m.view] == nil {
		m.selected[m.view] = map[string]bool{}
	}
	m.selected[m.view][id] = !m.selected[m.view][id]
}

func (m *Model) selectedIDsForView() []string {
	var ids []string
	switch m.view {
	case ViewActive:
		for _, group := range m.visibleActiveGroups() {
			if m.selected[ViewActive][group.ID] {
				ids = append(ids, group.ID)
			}
		}
	case ViewRepo:
		for _, skill := range m.visibleRepoSkills() {
			id := repoID(skill.Name)
			if m.selected[ViewRepo][id] {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		return ids
	}
	id, ok := m.currentID()
	if !ok {
		return nil
	}
	return []string{id}
}
```

- [ ] **Step 3: Update row rendering**

In `internal/tui/views.go`, update `renderActiveRows` and `renderRepoRows` to pass the correct view-scoped selections:
```go
// In renderActiveRows:
			i == m.cursor,
			m.selected[ViewActive][group.ID],
			width-6,

// In renderRepoRows:
			i == m.cursor,
			m.selected[ViewRepo][id],
			width-6,
```

And update `rowPrefix`:
```go
func rowPrefix(m Model, index int, id string) string {
	cursor := " "
	if index == m.cursor {
		cursor = m.symbols.Cursor
	}
	selected := m.symbols.Unchecked
	if m.selected[m.view][id] {
		selected = m.symbols.Checked
	}
	return cursorStyle.Render(cursor) + " " + selectedStyle.Render(selected)
}
```

- [ ] **Step 4: Update all selection tests**

Update selections in tests in `internal/tui/rows_test.go`, `internal/tui/filter_test.go`, and `internal/tui/actions_test.go` to construct and verify the correct map-of-maps layout.

In `internal/tui/rows_test.go`:
```go
// In TestRenderActiveRowsUseSpecSymbols:
		selected: map[ViewName]map[string]bool{ViewActive: {}, ViewRepo: {}, ViewDoctor: {}},

// In TestRepoRowsShowUsageChipsAndSelectionMarkers:
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {"repo:zen-of-go": true},
			ViewDoctor: {},
		},

// In TestHighlightedRepoRowPreservesRootPills:
		selected: map[ViewName]map[string]bool{
			ViewActive: {},
			ViewRepo:   {"repo:zen-of-go": true},
			ViewDoctor: {},
		},

// In TestDoctorRowsShowIssueReasonAndLocation:
		selected: map[ViewName]map[string]bool{ViewActive: {}, ViewRepo: {}, ViewDoctor: {}},
```

In `internal/tui/filter_test.go`:
```go
// In TestSelectionClearsOnViewSwitch:
	if len(m.selected[ViewActive]) == 0 { ... }
	...
	if len(m.selected[ViewActive]) != 0 || len(m.selected[ViewRepo]) != 0 { ... }

// In TestClearSelectionKeyClearsCurrentSelection:
	if len(m.selected[ViewActive]) == 0 { ... }
	...
	if len(m.selected[ViewActive]) != 0 { ... }

// In TestFilterCursorAndActionsUseFilteredRepoRows:
	if !m.selected[ViewRepo][repoID("target-skill")] { ... }
```

In `internal/tui/actions_test.go`:
```go
// In TestActiveUnlinkWorkbenchShowsAllSelections:
	m := New(cfg)
	m.selected = map[ViewName]map[string]bool{
		ViewActive: {},
		ViewRepo:   {},
		ViewDoctor: {},
	}
	for _, group := range m.active {
		m.selected[ViewActive][group.ID] = true
	}
```

- [ ] **Step 5: Run tests and confirm compiler safety**

Run:
```bash
go test ./internal/tui -count=1 -v
```
Expected: PASS

- [ ] **Step 6: Update help wording**

In `internal/tui/modal_help.go`, update the help shortcuts for "I" and "space":
```go
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "I", Label: "Install (design in progress, not yet available)"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "space", Label: "toggle Active/Repo row selection"}),
```

- [ ] **Step 7: Strengthen help modal test**

In `internal/tui/modal_test.go`, replace the loop inside `TestQuestionMarkOpensHelpModalWithGlobalKeys` with:
```go
	for _, want := range []string{"Help", "A", "R", "D", "I", "^R", ".Ag", "~Cd", "Install (design in progress, not yet available)", "toggle Active/Repo row selection"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "reserved for Install view") {
		t.Fatalf("help modal still uses stale Install wording:\n%s", view)
	}
```

- [ ] **Step 8: Run help modal tests**

Run:
```bash
go test ./internal/tui -run TestQuestionMarkOpensHelpModalWithGlobalKeys -count=1 -v
```
Expected: PASS

- [ ] **Step 9: Update ADR 0010 and full-parity-design spec**

Replace the body of `docs/adr/0010-tui-actions-use-current-page-selection.md` with:
```markdown
# TUI actions use current-page selection with cursor fallback

All TUI actions operate on selected rows in the current page, falling back to the highlighted row only when that page has no selection. Selection sets are stored per page using view-keyed maps, so Active, Repo, Doctor, and Install can preserve local context without leaking actions across pages.

Active and Repo support row selection. Doctor intentionally does not: Doctor fix operates on all current Doctor issues after confirmation. Selection state is cleared on view switches to avoid leaking actions across pages and to match the accepted full-parity spec.

Actions never pull selections from another page. This keeps workflows independent and fixes cases where Repo actions such as link acted only on the cursor despite selected Repo rows.
```

In `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`, add this row after the `space` row in the main keymap table:
```markdown
| `c` | clear selection | clear selection | none |
```

- [ ] **Step 10: Verify docs and code clean**

Run:
```bash
rg -n 'reserved for Install view|`c` \\| clear selection|Doctor intentionally does not' internal/tui/modal_help.go docs/adr/0010-tui-actions-use-current-page-selection.md docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md
```
Expected: The correct text matches in respective files.

- [ ] **Step 11: Commit**

```bash
git add internal/tui/model.go internal/tui/views.go internal/tui/modal_help.go internal/tui/modal_test.go internal/tui/rows_test.go internal/tui/filter_test.go internal/tui/actions_test.go docs/adr/0010-tui-actions-use-current-page-selection.md docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md
git commit -m "docs/refactor: split selection map per page and update help"
```

---

### Task 6: Parameterize Conflict Diff Incoming Labels

**Files:**
- Modify: `internal/tui/modal_diff.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/modal_test.go`

- [ ] **Step 1: Write custom label test**

Add this test to `internal/tui/modal_test.go` after `TestConflictModalShowsFileListAndDiff`:

```go
func TestConflictModalSupportsIncomingRemoteLabel(t *testing.T) {
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "--- archive\n+++ remote\n-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModalWithIncomingLabel("zen-of-go", diff, "Incoming remote", func(string) {})

	view := plain(m.modal.View(120, 40, m))
	for _, want := range []string{"Legend:", "Archive", "Incoming remote"} {
		if !strings.Contains(view, want) {
			t.Fatalf("conflict modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Incoming active") {
		t.Fatalf("remote conflict modal should not say Incoming active:\n%s", view)
	}
}
```

- [ ] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestConflictModalSupportsIncomingRemoteLabel -count=1 -v
```

Expected before implementation:

```text
FAIL: undefined: newConflictDiffModalWithIncomingLabel
```

- [ ] **Step 3: Add incoming label field and constructor**

In `internal/tui/modal_diff.go`, update `conflictDiffModal` and constructors:

```go
type conflictDiffModal struct {
	name          string
	diff          directoryDiff
	selected      int
	scroll        int
	incomingLabel string
	apply         func(string)
}

func newConflictDiffModal(name string, diff directoryDiff, apply func(string)) modal {
	return newConflictDiffModalWithIncomingLabel(name, diff, "Incoming active", apply)
}

func newConflictDiffModalWithIncomingLabel(name string, diff directoryDiff, incomingLabel string, apply func(string)) modal {
	return conflictDiffModal{name: name, diff: diff, incomingLabel: incomingLabel, apply: apply}
}
```

- [ ] **Step 4: Use the label in rendering**

In `internal/tui/modal_diff.go`, change:

```go
		diffLegend(),
```

to:

```go
		diffLegend(c.incomingLabel),
```

Then replace `diffLegend` with:

```go
func diffLegend(incomingLabel string) string {
	return "Legend: " + archiveStyle.Render("Archive") + "  " + incomingStyle.Render(incomingLabel)
}
```

- [ ] **Step 5: Verify current caller keeps active label**

No production caller needs a behavior change now. `internal/tui/actions.go` should keep using:

```go
m.modal = newConflictDiffModal(conflict.Name, diff, func(chosen string) {
	m.applyMigrateTargets([]actions.ActiveSkill{skill}, chosen)
})
```

- [ ] **Step 6: Verify tests pass**

Run:

```bash
go test ./internal/tui -run 'TestConflictModal(ShowsFileListAndDiff|SupportsIncomingRemoteLabel)' -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit**

```bash
git add internal/tui/modal_diff.go internal/tui/actions.go internal/tui/modal_test.go
git commit -m "refactor: parameterize conflict diff incoming label"
```

---

### Task 7: Add Conflict Diff Minimum-Size Guard

**Files:**
- Modify: `internal/tui/modal_diff.go`
- Modify: `internal/tui/modal_test.go`

- [ ] **Step 1: Write resize prompt test**

Add this test to `internal/tui/modal_test.go`:

```go
func TestConflictModalPromptsResizeWhenTooSmall(t *testing.T) {
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(string) {})

	view := plain(m.modal.View(50, 12, m))
	for _, want := range []string{"Archive conflict: zen-of-go", "Terminal too small", "resize", "Esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("small diff modal missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "SKILL.md") {
		t.Fatalf("small diff modal should not squeeze diff content:\n%s", view)
	}
}
```

- [ ] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestConflictModalPromptsResizeWhenTooSmall -count=1 -v
```

Expected before implementation:

```text
FAIL
```

- [ ] **Step 3: Add size guard**

At the top of `conflictDiffModal.View` in `internal/tui/modal_diff.go`, after the constants block, add:

```go
	if width < 72 || height < 18 {
		lines := []string{
			accentStyle.Render("Archive conflict: " + c.name),
			"",
			"Terminal too small to review this diff.",
			"Please resize to at least 72x18.",
			"",
			mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
				{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
				{ASCII: "q", Label: "cancel"},
			})),
		}
		return modalStyle(width, height).Render(strings.Join(lines, "\n"))
	}
```

- [ ] **Step 4: Verify diff modal tests pass**

Run:

```bash
go test ./internal/tui -run 'TestConflictModal' -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/modal_diff.go internal/tui/modal_test.go
git commit -m "fix: guard conflict diff on tiny terminals"
```

---

### Task 8: Remove Dead `chipStyle`

**Files:**
- Modify: `internal/tui/styles.go`

- [ ] **Step 1: Confirm the symbol is dead**

Run:

```bash
rg -n 'chipStyle' internal/tui
```

Expected before cleanup:

Two matches in `internal/tui/styles.go`: the variable definition and the `NO_COLOR` reset.

- [ ] **Step 2: Remove style definition and NO_COLOR reset**

In `internal/tui/styles.go`, delete:

```go
	chipStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("60")).Padding(0, 1)
```

And delete:

```go
		chipStyle = chipStyle.UnsetForeground().UnsetBackground()
```

- [ ] **Step 3: Verify no references remain**

Run:

```bash
rg -n 'chipStyle' internal/tui || true
go test ./internal/tui -count=1
```

Expected:

```text
ok  	github.com/InkyQuill/x-skills/internal/tui
```

- [ ] **Step 4: Commit**

```bash
git add internal/tui/styles.go
git commit -m "chore: remove dead tui chip style"
```

---

### Task 9: Defensively Cap Unknown Root Chips

**Files:**
- Modify: `internal/tui/symbols.go`
- Modify: `internal/tui/rows_test.go`

- [ ] **Step 1: Write root chip fallback test**

Add this test to `internal/tui/rows_test.go` after `TestRenderPillUsesRoundedCapsuleShape`:

```go
func TestRootChipCapsUnknownTargets(t *testing.T) {
	if got := rootChip("project", "opencode"); got != ".Op" {
		t.Fatalf("rootChip(project, opencode) = %q, want .Op", got)
	}
	if got := rootChip("global", "hermes"); got != "~He" {
		t.Fatalf("rootChip(global, hermes) = %q, want ~He", got)
	}
}
```

- [ ] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestRootChipCapsUnknownTargets -count=1 -v
```

Expected before implementation:

```text
FAIL
```

- [ ] **Step 3: Update fallback logic**

In `internal/tui/symbols.go`, replace the default branch of `rootChip` with a width-capped fallback:

```go
func rootChip(scope, target string) string {
	prefix := "."
	if scope == "global" {
		prefix = "~"
	}
	switch target {
	case "agents":
		return prefix + "Ag"
	case "claude":
		return prefix + "Cl"
	case "codex":
		return prefix + "Cd"
	default:
		runes := []rune(target)
		if len(runes) == 0 {
			return prefix + "??"
		}
		first := strings.ToUpper(string(runes[0]))
		second := "?"
		if len(runes) > 1 {
			second = strings.ToLower(string(runes[1]))
		}
		return prefix + first + second
	}
}
```

Add `strings` to the import list if `symbols.go` does not already import it:

```go
import "strings"
```

- [ ] **Step 4: Verify test passes**

Run:

```bash
go test ./internal/tui -run TestRootChipCapsUnknownTargets -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/symbols.go internal/tui/rows_test.go
git commit -m "fix: cap unknown root chips"
```

---

### Task 10: Migrate Preview Scrolling To Bubbles Viewport

**Files:**
- Modify: `go.mod`
- Modify: `internal/tui/modal_preview.go`
- Modify: `internal/tui/modal_test.go`

- [ ] **Step 1: Promote Bubbles to direct dependency**

Run:

```bash
go get github.com/charmbracelet/bubbles@v0.21.1-0.20250623103423-23b8fd6302d7
```

Expected: `github.com/charmbracelet/bubbles` moves from the indirect block to the direct `require (...)` block in `go.mod`.

- [ ] **Step 2: Add bounded scroll regression test**

Add this test to `internal/tui/modal_test.go`:

```go
func TestPreviewModalScrollStateStaysBounded(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	skill := filepath.Join(cfg.MustActiveRoot("project", "agents"), "short-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: short-skill\n---\n# Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.modal = newPreviewModal("short-skill", skill)

	for i := 0; i < 100; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mustModel(t, updated)
	}
	before := plain(m.modal.View(100, 16, m))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	after := plain(m.modal.View(100, 16, m))
	if before != after {
		t.Fatalf("preview should stay stable after scrolling past end:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
```

- [ ] **Step 3: Replace manual scroll state with viewport**

In `internal/tui/modal_preview.go`, refactor `previewModal` to own a `viewport.Model`:

```go
type previewModal struct {
	title    string
	path     string
	raw      string
	rendered bool
	viewport viewport.Model
}
```

Use this import:

```go
	"github.com/charmbracelet/bubbles/viewport"
```

In `newPreviewModal`, initialize:

```go
	vp := viewport.New(0, 0)
	p := previewModal{title: title, path: filepath.Join(skillPath, "SKILL.md"), raw: raw, rendered: true, viewport: vp}
	p.viewport.SetContent(p.renderContent())
	return p
```

Add this method and use the existing rendering logic inside it:

```go
func (p previewModal) renderContent() string {
	if p.rendered {
		rendered, err := glamour.Render(p.raw, "dark")
		if err == nil {
			return strings.TrimRight(rendered, "\n")
		}
	}
	return strings.TrimRight(p.raw, "\n")
}
```

Replace `previewModal.View` with:

```go
func (p previewModal) View(width, height int, m Model) string {
	mode := "rendered with Glamour"
	if !p.rendered {
		mode = "raw SKILL.md"
	}
	bodyHeight := height - 12
	if bodyHeight < 4 {
		bodyHeight = 4
	}
	bodyWidth := width - 12
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	p.viewport.Width = bodyWidth
	p.viewport.Height = bodyHeight
	p.viewport.SetContent(p.renderContent())
	lines := []string{
		accentStyle.Render(p.title),
		p.path + "       " + mode,
		strings.Repeat("-", 60),
		p.viewport.View(),
		"",
		mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "up/down", Unicode: "↑/↓", Label: "scroll"},
			{ASCII: "r", Label: "raw/rendered"},
			{ASCII: "esc", Unicode: "Esc", Label: "close"},
			{ASCII: "q", Label: "close"},
		})),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}
```

Replace `previewModal.Update` with:

```go
func (p previewModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if msg.String() == "r" {
		p.rendered = !p.rendered
		p.viewport.SetContent(p.renderContent())
		p.viewport.GotoTop()
		m.modal = p
		return false, nil
	}
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	m.modal = p
	return false, cmd
}
```

This removes the manual `scroll` field and relies on `viewport.Model` to clamp bounds.

- [ ] **Step 4: Verify preview tests**

Run:

```bash
go test ./internal/tui -run 'TestPreviewModal' -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/tui/modal_preview.go internal/tui/modal_test.go
git commit -m "refactor: use viewport for preview modal"
```

---

### Task 11: Migrate Filter Editing To Bubbles Textinput

**Files:**
- Modify: `internal/tui/filter.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/filter_test.go`

- [ ] **Step 1: Preserve existing filter tests**

Run:

```bash
go test ./internal/tui -run 'Test.*Filter|Test.*Selection' -count=1 -v
```

Expected before refactor:

```text
PASS
```

- [ ] **Step 2: Add cursor/editing regression test**

Add this test to `internal/tui/filter_test.go`:

```go
func TestFilterSupportsBackspaceEditing(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zeta-skill", "Zeta.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zetx"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = mustModel(t, updated)

	if m.filter.Query != "zet" {
		t.Fatalf("filter query = %q, want zet", m.filter.Query)
	}
	view := plain(m.View())
	if !strings.Contains(view, "zeta-skill") || strings.Contains(view, "zen-of-go") {
		t.Fatalf("filter did not apply edited query:\n%s", view)
	}
}
```

- [ ] **Step 3: Refactor `filterState` to wrap textinput**

In `internal/tui/filter.go`, change the type to:

```go
type filterState struct {
	Active bool
	Query  string
	input  textinput.Model
}
```

Add import:

```go
	"github.com/charmbracelet/bubbles/textinput"
```

Add constructor:

```go
func newFilterState() filterState {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 200
	return filterState{input: input}
}
```

Update `filterState.update` to:

```go
func (f *filterState) update(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		f.Active = false
		f.Query = ""
		f.input.SetValue("")
		return true
	case "enter":
		f.Active = false
		return true
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	_ = cmd
	f.Query = f.input.Value()
	return true
}
```

- [ ] **Step 4: Initialize and reset filter with constructor**

In `internal/tui/model.go`, set `filter: newFilterState()` in `New`.

When opening filter mode, replace:

```go
			m.filter.Active = true
			m.filter.Query = ""
```

with:

```go
			m.filter = newFilterState()
			m.filter.Active = true
			m.filter.input.Focus()
```

In `setView`, replace:

```go
	m.filter = filterState{}
```

with:

```go
	m.filter = newFilterState()
```

- [ ] **Step 5: Verify filter tests**

Run:

```bash
go test ./internal/tui -run 'Test.*Filter' -count=1 -v
```

Expected:

```text
PASS
```

- [ ] **Step 6: Commit**

```bash
git add internal/tui/filter.go internal/tui/model.go internal/tui/filter_test.go
git commit -m "refactor: use textinput for tui filter"
```

---

### Task 12: Add Async Reload Command Scaffold

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Add reload message types**

In `internal/tui/model.go`, add:

```go
type reloadResultMsg struct {
	active    []ActiveGroup
	repo      []repo.Skill
	issues    []doctor.Issue
	repoUsage map[string][]string
	err       error
	token     int
}
```

Add `reloadToken int` to `Model`.

- [ ] **Step 2: Extract reload data loading**

In `internal/tui/model.go`, add:

```go
func loadTUIData(cfg config.Config) ([]ActiveGroup, []repo.Skill, []doctor.Issue, map[string][]string, error) {
	var firstErr error
	activeSkills, err := actions.ScanActive(cfg, actions.ScanFilter{})
	if err != nil {
		firstErr = err
	}
	activeGroups := groupActiveSkills(activeSkills)
	repoUsage := usageByRepoName(activeGroups)

	repoSkills, err := repo.List(cfg)
	if err != nil && firstErr == nil {
		firstErr = err
	}

	issues, err := doctor.Diagnose(cfg, doctor.Filter{})
	if err != nil && firstErr == nil {
		firstErr = err
	}
	sort.Slice(issues, func(i, j int) bool {
		return skillNameLess(issues[i].Name, issues[j].Name)
	})
	return activeGroups, repoSkills, issues, repoUsage, firstErr
}
```

Change `reload` to call `loadTUIData` synchronously and apply the result.

- [ ] **Step 3: Add command builder and stale detection**

In `internal/tui/model.go`, add:

```go
func (m *Model) reloadCmd() tea.Cmd {
	m.reloadToken++
	token := m.reloadToken
	cfg := m.cfg
	return func() tea.Msg {
		activeGroups, repoSkills, issues, repoUsage, err := loadTUIData(cfg)
		return reloadResultMsg{
			active:    activeGroups,
			repo:      repoSkills,
			issues:    issues,
			repoUsage: repoUsage,
			err:       err,
			token:     token,
		}
	}
}

func (m *Model) applyReloadResult(msg reloadResultMsg) {
	if msg.token != m.reloadToken {
		return
	}
	m.err = msg.err
	m.active = msg.active
	m.repo = msg.repo
	m.issues = msg.issues
	m.repoUsage = msg.repoUsage
	m.clampCursor()
}
```

In `Update`, handle `reloadResultMsg`:

```go
	case reloadResultMsg:
		m.applyReloadResult(msg)
		return m, nil
```

Keep `ctrl+r` synchronous for this task unless tests are expanded to execute returned commands. The point of this task is to create a safe command seam without changing user-visible behavior.

- [ ] **Step 4: Add stale-result test**

Add this test to `internal/tui/model_test.go`:

```go
func TestReloadResultIgnoresStaleToken(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.reloadToken = 2
	m.active = []ActiveGroup{{Name: "current"}}

	updated, _ := m.Update(reloadResultMsg{
		token:  1,
		active: []ActiveGroup{{Name: "stale"}},
	})
	m = mustModel(t, updated)
	if len(m.active) != 1 || m.active[0].Name != "current" {
		t.Fatalf("stale reload result applied: %#v", m.active)
	}
}
```

- [ ] **Step 5: Verify tests**

Run:

```bash
go test ./internal/tui -run 'TestReloadResultIgnoresStaleToken|TestCtrlRRefreshesWithoutTakingRepoKey' -count=1 -v
go test ./...
```

Expected:

```text
PASS
```

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "refactor: add async reload result scaffold"
```

---

## Final Verification

- [ ] **Step 1: Run full tests**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Scan for stale report terms**

```bash
rg -n 'reserved for Install view|chipStyle|Incoming active' internal/tui docs/adr docs/superpowers/specs
```

Expected: the remaining `Incoming active` hits are the default active-migration conflict label in `internal/tui/modal_diff.go` and tests that assert that default behavior.

- [ ] **Step 3: Manual TUI smoke check**

Run:

```bash
go run . tui --ascii
```

Manually verify:

- `R`, `D`, `A` switch views.
- `enter` opens details in Active, Repo, and Doctor.
- Doctor rows have cursor markers but no selection checkboxes.
- Repo `l`, then `enter`, shows a successful link result once.
- `?` help says Install is not available yet.
- Conflict diff still says `Incoming active` for active migration conflicts.

- [ ] **Step 4: Final commit if manual smoke changed nothing**

```bash
git status --short
```

Expected:

```text
```

No uncommitted changes should remain after the task commits.
