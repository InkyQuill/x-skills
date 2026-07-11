# TUI Production Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Go TUI build cleanly, stop stale Install work promptly, bound background checkout concurrency, expose background failures, and move refresh work off the Bubble Tea update loop.

**Architecture:** Keep the existing `internal/tui` model/message design, but make every background operation carry an operation generation and return typed results rather than synchronously unwrapping arbitrary `tea.Cmd` messages. Add one bounded state-check coordinator that coalesces work by Git source and one reload snapshot command guarded by a generation token.

**Tech Stack:** Go 1.26.5, Bubble Tea, existing remote checkout cache, standard-library contexts and semaphores, Go race detector.

## Global Constraints

- Preserve the current working-tree rollback behavior: a failed install-and-use operation removes newly archived content or restores the previous archive.
- `Update` and `View` must not perform filesystem or network work.
- Leaving Install, starting a new Install query, or quitting must cancel obsolete work.
- Background update-check failures are advisory and must not block Install, but must remain distinguishable from “up to date.”
- Keep all user changes already present in the dirty working tree.

---

## File Structure

- Modify `internal/tui/install.go`: typed row operations, generation checks, cancellation, progress, and archive-state result handling.
- Modify `internal/tui/install_test.go`: cancellation, typed-message, concurrency, error-state, rollback, and progress tests.
- Modify `internal/tui/model.go`: async reload lifecycle and view/quit cancellation.
- Modify `internal/tui/model_test.go`: refresh non-blocking and stale reload tests.
- Modify `internal/tui/rows.go`: render advisory archive-check state.
- Modify `internal/tui/rows_test.go`: failed-check badge assertions.
- Modify `docs/tui-review.md`: mark verified findings resolved and record final commands.

### Task 1: Restore A Green Build And Lock In Rollback Semantics

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`

**Interfaces:**
- Consumes: `repo.SkillPath(config.Config, string) (string, error)` and `remote.ApplyArchive` through the existing archive commands.
- Produces: `prepareInstallUseArchiveRollback(archivePath string) (backupPath string, err error)`, `rollbackInstallUseArchive(archivePath, backupPath string) error`, and `discardInstallUseArchiveRollback(backupPath string) error`.

- [ ] **Step 1: Add failing tests for new-archive and replaced-archive rollback**

Add two focused cases beside `TestInstallAndUseRollsBackPartialLinksAfterLateFailure`: one starts without an archive and asserts the archive is absent after the late link failure; the other writes an old `SKILL.md`, triggers replacement followed by a late link failure, and asserts the old bytes are restored.

```go
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
```

- [ ] **Step 2: Verify the current build failure**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestInstallAndUseRollsBack|TestInspectorRendersBlockValue'
```

Expected before the fix: FAIL to compile if any rollback helper remains undefined, or FAIL the replaced-archive assertion if cleanup deletes the previous archive.

- [ ] **Step 3: Complete the rollback helper trio**

Use sibling-directory rename semantics so backup and restore remain atomic on the same filesystem. `prepareInstallUseArchiveRollback` returns an empty path when no archive exists; `rollbackInstallUseArchive` removes the attempted archive and restores a non-empty backup; `discardInstallUseArchiveRollback` removes only the backup.

```go
func discardInstallUseArchiveRollback(backupPath string) error {
	if backupPath == "" {
		return nil
	}
	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("discard install-use archive backup: %w", err)
	}
	return nil
}
```

Call `discardInstallUseArchiveRollback` once, after every destination link succeeds. Join rollback errors with the triggering error via `errors.Join`.

- [ ] **Step 4: Run focused and race tests**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestInstallAndUseRollsBack|TestInspectorRendersBlockValue'
go test ./internal/tui/... -race -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/install.go internal/tui/install_test.go
git commit -m "fix(tui): complete install rollback handling"
```

### Task 2: Replace Synchronous Command Unwraps With Typed Row Operations

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`

**Interfaces:**
- Produces: `type installArchiveRowOperation func(context.Context) installArchiveMsg` and `runInstallArchiveRow(ctx context.Context, op installArchiveRowOperation) installArchiveMsg`.
- Consumes: the existing `installArchiveMsg`, `installArchiveBatchResult`, and `installArchiveRowCommand.row` identity.

- [ ] **Step 1: Write tests for nil and wrong-message invariants**

Replace tests that directly depend on `cmd().(installArchiveMsg)` with tests of a typed operation. Add one batch test whose operation returns an error result and assert the failure remains attributed to its own row.

