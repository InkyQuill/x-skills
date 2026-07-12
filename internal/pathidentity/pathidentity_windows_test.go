//go:build windows

package pathidentity

import (
	"testing"

	"golang.org/x/sys/windows"
)

// windowsShortPath returns Windows' short-name spelling for path when available.
func windowsShortPath(t *testing.T, path string) string {
	t.Helper()
	longPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(longPath, &buf[0], uint32(len(buf)))
	if n == 0 {
		t.Skipf("could not query Windows short path for %q: %v", path, err)
	}
	if n > uint32(len(buf)) {
		buf = make([]uint16, n)
		n, err = windows.GetShortPathName(longPath, &buf[0], uint32(len(buf)))
		if n == 0 {
			t.Skipf("could not query Windows short path for %q: %v", path, err)
		}
	}
	return windows.UTF16ToString(buf[:n])
}
