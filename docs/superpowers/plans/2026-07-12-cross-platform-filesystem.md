# Cross-Platform Filesystem Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `x-skills` pass and behave correctly on Linux, macOS, and Windows by using semantic path identity and supporting archive rename on all three platforms.

**Architecture:** Add `internal/pathidentity` as the single path canonicalization/equality boundary, then replace scattered path comparisons in actions and syncer. Add platform-specific `renameNoReplace` files and update tests to assert path identity instead of raw path spelling.

**Tech Stack:** Go 1.26.5, standard library filesystem APIs, `golang.org/x/sys/unix`, `golang.org/x/sys/windows`, GitHub Actions matrix.

---

### Task 1: Add Path Identity Package

**Files:**
- Create: `internal/pathidentity/pathidentity.go`
- Create: `internal/pathidentity/pathidentity_generic.go`
- Create: `internal/pathidentity/pathidentity_windows.go`
- Create: `internal/pathidentity/pathidentity_test.go`

- [ ] **Step 1: Write failing tests for existing paths, symlinks, missing entries, and errors**

Create `internal/pathidentity/pathidentity_test.go`:

```go
package pathidentity

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEquivalentEAcceptsSameExistingDirectory(t *testing.T) {
	root := t.TempDir()
	got, err := EquivalentE(root, filepath.Clean(root))
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("EquivalentE(%q, %q) = false, want true", root, filepath.Clean(root))
	}
}

func TestEquivalentEAcceptsSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable on %s: %v", runtime.GOOS, err)
	}

	got, err := EquivalentE(link, target)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("EquivalentE(%q, %q) = false, want true", link, target)
	}
}

func TestCanonicalEntryPreservesMissingBaseAndCanonicalizesParent(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "parent-link")
	parent := filepath.Join(root, "parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parent, link); err != nil {
		t.Skipf("symlink unavailable on %s: %v", runtime.GOOS, err)
	}

	got, err := CanonicalEntry(filepath.Join(link, "missing-skill"))
	if err != nil {
		t.Fatal(err)
	}
	wantParent, err := Canonical(parent)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(wantParent, "missing-skill")
	if got != want {
		t.Fatalf("CanonicalEntry() = %q, want %q", got, want)
	}
}

func TestEquivalentEFallsBackToCanonicalEntryForMissingPaths(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}

	first := filepath.Join(parent, "missing")
	second := filepath.Join(parent, ".", "missing")
	got, err := EquivalentE(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("EquivalentE(%q, %q) = false, want true", first, second)
	}
}

func TestEquivalentEReturnsUnexpectedStatErrors(t *testing.T) {
	got, err := EquivalentE("", t.TempDir())
	if err == nil {
		t.Fatalf("EquivalentE empty path = %v, nil; want error", got)
	}
}

func TestEquivalentWrapsErrorsAsFalse(t *testing.T) {
	if Equivalent("", t.TempDir()) {
		t.Fatal("Equivalent empty path = true, want false")
	}
}

func TestDarwinVarAliasIsEquivalent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only alias")
	}
	tmp := t.TempDir()
	if !strings.HasPrefix(tmp, "/var/") {
		t.Skipf("temp dir does not use /var alias: %s", tmp)
	}
	alias := "/private" + tmp
	got, err := EquivalentE(tmp, alias)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("EquivalentE(%q, %q) = false, want true", tmp, alias)
	}
}

func TestWindowsCanonicalizationHandlesShortAndLongNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path spelling")
	}
	tmp := t.TempDir()
	canonical, err := Canonical(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(canonical, "~1") {
		t.Fatalf("Canonical(%q) = %q, want long path form", tmp, canonical)
	}
}
```

- [ ] **Step 2: Run pathidentity tests to verify they fail**

Run: `mise exec -- go test ./internal/pathidentity`

Expected: FAIL because `internal/pathidentity` does not exist.

- [ ] **Step 3: Implement path identity helpers**

Create `internal/pathidentity/pathidentity.go`:

