//go:build !windows

package pathidentity

import "testing"

// windowsShortPath skips short-name lookup on non-Windows platforms.
func windowsShortPath(t *testing.T, path string) string {
	t.Helper()
	t.Skipf("Windows short paths unavailable for %q", path)
	return ""
}
