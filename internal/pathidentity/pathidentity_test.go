package pathidentity

import (
	"errors"
	"os"
	"os/exec"
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

func TestCanonicalRejectsMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	got, err := Canonical(missing)
	if err == nil {
		t.Fatalf("Canonical(%q) = %q, nil; want error", missing, got)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Canonical(%q) error = %v, want ErrNotExist", missing, err)
	}
}

func TestCanonicalEntryRejectsMissingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-parent", "missing-entry")
	got, err := CanonicalEntry(path)
	if err == nil {
		t.Fatalf("CanonicalEntry(%q) = %q, nil; want error", path, got)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CanonicalEntry(%q) error = %v, want ErrNotExist", path, err)
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

func TestCanonicalReturnsInvalidPathErrors(t *testing.T) {
	got, err := Canonical(string([]byte{0}))
	if err == nil {
		t.Fatalf("Canonical NUL path = %q, nil; want error", got)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Canonical NUL path error = %v, want non-ErrNotExist error", err)
	}
}

func TestCanonicalEntryReturnsInvalidParentErrors(t *testing.T) {
	path := filepath.Join(string([]byte{0}), "entry")
	got, err := CanonicalEntry(path)
	if err == nil {
		t.Fatalf("CanonicalEntry invalid parent = %q, nil; want error", got)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CanonicalEntry invalid parent error = %v, want non-ErrNotExist error", err)
	}
}

func TestEquivalentEReturnsInvalidPathErrors(t *testing.T) {
	got, err := EquivalentE(string([]byte{0}), t.TempDir())
	if err == nil {
		t.Fatalf("EquivalentE NUL path = %v, nil; want error", got)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("EquivalentE NUL path error = %v, want non-ErrNotExist error", err)
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
	root := t.TempDir()
	longPath := filepath.Join(root, "directory with a long name for short path testing")
	if err := os.Mkdir(longPath, 0o755); err != nil {
		t.Fatal(err)
	}
	shortPath := windowsShortPath(t, longPath)
	if !strings.Contains(shortPath, "~") {
		t.Skipf("short names unavailable for %q: got %q", longPath, shortPath)
	}

	canonicalLong, err := Canonical(longPath)
	if err != nil {
		t.Fatal(err)
	}
	canonicalShort, err := Canonical(shortPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(canonicalShort, canonicalLong) {
		t.Fatalf("Canonical(%q) = %q, want %q", shortPath, canonicalShort, canonicalLong)
	}
	got, err := EquivalentE(shortPath, longPath)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("EquivalentE(%q, %q) = false, want true", shortPath, longPath)
	}
}

// windowsShortPath returns Windows' short-name spelling for path when available.
func windowsShortPath(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("cmd", "/d", "/c", `for %I in ("%X_SKILLS_SHORT_PATH%") do @echo %~sI`)
	cmd.Env = append(os.Environ(), "X_SKILLS_SHORT_PATH="+path)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("could not query Windows short path for %q: %v", path, err)
	}
	short := strings.ReplaceAll(strings.TrimSpace(string(out)), `"`, "")
	if drive := strings.Index(short, `:`); drive > 0 && isWindowsDriveLetter(short[drive-1]) {
		short = short[drive-1:]
	}
	return short
}

// isWindowsDriveLetter reports whether b can start a Windows drive-qualified path.
func isWindowsDriveLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
