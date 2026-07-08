# Go TUI Full Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `x-skills tui` into the spec-complete Go/Charm maintenance manager described in `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`.

**Architecture:** Keep filesystem mutations in `internal/actions`, `internal/repo`, and `internal/doctor`; move terminal interaction into focused `internal/tui` models. Replace the current inline wizard with typed modal models, a list-plus-inspector shell, deterministic row/group view models, and small renderers that can be tested with string assertions rather than fragile full-screen snapshots.

**Tech Stack:** Go 1.26, Bubble Tea, Bubbles `viewport` and `textinput`, Lip Gloss, latest Glamour, Cobra, existing `internal/actions` mutation APIs.

---

## Source Specs

- `docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md`
- `docs/superpowers/specs/2026-07-06-go-tui-views-mockups.md`
- `docs/backlog.md`

## Scope Split

This is one implementation plan because the feature is one coherent TUI rewrite, but it is split into independently testable vertical tasks:

1. input schema and render settings;
2. active/repo/doctor view models;
3. modal infrastructure;
4. filter and selection semantics;
5. preview/details/help/result modals;
6. archive conflict diff data and modal;
7. action workbench flows;
8. shell layout and responsive rendering;
9. CLI flags and README cleanup.

Remote `skills.sh` install/search, theme switching, mouse support, fuzzy filtering, command palette, and per-file conflict merge choices remain out of scope and are already tracked in `docs/backlog.md`.

## File Structure

### Existing Files To Modify

- `go.mod`, `go.sum`: add `github.com/charmbracelet/bubbles` and `github.com/charmbracelet/glamour`.
- `internal/cli/tui.go`: add `--ascii`, pass render options into `tui.New`.
- `internal/tui/model.go`: keep Bubble Tea entry point; refactor state into shell, modal, filter, and action submodels.
- `internal/tui/views.go`: shrink to high-level `View()` composition once row renderers move into focused files.
- `internal/tui/styles.go`: define semantic styles, ASCII symbol fallback, and `NO_COLOR` handling.
- `internal/tui/model_test.go`: keep existing tests, update for uppercase global tabs and modal replacement.
- `internal/actions/migrate.go`: expose conflict diff data or add a helper beside existing `ArchiveConflictError`.
- `README.md`: replace stale `x-skills interactive` Go-path references with `x-skills tui`.

### New Files To Create

- `internal/tui/options.go`: TUI render/runtime options.
- `internal/tui/keys.go`: key constants and key routing helpers.
- `internal/tui/symbols.go`: Unicode/ASCII symbols and root chip labels.
- `internal/tui/rows.go`: active/repo/doctor row view models and grouping helpers.
- `internal/tui/rows_test.go`: row grouping, chips, aliases, status, and usage tests.
- `internal/tui/filter.go`: filter state and matching helpers.
- `internal/tui/filter_test.go`: filter matching/reset tests.
- `internal/tui/modal.go`: modal interface, modal kinds, modal routing.
- `internal/tui/modal_confirm.go`: compact confirmation modal.
- `internal/tui/modal_choice.go`: workbench/usage chooser modal.
- `internal/tui/modal_detail.go`: operational detail modal.
- `internal/tui/modal_preview.go`: Glamour/raw preview modal.
- `internal/tui/modal_diff.go`: fullscreen conflict diff modal.
- `internal/tui/modal_result.go`: result modal.
- `internal/tui/modal_help.go`: help modal.
- `internal/tui/modal_test.go`: modal routing, close/apply/cancel tests.
- `internal/tui/diff.go`: directory diff model for conflict review.
- `internal/tui/diff_test.go`: full-file text diff, added/removed file, binary/unreadable metadata tests.
- `internal/tui/actions.go`: action plans and apply functions used by modal flows.
- `internal/tui/actions_test.go`: active migrate/unlink, repo link/unlink/delete, doctor fix flow tests.
- `internal/tui/render_test.go`: shell/header/footer/responsive rendering tests.

### Files Not To Touch For This Plan

- Python/Textual files from old `interactive` specs: the Go path is `x-skills tui`.
- Remote search/install code: the Install tab is reserved but not implemented in this pass.

## Shared Test Helpers

Before starting individual tasks, keep or add these helpers in `internal/tui/model_test.go` or move them to `internal/tui/testhelpers_test.go` when multiple test files need them:

```go
func makeSkill(t *testing.T, root, name, description string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mustModel(t *testing.T, updated tea.Model) Model {
	t.Helper()
	m, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T, want tui.Model", updated)
	}
	return m
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func keyCtrlR() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlR}
}
```

Run the whole TUI package frequently:

```bash
go test ./internal/tui
```

Expected after each task: `ok github.com/InkyQuill/x-skills/internal/tui`.

---

### Task 1: TUI Options, Symbols, And Global Key Schema

**Files:**
- Create: `internal/tui/options.go`
- Create: `internal/tui/keys.go`
- Create: `internal/tui/symbols.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Write failing key schema tests**

Add these tests to `internal/tui/model_test.go`:

```go
func TestModelSwitchesViewsWithUppercaseGlobalTabs(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if m.view != ViewRepo {
		t.Fatalf("view = %q, want repo", m.view)
	}

	updated, _ = m.Update(keyRunes("D"))
	m = mustModel(t, updated)
	if m.view != ViewDoctor {
		t.Fatalf("view = %q, want doctor", m.view)
	}

	updated, _ = m.Update(keyRunes("A"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active", m.view)
	}
}

func TestLowercaseTabKeysDoNotSwitchViews(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("r"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active because lowercase r is not a tab key", m.view)
	}

	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active because lowercase d is not a tab key", m.view)
	}
}

func TestCtrlRRefreshesWithoutTakingRepoKey(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.status = "old status"

	updated, _ := m.Update(keyCtrlR())
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active after refresh", m.view)
	}
	if m.status != "refreshed" {
		t.Fatalf("status = %q, want refreshed", m.status)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestModelSwitchesViewsWithUppercaseGlobalTabs|TestLowercaseTabKeysDoNotSwitchViews|TestCtrlRRefreshesWithoutTakingRepoKey' -count=1
```

Expected: FAIL because lowercase `r`/`d` still switch views and `ctrl+r` is not handled.

- [ ] **Step 3: Add options and symbols**

Create `internal/tui/options.go`:

```go
package tui

type Options struct {
	ASCII bool
}

func defaultOptions() Options {
	return Options{}
}
```

Create `internal/tui/symbols.go`:

```go
package tui

type symbols struct {
	ProductMark string
	Cursor      string
	Unchecked   string
	Checked     string
	Managed     string
	Unmanaged   string
	Broken      string
	CountPrefix string
}

func symbolsFor(opts Options) symbols {
	if opts.ASCII {
		return symbols{
			ProductMark: "*",
			Cursor:      ">",
			Unchecked:   "[ ]",
			Checked:     "[x]",
			Managed:     "@",
			Unmanaged:   "#",
			Broken:      "!",
			CountPrefix: "x",
		}
	}
	return symbols{
		ProductMark: "◆",
		Cursor:      "›",
		Unchecked:   "□",
		Checked:     "■",
		Managed:     "✓",
		Unmanaged:   "◆",
		Broken:      "▲",
		CountPrefix: "×",
	}
}

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
		return prefix + target
	}
}
```

Create `internal/tui/keys.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

const (
	keyActive = "A"
	keyRepo   = "R"
	keyDoctor = "D"
	keyHelp   = "?"
)

func isRefreshKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyCtrlR || msg.String() == "ctrl+r"
}
```

- [ ] **Step 4: Update model construction and key routing**

Change `Model` in `internal/tui/model.go` to include options and symbols:

```go
type Model struct {
	cfg      config.Config
	opts     Options
	symbols  symbols
	view     ViewName
	width    int
	height   int
	cursor   int
	selected map[string]bool

	active []ActiveGroup
	repo   []repo.Skill
	issues []doctor.Issue

	wizard Wizard
	status string
	err    error
}

func New(cfg config.Config, opts ...Options) Model {
	options := defaultOptions()
	if len(opts) > 0 {
		options = opts[0]
	}
	m := Model{
		cfg:      cfg,
		opts:     options,
		symbols:  symbolsFor(options),
		view:     ViewActive,
		selected: map[string]bool{},
	}
	m.reload()
	return m
}
```

Update `handleKey` main-shell routing:

```go
switch {
case isRefreshKey(msg):
	m.reload()
	m.status = "refreshed"
	return m, nil
}

switch msg.String() {
case "ctrl+c", "q":
	return m, tea.Quit
case keyActive:
	m.setView(ViewActive)
case keyRepo:
	m.setView(ViewRepo)
case keyDoctor:
	m.setView(ViewDoctor)
case "up", "k":
	m.moveCursor(-1)
case "down", "j":
	m.moveCursor(1)
case " ":
	m.toggleSelection()
case "m":
	m.openWizard(ActionMigrate)
case "u":
	m.openWizard(ActionUnlink)
case "f":
	m.openWizard(ActionFixDoctor)
}
```

Do not keep lowercase `a`, `r`, or `d` as tab switches.

- [ ] **Step 5: Update header/footer rendering**

In `internal/tui/views.go`, update `renderHeader` and `renderStatus`:

```go
func renderHeader(m Model, width int) string {
	tabs := []string{
		tabLabel(m.view == ViewActive, "A", "Active"),
		tabLabel(m.view == ViewRepo, "R", "Repo"),
		tabLabel(m.view == ViewDoctor, "D", "Doctor"),
	}
	title := titleStyle.Render(m.symbols.ProductMark+" x-skills") + "  " + strings.Join(tabs, " ")
	return truncate(title, width)
}

