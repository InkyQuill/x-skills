//go:build darwin

package actions

import (
	"golang.org/x/sys/unix"
)

// renameNoReplace atomically moves oldPath to newPath when newPath is absent.
func renameNoReplace(oldPath, newPath string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_EXCL)
}