```go
package pathidentity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Canonical(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return platformCanonical(filepath.Clean(abs))
}

func CanonicalEntry(path string) (string, error) {
	if path == "" || filepath.Base(filepath.Clean(path)) == "." {
		return "", fmt.Errorf("invalid entry path %q", path)
	}
	parent, err := Canonical(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(filepath.Clean(path))), nil
}

func Equivalent(a, b string) bool {
	ok, err := EquivalentE(a, b)
	return err == nil && ok
}

func EquivalentE(a, b string) (bool, error) {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	switch {
	case aErr == nil && bErr == nil:
		if os.SameFile(aInfo, bInfo) {
			return true, nil
		}
	case aErr != nil && !errors.Is(aErr, os.ErrNotExist):
		return false, aErr
	case bErr != nil && !errors.Is(bErr, os.ErrNotExist):
		return false, bErr
	}

	canonA, err := CanonicalEntry(a)
	if err != nil {
		return false, err
	}
	canonB, err := CanonicalEntry(b)
	if err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(canonA, canonB), nil
	}
	return canonA == canonB, nil
}
```

Create `internal/pathidentity/pathidentity_generic.go`:

```go
//go:build !windows

package pathidentity

func platformCanonical(path string) (string, error) {
	return path, nil
}
```

- [ ] **Step 4: Add Windows final-path canonicalization hook**

Create `internal/pathidentity/pathidentity_windows.go`:

```go
package pathidentity

import (
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func platformCanonical(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		p,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return path, nil
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), windows.FILE_NAME_NORMALIZED)
	if err != nil || n == 0 {
		return path, nil
	}
	if n > uint32(len(buf)) {
		buf = make([]uint16, n)
		n, err = windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), windows.FILE_NAME_NORMALIZED)
		if err != nil || n == 0 {
			return path, nil
		}
	}
	result := string(utf16.Decode(buf[:n]))
	result = strings.TrimPrefix(result, `\\?\`)
	result = strings.TrimPrefix(result, `\\?\UNC\`)
	if strings.HasPrefix(result, `UNC\`) {
		result = `\\` + strings.TrimPrefix(result, `UNC\`)
	}
	return result, nil
}
```

- [ ] **Step 5: Run pathidentity tests to verify they pass**

Run: `mise exec -- go test ./internal/pathidentity`

Expected: PASS.

- [ ] **Step 6: Commit path identity package**

```bash
git add internal/pathidentity
git commit -m "fix: add cross-platform path identity helpers"
```

### Task 2: Add Cross-Platform No-Replace Rename

**Files:**
- Modify: `internal/actions/rename_noreplace_other.go`
- Create: `internal/actions/rename_noreplace_darwin.go`
- Create: `internal/actions/rename_noreplace_windows.go`
- Create or modify: `internal/actions/rename_noreplace_test.go`

- [ ] **Step 1: Write no-replace rename tests**

Create `internal/actions/rename_noreplace_test.go`:

```go
package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameNoReplaceMovesWhenDestinationMissing(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old")
	newPath := filepath.Join(root, "new")
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := renameNoReplace(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new path missing: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old path still exists or unexpected error: %v", err)
	}
}

func TestRenameNoReplaceRefusesExistingDestination(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old")
	newPath := filepath.Join(root, "new")
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(newPath, 0o755); err != nil {
		t.Fatal(err)
	}

	err := renameNoReplace(oldPath, newPath)
	if err == nil {
		t.Fatal("renameNoReplace replaced existing destination")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "exist") {
		t.Fatalf("error = %v, want existing destination error", err)
	}
	if _, statErr := os.Stat(oldPath); statErr != nil {
		t.Fatalf("old path changed after failed rename: %v", statErr)
	}
	if _, statErr := os.Stat(newPath); statErr != nil {
		t.Fatalf("new path changed after failed rename: %v", statErr)
	}
}
```

- [ ] **Step 2: Run rename tests to verify macOS/Windows currently fail**

Run: `mise exec -- go test ./internal/actions -run TestRenameNoReplace -count=1`

Expected on macOS before implementation: FAIL with unsupported darwin. Expected on Linux: PASS.

- [ ] **Step 3: Fix fallback build tags**

Update `internal/actions/rename_noreplace_other.go`:

```go
//go:build !linux && !darwin && !windows

package actions

import (
	"fmt"
	"runtime"
)

func renameNoReplace(oldPath, newPath string) error {
	return fmt.Errorf("atomic no-replace rename is unsupported on %s", runtime.GOOS)
}
```

- [ ] **Step 4: Add macOS no-replace rename**

Create `internal/actions/rename_noreplace_darwin.go`:

```go
//go:build darwin

package actions

import (
	"errors"
	"os"
)

func renameNoReplace(oldPath, newPath string) error {
	placeholder, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if closeErr := placeholder.Close(); closeErr != nil {
		_ = os.Remove(newPath)
		return closeErr
	}
	if removeErr := os.Remove(newPath); removeErr != nil {
		return removeErr
	}
	if _, err := os.Lstat(newPath); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return err
		}
		if _, statErr := os.Lstat(newPath); statErr == nil {
			return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: os.ErrExist}
		}
		return err
	}
	return nil
}
```