func renderStatus(m Model, width int) string {
	var lines []string
	switch {
	case m.err != nil:
		lines = append(lines, dangerStyle.Render(m.err.Error()))
	case m.status != "":
		lines = append(lines, okStyle.Render(m.status))
	}
	lines = append(lines, mutedStyle.Render("enter details  / filter  p preview  m migrate  u unlink  ^R refresh  ? help  q quit"))
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 6: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS after updating any existing tests that still press lowercase `r` or `d` for tab navigation.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/options.go internal/tui/keys.go internal/tui/symbols.go internal/tui/model.go internal/tui/views.go internal/tui/styles.go internal/tui/model_test.go
git commit -m "feat: align tui global key schema"
```

---

### Task 2: CLI `--ascii` Flag And `NO_COLOR` Styling

**Files:**
- Modify: `internal/cli/tui.go`
- Modify: `internal/cli/tui_test.go`
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Write failing CLI option test**

Add to `internal/cli/tui_test.go`:

```go
func TestTUIAcceptsASCIIFlagWithNoInput(t *testing.T) {
	err := Execute([]string{"tui", "--ascii", "--no-input"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected no-input error")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %v, want interactive terminal message", err)
	}
}
```

- [ ] **Step 2: Write failing ASCII render test**

Add to `internal/tui/model_test.go`:

```go
func TestASCIIOptionUsesASCIISymbols(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")
	m := New(cfg, Options{ASCII: true})
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	view := m.View()
	if strings.Contains(view, "◆") || strings.Contains(view, "□") || strings.Contains(view, "■") {
		t.Fatalf("view contains unicode symbols in ASCII mode:\n%s", view)
	}
	if !strings.Contains(view, "* x-skills") {
		t.Fatalf("view missing ASCII product mark:\n%s", view)
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
go test ./internal/cli ./internal/tui -run 'TestTUIAcceptsASCIIFlagWithNoInput|TestASCIIOptionUsesASCIISymbols' -count=1
```

Expected: FAIL because `--ascii` is not wired and row rendering still hardcodes symbols.

- [ ] **Step 4: Wire CLI flag**

Change `internal/cli/tui.go`:

```go
type tuiOptions struct {
	noInput bool
	ascii   bool
}

func newTUICommand(rootOptions *options) *cobra.Command {
	var opts tuiOptions
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Open the guided skill manager",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.noInput {
				return fmt.Errorf("tui requires an interactive terminal")
			}
			program := tea.NewProgram(
				tui.New(rootOptions.config(), tui.Options{ASCII: opts.ascii}),
				tea.WithAltScreen(),
			)
			_, err := program.Run()
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.noInput, "no-input", false, "fail instead of opening the interactive manager")
	cmd.Flags().BoolVar(&opts.ascii, "ascii", false, "render ASCII symbols instead of Unicode")
	return cmd
}
```

- [ ] **Step 5: Route symbols into row prefixes**

Update `rowPrefix` in `internal/tui/views.go`:

```go
func rowPrefix(m Model, index int, id string) string {
	cursor := " "
	if index == m.cursor {
		cursor = m.symbols.Cursor
	}
	selected := m.symbols.Unchecked
	if m.selected[id] {
		selected = m.symbols.Checked
	}
	return cursorStyle.Render(cursor) + " " + selectedStyle.Render(selected)
}
```

- [ ] **Step 6: Disable colors when `NO_COLOR` is set**

In `internal/tui/styles.go`, add:

```go
import "os"

func init() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		titleStyle = titleStyle.UnsetForeground().UnsetBackground()
		tabStyle = tabStyle.UnsetForeground().UnsetBackground()
		activeTab = activeTab.UnsetForeground().UnsetBackground()
		panelStyle = panelStyle.UnsetBorderForeground()
		wizardStyle = wizardStyle.UnsetBorderForeground()
		cursorStyle = cursorStyle.UnsetForeground().UnsetBackground()
		selectedStyle = selectedStyle.UnsetForeground().UnsetBackground()
		mutedStyle = mutedStyle.UnsetForeground().UnsetBackground()
		chipStyle = chipStyle.UnsetForeground().UnsetBackground()
		okStyle = okStyle.UnsetForeground().UnsetBackground()
		accentStyle = accentStyle.UnsetForeground().UnsetBackground()
		dangerStyle = dangerStyle.UnsetForeground().UnsetBackground()
		managedStyle = managedStyle.UnsetForeground().UnsetBackground()
		unmanaged = unmanaged.UnsetForeground().UnsetBackground()
	}
}
```

- [ ] **Step 7: Run tests and verify pass**

Run:

```bash
go test ./internal/cli ./internal/tui -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/tui.go internal/cli/tui_test.go internal/tui/styles.go internal/tui/views.go internal/tui/model_test.go
git commit -m "feat: add tui ascii and no-color support"
```

---

### Task 3: Active Row View Models, Aliases, Chips, And Group Counts

**Files:**
- Create: `internal/tui/rows.go`
- Create: `internal/tui/rows_test.go`
- Modify: `internal/tui/views.go`

- [ ] **Step 1: Write failing row model tests**

Create `internal/tui/rows_test.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestActiveGroupRowsShowRootChipsAliasesAndCount(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	projectRoot := cfg.MustActiveRoot("project", "agents")
	globalRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(projectRoot, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(globalRoot, "go-style")); err != nil {
		t.Fatal(err)
	}

	groups := groupActiveSkills([]actions.ActiveSkill{
		{Name: "zen-of-go", Path: filepath.Join(projectRoot, "zen-of-go"), Root: roots.ActiveRoot{Scope: "project", Target: "agents", Label: ".agents", Path: projectRoot}, Status: actions.StatusManaged, Description: "Go style."},
		{Name: "zen-of-go", Path: filepath.Join(globalRoot, "go-style"), Root: roots.ActiveRoot{Scope: "global", Target: "claude", Label: "~/.claude", Path: globalRoot}, Status: actions.StatusManaged, Description: "Go style."},
	})

	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if groups[0].Name != "zen-of-go" {
		t.Fatalf("Name = %q, want zen-of-go", groups[0].Name)
	}
	if !containsString(groups[0].Aliases, "go-style") {
		t.Fatalf("Aliases = %#v, want go-style", groups[0].Aliases)
	}
	if !containsString(groups[0].Chips, ".Ag") || !containsString(groups[0].Chips, "~Cl") {
		t.Fatalf("Chips = %#v, want .Ag and ~Cl", groups[0].Chips)
	}
}