```go
op := installArchiveRowOperation(func(context.Context) installArchiveMsg {
	return installArchiveMsg{name: "beta", err: errors.New("boom")}
})
msg := runInstallArchiveRow(context.Background(), op)
if msg.name != "beta" || msg.err == nil {
	t.Fatalf("msg = %#v", msg)
}
```

- [ ] **Step 2: Run the typed-operation tests and verify failure**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestRunInstallArchiveRow|TestInstallArchiveBatchAttributesFailure'
```

Expected: FAIL because `installArchiveRowOperation` is undefined.

- [ ] **Step 3: Convert archive row builders to typed operations**

Define:

```go
type installArchiveRowOperation func(context.Context) installArchiveMsg

func runInstallArchiveRow(ctx context.Context, operation installArchiveRowOperation) installArchiveMsg {
	if operation == nil {
		return installArchiveMsg{err: errors.New("nil install archive row operation")}
	}
	return operation(ctx)
}
```

Change the internal row command struct to hold `operation installArchiveRowOperation`. Tea commands become adapters at the Bubble Tea boundary only:

```go
func installArchiveOperationCmd(operation installArchiveRowOperation, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return runInstallArchiveRow(ctx, operation)
	}
}
```

Remove all production `cmd().(installArchiveMsg)` assertions.

- [ ] **Step 4: Verify no production synchronous assertions remain**

Run:

```bash
rg -n 'cmd\(\)\.\(installArchiveMsg\)' internal/tui --glob '!**/*_test.go'
go test ./internal/tui/... -race -count=1
```

Expected: the search prints no lines and tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/install.go internal/tui/install_test.go
git commit -m "refactor(tui): type install archive operations"
```

### Task 3: Cancel Stale Install Batches And Report Per-Item Progress

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`
- Modify: `internal/tui/model.go`

**Interfaces:**
- Produces: `installBatchProgressMsg{token, completed, total, name string}` and `installBatchCancelledMsg{token int}`.
- Consumes: `installUseGeneration.isCurrent(token)` and the typed row operations from Task 2.

- [ ] **Step 1: Add a cancellation test with two observable operations**

Create two operations backed by atomic counters. Invalidate the generation after the first operation and assert the second counter remains zero and the batch reports cancellation.

```go
var first, second atomic.Int32
operations := []installArchiveRowOperation{
	func(context.Context) installArchiveMsg { first.Add(1); generation.value.Add(1); return installArchiveMsg{name: "one"} },
	func(context.Context) installArchiveMsg { second.Add(1); return installArchiveMsg{name: "two"} },
}
```

- [ ] **Step 2: Verify the cancellation test fails**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestArchiveInstallRowsStopsWhenGenerationChanges|TestInstallBatchProgress'
```

Expected: FAIL because archive batches do not check generation per row and no progress message exists.

- [ ] **Step 3: Check liveness before each row and between archive/link phases**

Capture the operation token and shared generation before returning the batch command. Before each operation:

```go
if !generation.isCurrent(token) {
	return installArchiveMsg{token: token, batch: result, stale: true}
}
```

Add `stale bool` to `installArchiveMsg`; stale results must stop the queue without opening conflict continuations. Increment completed count after each terminal row result and include `completed`, `total`, and `currentName` in the batch result so the status becomes `archiving 3/8: skill-name`.

- [ ] **Step 4: Invalidate and cancel on navigation/query/quit**

In `setView`, invalidate both preview and mutation generations when leaving Install. Before returning `tea.Quit` for `q` or `ctrl+c`, call a small `m.cancelInstallWork()` method that invalidates generations and cancels the active Install context introduced in Task 5.

- [ ] **Step 5: Verify cancellation and progress**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestArchiveInstallRowsStops|TestInstallBatchProgress|TestLeavingInstallInvalidates'
go test ./internal/tui/... -race -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
git commit -m "fix(tui): cancel stale install batches"
```

### Task 4: Bound And Coalesce Archive-State Checks

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`

**Interfaces:**
- Produces: `checkInstallArchiveStates(ctx context.Context, token int, results []remote.SearchResult, limit int) installArchiveStatesMsg`.
- Produces: `installArchiveStateResult{Identity installArchiveIdentity, State string, Err error}`.

- [ ] **Step 1: Write a concurrency/coalescing test**

Inject a fake checkout function that records active count and source key. Feed results containing multiple skills from the same repository and assert maximum concurrency is `3` and checkout runs once per `(clone URL, ref)`.

```go
if maxActive.Load() > 3 {
	t.Fatalf("max concurrent checks = %d, want <= 3", maxActive.Load())
}
if got := calls["https://github.com/acme/skills.git@main"]; got != 1 {
	t.Fatalf("checkout calls = %d, want 1", got)
}
```