- [ ] **Step 5: Add Windows no-replace rename**

Create `internal/actions/rename_noreplace_windows.go`:

```go
//go:build windows

package actions

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func renameNoReplace(oldPath, newPath string) error {
	if _, err := os.Lstat(newPath); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	oldPtr, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPtr, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(oldPtr, newPtr, windows.MOVEFILE_WRITE_THROUGH)
	if err == nil {
		return nil
	}
	if _, statErr := os.Lstat(newPath); statErr == nil {
		return os.ErrExist
	}
	return err
}
```

- [ ] **Step 6: Run rename tests**

Run: `mise exec -- go test ./internal/actions -run TestRenameNoReplace -count=1`

Expected: PASS on the current platform.

- [ ] **Step 7: Commit rename support**

```bash
git add internal/actions/rename_noreplace_other.go internal/actions/rename_noreplace_darwin.go internal/actions/rename_noreplace_windows.go internal/actions/rename_noreplace_test.go
git commit -m "fix: support archive rename across platforms"
```

### Task 3: Integrate Path Identity Into Actions

**Files:**
- Modify: `internal/actions/scan.go`
- Modify: `internal/actions/migrate.go`
- Modify: `internal/actions/rename.go`
- Modify: `internal/symlinkcheck/symlinkcheck.go`
- Modify tests under `internal/actions`, `internal/cli`, `internal/doctor`, and `internal/symlinkcheck` that compare raw symlink targets.

- [ ] **Step 1: Write or update failing assertions to use semantic identity**

Add this helper to affected test files that compare `filepath.EvalSymlinks` output to a temp path:

```go
func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	ok, err := pathidentity.EquivalentE(got, want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("path = %q, want same location as %q", got, want)
	}
}
```

Update imports in those test files to include:

```go
"github.com/InkyQuill/x-skills/internal/pathidentity"
```

Run: `mise exec -- go test ./internal/actions ./internal/cli ./internal/doctor ./internal/symlinkcheck`

Expected before production integration: remaining failures around managed classification and rename drift.

- [ ] **Step 2: Replace `samePath` implementation in actions**

Update `internal/actions/scan.go`:

```go
import "github.com/InkyQuill/x-skills/internal/pathidentity"

func samePath(a, b string) bool {
	return pathidentity.Equivalent(a, b)
}
```

- [ ] **Step 3: Surface path identity errors in migrate**

Update `internal/actions/migrate.go` in `ensureUnmanagedSkillDirectory`:

```go
same, sameErr := pathidentity.EquivalentE(resolved, archived)
if sameErr != nil {
	return fmt.Errorf("compare active skill target %q with archive %q: %w", active, archived, sameErr)
}
if same {
	return fmt.Errorf("active skill already managed: %s", active)
}
```

- [ ] **Step 4: Use path identity in rename discovery and drift checks**

Update `internal/actions/rename.go`:

```go
canonicalOld, err := pathidentity.Canonical(oldPath)
...
same, sameErr := pathidentity.EquivalentE(resolved, canonicalOld)
if sameErr != nil || !same {
	continue
}
```

Update `revalidateRenameUsageTarget`:

```go
same, sameErr := pathidentity.EquivalentE(resolved, oldPath)
if sameErr != nil || !same {
	return fmt.Errorf("visible usage %q target drifted before mutation", usage.path)
}
```

- [ ] **Step 5: Canonicalize symlinkcheck resolved paths**

Update `internal/symlinkcheck/symlinkcheck.go` after `EvalSymlinks`:

```go
canonical, err := pathidentity.Canonical(resolvedPath)
if err != nil {
	return Result{Broken: true, Reason: fmt.Sprintf("canonicalize target: %v", err)}
}
resolvedPath = canonical
```

- [ ] **Step 6: Run actions-related packages**

Run:

```bash
mise exec -- go test ./internal/actions ./internal/cli ./internal/doctor ./internal/symlinkcheck
```

Expected: PASS on the current platform.

- [ ] **Step 7: Commit actions integration**

```bash
git add internal/actions internal/cli internal/doctor internal/symlinkcheck
git commit -m "fix: use semantic path identity in actions"
```

### Task 4: Integrate Path Identity Into Syncer

