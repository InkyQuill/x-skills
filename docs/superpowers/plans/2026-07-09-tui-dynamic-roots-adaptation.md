# TUI Dynamic Roots Adaptation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the TUI use dynamic managed roots from config instead of hardcoded built-in roots.

**Architecture:** Reuse `roots.ActiveRoots(cfg, roots.Filter{})` as the single source of visible roots. Replace fixed six-item destination lists with generated locations carrying label, scope, target, and path.

**Tech Stack:** Go 1.26, Bubble Tea, Lip Gloss, existing `tui`, `config`, and `roots` packages.

---

## Files

- Modify: `internal/tui/install.go` for dynamic install destination checklist.
- Modify: `internal/tui/actions.go` for dynamic repo link modal target choices.
- Modify: `internal/tui/views.go`, `internal/tui/inspector.go`, and row rendering only if labels assume fixed roots.
- Modify: `internal/tui/install_test.go`, `internal/tui/actions_test.go`, `internal/tui/render_test.go`.
- Modify: `README.md` if TUI docs mention six fixed roots.
- Modify: `docs/backlog.md` after verification.

### Task 1: Dynamic Install Destinations

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`

- [ ] **Step 1: Write failing test for custom install destination**

Add to `internal/tui/install_test.go`:

```go
func TestInstallDestinationModalUsesConfiguredRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(config.Default(project, home))
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{
		Result:       remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateArchived,
	}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("destination modal is nil")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, ".Oc") {
		t.Fatalf("destination modal missing configured root:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/tui -count=1 -run TestInstallDestinationModalUsesConfiguredRoots
```

Expected: FAIL because install destinations are hardcoded.

- [ ] **Step 3: Generate destinations from roots**

In `internal/tui/install.go`, replace fixed destination construction with:

```go
func installDestinations(cfg config.Config) []installDestination {
	activeRoots := roots.ActiveRoots(cfg, roots.Filter{})
	destinations := make([]installDestination, 0, len(activeRoots))
	for i, root := range activeRoots {
		destinations = append(destinations, installDestination{
			Scope:   root.Scope,
			Target:  root.Target,
			Label:   root.Label,
			Checked: i == 0 && root.Scope == config.ScopeProject,
		})
	}
	return destinations
}
```

If `.Ag` exists, default-check `.Ag`; otherwise default-check the first project root; otherwise no default.

- [ ] **Step 4: Run install TUI tests**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestInstallDestination|TestInstallAndUse'
```

Expected: PASS.

### Task 2: Dynamic Repo Link Modal

**Files:**
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`

- [ ] **Step 1: Write failing test for custom repo link destination**

Add to `internal/tui/actions_test.go`:

```go
func TestRepoLinkModalUsesConfiguredRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(config.Default(project, home))
	if err != nil {
		t.Fatal(err)
	}
	makeSkill(t, cfg.ArchiveSkillsRoot(), "zen-of-go", "Go style.")
	m := New(cfg)
	m.setView(ViewRepo)

	updated, _ := m.Update(keyRunes("l"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("repo link modal is nil")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, ".Oc") {
		t.Fatalf("repo link modal missing configured root:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/tui -count=1 -run TestRepoLinkModalUsesConfiguredRoots
```

Expected: FAIL because repo link modal has fixed target choices.

- [ ] **Step 3: Replace fixed scope/target controls**

In `internal/tui/actions.go`, replace target-specific modal fields with a `locations []roots.ActiveRoot` and `index int`.

Use:

```go
locations := roots.ActiveRoots(m.cfg, roots.Filter{})
```

Render each location using `root.Label` and `root.Location` equivalent (`scope:target`). On apply, pass selected `Scope` and `Target` to `actions.Link`.

- [ ] **Step 4: Run action TUI tests**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestRepoLink|TestRepoLinkModalUsesConfiguredRoots'
```

Expected: PASS.

### Task 3: Dynamic Labels In Rows, Inspectors, And Modals

**Files:**
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/inspector.go`
- Modify: `internal/tui/render_test.go`
- Modify: `internal/tui/rows_test.go`

- [ ] **Step 1: Write rendered custom label test**

Add to `internal/tui/render_test.go`:

```go
func TestActiveViewRendersConfiguredRootLabel(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(config.Default(project, home))
	if err != nil {
		t.Fatal(err)
	}
	makeSkill(t, filepath.Join(project, ".opencode", "skills"), "zen-of-go", "Go style.")
	m := New(cfg)
	view := plain(m.View())
	if !strings.Contains(view, ".Oc") {
		t.Fatalf("active view missing configured label:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify current behavior**

Run:

```bash
go test ./internal/tui -count=1 -run TestActiveViewRendersConfiguredRootLabel
```

Expected: PASS if row rendering already uses `root.Label`; FAIL if any hardcoded label remains.

- [ ] **Step 3: Remove hardcoded label assumptions**

Search:

```bash
rg -n 'Ag|Cl|Cd|agents|claude|codex|TargetAgents|TargetClaude|TargetCodex' internal/tui
```

Replace only UI destination assumptions with dynamic labels. Keep tests that intentionally exercise built-in roots.

- [ ] **Step 4: Run TUI tests**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: PASS.

### Task 4: Documentation And Backlog Cleanup

**Files:**
- Modify: `README.md`
- Modify: `docs/backlog.md`

- [ ] **Step 1: Update TUI docs**

Add:

```markdown
The TUI reads managed roots from `~/.x-skills/config.yaml`; destination checklists and root chips use configured labels.
```

- [ ] **Step 2: Remove completed TUI backlog item**

Remove the dynamic TUI roots backlog item only after dynamic install destinations, repo link modal, and rendered root labels are verified.

- [ ] **Step 3: Run verification**

Run:

```bash
go test ./cmd/x-skills ./internal/... -count=1
go build -o bin/x-skills ./cmd/x-skills
git diff --check
```

Expected: all commands exit 0.