- [ ] **Step 2: Verify the test fails against `tea.Batch`**

Run:

```bash
go test ./internal/tui -count=1 -run TestInstallArchiveStateChecksAreBoundedAndCoalesced
```

Expected: FAIL because each result currently launches its own checkout.

- [ ] **Step 3: Implement one coordinator command per search result page**

Group eligible results by normalized source key, run at most three source groups concurrently, perform one checkout per group, and call `FindSkillContext`/`PlanArchive` for each result inside that checkout. Preserve input order in the returned slice. Use a buffered semaphore:

```go
semaphore := make(chan struct{}, limit)
semaphore <- struct{}{}
defer func() { <-semaphore }()
```

Replace `tea.Batch(stateChecks...)` in `applyInstallSearchResult` with one coordinator command.

- [ ] **Step 4: Represent failures explicitly**

Add `ArchiveCheckError string` to `installResultView`. Apply `Err` results only when token and identity still match; render a muted `check failed` pill and show the error in the inspector. A failure must not set `ArchiveState` to up-to-date.

- [ ] **Step 5: Verify bounded work and visible errors**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestInstallArchiveStateChecks|TestInstallRowShowsCheckFailed'
go test ./internal/tui/... -race -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/rows.go internal/tui/rows_test.go
git commit -m "fix(tui): bound install state checks"
```

### Task 5: Make Refresh Asynchronous And Cancelable

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

**Interfaces:**
- Produces: `reloadResultMsg{token int; active []ActiveGroup; repo []repo.Skill; issues []doctor.Issue; repoUsage map[string][]string; err error}`.
- Produces: `func (m *Model) reloadCmd() tea.Cmd` and `func (m *Model) beginReload() tea.Cmd`.

- [ ] **Step 1: Add refresh lifecycle tests**

Press `ctrl+r`, assert `Update` immediately returns a command and sets `status == "refreshing..."`; execute the command and assert its snapshot applies. Start two refreshes and assert the first result is ignored.

- [ ] **Step 2: Verify tests fail**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestRefreshReturnsCommand|TestStaleReloadResultIgnored'
```

Expected: FAIL because `ctrl+r` calls `m.reload()` synchronously.

- [ ] **Step 3: Add generation-guarded reload messages**

Add `reloadToken int` and `reloadInFlight bool` to `Model`. `beginReload` increments the token, marks pending state, captures `cfg`, and returns a command that calls `loadTUIData`. `Update` applies only matching tokens, clears pending state, clamps the cursor, and reports either `refreshed` or the error.

- [ ] **Step 4: Keep startup behavior explicit**

Keep `New` synchronously populated for now, matching the accepted design. Only manual refresh and post-mutation refresh use `beginReload`.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/tui -count=1 -run 'TestRefresh|TestStaleReload'
go test ./internal/tui/... -race -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "fix(tui): refresh through async snapshots"
```

### Task 6: Add Operation Context Cancellation And Final Verification

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/install_test.go`
- Modify: `docs/tui-review.md`

**Interfaces:**
- Produces: `installState.operationContext() (context.Context, int)` and `installState.cancelOperations()`.

- [ ] **Step 1: Add cancellation propagation tests**

Inject a checkout operation that blocks on `<-ctx.Done()`. Start it, leave Install, and assert the command returns `context.Canceled` before its timeout. Repeat for quit.

- [ ] **Step 2: Add a replaceable operation context**

Store `operationCancel context.CancelFunc` and `operationGeneration int` in `installState`. Starting a new search or mutation cancels the prior context, creates a new `context.WithCancel(context.Background())`, and increments the generation. Pass this parent context into per-operation timeouts rather than starting each timeout from `context.Background()`.

- [ ] **Step 3: Keep batch timeout policy simple**

Do not add a separate aggregate wall-clock timeout. Cancellation plus per-item progress and per-operation timeouts provide control without aborting a legitimate large batch at an arbitrary total duration.

- [ ] **Step 4: Run the full verification chain**

Run:

```bash
gofmt -w internal/tui
go test ./... -count=1
go test ./internal/tui/... -race -count=1
go vet ./...
staticcheck ./...
go build -o /tmp/x-skills ./cmd/x-skills
```

Expected: every command exits 0.

- [ ] **Step 5: Update the review ledger**

In `docs/tui-review.md`, mark findings 1–8 with their resolved commit and verification command. Keep finding 9 as passed. Do not delete the original rationale.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go docs/tui-review.md
git commit -m "fix(tui): cancel obsolete background work"
```
