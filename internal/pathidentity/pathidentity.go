package pathidentity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Canonical returns a stable absolute spelling for an existing path. It resolves
// symlinks and applies platform-specific normalization so equivalent paths
// produce identical strings.
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
	} else {
		return "", err
	}
	return platformCanonical(filepath.Clean(abs))
}

// CanonicalEntry canonicalizes a path whose final component may not exist. It
// canonicalizes the parent directory and rejoins the base name.
func CanonicalEntry(path string) (string, error) {
	clean := filepath.Clean(path)
	if path == "" || filepath.Base(clean) == "." {
		return "", fmt.Errorf("invalid entry path %q", path)
	}
	parent, err := Canonical(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(clean)), nil
}

// Equivalent reports whether two paths refer to the same filesystem location.
// Errors degrade to false. Use EquivalentE when callers need the error.
func Equivalent(a, b string) bool {
	ok, err := EquivalentE(a, b)
	return err == nil && ok
}

// EquivalentE reports whether two paths refer to the same filesystem location.
// It uses os.SameFile when both paths exist; otherwise it falls back to
// CanonicalEntry and compares the resulting canonical strings.
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
