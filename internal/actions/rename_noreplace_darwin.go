//go:build darwin

package actions

import (
	"os"
)

// renameNoReplace moves oldPath to newPath when newPath is absent.
func renameNoReplace(oldPath, newPath string) error {
	placeholder, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := placeholder.Close(); err != nil {
		_ = os.Remove(newPath)
		return err
	}
	if _, err := os.Lstat(oldPath); err != nil {
		_ = os.Remove(newPath)
		return err
	}
	if err := os.Remove(newPath); err != nil {
		return err
	}
	// Darwin's fallback reserves the destination first, then removes the
	// placeholder before rename. That preserves same-process no-replace behavior,
	// but it leaves a residual cross-process TOCTOU gap before the final rename.
	return os.Rename(oldPath, newPath)
}