**Files:**
- Modify: `internal/syncer/plan.go`
- Modify: `internal/syncer/apply.go`
- Modify: `internal/syncer/*_test.go`

- [ ] **Step 1: Update syncer tests to assert semantic identity**

Replace raw `filepath.EvalSymlinks(...) == want` helpers such as `assertApplyLink` in `internal/syncer/apply_test.go` with `pathidentity.EquivalentE`.

Run: `mise exec -- go test ./internal/syncer`

Expected before integration: failures around drift checks, `ManagedTarget`, and canonical plan paths on macOS/Windows.

- [ ] **Step 2: Replace syncer canonical helpers**

In `internal/syncer/plan.go`, import:

```go
"github.com/InkyQuill/x-skills/internal/pathidentity"
```

Update:

```go
func canonicalEntryPath(path string) (string, error) {
	return pathidentity.CanonicalEntry(path)
}

func sameCanonicalPath(a, b string) bool {
	return pathidentity.Equivalent(a, b)
}
```

Update `canonicalPath` call sites, or replace the local `canonicalPath` helper body with:

```go
func canonicalPath(path string) (string, error) {
	return pathidentity.Canonical(path)
}
```

- [ ] **Step 3: Use `EquivalentE` for replacement identity**

Update `classificationMatchesConflict` in `internal/syncer/apply.go`:

```go
managedTargetMatches := classification.managedTarget == conflict.ManagedTarget
if classification.managedTarget != "" || conflict.ManagedTarget != "" {
	managedTargetMatches = pathidentity.Equivalent(classification.managedTarget, conflict.ManagedTarget)
}
return classification.status == conflict.DestinationStatus &&
	classification.fingerprint == conflict.DestinationFingerprint &&
	managedTargetMatches
```

Update direct `sameCanonicalPath` drift checks to call `pathidentity.Equivalent`.

- [ ] **Step 4: Run syncer tests**

Run: `mise exec -- go test ./internal/syncer`

Expected: PASS on the current platform.

- [ ] **Step 5: Commit syncer integration**

```bash
git add internal/syncer
git commit -m "fix: use semantic path identity in syncer"
```

### Task 5: Update TUI Tests and Rendered Path Assertions

**Files:**
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/install_test.go`
- Modify: `internal/tui/modal_test.go`

- [ ] **Step 1: Replace raw symlink target assertions**

In TUI tests that currently compare `filepath.EvalSymlinks(active)` to a raw temp path, use a local helper backed by `pathidentity.EquivalentE`.

- [ ] **Step 2: Stabilize modal path assertions**

For modal tests that assert full rendered temp paths, assert a stable path suffix or canonicalized expected path. Example:

```go
want := filepath.Join(".x-skills", "skills")
if !strings.Contains(view, want) {
	t.Fatalf("repo detail modal missing %q:\n%s", want, view)
}
```

Keep assertions that verify the modal still shows the relevant section labels, issue kind, and path context.

- [ ] **Step 3: Run TUI tests**

Run: `mise exec -- go test ./internal/tui`

Expected: PASS on the current platform.

- [ ] **Step 4: Commit TUI test updates**

```bash
git add internal/tui
git commit -m "test: make tui path assertions portable"
```

### Task 6: Cross-Platform Verification and CI Follow-Up

**Files:**
- Modify only if needed after verification: `.github/workflows/ci.yml`

- [ ] **Step 1: Run full local verification**

Run:

```bash
mise exec -- go test ./...
mise exec -- go build ./cmd/x-skills
```

Expected: PASS locally. If platform-specific failures remain, fix them with a failing test first in the relevant package.

- [ ] **Step 2: Push branch and inspect GitHub Actions**

Run:

```bash
git push origin main
gh run list --limit 5 --json databaseId,workflowName,headSha,status,conclusion,url
```

Expected: a new `CI` run starts for the pushed commit.

- [ ] **Step 3: Inspect failed jobs if any**

If CI fails, run:

```bash
gh run view <run-id> --json jobs
gh run view <run-id> --log-failed
```

Expected after fixes: `Go ubuntu-latest`, `Go macos-latest`, and `Go windows-latest` all pass.

- [ ] **Step 4: Final commit if verification required workflow changes**

If `.github/workflows/ci.yml` or any verification-only fix changed:

```bash
git add .
git commit -m "ci: verify cross-platform filesystem support"
git push origin main
```

Expected: `origin/main` contains the portability fix and CI is green across Linux, macOS, and Windows.