func TestRenderActiveRowsUseSpecSymbols(t *testing.T) {
	m := Model{
		symbols:  symbolsFor(Options{}),
		view:     ViewActive,
		selected: map[string]bool{},
		active: []ActiveGroup{{
			ID:          "active:one",
			Name:        "zen-of-go",
			Status:      actions.StatusUnmanaged,
			Description: "Go style.",
			Chips:       []string{".Ag", "~Cl"},
			Members:     []actions.ActiveSkill{{Path: "/a"}, {Path: "/b"}},
		}},
	}

	rows := renderActiveRows(m, 100)
	got := strings.Join(rows, "\n")
	for _, want := range []string{"› □", "zen-of-go", ".Ag", "~Cl", "◆ unmanaged", "×2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("row missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestActiveGroupRowsShowRootChipsAliasesAndCount|TestRenderActiveRowsUseSpecSymbols' -count=1
```

Expected: FAIL because `Aliases` and `Chips` do not exist and rows do not render `xN` badges with spec symbols.

- [ ] **Step 3: Add row model fields and helpers**

Move active row model code from `views.go` into `internal/tui/rows.go`, and define:

```go
package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
)

type ActiveGroup struct {
	ID          string
	Name        string
	Status      string
	Description string
	Chips       []string
	Aliases     []string
	Members     []actions.ActiveSkill
	Reason      string
	Fingerprint string
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	if value == "" || containsString(values, value) {
		return values
	}
	return append(values, value)
}
```

Update `groupActiveSkills` so each member appends `rootChip(skill.Root.Scope, skill.Root.Target)` to `Chips`, and basename aliases that differ from `Name` to `Aliases`.

- [ ] **Step 4: Render active rows from chips and symbols**

Update `renderActiveRows` and `activeDetail` in `internal/tui/views.go`:

```go
func renderActiveRows(m Model, width int) []string {
	var rows []string
	for i, group := range m.active {
		prefix := rowPrefix(m, i, group.ID)
		chips := chipStyle.Render(strings.Join(group.Chips, " "))
		status := renderStatusChip(m, group.Status)
		count := ""
		if len(group.Members) > 1 {
			count = " " + mutedStyle.Render(fmt.Sprintf("%s%d", m.symbols.CountPrefix, len(group.Members)))
		}
		text := fmt.Sprintf("%s %s %s %s%s  %s", prefix, group.Name, chips, status, count, activeDetail(group))
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}

func renderStatusChip(m Model, status string) string {
	switch status {
	case actions.StatusManaged:
		return managedStyle.Render(m.symbols.Managed + " managed")
	case actions.StatusUnmanaged:
		return unmanaged.Render(m.symbols.Unmanaged + " unmanaged")
	case actions.StatusBroken:
		return dangerStyle.Render(m.symbols.Broken + " broken")
	default:
		return mutedStyle.Render(status)
	}
}
```

Remove the old `renderStatusChip(status string)` from `styles.go`.

- [ ] **Step 5: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/rows.go internal/tui/rows_test.go internal/tui/views.go internal/tui/styles.go
git commit -m "feat: render active rows with chips and aliases"
```

---

### Task 4: Repo Usage Rows And Doctor Issue Rows

**Files:**
- Modify: `internal/tui/rows.go`
- Modify: `internal/tui/rows_test.go`
- Modify: `internal/tui/views.go`

- [ ] **Step 1: Write failing repo and doctor render tests**

Add to `internal/tui/rows_test.go`:

```go
func TestRepoRowsShowUsageChipsAndSelectionMarkers(t *testing.T) {
	m := Model{
		symbols:  symbolsFor(Options{}),
		view:     ViewRepo,
		cursor:   0,
		selected: map[string]bool{"repo:zen-of-go": true},
		repo: []repo.Skill{{
			Name:        "zen-of-go",
			Description: "Go style guide",
		}},
		repoUsage: map[string][]string{"zen-of-go": {".Ag", "~Cl"}},
	}

	got := strings.Join(renderRepoRows(m, 100), "\n")
	for _, want := range []string{"› ■", "zen-of-go", "Go style guide", ".Ag", "~Cl"} {
		if !strings.Contains(got, want) {
			t.Fatalf("repo row missing %q:\n%s", want, got)
		}
	}
}

func TestDoctorRowsShowIssueReasonAndLocation(t *testing.T) {
	m := Model{
		symbols:  symbolsFor(Options{}),
		view:     ViewDoctor,
		selected: map[string]bool{},
		issues: []doctor.Issue{{
			Kind:     doctor.KindBrokenSymlink,
			Name:     "zen-of-go",
			Location: ".Ag",
			Reason:   "symlink target missing",
		}},
	}

	got := strings.Join(renderDoctorRows(m, 100), "\n")
	for _, want := range []string{"›", "▲", "broken-symlink", "zen-of-go", ".Ag", "symlink target missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor row missing %q:\n%s", want, got)
		}
	}
}
```

Add imports for `github.com/InkyQuill/x-skills/internal/doctor` and `github.com/InkyQuill/x-skills/internal/repo`.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestRepoRowsShowUsageChipsAndSelectionMarkers|TestDoctorRowsShowIssueReasonAndLocation' -count=1
```

Expected: FAIL because `Model.repoUsage` does not exist and doctor rows do not use spec symbols.

- [ ] **Step 3: Add usage map to model reload**

In `internal/tui/model.go`, add:

```go
repoUsage map[string][]string
```

In `reload`, after active groups are built:

```go
m.repoUsage = usageByRepoName(m.active)
```

In `internal/tui/rows.go`, add:

```go
func usageByRepoName(groups []ActiveGroup) map[string][]string {
	usage := map[string][]string{}
	for _, group := range groups {
		for _, member := range group.Members {
			if member.Status != actions.StatusManaged {
				continue
			}
			for _, chip := range group.Chips {
				usage[member.Name] = appendUnique(usage[member.Name], chip)
			}
		}
	}
	for name := range usage {
		sort.Strings(usage[name])
	}
	return usage
}
```

- [ ] **Step 4: Render repo and doctor rows**

Update `renderRepoRows`:

```go
func renderRepoRows(m Model, width int) []string {
	var rows []string
	for i, skill := range m.repo {
		id := repoID(skill.Name)
		prefix := rowPrefix(m, i, id)
		usages := strings.Join(m.repoUsage[skill.Name], " ")
		text := fmt.Sprintf("%s %s %s %s", prefix, skill.Name, mutedStyle.Render(skill.Description), chipStyle.Render(usages))
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}
```

Update `renderDoctorRows`:

```go
func renderDoctorRows(m Model, width int) []string {
	var rows []string
	for i, issue := range m.issues {
		id := issueID(issue)
		prefix := rowPrefix(m, i, id)
		text := fmt.Sprintf("%s %s %s %s  %s", prefix, dangerStyle.Render(m.symbols.Broken), issue.Kind, chipStyle.Render(issue.Location), issue.Name+" "+issue.Reason)
		rows = append(rows, truncate(text, width-6))
	}
	return rows
}
```

- [ ] **Step 5: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/rows.go internal/tui/rows_test.go internal/tui/views.go
git commit -m "feat: render repo usage and doctor issue rows"
```

---

### Task 5: Filter State And Selection Reset Semantics

**Files:**
- Create: `internal/tui/filter.go`
- Create: `internal/tui/filter_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Write failing filter tests**

Create `internal/tui/filter_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestFilterNarrowsActiveRowsAndExcludesFullPaths(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	makeSkill(t, cfg.MustActiveRoot("project", "claude"), "prompt-master", "Prompts.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zen"))
	m = mustModel(t, updated)

	view := m.View()
	if !strings.Contains(view, "zen-of-go") {
		t.Fatalf("filtered view missing zen-of-go:\n%s", view)
	}
	if strings.Contains(view, "prompt-master") {
		t.Fatalf("filtered view still contains prompt-master:\n%s", view)
	}
	if strings.Contains(view, cfg.ProjectRoot) {
		t.Fatalf("filtered row leaked absolute path:\n%s", view)
	}
}

func TestFilterClearsOnViewSwitch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("zen"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("R"))
	m = mustModel(t, updated)

	if m.filter.Active {
		t.Fatal("filter mode still active after view switch")
	}
	if m.filter.Query != "" {
		t.Fatalf("filter query = %q, want empty", m.filter.Query)
	}
}

func TestSelectionClearsOnViewSwitch(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mustModel(t, updated)
	if len(m.selected) == 0 {
		t.Fatal("selection was not set")
	}
	updated, _ = m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if len(m.selected) != 0 {
		t.Fatalf("selected = %#v, want cleared", m.selected)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestFilterNarrowsActiveRowsAndExcludesFullPaths|TestFilterClearsOnViewSwitch|TestSelectionClearsOnViewSwitch' -count=1
```

Expected: FAIL because filter state is not implemented and selections persist.

- [ ] **Step 3: Implement filter model**

Create `internal/tui/filter.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type filterState struct {
	Active bool
	Query  string
}

func (f filterState) matches(values ...string) bool {
	query := strings.TrimSpace(strings.ToLower(f.Query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (f *filterState) update(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		f.Active = false
		f.Query = ""
		return true
	case "enter":
		f.Active = false
		return true
	case "backspace":
		if len(f.Query) > 0 {
			f.Query = f.Query[:len(f.Query)-1]
		}
		return true
	}
	if len(msg.Runes) > 0 {
		f.Query += string(msg.Runes)
		return true
	}
	return true
}
```

- [ ] **Step 4: Wire filter state into model**

Add to `Model`:

```go
filter filterState
```

At the top of `handleKey`, after modal/wizard handling and before main-shell routing:

```go
if m.filter.Active {
	if m.filter.update(msg) {
		m.cursor = 0
		return m, nil
	}
}
```

In main-shell key routing:

```go
case "/":
	if m.view == ViewActive || m.view == ViewRepo {
		m.filter.Active = true
		m.filter.Query = ""
	}
```

Update `setView`:

```go
func (m *Model) setView(view ViewName) {
	if m.view == view {
		return
	}
	m.view = view
	m.cursor = 0
	m.selected = map[string]bool{}
	m.filter = filterState{}
	m.wizard = Wizard{}
}
```

- [ ] **Step 5: Apply filter in row renderers**

In `renderActiveRows`, skip groups that do not match:

```go
if !m.filter.matches(group.Name, group.Description, group.Status, strings.Join(group.Chips, " "), strings.Join(group.Aliases, " ")) {
	continue
}
```

In `renderRepoRows`, skip skills that do not match:

```go
if !m.filter.matches(skill.Name, skill.Description, strings.Join(m.repoUsage[skill.Name], " ")) {
	continue
}
```

Do not match absolute paths.

- [ ] **Step 6: Render filter command bar above footer**

In `renderStatus`, before footer line:

```go
if m.filter.Active {
	lines = append(lines, accentStyle.Render("/ filter: "+m.filter.Query+"_"))
	lines = append(lines, mutedStyle.Render("enter accept   esc clear/exit"))
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 7: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/filter.go internal/tui/filter_test.go internal/tui/model.go internal/tui/views.go internal/tui/model_test.go
git commit -m "feat: add tui filtering semantics"
```

---

### Task 6: Typed Modal Infrastructure

**Files:**
- Create: `internal/tui/modal.go`
- Create: `internal/tui/modal_confirm.go`
- Create: `internal/tui/modal_choice.go`
- Create: `internal/tui/modal_result.go`
- Create: `internal/tui/modal_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/views.go`
- Keep temporarily: `internal/tui/wizard.go`

- [ ] **Step 1: Write failing modal routing tests**

Create `internal/tui/modal_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestModalConsumesBackgroundKeys(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.modal = newResultModal("Done", []string{"ok"})

	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	if m.view != ViewActive {
		t.Fatalf("view = %q, want active while modal is open", m.view)
	}
	if m.modal == nil {
		t.Fatal("modal closed unexpectedly")
	}
}

func TestEscClosesModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.modal = newResultModal("Done", []string{"ok"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mustModel(t, updated)
	if m.modal != nil {
		t.Fatalf("modal = %#v, want nil", m.modal)
	}
}

func TestModalRendersOverShellWithoutRemovingFooter(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.width = 100
	m.height = 30
	m.modal = newResultModal("Migration Results", []string{"2 succeeded"})

	view := m.View()
	if !strings.Contains(view, "Migration Results") {
		t.Fatalf("view missing modal:\n%s", view)
	}
	if !strings.Contains(view, "^R refresh") {
		t.Fatalf("view missing footer shortcuts:\n%s", view)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestModalConsumesBackgroundKeys|TestEscClosesModal|TestModalRendersOverShellWithoutRemovingFooter' -count=1
```

Expected: FAIL because `Model.modal` and result modal do not exist.

- [ ] **Step 3: Add modal interface and result modal**

Create `internal/tui/modal.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type modal interface {
	Title() string
	View(width, height int, m Model) string
	Update(msg tea.KeyMsg, m *Model) (close bool, cmd tea.Cmd)
}

func closeOnEscapeOrQuit(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "q":
		return true
	default:
		return false
	}
}
```

Create `internal/tui/modal_result.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type resultModal struct {
	title string
	lines []string
}

func newResultModal(title string, lines []string) modal {
	return resultModal{title: title, lines: lines}
}

func (r resultModal) Title() string {
	return r.title
}

func (r resultModal) View(width, height int, m Model) string {
	body := append([]string{accentStyle.Render(r.title), ""}, r.lines...)
	body = append(body, "", mutedStyle.Render("enter close   esc close   q close"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (r resultModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		return true, nil
	default:
		return false, nil
	}
}
```

Add `modalStyle` to `styles.go`:

```go
func modalStyle(width, height int) lipgloss.Style {
	modalWidth := width - 8
	if modalWidth > 88 {
		modalWidth = 88
	}
	if modalWidth < 40 {
		modalWidth = width - 2
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("110")).
		Padding(0, 1).
		Width(modalWidth)
}
```

- [ ] **Step 4: Route modal keys in model**

Add to `Model`:

```go
modal modal
```

At the top of `handleKey`, before wizard handling:

```go
if m.modal != nil {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	close, cmd := m.modal.Update(msg, &m)
	if close {
		m.modal = nil
	}
	return m, cmd
}
```

- [ ] **Step 5: Render modal above status/footer**

In `View`, replace wizard rendering with modal rendering:

```go
parts := []string{
	renderHeader(m, width),
	renderRows(m, width, bodyHeight),
}
if m.modal != nil {
	parts = append(parts, m.modal.View(width, height, m))
}
parts = append(parts, renderStatus(m, width))
```

Leave `wizard` code in place for one task if needed, but do not render it once modal exists.

- [ ] **Step 6: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS after updating tests that expected `wizard.Open` rendering.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/modal.go internal/tui/modal_result.go internal/tui/modal_test.go internal/tui/model.go internal/tui/views.go internal/tui/styles.go
git commit -m "feat: add typed tui modal routing"
```

---

### Task 7: Detail, Help, And Preview Modals

**Files:**
- Create: `internal/tui/modal_detail.go`
- Create: `internal/tui/modal_help.go`
- Create: `internal/tui/modal_preview.go`
- Modify: `internal/tui/modal_test.go`
- Modify: `internal/tui/model.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add Glamour dependency**

Run:

```bash
go get github.com/charmbracelet/glamour@latest
```

Expected: `go.mod` includes `github.com/charmbracelet/glamour`.

- [ ] **Step 2: Write failing modal open tests**

Add to `internal/tui/modal_test.go`:

```go
func TestEnterOpensActiveDetailModal(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Detail: zen-of-go", "Canonical name", "Active members", "Debug"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail modal missing %q:\n%s", want, view)
		}
	}
}

func TestQuestionMarkOpensHelpModalWithGlobalKeys(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)

	updated, _ := m.Update(keyRunes("?"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Help", "A", "R", "D", "I", "^R", ".Ag", "~Cd"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help modal missing %q:\n%s", want, view)
		}
	}
}

func TestPreviewModalTogglesRawAndRendered(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)

	updated, _ := m.Update(keyRunes("p"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	rendered := m.modal.View(100, 30, m)
	if !strings.Contains(rendered, "rendered with Glamour") {
		t.Fatalf("preview missing rendered marker:\n%s", rendered)
	}

	updated, _ = m.Update(keyRunes("r"))
	m = mustModel(t, updated)
	raw := m.modal.View(100, 30, m)
	if !strings.Contains(raw, "raw SKILL.md") {
		t.Fatalf("preview missing raw marker:\n%s", raw)
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestEnterOpensActiveDetailModal|TestQuestionMarkOpensHelpModalWithGlobalKeys|TestPreviewModalTogglesRawAndRendered' -count=1
```

Expected: FAIL because detail/help/preview modals are not implemented.

- [ ] **Step 4: Implement detail modal**

Create `internal/tui/modal_detail.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type detailModal struct {
	title string
	lines []string
}

func newDetailModal(title string, lines []string) modal {
	return detailModal{title: title, lines: lines}
}

func activeDetailModal(group ActiveGroup) modal {
	lines := []string{
		"Canonical name: " + group.Name,
		"Status: " + group.Status,
		"Aliases: " + strings.Join(group.Aliases, ", "),
		"",
		"Active members",
	}
	for _, member := range group.Members {
		lines = append(lines, fmt.Sprintf("  %s  %s", rootChip(member.Root.Scope, member.Root.Target), member.Path))
	}
	lines = append(lines, "", "Debug", "  fingerprint: "+group.Fingerprint)
	return newDetailModal("Detail: "+group.Name+" (Active)", lines)
}

func (d detailModal) Title() string { return d.title }

func (d detailModal) View(width, height int, m Model) string {
	body := append([]string{accentStyle.Render(d.title), ""}, d.lines...)
	body = append(body, "", mutedStyle.Render("up/down scroll   esc close   q close"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (d detailModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	return closeOnEscapeOrQuit(msg), nil
}
```

- [ ] **Step 5: Implement help modal**

Create `internal/tui/modal_help.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type helpModal struct{}

func newHelpModal() modal { return helpModal{} }
func (h helpModal) Title() string { return "Help" }

func (h helpModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render("Help"),
		"Keyboard Shortcuts",
		"  A        switch to Active view",
		"  R        switch to Repo view",
		"  D        switch to Doctor view",
		"  I        Install (design in progress, not yet available)",
		"  enter    view row details",
		"  /        enter local filter mode",
		"  space    toggle Active/Repo row selection",
		"  p        preview SKILL.md",
		"  ^R       rescan filesystem",
		"  ?        show this help screen",
		"  q        quit application",
		"",
		"Symbol Legend",
		"  " + m.symbols.Cursor + "  cursor position",
		"  " + m.symbols.Unchecked + "  unselected item",
		"  " + m.symbols.Checked + "  selected item",
		"  " + m.symbols.CountPrefix + "N group count badge",
		"",
		"Root Chip Legend",
		"  .Ag  project agents",
		"  .Cl  project claude",
		"  .Cd  project codex",
		"  ~Ag  global agents",
		"  ~Cl  global claude",
		"  ~Cd  global codex",
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (h helpModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	return closeOnEscapeOrQuit(msg), nil
}
```

- [ ] **Step 6: Implement preview modal**

Create `internal/tui/modal_preview.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

type previewModal struct {
	title    string
	path     string
	raw      string
	rendered bool
}

func newPreviewModal(title, skillPath string) modal {
	rawBytes, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	raw := ""
	if err != nil {
		raw = "read SKILL.md: " + err.Error()
	} else {
		raw = string(rawBytes)
	}
	return previewModal{title: title, path: filepath.Join(skillPath, "SKILL.md"), raw: raw, rendered: true}
}

func (p previewModal) Title() string { return p.title }

func (p previewModal) View(width, height int, m Model) string {
	mode := "rendered with Glamour"
	bodyText := p.raw
	if p.rendered {
		if rendered, err := glamour.Render(p.raw, "dark"); err == nil {
			bodyText = rendered
		}
	} else {
		mode = "raw SKILL.md"
	}
	lines := []string{
		accentStyle.Render(p.title),
		p.path + "       " + mode,
		strings.Repeat("-", 60),
		bodyText,
		"",
		mutedStyle.Render("up/down scroll   r raw/rendered   esc close   q close"),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (p previewModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	if closeOnEscapeOrQuit(msg) {
		return true, nil
	}
	if msg.String() == "r" {
		p.rendered = !p.rendered
		m.modal = p
	}
	return false, nil
}
```

- [ ] **Step 7: Open modals from model**

In `handleKey`, add:

```go
case "enter":
	m.openDetailModal()
case "?":
	m.modal = newHelpModal()
case "p":
	m.openPreviewModal()
```

Add helper methods in `model.go`:

```go
func (m *Model) openDetailModal() {
	switch m.view {
	case ViewActive:
		if m.cursor >= 0 && m.cursor < len(m.active) {
			m.modal = activeDetailModal(m.active[m.cursor])
		}
	}
}

func (m *Model) openPreviewModal() {
	switch m.view {
	case ViewActive:
		if m.cursor >= 0 && m.cursor < len(m.active) && len(m.active[m.cursor].Members) > 0 {
			m.modal = newPreviewModal("Preview: "+m.active[m.cursor].Name, resolvedSkillPath(m.active[m.cursor].Members[0].Path))
		}
	case ViewRepo:
		if m.cursor >= 0 && m.cursor < len(m.repo) {
			if path, err := repo.SkillPath(m.cfg, m.repo[m.cursor].Name); err == nil {
				m.modal = newPreviewModal("Preview: "+m.repo[m.cursor].Name, path)
			}
		}
	}
}

func resolvedSkillPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
```

Import `path/filepath` and `github.com/InkyQuill/x-skills/internal/repo`.

- [ ] **Step 8: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/tui/modal_detail.go internal/tui/modal_help.go internal/tui/modal_preview.go internal/tui/modal_test.go internal/tui/model.go
git commit -m "feat: add tui detail help and preview modals"
```

---

### Task 8: Directory Conflict Diff Model

**Files:**
- Create: `internal/tui/diff.go`
- Create: `internal/tui/diff_test.go`
- Modify: `internal/actions/migrate.go`

- [ ] **Step 1: Write failing diff tests**

Create `internal/tui/diff_test.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDirectoryDiffShowsFullFileUnifiedDiff(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "zen-of-go", "Active.")
	makeSkill(t, archive, "zen-of-go", "Archived.")

	diff, err := buildDirectoryDiff(filepath.Join(active, "zen-of-go"), filepath.Join(archive, "zen-of-go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Files) == 0 {
		t.Fatal("diff files is empty")
	}
	text := diff.Files[0].Text
	for _, want := range []string{"---", "-description: Archived.", "+description: Active.", "# zen-of-go"} {
		if !strings.Contains(text, want) {
			t.Fatalf("diff text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildDirectoryDiffMarksAddedAndRemovedFiles(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "skill", "Active.")
	makeSkill(t, archive, "skill", "Active.")
	if err := os.WriteFile(filepath.Join(active, "skill", "new.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "skill", "old.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := buildDirectoryDiff(filepath.Join(active, "skill"), filepath.Join(archive, "skill"))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, file := range diff.Files {
		kinds[file.Path] = file.Kind
	}
	if kinds["new.md"] != "added" {
		t.Fatalf("new.md kind = %q, want added", kinds["new.md"])
	}
	if kinds["old.md"] != "removed" {
		t.Fatalf("old.md kind = %q, want removed", kinds["old.md"])
	}
}

func TestBuildDirectoryDiffShowsBinaryMetadata(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "skill", "Active.")
	makeSkill(t, archive, "skill", "Active.")
	if err := os.WriteFile(filepath.Join(active, "skill", "logo.png"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "skill", "logo.png"), []byte{0, 1, 9, 9}, 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := buildDirectoryDiff(filepath.Join(active, "skill"), filepath.Join(archive, "skill"))
	if err != nil {
		t.Fatal(err)
	}
	var binary string
	for _, file := range diff.Files {
		if file.Path == "logo.png" {
			binary = file.Text
		}
	}
	for _, want := range []string{"Binary file", "archive:", "active:", "sha256:"} {
		if !strings.Contains(binary, want) {
			t.Fatalf("binary metadata missing %q:\n%s", want, binary)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestBuildDirectoryDiff' -count=1
```

Expected: FAIL because `buildDirectoryDiff` does not exist.

- [ ] **Step 3: Implement diff model**

Create `internal/tui/diff.go`:

```go
package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type directoryDiff struct {
	ActivePath  string
	ArchivePath string
	Files       []diffFile
}

type diffFile struct {
	Path string
	Kind string
	Text string
}

func buildDirectoryDiff(active, archive string) (directoryDiff, error) {
	activeFiles, err := readDiffFiles(active)
	if err != nil {
		return directoryDiff{}, err
	}
	archiveFiles, err := readDiffFiles(archive)
	if err != nil {
		return directoryDiff{}, err
	}
	keys := mergedKeys(activeFiles, archiveFiles)
	result := directoryDiff{ActivePath: active, ArchivePath: archive}
	for _, key := range keys {
		activeData, activeOK := activeFiles[key]
		archiveData, archiveOK := archiveFiles[key]
		if activeOK && archiveOK && string(activeData) == string(archiveData) {
			continue
		}
		file := diffFile{Path: key, Kind: "changed"}
		switch {
		case activeOK && !archiveOK:
			file.Kind = "added"
			file.Text = fullFileDiff("", string(activeData))
		case !activeOK && archiveOK:
			file.Kind = "removed"
			file.Text = fullFileDiff(string(archiveData), "")
		case isBinary(activeData) || isBinary(archiveData):
			file.Kind = "binary"
			file.Text = binaryMetadata(activeData, archiveData)
		default:
			file.Text = fullFileDiff(string(archiveData), string(activeData))
		}
		result.Files = append(result.Files, file)
	}
	return result, nil
}

func readDiffFiles(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			files[filepath.ToSlash(rel)] = []byte("unreadable: " + err.Error())
			return nil
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files, err
}

func mergedKeys(left, right map[string][]byte) []string {
	seen := map[string]bool{}
	var keys []string
	for key := range left {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range right {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if !utf8.Valid(data) {
		return true
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func fullFileDiff(archiveText, activeText string) string {
	var lines []string
	lines = append(lines, "--- archive", "+++ active")
	archiveLines := strings.Split(strings.TrimSuffix(archiveText, "\n"), "\n")
	activeLines := strings.Split(strings.TrimSuffix(activeText, "\n"), "\n")
	max := len(archiveLines)
	if len(activeLines) > max {
		max = len(activeLines)
	}
	for i := 0; i < max; i++ {
		var archiveLine, activeLine string
		if i < len(archiveLines) {
			archiveLine = archiveLines[i]
		}
		if i < len(activeLines) {
			activeLine = activeLines[i]
		}
		switch {
		case archiveLine == activeLine:
			lines = append(lines, " "+archiveLine)
		case archiveLine == "":
			lines = append(lines, "+"+activeLine)
		case activeLine == "":
			lines = append(lines, "-"+archiveLine)
		default:
			lines = append(lines, "-"+archiveLine, "+"+activeLine)
		}
	}
	return strings.Join(lines, "\n")
}

func binaryMetadata(active, archive []byte) string {
	return fmt.Sprintf(
		"Binary file\narchive: %d B  sha256: %s\nactive:  %d B  sha256: %s\n\nNo text diff is available.",
		len(archive), shortSHA(archive), len(active), shortSHA(active),
	)
}

func shortSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run 'TestBuildDirectoryDiff' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/diff.go internal/tui/diff_test.go
git commit -m "feat: add tui archive conflict diff model"
```

---

### Task 9: Fullscreen Conflict Diff Modal

**Files:**
- Create: `internal/tui/modal_diff.go`
- Modify: `internal/tui/modal_test.go`
- Modify: `internal/tui/wizard.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing conflict modal tests**

Add to `internal/tui/modal_test.go`:

```go
func TestConflictModalShowsFileListAndDiff(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "zen-of-go", "Active.")
	makeSkill(t, archive, "zen-of-go", "Archived.")
	diff, err := buildDirectoryDiff(filepath.Join(active, "zen-of-go"), filepath.Join(archive, "zen-of-go"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(actions.ConflictResolution) {})

	view := m.modal.View(120, 40, m)
	for _, want := range []string{"Archive conflict: zen-of-go", "Decision applies to the whole skill directory", "Files", "SKILL.md", "-description: Archived.", "+description: Active.", "k keep archive", "l save active"} {
		if !strings.Contains(view, want) {
			t.Fatalf("conflict modal missing %q:\n%s", want, view)
		}
	}
}

func TestConflictModalAppliesKeepArchiveKey(t *testing.T) {
	called := ""
	diff := directoryDiff{Files: []diffFile{{Path: "SKILL.md", Kind: "changed", Text: "-old\n+new"}}}
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.modal = newConflictDiffModal("zen-of-go", diff, func(resolution actions.ConflictResolution) {
		called = string(resolution)
	})

	updated, _ := m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	if called != actions.ConflictResolutionKeepArchive {
		t.Fatalf("called = %q, want keep archive", called)
	}
	if m.modal != nil {
		t.Fatal("modal still open after resolution")
	}
}
```

If `actions.ConflictResolution` is not a named type, use `func(string)` in `newConflictDiffModal`.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestConflictModal' -count=1
```

Expected: FAIL because `newConflictDiffModal` does not exist.

- [ ] **Step 3: Implement conflict modal**

Create `internal/tui/modal_diff.go`:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/actions"
)

type conflictDiffModal struct {
	name     string
	diff     directoryDiff
	selected int
	apply    func(string)
}

func newConflictDiffModal(name string, diff directoryDiff, apply func(string)) modal {
	return conflictDiffModal{name: name, diff: diff, apply: apply}
}

func (c conflictDiffModal) Title() string {
	return "Archive conflict: " + c.name
}

func (c conflictDiffModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render("Archive conflict: " + c.name),
		"Decision applies to the whole skill directory.",
		"",
		"Files                         | full file diff",
		strings.Repeat("-", 72),
	}
	for i, file := range c.diff.Files {
		cursor := " "
		if i == c.selected {
			cursor = m.symbols.Cursor
		}
		marker := diffMarker(file.Kind)
		firstLine := firstDiffLine(file.Text)
		lines = append(lines, fmt.Sprintf("%s %s %-26s | %s", cursor, marker, file.Path, firstLine))
	}
	if len(c.diff.Files) > 0 {
		lines = append(lines, "", c.diff.Files[c.selected].Text)
	}
	lines = append(lines, "", mutedStyle.Render("up/down scroll   tab focus   k keep archive   l save active   esc cancel   q close"))
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (c conflictDiffModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up", "k":
		if c.selected > 0 {
			c.selected--
		}
		m.modal = c
		return false, nil
	case "down", "j":
		if c.selected+1 < len(c.diff.Files) {
			c.selected++
		}
		m.modal = c
		return false, nil
	case "k":
		c.apply(actions.ConflictResolutionKeepArchive)
		return true, nil
	case "l":
		c.apply(actions.ConflictResolutionUseActive)
		return true, nil
	default:
		return false, nil
	}
}

func diffMarker(kind string) string {
	switch kind {
	case "added":
		return "+"
	case "removed":
		return "-"
	case "binary":
		return "!"
	default:
		return "±"
	}
}

func firstDiffLine(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}
```

- [ ] **Step 4: Fix key conflict inside diff modal**

The modal spec uses `k` for keep archive and arrow keys for scrolling. Remove `"k"` from the scroll case in `Update`:

```go
case "up":
	...
case "down":
	...
case "k":
	c.apply(actions.ConflictResolutionKeepArchive)
	return true, nil
```

- [ ] **Step 5: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run 'TestConflictModal' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/modal_diff.go internal/tui/modal_test.go
git commit -m "feat: add fullscreen conflict diff modal"
```

---

### Task 10: Active Migrate Flow With Same-SHA Relink And Divergent Conflict

**Files:**
- Create: `internal/tui/actions.go`
- Create: `internal/tui/actions_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/wizard.go`

- [ ] **Step 1: Write failing migrate flow tests**

Create `internal/tui/actions_test.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func TestActiveMigrateSameSHAArchivesRelinkWithoutConflict(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	active := makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Same.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Same.")

	m := New(cfg)
	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("migrate modal did not open")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.modal == nil || !strings.Contains(m.modal.View(100, 30, m), "Migration Results") {
		t.Fatalf("expected result modal, got %#v", m.modal)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("active resolved to %q, want %q", resolved, archived)
	}
	if !strings.Contains(m.status, "relinked") {
		t.Fatalf("status = %q, want relinked", m.status)
	}
}

func TestActiveMigrateDivergentArchiveOpensConflictModal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Active.")
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Archived.")

	m := New(cfg)
	updated, _ := m.Update(keyRunes("m"))
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := m.modal.View(120, 40, m)
	if !strings.Contains(view, "Archive conflict: zen-of-go") {
		t.Fatalf("expected conflict modal:\n%s", view)
	}

	updated, _ = m.Update(keyRunes("k"))
	m = mustModel(t, updated)
	info, err := skills.Read(archived)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Archived." {
		t.Fatalf("archive description = %q, want Archived.", info.Description)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestActiveMigrate' -count=1
```

Expected: FAIL because `m` still opens the old wizard or does not open modal flow.

- [ ] **Step 3: Create action result types**

Create `internal/tui/actions.go`:

```go
package tui

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/actions"
)

type actionResult struct {
	Name   string
	Status string
	Error  error
}

func (m *Model) activeTargets() []actions.ActiveSkill {
	return m.selectedActiveSkills(ActionMigrate)
}

func (m *Model) openMigrateModal() {
	targets := m.activeTargets()
	if len(targets) == 0 {
		m.modal = newResultModal("Migrate active skills", []string{"No unmanaged active skill directories selected."})
		return
	}
	lines := []string{"Targets"}
	for _, target := range targets {
		lines = append(lines, "  "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target))
	}
	lines = append(lines, "", "Plan", "  1. Compare active content with archive", "  2. If identical, relink active copies", "  3. If different, review full-file diff")
	m.modal = newConfirmModal("Migrate active skills", lines, false, func() {
		m.applyMigrateTargets(targets, actions.ConflictResolutionAsk)
	})
}

func (m *Model) applyMigrateTargets(targets []actions.ActiveSkill, resolution string) {
	var lines []string
	for _, skill := range targets {
		result, err := actions.Migrate(m.cfg, actions.MigrateRequest{
			Name:               filepath.Base(skill.Path),
			Scope:              skill.Root.Scope,
			Target:             skill.Root.Target,
			Confirmed:          true,
			ConflictResolution: resolution,
		})
		if err != nil {
			var conflict *actions.ArchiveConflictError
			if errors.As(err, &conflict) {
				diff, diffErr := buildDirectoryDiff(conflict.ActivePath, conflict.ArchivedPath)
				if diffErr != nil {
					m.modal = newResultModal("Migration Results", []string{fmt.Sprintf("failed to build conflict diff: %v", diffErr)})
					return
				}
				m.modal = newConflictDiffModal(conflict.Name, diff, func(chosen string) {
					m.applyMigrateTargets([]actions.ActiveSkill{skill}, chosen)
				})
				return
			}
			lines = append(lines, "x "+filepath.Base(skill.Path)+"  "+err.Error())
			continue
		}
		lines = append(lines, "✓ "+result.Name+"  "+result.Status)
		m.status = result.Status + " " + result.Name
	}
	m.reload()
	m.modal = newResultModal("Migration Results", lines)
}
```

This uses `newConfirmModal`, created in the next step.

- [ ] **Step 4: Implement compact confirmation modal**

Create `internal/tui/modal_confirm.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type confirmModal struct {
	title       string
	lines       []string
	destructive bool
	choice      int
	apply       func()
}

func newConfirmModal(title string, lines []string, destructive bool, apply func()) modal {
	choice := 0
	if destructive {
		choice = 1
	}
	return confirmModal{title: title, lines: lines, destructive: destructive, choice: choice, apply: apply}
}

func (c confirmModal) Title() string { return c.title }

func (c confirmModal) View(width, height int, m Model) string {
	buttons := "[ Apply ]   Cancel"
	if c.choice == 1 {
		buttons = "Apply   [ Cancel ]"
	}
	body := append([]string{accentStyle.Render(c.title), ""}, c.lines...)
	body = append(body, "", buttons, mutedStyle.Render("left/right choose   enter apply   y/n select   esc cancel"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (c confirmModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "n":
		return true, nil
	case "left", "right":
		if c.choice == 0 {
			c.choice = 1
		} else {
			c.choice = 0
		}
		m.modal = c
		return false, nil
	case "y":
		c.apply()
		return false, nil
	case "enter":
		if c.choice == 0 {
			c.apply()
			return false, nil
		}
		return true, nil
	default:
		return false, nil
	}
}
```

- [ ] **Step 5: Route migrate key**

In `handleKey`, replace `m.openWizard(ActionMigrate)` with:

```go
m.openMigrateModal()
```

- [ ] **Step 6: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run 'TestActiveMigrate' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/actions.go internal/tui/actions_test.go internal/tui/modal_confirm.go internal/tui/model.go
git commit -m "feat: migrate active skills through modals"
```

---

### Task 11: Active Unlink Workbench Modal

**Files:**
- Create: `internal/tui/modal_choice.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing active unlink test**

Add to `internal/tui/actions_test.go`:

```go
func TestActiveUnlinkGroupsManagedBrokenAndUnmanaged(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "managed", "Managed.")
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(root, "managed")); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, root, "unmanaged", "Unmanaged.")
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	m.selected = map[string]bool{}
	for _, group := range m.active {
		m.selected[group.ID] = true
	}
	updated, _ := m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("unlink modal is nil")
	}
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Unlink active skills", "Managed links", "Broken links", "Unmanaged directories", "Migrate to repo, then unlink active copies"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unlink modal missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/tui -run TestActiveUnlinkGroupsManagedBrokenAndUnmanaged -count=1
```

Expected: FAIL because `u` still opens wizard.

- [ ] **Step 3: Implement choice modal**

Create `internal/tui/modal_choice.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type choiceModal struct {
	title   string
	lines   []string
	choices []string
	index   int
	apply   func(int)
}

func newChoiceModal(title string, lines, choices []string, defaultIndex int, apply func(int)) modal {
	return choiceModal{title: title, lines: lines, choices: choices, index: defaultIndex, apply: apply}
}

func (c choiceModal) Title() string { return c.title }

func (c choiceModal) View(width, height int, m Model) string {
	body := append([]string{accentStyle.Render(c.title), ""}, c.lines...)
	body = append(body, "")
	for i, choice := range c.choices {
		prefix := "  "
		if i == c.index {
			prefix = m.symbols.Cursor + " "
		}
		body = append(body, prefix+choice)
	}
	body = append(body, "", mutedStyle.Render("up/down choose   enter apply   esc cancel   q close"))
	return modalStyle(width, height).Render(strings.Join(body, "\n"))
}

func (c choiceModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up":
		if c.index > 0 {
			c.index--
		}
		m.modal = c
	case "down":
		if c.index+1 < len(c.choices) {
			c.index++
		}
		m.modal = c
	case "enter":
		c.apply(c.index)
	}
	return false, nil
}
```

- [ ] **Step 4: Implement unlink modal opener**

Add to `internal/tui/actions.go`:

```go
func (m *Model) openUnlinkModal() {
	targets := m.selectedActiveSkills(ActionUnlink)
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink active skills", []string{"No active skills selected."})
		return
	}
	lines := []string{"Managed links"}
	for _, target := range targets {
		if target.Status == actions.StatusManaged {
			lines = append(lines, "  ✓ "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target)+"  remove symlink only")
		}
	}
	lines = append(lines, "", "Broken links")
	for _, target := range targets {
		if target.Status == actions.StatusBroken {
			lines = append(lines, "  ▲ "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target)+"  remove broken symlink")
		}
	}
	lines = append(lines, "", "Unmanaged directories")
	for _, target := range targets {
		if target.Status == actions.StatusUnmanaged {
			lines = append(lines, "  ◆ "+filepath.Base(target.Path)+"  "+rootChip(target.Root.Scope, target.Root.Target))
		}
	}
	choices := []string{"Migrate to repo, then unlink active copies", "Delete active copies without archiving", "Cancel"}
	m.modal = newChoiceModal("Unlink active skills", lines, choices, 0, func(choice int) {
		if choice == 2 {
			m.modal = nil
			return
		}
		m.applyUnlinkTargets(targets, choice == 1)
	})
}

func (m *Model) applyUnlinkTargets(targets []actions.ActiveSkill, deleteUnmanaged bool) {
	var lines []string
	for _, skill := range targets {
		result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{
			Name:            filepath.Base(skill.Path),
			Scope:           skill.Root.Scope,
			Target:          skill.Root.Target,
			Confirmed:       true,
			DeleteUnmanaged: deleteUnmanaged,
		})
		if err != nil {
			lines = append(lines, "x "+filepath.Base(skill.Path)+"  "+err.Error())
			continue
		}
		lines = append(lines, "✓ "+result.Name+"  "+result.Status)
	}
	m.reload()
	m.modal = newResultModal("Unlink Results", lines)
}
```

- [ ] **Step 5: Route unlink key**

In `handleKey`, replace `m.openWizard(ActionUnlink)` with:

```go
m.openUnlinkModal()
```

- [ ] **Step 6: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run 'TestActiveUnlink|TestActiveMigrate' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/modal_choice.go internal/tui/actions.go internal/tui/actions_test.go internal/tui/model.go
git commit -m "feat: add active unlink workbench modal"
```

---

### Task 12: Repo Link, Repo Unlink Usages, And Repo Delete Flows

**Files:**
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/repo/repo.go` if no delete helper exists

- [ ] **Step 1: Write failing repo action tests**

Add to `internal/tui/actions_test.go`:

```go
func TestRepoLinkModalShowsDestinationAndCreatesLink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("repo link modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Link repo skill", "scope", "project", "target", ".Ag", "Will create"} {
		if !strings.Contains(view, want) {
			t.Fatalf("link modal missing %q:\n%s", want, view)
		}
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot("project", "agents"), "zen-of-go")); err != nil {
		t.Fatalf("link was not created: %v", err)
	}
}

func TestRepoLinkModalCanChangeDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if _, err := os.Lstat(filepath.Join(cfg.GlobalCodexRoot, "zen-of-go")); err != nil {
		t.Fatalf("global codex link was not created: %v", err)
	}
}

func TestRepoUnlinkUsageChooserDefaultsAllUsagesSelected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	projectRoot := cfg.MustActiveRoot("project", "agents")
	globalRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(projectRoot, "zen-of-go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(globalRoot, "zen-of-go")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("u"))
	m = mustModel(t, updated)
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Unlink usages: zen-of-go", "■ .Ag", "■ ~Cl", "Unlink selected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("usage chooser missing %q:\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(filepath.Join(projectRoot, "zen-of-go")); !os.IsNotExist(err) {
		t.Fatalf("project usage still exists or unexpected error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(globalRoot, "zen-of-go")); !os.IsNotExist(err) {
		t.Fatalf("global usage still exists or unexpected error: %v", err)
	}
}

func TestRepoDeleteWithUsagesShowsScopeLimit(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archived, filepath.Join(root, "zen-of-go")); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("d"))
	m = mustModel(t, updated)
	view := m.modal.View(110, 35, m)
	for _, want := range []string{"Delete archive: zen-of-go", "Visible usages", "Only current project roots and global roots are known", "Unlink visible usages, then delete archive"} {
		if !strings.Contains(view, want) {
			t.Fatalf("delete modal missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestRepoLinkModal|TestRepoUnlinkUsageChooser|TestRepoDeleteWithUsagesShowsScopeLimit' -count=1
```

Expected: FAIL because repo action modals are not implemented.

- [ ] **Step 3: Implement repo link modal with destination state**

Add to `internal/tui/actions.go`:

```go
type repoLinkModal struct {
	name        string
	scope       string
	target      string
	field       int
	destination string
}

func (m *Model) currentRepoSkillName() (string, bool) {
	if m.view != ViewRepo || m.cursor < 0 || m.cursor >= len(m.repo) {
		return "", false
	}
	return m.repo[m.cursor].Name, true
}

func (m *Model) openRepoLinkModal() {
	name, ok := m.currentRepoSkillName()
	if !ok {
		return
	}
	modal := repoLinkModal{name: name, scope: config.ScopeProject, target: config.TargetAgents}
	modal.destination = modal.destinationPath(m)
	m.modal = modal
}

func (r repoLinkModal) Title() string {
	return "Link repo skill"
}

func (r repoLinkModal) destinationPath(m *Model) string {
	root, err := m.cfg.ActiveRoot(r.scope, r.target)
	if err != nil {
		return err.Error()
	}
	return filepath.Join(root, r.name)
}

func (r repoLinkModal) View(width, height int, m Model) string {
	projectCursor := " "
	globalCursor := " "
	agentsCursor := " "
	claudeCursor := " "
	codexCursor := " "
	if r.field == 0 && r.scope == config.ScopeProject {
		projectCursor = m.symbols.Cursor
	}
	if r.field == 0 && r.scope == config.ScopeGlobal {
		globalCursor = m.symbols.Cursor
	}
	if r.field == 1 && r.target == config.TargetAgents {
		agentsCursor = m.symbols.Cursor
	}
	if r.field == 1 && r.target == config.TargetClaude {
		claudeCursor = m.symbols.Cursor
	}
	if r.field == 1 && r.target == config.TargetCodex {
		codexCursor = m.symbols.Cursor
	}
	lines := []string{
		accentStyle.Render("Link repo skill"),
		"Skill",
		"  " + r.name,
		"",
		"Destination",
		"  scope   " + projectCursor + " project    " + globalCursor + " global",
		"  target  " + agentsCursor + " .Ag        " + claudeCursor + " .Cl        " + codexCursor + " .Cd",
		"",
		"Will create",
		"  " + r.destination + " -> " + filepath.Join(m.cfg.ArchiveSkillsRoot(), r.name),
		"",
		mutedStyle.Render("left/right change option   tab field   enter link   esc cancel"),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

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
		result, err := actions.Link(m.cfg, actions.LinkRequest{Name: r.name, Scope: r.scope, Target: r.target})
		if err != nil {
			m.modal = newResultModal("Link Results", []string{"x " + err.Error()})
			return false, nil
		}
		m.reload()
		m.modal = newResultModal("Link Results", []string{"✓ " + result.Name + " linked"})
	}
	return false, nil
}

func (r *repoLinkModal) move(direction string) {
	if r.field == 0 {
		if r.scope == config.ScopeProject {
			r.scope = config.ScopeGlobal
		} else {
			r.scope = config.ScopeProject
		}
		return
	}
	targets := []string{config.TargetAgents, config.TargetClaude, config.TargetCodex}
	current := 0
	for i, target := range targets {
		if target == r.target {
			current = i
			break
		}
	}
	if direction == "right" {
		current = (current + 1) % len(targets)
	} else {
		current = (current + len(targets) - 1) % len(targets)
	}
	r.target = targets[current]
}
```

- [ ] **Step 4: Implement repo usage chooser**

Add to `internal/tui/actions.go`:

```go
type repoUsageTarget struct {
	Name   string
	Scope  string
	Target string
	Chip   string
	Path   string
}

type repoUsageModal struct {
	name     string
	targets  []repoUsageTarget
	selected map[int]bool
	index    int
}

func (m *Model) openRepoUnlinkModal() {
	name, ok := m.currentRepoSkillName()
	if !ok {
		return
	}
	targets := m.repoUsageTargets(name)
	if len(targets) == 0 {
		m.modal = newResultModal("Unlink Results", []string{"No current usages for " + name + "."})
		return
	}
	selected := map[int]bool{}
	for i := range targets {
		selected[i] = true
	}
	m.modal = repoUsageModal{name: name, targets: targets, selected: selected}
}

func (m Model) repoUsageTargets(name string) []repoUsageTarget {
	var targets []repoUsageTarget
	for _, group := range m.active {
		for _, member := range group.Members {
			if member.Name == name && member.Status == actions.StatusManaged {
				targets = append(targets, repoUsageTarget{
					Name:   filepath.Base(member.Path),
					Scope:  member.Root.Scope,
					Target: member.Root.Target,
					Chip:   rootChip(member.Root.Scope, member.Root.Target),
					Path:   member.Path,
				})
			}
		}
	}
	return targets
}

func (r repoUsageModal) Title() string {
	return "Unlink usages: " + r.name
}

func (r repoUsageModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render("Unlink usages: " + r.name),
		"Select current usages to remove.",
		"",
	}
	for i, target := range r.targets {
		cursor := " "
		if i == r.index {
			cursor = m.symbols.Cursor
		}
		check := m.symbols.Unchecked
		if r.selected[i] {
			check = m.symbols.Checked
		}
		lines = append(lines, cursor+" "+check+" "+target.Chip+"  "+target.Path)
	}
	lines = append(lines, "", "[ Unlink selected ]   Cancel", mutedStyle.Render("up/down move   space toggle   enter choose   esc cancel"))
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (r repoUsageModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up":
		if r.index > 0 {
			r.index--
		}
		m.modal = r
	case "down":
		if r.index+1 < len(r.targets) {
			r.index++
		}
		m.modal = r
	case " ":
		r.selected[r.index] = !r.selected[r.index]
		m.modal = r
	case "enter":
		m.applyRepoUsageUnlink(r)
	}
	return false, nil
}

func (m *Model) applyRepoUsageUnlink(r repoUsageModal) {
	var lines []string
	for i, target := range r.targets {
		if !r.selected[i] {
			continue
		}
		result, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: target.Name, Scope: target.Scope, Target: target.Target, Confirmed: true})
		if err != nil {
			lines = append(lines, "x "+target.Path+": "+err.Error())
			continue
		}
		lines = append(lines, "✓ "+result.Name+"  "+result.Status)
	}
	m.reload()
	m.modal = newResultModal("Unlink Results", lines)
}
```

- [ ] **Step 5: Implement repo delete modal**

Add to `internal/tui/actions.go`:

```go
func (m *Model) openRepoDeleteModal() {
	name, ok := m.currentRepoSkillName()
	if !ok {
		return
	}
	lines := []string{"This archive is used in the current working set.", "", "Visible usages"}
	for _, group := range m.active {
		for _, member := range group.Members {
			if member.Name == name && member.Status == actions.StatusManaged {
				lines = append(lines, "  "+rootChip(member.Root.Scope, member.Root.Target)+"  "+member.Path)
			}
		}
	}
	lines = append(lines, "", "Scope limit", "  Only current project roots and global roots are known. Other projects may need x-skills doctor afterwards.")
	m.modal = newChoiceModal("Delete archive: "+name, lines, []string{"Cancel", "Unlink visible usages, then delete archive"}, 0, func(choice int) {
		if choice == 0 {
			m.modal = nil
			return
		}
		m.applyRepoDelete(name)
	})
}

func (m *Model) applyRepoDelete(name string) {
	var lines []string
	for _, group := range m.active {
		for _, member := range group.Members {
			if member.Name == name && member.Status == actions.StatusManaged {
				_, err := actions.Unlink(m.cfg, actions.UnlinkRequest{Name: filepath.Base(member.Path), Scope: member.Root.Scope, Target: member.Root.Target, Confirmed: true})
				if err != nil {
					lines = append(lines, "x unlink "+member.Path+": "+err.Error())
				}
			}
		}
	}
	archivePath, err := repo.SkillPath(m.cfg, name)
	if err != nil {
		lines = append(lines, "x "+err.Error())
		m.modal = newResultModal("Delete Results", lines)
		return
	}
	if err := os.RemoveAll(archivePath); err != nil {
		lines = append(lines, "x delete "+archivePath+": "+err.Error())
	} else {
		lines = append(lines, "✓ deleted "+name)
	}
	m.reload()
	m.modal = newResultModal("Delete Results", lines)
}
```

Add imports for `os`, `strings`, `tea "github.com/charmbracelet/bubbletea"`, `internal/config`, and `internal/repo`.

- [ ] **Step 6: Route repo keys**

In `handleKey`:

```go
case "l":
	if m.view == ViewRepo {
		m.openRepoLinkModal()
	}
case "u":
	if m.view == ViewRepo {
		m.openRepoUnlinkModal()
	} else {
		m.openUnlinkModal()
	}
case "d":
	if m.view == ViewRepo {
		m.openRepoDeleteModal()
	}
```

Keep lowercase `d` as a Repo-only action; it must not switch Doctor.

- [ ] **Step 7: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run 'TestRepoLink|TestRepoUnlink|TestRepoDelete' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/actions.go internal/tui/actions_test.go internal/tui/model.go
git commit -m "feat: add repo link unlink and delete modals"
```

---

### Task 13: Doctor Fix Confirmation Modal

**Files:**
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing doctor fix test**

Add to `internal/tui/actions_test.go`:

```go
func TestDoctorFixModalShowsIssueCountsAndApplies(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "zen-of-go")
	if err := os.Symlink(filepath.Join(home, "missing"), broken); err != nil {
		t.Fatal(err)
	}

	m := New(cfg)
	updated, _ := m.Update(keyRunes("D"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("f"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("doctor fix modal is nil")
	}
	view := m.modal.View(100, 30, m)
	for _, want := range []string{"Confirm", "Apply", "Doctor fixes", "broken symlink"} {
		if !strings.Contains(view, want) {
			t.Fatalf("doctor fix modal missing %q:\n%s", want, view)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if _, err := os.Lstat(broken); !os.IsNotExist(err) {
		t.Fatalf("broken symlink still exists or unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/tui -run TestDoctorFixModalShowsIssueCountsAndApplies -count=1
```

Expected: FAIL because `f` still uses wizard or old flow.

- [ ] **Step 3: Implement doctor fix modal**

Add to `internal/tui/actions.go`:

```go
func (m *Model) openDoctorFixModal() {
	if len(m.issues) == 0 {
		m.modal = newResultModal("Doctor Results", []string{"No doctor issues."})
		return
	}
	brokenCount := 0
	for _, issue := range m.issues {
		if issue.Kind == doctor.KindBrokenSymlink {
			brokenCount++
		}
	}
	lines := []string{
		fmt.Sprintf("Apply %d Doctor fixes?", len(m.issues)),
		"",
		fmt.Sprintf("  - %d broken symlink issues", brokenCount),
	}
	m.modal = newConfirmModal("Confirm", lines, false, func() {
		results, err := doctor.FixIssues(m.issues)
		var output []string
		for _, result := range results {
			output = append(output, "✓ "+result.Name+"  "+result.Action)
		}
		if err != nil {
			output = append(output, "x "+err.Error())
		}
		m.reload()
		m.modal = newResultModal("Doctor Results", output)
	})
}
```

Add import for `internal/doctor` if missing.

- [ ] **Step 4: Route doctor fix key**

In `handleKey`:

```go
case "f":
	if m.view == ViewDoctor {
		m.openDoctorFixModal()
	}
```

- [ ] **Step 5: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run TestDoctorFixModalShowsIssueCountsAndApplies -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/actions.go internal/tui/actions_test.go internal/tui/model.go
git commit -m "feat: add doctor fix confirmation modal"
```

---

### Task 14: List Plus Inspector Shell And Responsive Layout

**Files:**
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/styles.go`
- Create: `internal/tui/render_test.go`

- [ ] **Step 1: Write failing render tests**

Create `internal/tui/render_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestWideShellRendersListInspectorStatusAndFooter(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 120
	m.height = 34
	m.status = "relinked zen-of-go to existing archive"

	view := m.View()
	for _, want := range []string{"A Active", "R Repo", "D Doctor", "Active skills", "Inspector", "zen-of-go", "relinked zen-of-go", "^R refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wide shell missing %q:\n%s", want, view)
		}
	}
}

func TestNarrowShellCollapsesInspector(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.MustActiveRoot("project", "agents"), "zen-of-go", "Go style.")
	m := New(cfg)
	m.width = 80
	m.height = 24

	view := m.View()
	if strings.Contains(view, "Inspector") {
		t.Fatalf("narrow shell should not show inspector:\n%s", view)
	}
	if !strings.Contains(view, "Active skills") || !strings.Contains(view, "^R refresh") {
		t.Fatalf("narrow shell missing list/footer:\n%s", view)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'TestWideShellRendersListInspectorStatusAndFooter|TestNarrowShellCollapsesInspector' -count=1
```

Expected: FAIL because there is no inspector shell.

- [ ] **Step 3: Render shell panels**

In `internal/tui/views.go`, replace `renderRows` usage in `View` with:

```go
body := renderBody(m, width, bodyHeight)
parts := []string{renderHeader(m, width), body}
```

Add:

```go
func renderBody(m Model, width, height int) string {
	list := renderListPanel(m, width, height)
	if width < 100 {
		return list
	}
	inspectorWidth := 32
	listWidth := width - inspectorWidth - 3
	left := renderListPanel(m, listWidth, height)
	right := renderInspector(m, inspectorWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func renderListPanel(m Model, width, height int) string {
	title := "Active skills"
	if m.view == ViewRepo {
		title = "Repo skills"
	}
	if m.view == ViewDoctor {
		title = "Doctor issues"
	}
	rows := rowsForView(m, width)
	if len(rows) == 0 {
		rows = []string{mutedStyle.Render("No items.")}
	}
	if len(rows) > height {
		start := visibleStart(m.cursor, len(rows), height)
		rows = rows[start : start+height]
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return panelStyle.Width(width - 2).Render(title + "\n" + strings.Join(rows, "\n"))
}

func rowsForView(m Model, width int) []string {
	switch m.view {
	case ViewActive:
		return renderActiveRows(m, width)
	case ViewRepo:
		return renderRepoRows(m, width)
	case ViewDoctor:
		return renderDoctorRows(m, width)
	default:
		return nil
	}
}
```

- [ ] **Step 4: Render inspector**

Add:

```go
func renderInspector(m Model, width, height int) string {
	lines := []string{"Inspector", ""}
	switch m.view {
	case ViewActive:
		if m.cursor >= 0 && m.cursor < len(m.active) {
			group := m.active[m.cursor]
			lines = append(lines, "◇ "+group.Name, "aliases", "  "+strings.Join(group.Aliases, ", "), "repo", "  "+group.Status)
		}
	case ViewRepo:
		if m.cursor >= 0 && m.cursor < len(m.repo) {
			skill := m.repo[m.cursor]
			lines = append(lines, "◇ "+skill.Name, "description", skill.Description, "usages", "  "+strings.Join(m.repoUsage[skill.Name], " "))
		}
	case ViewDoctor:
		if m.cursor >= 0 && m.cursor < len(m.issues) {
			issue := m.issues[m.cursor]
			lines = append(lines, "◇ "+issue.Kind, "path", issue.Path, "reason", issue.Reason, "fix", issue.SafeFix)
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return panelStyle.Width(width - 2).Render(strings.Join(lines[:height], "\n"))
}
```

- [ ] **Step 5: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -run 'TestWideShell|TestNarrowShell|TestRowsScroll' -count=1
```

Expected: PASS, including the existing scroll test.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/views.go internal/tui/styles.go internal/tui/render_test.go
git commit -m "feat: render tui list inspector shell"
```

---

### Task 15: Remove Wizard Path And Stabilize Model Tests

**Files:**
- Delete or empty: `internal/tui/wizard.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/actions_test.go`

- [ ] **Step 1: Write failing assertion that wizard is gone**

Add to `internal/tui/model_test.go`:

```go
func TestModelUsesModalInsteadOfWizard(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "opentui-react", "OpenTUI.")
	m := New(cfg)
	updated, _ := m.Update(keyRunes("R"))
	m = mustModel(t, updated)
	updated, _ = m.Update(keyRunes("l"))
	m = mustModel(t, updated)

	if m.modal == nil {
		t.Fatal("modal is nil")
	}
	view := m.View()
	if strings.Contains(strings.ToLower(view), "wizard") {
		t.Fatalf("view still contains wizard language:\n%s", view)
	}
}
```

- [ ] **Step 2: Run tests and verify failure if wizard remains visible**

Run:

```bash
go test ./internal/tui -run TestModelUsesModalInsteadOfWizard -count=1
```

Expected: FAIL if old wizard strings are still rendered or old key paths are still active.

- [ ] **Step 3: Remove wizard state**

Remove from `Model`:

```go
wizard Wizard
```

Remove wizard-specific handling from `handleKey`, including scope/target keys `p/g/1/2/3`.

Delete `internal/tui/wizard.go` after all replacement action paths compile:

```bash
git rm internal/tui/wizard.go
```

If `selectedActiveSkills`, `selectedRepoNames`, or `selectedIssues` are still needed, move them into `internal/tui/actions.go` and remove `WizardAction` from their signatures:

```go
func (m Model) selectedActiveSkillsForAction(action string) []actions.ActiveSkill
```

Use action strings `"migrate"` and `"unlink"` or define local constants in `actions.go`.

- [ ] **Step 4: Update obsolete tests**

Replace old wizard tests:

- `TestWizardPreviewIncludesDestination` becomes repo link modal test.
- `TestWizardConsumesBackgroundKeys` becomes `TestModalConsumesBackgroundKeys`.
- `TestFooterShortcutsStayVisibleWithStatusAndWizard` becomes status plus modal footer test.
- `TestMigrateWizardIgnoresManagedDuplicateLinks` becomes active migrate modal no-target result test.
- `TestMigrateWizardPromptsForArchiveConflict` becomes conflict modal test.

Do not delete coverage; rename tests and assert modal behavior.

- [ ] **Step 5: Run tests and verify pass**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS and no references to `Wizard` remain.

- [ ] **Step 6: Search for wizard references**

Run:

```bash
rg -n 'Wizard|wizard|ActionInstall|openWizard|renderWizard' internal/tui
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/tui
git commit -m "refactor: replace tui wizard with modals"
```

---

### Task 16: README Command Cleanup

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Write failing README check**

Run:

```bash
rg -n 'x-skills interactive|interactive command|Textual-based manager' README.md
```

Expected before edit: lines still mention the old interactive command.

- [ ] **Step 2: Update README wording**

Replace Go-path references to `x-skills interactive` with `x-skills tui`. Keep historical Python/design docs untouched.

Required README snippets:

```markdown
go run ./cmd/x-skills tui
```

```markdown
`x-skills tui` opens the Bubble Tea maintenance manager for active skills, repo skills, and doctor issues.
```

```markdown
Use `x-skills tui` for longer maintenance sessions where you need previews, conflict review, or grouped link/unlink operations.
```

- [ ] **Step 3: Verify README no longer points users to old command**

Run:

```bash
rg -n 'x-skills interactive|Textual-based manager' README.md
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document go tui command"
```

---

### Task 17: Full Verification And Manual Smoke

**Files:**
- No planned source edits.

- [ ] **Step 1: Run package tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run focused TUI tests with race detector**

Run:

```bash
go test -race ./internal/tui
```

Expected: PASS.

- [ ] **Step 3: Build binary**

Run:

```bash
go build ./cmd/x-skills
```

Expected: PASS and binary `./x-skills` is created in the repository root.

- [ ] **Step 4: Smoke non-interactive guard**

Run:

```bash
go run ./cmd/x-skills tui --no-input
```

Expected: exits non-zero with:

```text
tui requires an interactive terminal
```

- [ ] **Step 5: Manual TUI smoke**

Run:

```bash
go run ./cmd/x-skills tui
```

Expected manual checks:

- header shows `A Active  R Repo  D Doctor`;
- `R`, `D`, and `A` switch views;
- lowercase `d` does not switch Doctor from Active;
- `ctrl+r` refreshes and shows a status line;
- footer remains visible when modal/result text is visible;
- list scroll keeps cursor visible;
- `?` opens Help and `esc` closes it;
- `enter` opens details and `esc` closes it;
- `/` opens filter in Active/Repo and clears on view switch;
- `p` opens preview and `r` toggles raw/rendered;
- same-SHA migrate relinks without conflict;
- divergent migrate opens fullscreen conflict diff;
- Repo delete modal states current-project/global visibility limit.

- [ ] **Step 6: Final consistency search**

Run:

```bash
rg -n 'x-skills interactive|wizard|R refresh|`a`|`r`' README.md internal/tui docs/superpowers/specs/2026-07-06-go-tui-full-parity-design.md docs/superpowers/specs/2026-07-06-go-tui-views-mockups.md
```

Expected:

- no `x-skills interactive` in README or `internal/tui`;
- no `wizard` in `internal/tui`;
- no `R refresh`;
- no lowercase tab-key spec entries;
- no stale user-facing command or keymap wording.

- [ ] **Step 7: Commit any verification-only fixes**

If Step 6 required edits, run:

```bash
go test ./...
git add README.md internal/tui docs/superpowers/specs
git commit -m "test: verify go tui parity"
```

Expected: commit only if there were actual fixes.

## Self-Review

### Spec Coverage

- List + inspector shell: Tasks 3, 4, 14.
- Unicode symbols, ASCII fallback, `NO_COLOR`: Tasks 1, 2.
- Uppercase global navigation and `ctrl+r` refresh: Task 1.
- Typed modal system: Tasks 6, 7, 9, 10, 11, 12, 13, 15.
- Fullscreen conflict diff with full-file unified diff and binary metadata: Tasks 8, 9.
- Active migrate same-SHA relink and divergent conflict: Task 10.
- Active unlink grouped workbench: Task 11.
- Repo preview/link/unlink/delete: Tasks 7 and 12, including destination switching and the usage chooser.
- Doctor browse/fix: Tasks 4 and 13.
- Filtering and selection reset: Task 5.
- Responsive layout: Task 14.
- README cleanup: Task 16.
- Verification: Task 17.

### Placeholder Scan

The plan intentionally contains only concrete implementation and verification steps.

### Type Consistency

The plan uses these stable names across tasks:

- `Options`, `symbols`, `symbolsFor`, `rootChip`
- `filterState`
- `modal`, `newResultModal`, `newConfirmModal`, `newChoiceModal`, `newHelpModal`, `newPreviewModal`, `newConflictDiffModal`
- `directoryDiff`, `diffFile`, `buildDirectoryDiff`
- `openMigrateModal`, `openUnlinkModal`, `openRepoLinkModal`, `openRepoDeleteModal`, `openDoctorFixModal`

If an executor chooses to rename one of these, update all subsequent tasks in the plan before continuing.
