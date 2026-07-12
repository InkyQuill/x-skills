//go:build windows

package actions

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// renameNoReplace moves oldPath to newPath when newPath is absent.
func renameNoReplace(oldPath, newPath string) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination exists: %w", os.ErrExist)
	} else if !os.IsNotExist(err) {
		return err
	}

	from, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}

	// Omitting MOVEFILE_REPLACE_EXISTING makes Windows reject an existing
	// destination instead of silently replacing it.
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}
